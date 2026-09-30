import Foundation

public enum ClientConfiguration {
    public static let defaultServerURL = "https://tink.mrasong.com"
    public static let githubURL = URL(string: "https://github.com/mrasong/tink")!
    public static let sseRequestTimeout: TimeInterval = 60
    public static let sseResourceTimeout: TimeInterval = 24 * 60 * 60
    public static let reconnectBaseDelay: TimeInterval = 1
    public static let reconnectMaxDelay: TimeInterval = 300
    public static let reconnectJitter: ClosedRange<Double> = 0.1...0.5

    // 默认应用标识与数据库配置
    public static let defaultBundleIdentifier = "com.mrasong.tink"
    public static let storeFileName = "tink.store"

    // AppStorage / UserDefaults 键名
    public enum StorageKeys {
        public static let lastEventID = "tink_last_event_id"
        public static let serverURL = "tink_server_url"
        public static let deviceID = "tink_device_id"
        public static let deviceName = "tink_device_name"
        public static let menuBarSymbolName = "menuBarSymbolName"
        public static let menuBarDefaultIcon = "default"
    }

    // Keychain 键名
    public enum KeychainKeys {
        public static let apiToken = "api_token"
        public static let deviceID = "tink_device_id"
    }
}
