import Foundation

// 版本号读取的唯一来源：菜单栏 header、Settings 头部、About 窗口共用。
public enum AppVersion {
    public static var short: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String
            ?? "Unknown"
    }

    public static var display: String { "v\(short)" }
}
