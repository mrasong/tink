import Foundation

public final class APIClient: @unchecked Sendable {
    @MainActor public static let shared = APIClient()

    private let session: URLSession

    init(session: URLSession = .shared) {
        self.session = session
    }

    enum APIError: LocalizedError {
        case http(Int, String)
        var errorDescription: String? {
            if case .http(let code, let message) = self {
                return String(
                    format: String(localized: "Server error (HTTP %1$lld): %2$@"), code, message)
            }
            return nil
        }
    }

    private func responseData(for request: URLRequest) async throws -> Data {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw APIError.http(0, "Invalid response")
        }
        guard (200...299).contains(http.statusCode) else {
            throw APIError.http(
                http.statusCode, String(data: data, encoding: .utf8) ?? "Request failed")
        }
        return data
    }

    private func makeJSONDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        return decoder
    }

    private func decoded<T: Decodable>(_ type: T.Type, request: URLRequest) async throws -> T {
        try makeJSONDecoder().decode(T.self, from: await responseData(for: request))
    }

    private func envelope<T: Codable>(_ type: T.Type, request: URLRequest) async throws -> T {
        let response = try makeJSONDecoder().decode(
            APIResponse<T>.self, from: await responseData(for: request))
        guard response.code == 0, let value = response.data else {
            throw APIError.http(response.code, response.message)
        }
        return value
    }

    public struct APIResponse<T: Codable>: Codable {
        public let code: Int
        public let message: String
        public let data: T?
    }

    public struct PingResponse: Codable, Equatable {
        public let message: String
        public let version: String
        public let build: String
        public let serverTime: Int64

        enum CodingKeys: String, CodingKey {
            case message, version, build
            case serverTime = "st"
        }
    }

    private func makeRequest(endpoint: String, method: String = "GET", body: Data? = nil) throws
        -> URLRequest
    {
        let url = getAPIURL(endpoint)
        let storage = LocalStorage.shared
        var req = URLRequest(url: url)
        req.timeoutInterval = 15
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let token = storage.apiToken
        if !token.isEmpty {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        req.setValue(storage.deviceID, forHTTPHeaderField: "X-Device-ID")
        req.httpBody = body
        return req
    }

    public func registerCurrentDevice() async throws -> Device {
        let storage = LocalStorage.shared
        let payload: [String: String] = [
            "id": storage.deviceID,
            "name": storage.deviceName,
        ]
        let data = try JSONEncoder().encode(payload)
        let req = try makeRequest(endpoint: APIEndpoints.devices, method: "POST", body: data)

        return try await envelope(Device.self, request: req)
    }

    public func sendTestNotification() async throws {
        let storage = LocalStorage.shared
        let payload: [String: Any] = [
            "title": String(localized: "Tink Test Notification"),
            "body": String(localized: "Hello from Tink! Your connection is healthy."),
            "devices": [storage.deviceID],
        ]
        let data = try JSONSerialization.data(withJSONObject: payload)
        let req = try makeRequest(endpoint: APIEndpoints.messages, method: "POST", body: data)

        _ = try await envelope([String: AnyCodable].self, request: req)
    }

    public func ping(url: String) async throws -> (healthy: Bool, latencyMs: Int) {
        let targetURL = getAPIURL(APIEndpoints.ping, baseURL: url)
        var req = URLRequest(url: targetURL)
        req.httpMethod = "GET"
        req.timeoutInterval = 3.0

        let start = Date()
        let (data, response) = try await session.data(for: req)
        let latency = Int(Date().timeIntervalSince(start) * 1000)

        guard let httpResp = response as? HTTPURLResponse, (200...299).contains(httpResp.statusCode)
        else {
            return (false, latency)
        }

        guard let ping = try? JSONDecoder().decode(PingResponse.self, from: data) else {
            throw APIError.http(httpResp.statusCode, "Invalid ping response")
        }
        return (ping.message == "pong" && !ping.version.isEmpty && ping.serverTime > 0, latency)
    }
}
