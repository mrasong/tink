import Combine
import Foundation
import OSLog
import SwiftData

@MainActor
public final class ConnectionManager: ObservableObject, @preconcurrency SSEClientDelegate {
    public static let shared = ConnectionManager()
    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? ClientConfiguration.defaultBundleIdentifier,
        category: "Connection")

    @Published public var status: ConnectionStatus = .disconnected
    @Published public var notificationCount: Int = 0
    @Published public var storageError: String?
    public var unreadCount: Int {
        messageStore.unreadCount()
    }

    public func refreshFromStore() {
        notificationCount = messageStore.count()
    }

    public func isRead(_ id: UInt64) -> Bool {
        messageStore.message(id: id)?.isRead ?? false
    }

    public func markAsRead(_ id: UInt64) {
        messageStore.setRead(id, value: true)
        refreshCounts()
    }

    public func markAsUnread(_ id: UInt64) { setRead(id, value: false) }
    public func markAllAsRead() {
        messageStore.markAllAsRead()
        refreshCounts()
    }
    public func toggleStar(_ id: UInt64) {
        guard let message = messageStore.message(id: id) else { return }
        messageStore.setStarred(id, value: !message.isStarred)
        refreshCounts()
    }
    public func setStarred(_ id: UInt64, value: Bool) {
        messageStore.setStarred(id, value: value)
        refreshCounts()
    }
    private func setRead(_ id: UInt64, value: Bool) {
        messageStore.setRead(id, value: value)
        refreshCounts()
    }

    private let sseClient = SSEClient()
    private var registrationTask: Task<Void, Never>?
    public let modelContainer: ModelContainer
    public private(set) var isUsingInMemoryStore = false
    public let messageStore: MessageStore

    private init() {
        do {
            let appSupportURL = FileManager.default.urls(
                for: .applicationSupportDirectory, in: .userDomainMask
            ).first!
            let subfolder =
                Bundle.main.bundleIdentifier ?? ClientConfiguration.defaultBundleIdentifier
            let directoryURL = appSupportURL.appendingPathComponent(subfolder, isDirectory: true)
            try FileManager.default.createDirectory(
                at: directoryURL, withIntermediateDirectories: true)
            let storeURL = directoryURL.appendingPathComponent(ClientConfiguration.storeFileName)
            let configuration = ModelConfiguration(url: storeURL)
            modelContainer = try ModelContainer(
                for: StoredMessage.self, configurations: configuration)
        } catch {
            print("[Storage] Failed to initialize SwiftData: \(error)")
            isUsingInMemoryStore = true
            modelContainer = try! ModelContainer(
                for: StoredMessage.self,
                configurations: ModelConfiguration(isStoredInMemoryOnly: true))
        }
        messageStore = MessageStore(container: modelContainer)
        sseClient.delegate = self
        notificationCount = messageStore.count()
        storageError = messageStore.lastError?.localizedDescription
    }

    public func flushStorage() {
        messageStore.savePending()
        storageError = messageStore.lastError?.localizedDescription
    }

    public func retryStorage() {
        messageStore.replace(with: messageStore.fetchAll())
        storageError = messageStore.lastError?.localizedDescription
    }

    public var persistenceWarning: String? {
        isUsingInMemoryStore
            ? String(
                localized:
                    "Message history is stored temporarily because the local database could not be opened."
            )
            : nil
    }

    public func start() {
        guard status != .connecting && status != .connected else { return }
        status = .connecting

        registrationTask?.cancel()
        registrationTask = Task { @MainActor [weak self] in
            guard let self else { return }
            var attempt = 0
            let delays: [UInt64] = [1, 2, 4, 8, 16, 30, 60, 300]

            while !Task.isCancelled {
                do {
                    _ = try await APIClient.shared.registerCurrentDevice()
                    guard !Task.isCancelled else { return }
                    self.sseClient.connect()
                    return
                } catch {
                    guard !Task.isCancelled else { return }
                    let delay = delays[min(attempt, delays.count - 1)]
                    attempt += 1
                    logger.error(
                        "Device registration failed: \(error.localizedDescription, privacy: .public). Retrying in \(delay, privacy: .public)s..."
                    )
                    try? await Task.sleep(for: .seconds(delay))
                }
            }
        }
    }

    public func reconnect() {
        sseClient.disconnect()
        status = .disconnected
        start()
    }

    public func stop() {
        registrationTask?.cancel()
        registrationTask = nil
        sseClient.disconnect()
        status = .disconnected
    }

    // 删除单条通知
    public func deleteMessage(id: UInt64) {
        messageStore.delete(ids: [id])
        refreshCounts()
    }

    // 批量删除选中的多条通知
    public func deleteMessages(ids: Set<UInt64>) {
        messageStore.delete(ids: ids)
        refreshCounts()
    }

    // 清空所有通知
    public func clearAllMessages() {
        messageStore.deleteAll()
        refreshCounts()
    }

    // MARK: - SSEClientDelegate

    public func sseClientDidConnect(_ client: SSEClient) {
        status = .connected
    }

    public func sseClientDidDisconnect(_ client: SSEClient, error: Error?) {
        // SSEClient 已经安排自动重连，状态保持为连接中，避免界面看起来像停止重试。
        status = .connecting
    }

    public func sseClient(_ client: SSEClient, didReceiveMessage message: TinkMessage) {
        // 弹出系统通知
        NotificationManager.shared.postNotification(message: message)

        let existing = messageStore.message(id: message.id)
        var incoming = message
        incoming.isRead = existing?.isRead ?? false
        incoming.isStarred = existing?.isStarred ?? false
        var messages = messageStore.fetchAll().filter { $0.id != message.id }
        messages.insert(incoming, at: 0)
        messageStore.replace(with: Self.trimmed(messages))
        refreshCounts()
    }

    static func trimmed(_ messages: [TinkMessage], limit: Int = 5000) -> [TinkMessage] {
        guard messages.count > limit else { return messages }
        var result = messages
        while result.count > limit {
            if let index = result.lastIndex(where: { !$0.isStarred }) {
                result.remove(at: index)
            } else {
                result.removeLast()
            }
        }
        return result
    }

    private func refreshCounts() {
        notificationCount = messageStore.count()
        storageError = messageStore.lastError?.localizedDescription
    }
}
