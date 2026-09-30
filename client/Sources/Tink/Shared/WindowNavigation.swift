import AppKit
import SwiftUI

// 从 AppKit 上下文（状态栏菜单等）唤起 SwiftUI 场景窗口的统一入口。
@MainActor
public enum WindowNavigation {
    public static func showNotificationManager(group: NotificationGroup?) {
        if let group {
            NotificationWindowCoordinator.shared.selectedGroup = group
        }
        open(windowID: NotificationWindowCoordinator.windowID)
    }

    public static func showAbout() {
        open(windowID: AboutWindow.id)
    }

    public static func showSettings() {
        open(windowID: SettingsWindow.id)
    }

    // 菜单栏应用（accessory）下窗口无法成为 key window，先提权再开窗
    private static func open(windowID: String) {
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
        AppWindowOpener.shared.open(id: windowID)
    }
}
