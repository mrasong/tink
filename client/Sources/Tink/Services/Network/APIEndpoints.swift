import Foundation

/// 服务端 API 路径集中配置，便于后续调整接口版本或路由。
public enum APIEndpoints {
    public static let ping = "/api/v1/ping"
    public static let events = "/api/v1/events"
    public static let devices = "/api/v1/devices"
    public static let messages = "/api/v1/messages"
}


/// 构建完整的 API URL（保证返回有效 URL，异常时自动回退默认地址）
/// - Parameters:
///   - endpoint: 接口路径（如 APIEndpoints.devices 或 "/api/v1/devices"）
///   - baseURL: 服务端基础地址，默认使用 LocalStorage.shared.serverURL
/// - Returns: 构造好的有效 URL
public func getAPIURL(_ endpoint: String, baseURL: String = LocalStorage.shared.serverURL) -> URL {
    // 1. 去除首尾空白及斜杠
    let trimmed = baseURL.trimmingCharacters(in: .whitespacesAndNewlines).trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    
    // 2. 如果修剪后为空，优雅回退到系统的默认地址
    let finalBase = trimmed.isEmpty ? ClientConfiguration.defaultServerURL.trimmingCharacters(in: CharacterSet(charactersIn: "/")) : trimmed
    
    // 3. 规范化 endpoint 斜杠
    let cleanEndpoint = endpoint.hasPrefix("/") ? endpoint : "/\(endpoint)"
    
    // 4. 优先解析目标 URL，若格式异常则以默认地址兜底，保证返回非可选的 URL
    if let url = URL(string: "\(finalBase)\(cleanEndpoint)") {
        return url
    }
    let defaultBase = ClientConfiguration.defaultServerURL.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    return URL(string: "\(defaultBase)\(cleanEndpoint)") ?? URL(string: ClientConfiguration.defaultServerURL)!
}