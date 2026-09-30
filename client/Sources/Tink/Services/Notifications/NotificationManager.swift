import AppKit
import Foundation
@preconcurrency import UserNotifications

@MainActor
public final class NotificationManager: NSObject, @preconcurrency UNUserNotificationCenterDelegate {
    public static let shared = NotificationManager()

    private var center: UNUserNotificationCenter? {
        guard Bundle.main.bundleIdentifier != nil else {
            return nil
        }
        return UNUserNotificationCenter.current()
    }

    private override init() {
        super.init()
        if Bundle.main.bundleIdentifier != nil {
            UNUserNotificationCenter.current().delegate = self
        }
    }

    public func requestAuthorization() {
        guard let center = self.center else {
            print(
                "[Notification] Running outside an app bundle, skipping UNUserNotificationCenter authorization."
            )
            return
        }

        center.requestAuthorization(options: [.alert, .sound, .badge]) { granted, error in
            print(
                "[Notification] requestAuthorization result: granted=\(granted), error=\(String(describing: error))"
            )
        }
    }

    public func postNotification(message: TinkMessage) {
        guard let center = self.center else {
            deliverDirectNotification(message: message)
            return
        }

        center.getNotificationSettings { [weak self] settings in
            guard let owner = self else { return }
            print(
                "[Notification] Auth Status: \(settings.authorizationStatus.rawValue), AlertStyle: \(settings.alertStyle.rawValue)"
            )

            // 无论是已授权还是正在授权中，均先提交 UNNotificationRequest
            let content = UNMutableNotificationContent()
            content.title = message.title.isEmpty ? "Tink" : message.title
            // Banner 层仅展示纯文本，避免被 Markdown 排版符号（#, **, ``` 等）挤占有限空间
            content.body = MarkdownHelper.toPlainText(message.body)
            content.sound = .default

            var userInfo: [AnyHashable: Any] = [
                "id": String(message.id),
                "title": message.title,
                "body": message.body,
            ]
            if let u = message.url, !u.isEmpty {
                userInfo["url"] = u
            }
            if let g = message.group, !g.isEmpty {
                userInfo["group"] = g
            }
            content.userInfo = userInfo

            let request = UNNotificationRequest(
                identifier: "tink-\(message.id)-\(UUID().uuidString)",
                content: content,
                trigger: nil
            )

            center.add(request) { [weak owner] error in
                if let error = error {
                    print("[Notification] UNUserNotificationCenter add failed: \(error)")
                    Task { @MainActor [weak owner] in
                        owner?.deliverDirectNotification(message: message)
                    }
                } else {
                    print(
                        "[Notification] UNUserNotificationCenter added notification successfully.")
                    // 如果系统权限被拒或者未开启横幅显示，自动补充由本应用签名的通知
                    if settings.authorizationStatus == .denied || settings.alertStyle == .none {
                        Task { @MainActor [weak owner] in
                            owner?.deliverDirectNotification(message: message)
                        }
                    }
                }
            }
        }
    }

    // 备用系统横幅通知：绑定为当前应用 Bundle，不唤起 Script Editor
    private func deliverDirectNotification(message: TinkMessage) {
        // DispatchQueue.global(qos: .userInitiated).async {
        //     let title = message.title.replacingOccurrences(of: "\"", with: "\\\"")
        //     // 剥离 Markdown 格式为纯文本
        //     let plainBody = MarkdownHelper.toPlainText(message.body)
        //     let body = plainBody.replacingOccurrences(of: "\"", with: "\\\"")
        //     // 关键：tell application id "com.mrasong.tink" 强制归属于 Tink，绝不打开脚本编辑器
        //     let script = """
        //     tell application id "com.mrasong.tink"
        //         display notification "\(body)" with title "\(title)" sound name "default"
        //     end tell
        //     """

        //     let process = Process()
        //     process.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        //     process.arguments = ["-e", script]
        //     let pipe = Pipe()
        //     process.standardError = pipe
        //     try? process.run()
        //     process.waitUntilExit()

        //     // 如果指定 Bundle 失败，直接以当前进程名执行
        //     if process.terminationStatus != 0 {
        //         let fallback = "display notification \"\(body)\" with title \"\(title)\" sound name \"default\""
        //         let p2 = Process()
        //         p2.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        //         p2.arguments = ["-e", fallback]
        //         try? p2.run()
        //     }
        // }
    }

    // MARK: - UNUserNotificationCenterDelegate

    // 当应用在前台或当前激活时，强制弹出横幅通知与声音
    public func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler:
            @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .sound, .badge, .list])
    }

    // 用户点击通知横幅后触发：直接居中弹出支持 Markdown 的详情窗口
    public func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        defer { completionHandler() }

        let info = response.notification.request.content.userInfo
        let title = info["title"] as? String ?? response.notification.request.content.title
        let body = info["body"] as? String ?? response.notification.request.content.body
        let url = info["url"] as? String
        let group = info["group"] as? String
        let idStr = info["id"] as? String ?? "0"
        let msgID = UInt64(idStr) ?? 0

        let msg = TinkMessage(
            id: msgID,
            group: group,
            title: title,
            body: body,
            url: url,
            createdAt: Int64(response.notification.date.timeIntervalSince1970 * 1000)
        )

        // 立即展示 Markdown 详情窗口
        Task { @MainActor in
            MessageDetailWindowCoordinator.shared.requestShow(message: msg)
        }
    }
}
