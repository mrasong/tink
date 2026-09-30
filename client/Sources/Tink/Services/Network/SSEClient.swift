import Foundation
import OSLog

public protocol SSEClientDelegate: AnyObject {
    func sseClientDidConnect(_ client: SSEClient)
    func sseClientDidDisconnect(_ client: SSEClient, error: Error?)
    func sseClient(_ client: SSEClient, didReceiveMessage message: TinkMessage)
}

public final class SSEClient: NSObject, @unchecked Sendable, URLSessionDataDelegate {
    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? ClientConfiguration.defaultBundleIdentifier,
        category: "SSE")
    @MainActor public weak var delegate: SSEClientDelegate?

    private var session: URLSession?
    private var dataTask: URLSessionDataTask?
    private var buffer = Data()
    private var isManuallyStopped = false
    var isStoppedForTesting: Bool { isManuallyStopped }

    // 指数退避重连配置
    private var retryAttempt = 0
    private var reconnectWorkItem: DispatchWorkItem?

    public override init() {
        super.init()
    }

    public func connect() {
        dataTask?.cancel()
        session?.invalidateAndCancel()
        isManuallyStopped = false
        reconnectWorkItem?.cancel()
        reconnectWorkItem = nil
        buffer = Data()

        let storage = LocalStorage.shared
        guard
            var components = URLComponents(
                url: getAPIURL(APIEndpoints.events), resolvingAgainstBaseURL: false)
        else {
            Task { @MainActor [weak self] in
                self?.delegate?.sseClientDidDisconnect(self!, error: URLError(.badURL))
            }
            return
        }

        // 拼接查询参数兼容
        components.queryItems = [
            URLQueryItem(name: "device_id", value: storage.deviceID),
            URLQueryItem(name: "last_event_id", value: String(storage.lastEventID)),
        ]

        guard let url = components.url else { return }

        var req = URLRequest(url: url)
        req.timeoutInterval = ClientConfiguration.sseResourceTimeout
        req.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        req.setValue("no-cache", forHTTPHeaderField: "Cache-Control")
        req.setValue("keep-alive", forHTTPHeaderField: "Connection")
        req.setValue(storage.deviceID, forHTTPHeaderField: "X-Device-ID")
        req.setValue(String(storage.lastEventID), forHTTPHeaderField: "Last-Event-ID")

        let token = storage.apiToken
        if !token.isEmpty {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let config = URLSessionConfiguration.default
        config.waitsForConnectivity = false
        config.timeoutIntervalForRequest = ClientConfiguration.sseRequestTimeout
        config.timeoutIntervalForResource = ClientConfiguration.sseResourceTimeout

        let delegateQueue = OperationQueue()
        delegateQueue.maxConcurrentOperationCount = 1
        session = URLSession(configuration: config, delegate: self, delegateQueue: delegateQueue)
        dataTask = session?.dataTask(with: req)
        dataTask?.resume()
    }

    public func disconnect() {
        isManuallyStopped = true
        reconnectWorkItem?.cancel()
        reconnectWorkItem = nil
        dataTask?.cancel()
        session?.invalidateAndCancel()
        session = nil
        dataTask = nil
    }

    private func scheduleReconnect() {
        DispatchQueue.main.async { [weak self] in
            guard let self, !self.isManuallyStopped else { return }
            self.reconnectWorkItem?.cancel()

            // 指数退避: 1s, 2s, 4s, 8s, 16s, 30s, 60s, 300s，之后固定 300s。
            let delays: [TimeInterval] = [1, 2, 4, 8, 16, 30, 60, 300]
            let delay = delays[min(self.retryAttempt, delays.count - 1)]
            let jitter = Double.random(in: ClientConfiguration.reconnectJitter)
            let totalDelay = delay + jitter

            self.retryAttempt += 1
            self.logger.info(
                "Reconnecting in \(String(format: "%.1f", totalDelay), privacy: .public)s (attempt \(self.retryAttempt), privacy: .public)..."
            )

            let workItem = DispatchWorkItem { [weak self] in
                guard let self, !self.isManuallyStopped else { return }
                self.reconnectWorkItem = nil
                self.connect()
            }
            self.reconnectWorkItem = workItem
            DispatchQueue.main.asyncAfter(deadline: .now() + totalDelay, execute: workItem)
        }
    }

    // MARK: - URLSessionDataDelegate

    public func urlSession(
        _ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse,
        completionHandler: @escaping (URLSession.ResponseDisposition) -> Void
    ) {
        guard let httpResponse = response as? HTTPURLResponse else {
            completionHandler(.cancel)
            return
        }

        if httpResponse.statusCode == 200 {
            logger.info("SSE connected (HTTP 200)")
            retryAttempt = 0
            Task { @MainActor [weak self] in
                guard let self else { return }
                self.delegate?.sseClientDidConnect(self)
            }
            // 必须使用 .allow 保持作为数据流持续接收，使用 .becomeDownload 会将任务转为下载任务导致当前数据通道关闭并报错重连
            completionHandler(.allow)
        } else {
            logger.error("SSE rejected with HTTP \(httpResponse.statusCode, privacy: .public)")
            completionHandler(.cancel)
        }
    }

    public func urlSession(
        _ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data
    ) {
        buffer.append(data)

        // 解析 SSE 标准分块 (\n\n 分隔)。按字节边界切分,避免多字节字符跨 chunk 时被截断丢弃。
        let delimiter = Data([0x0A, 0x0A])
        while let delimiterRange = buffer.range(of: delimiter) {
            let blockData = buffer.subdata(in: buffer.startIndex..<delimiterRange.lowerBound)
            buffer.removeSubrange(buffer.startIndex..<delimiterRange.upperBound)
            let messageBlock = String(decoding: blockData, as: UTF8.self)
                .replacingOccurrences(of: "\r\n", with: "\n")
            parseSSEMessage(block: messageBlock)
        }
    }

    public func urlSession(
        _ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?
    ) {
        DispatchQueue.main.async { [weak self] in
            guard let self, !self.isManuallyStopped, self.dataTask === task else { return }
            // 只有当前连接断开时才更新状态并安排重连，忽略被替换连接的迟到回调。
            if let error {
                self.logger.error(
                    "SSE disconnected: \(error.localizedDescription, privacy: .public)")
            } else {
                self.logger.info("SSE disconnected without error")
            }
            self.delegate?.sseClientDidDisconnect(self, error: error)
            self.scheduleReconnect()
        }
    }

    // MARK: - Message Parsing

    private func parseSSEMessage(block: String) {
        guard let message = Self.message(from: block) else { return }
        let eventID =
            block.components(separatedBy: .newlines).first { $0.hasPrefix("id:") }.flatMap {
                UInt64($0.dropFirst(3).trimmingCharacters(in: .whitespaces))
            } ?? message.id
        LocalStorage.shared.lastEventID = eventID
        Task { @MainActor [weak self] in
            guard let self else { return }
            self.delegate?.sseClient(self, didReceiveMessage: message)
        }
    }

    static func message(from block: String) -> TinkMessage? {
        var currentEvent = "message"
        var dataLines: [String] = []

        let lines = block.components(separatedBy: .newlines)
        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty || trimmed.hasPrefix(":") {
                // 心跳或注释行，保持存活
                continue
            }

            if line.hasPrefix("id:") {
                // id 在 parseSSEMessage 中统一处理，此处跳过。
                continue
            } else if line.hasPrefix("event:") {
                currentEvent = String(line.dropFirst(6).trimmingCharacters(in: .whitespaces))
            } else if line.hasPrefix("data:") {
                dataLines.append(String(line.dropFirst(5).trimmingCharacters(in: .whitespaces)))
            }
        }

        if currentEvent == "notification" && !dataLines.isEmpty {
            let jsonString = dataLines.joined(separator: "\n")
            guard let jsonData = jsonString.data(using: .utf8) else { return nil }

            do {
                return try JSONDecoder().decode(TinkMessage.self, from: jsonData)
            } catch {
                print("[SSE] Failed to decode notification: \(error)")
            }
        }
        return nil
    }
}
