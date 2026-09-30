import Foundation

public final class LocalStorage: @unchecked Sendable {
    public static let shared = LocalStorage()

    private let defaults = UserDefaults.standard
    private let keyLastEventID = ClientConfiguration.StorageKeys.lastEventID
    private let keyServerURL = ClientConfiguration.StorageKeys.serverURL
    private let keyDeviceID = ClientConfiguration.StorageKeys.deviceID
    private let keyDeviceName = ClientConfiguration.StorageKeys.deviceName

    private init() {}

    // Last-Event-ID 管理 (设备断线重连核心游标)
    public var lastEventID: UInt64 {
        get {
            UInt64(defaults.string(forKey: keyLastEventID) ?? "0") ?? 0
        }
        set {
            defaults.set(String(newValue), forKey: keyLastEventID)
        }
    }

    public var serverURL: String {
        get {
            defaults.string(forKey: keyServerURL) ?? ClientConfiguration.defaultServerURL
        }
        set {
            defaults.set(newValue, forKey: keyServerURL)
        }
    }

    public var deviceName: String {
        get {
            if let name = defaults.string(forKey: keyDeviceName), !name.isEmpty {
                return name
            }
            return Host.current().localizedName ?? "Mac"
        }
        set {
            defaults.set(newValue, forKey: keyDeviceName)
        }
    }

    // Token 安全保存在 Keychain 中
    public var apiToken: String {
        get {
            KeychainHelper.loadString(key: ClientConfiguration.KeychainKeys.apiToken) ?? ""
        }
        set {
            if newValue.isEmpty {
                _ = KeychainHelper.delete(key: ClientConfiguration.KeychainKeys.apiToken)
            } else {
                _ = KeychainHelper.saveString(
                    key: ClientConfiguration.KeychainKeys.apiToken, value: newValue)
            }
        }
    }

    // Device ID 使用随机 UUID (RFC 4122 v4) 生成并保存在 Keychain，保持永久唯一
    public var deviceID: String {
        if let existing = KeychainHelper.loadString(key: ClientConfiguration.KeychainKeys.deviceID),
            !existing.isEmpty
        {
            return existing
        }
        // 生成新设备 ID
        let newID = UUID().uuidString.lowercased()
        _ = KeychainHelper.saveString(key: ClientConfiguration.KeychainKeys.deviceID, value: newID)
        return newID
    }

    // 完全重置所有本地数据与凭证
    public func resetAllData() {
        // 1. 清除 Keychain 凭证与设备信息
        _ = KeychainHelper.delete(key: ClientConfiguration.KeychainKeys.apiToken)
        _ = KeychainHelper.delete(key: ClientConfiguration.KeychainKeys.deviceID)

        // 2. 清除 UserDefaults
        let domain = Bundle.main.bundleIdentifier ?? ClientConfiguration.defaultBundleIdentifier
        defaults.removePersistentDomain(forName: domain)
        defaults.synchronize()

        // 3. 删除本地 SwiftData 数据库目录
        if let appSupportURL = FileManager.default.urls(
            for: .applicationSupportDirectory, in: .userDomainMask
        ).first {
            let directoryURL = appSupportURL.appendingPathComponent(domain, isDirectory: true)
            try? FileManager.default.removeItem(at: directoryURL)
        }
    }
}
