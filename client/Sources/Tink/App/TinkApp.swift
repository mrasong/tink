import AppKit
import SwiftData
import SwiftUI
import UserNotifications

@main
struct TinkApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        // 通知管理窗口使用 SwiftUI Window 场景，获得与系统设置一致的
        // 标题栏工具栏布局（侧边栏切换按钮自动落在红绿灯右侧）。
        // 所有 Window 场景都需 .suppressed：SwiftUI 会自动呈现启动时第一个未被抑制的场景。
        Window(
            "Notification Manager", id: NotificationWindowCoordinator.windowID
        ) {
            NotificationManagerView()
                .modelContainer(ConnectionManager.shared.modelContainer)
        }
        .defaultSize(width: 960, height: 800)
        .windowResizability(.contentMinSize)
        .windowToolbarStyle(.unified)
        .defaultLaunchBehavior(.suppressed)
        .commands {
            CommandGroup(replacing: .appInfo) {
                AboutMenuButton()
            }
            // Settings 场景在未被启动呈现时无法经 showSettingsWindow: 开窗，
            // 故设置窗口也是普通 Window 场景，这里自行占据标准菜单槽位与 ⌘, 快捷键
            CommandGroup(replacing: .appSettings) {
                SettingsMenuButton()
            }
        }

        Window("Settings", id: SettingsWindow.id) {
            SettingsView()
        }
        // 设置窗口形态：固定宽度、高度随表单内容自适应。
        .windowResizability(.contentSize)
        .defaultLaunchBehavior(.suppressed)

        // “关于 Tink”使用原生 Window 场景，hiddenTitleBar 与系统关于面板观感一致
        Window("About Tink", id: AboutWindow.id) {
            AboutView()
        }
        .windowStyle(.hiddenTitleBar)
        .windowResizability(.contentSize)
        .defaultLaunchBehavior(.suppressed)

        // 消息详情：按消息 ID 为值的 WindowGroup，不同消息各开一窗，同一消息复用其窗口
        WindowGroup(
            "Message Detail", id: MessageDetailWindowCoordinator.windowID, for: UInt64.self
        ) { $messageID in
            // nil 值只出现在冷启动（菜单栏形态）经 File ▸ New 菜单项打开的那扇窗，
            // 让它渲染点击的这条消息，不再补开第二扇
            MessageDetailView(messageID: messageID ?? MessageDetailWindowCoordinator.shared.bootstrapMessageID ?? 0)
                .modelContainer(ConnectionManager.shared.modelContainer)
        }
        .defaultSize(width: 640, height: 480)
        .windowResizability(.contentMinSize)

        // 图片预览：按图片地址为值的 WindowGroup，可多张图片各开一窗
        WindowGroup("Image Preview", id: ImagePreviewWindow.id, for: String.self) { $source in
            ImagePreviewView(source: source ?? "")
        }
        .defaultSize(width: 900, height: 700)
        .windowResizability(.contentMinSize)
    }
}

// 菜单命令需要 openWindow，environment 只能在视图内注入
struct AboutMenuButton: View {
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Button("About Tink") {
            NSApp.activate(ignoringOtherApps: true)
            openWindow(id: AboutWindow.id)
        }
    }
}

struct SettingsMenuButton: View {
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Button("Settings…") {
            NSApp.activate(ignoringOtherApps: true)
            openWindow(id: SettingsWindow.id)
        }
        .keyboardShortcut(",", modifiers: .command)
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        // 关键：在应用启动时立即强制绑定 UNUserNotificationCenter.current().delegate
        if Bundle.main.bundleIdentifier != nil {
            UNUserNotificationCenter.current().delegate = NotificationManager.shared
        }

        // 请求系统通知权限
        NotificationManager.shared.requestAuthorization()

        // 启动 SSE 连接管理器
        ConnectionManager.shared.start()

        // 状态栏入口：NSStatusItem + 原生 NSMenu
        StatusBarMenuController.shared.install()

        // 窗口全关后退回菜单栏应用形态。开窗方（WindowNavigation）负责提权为 regular，
        // 故只需监听关窗事件降级；原先在 applicationWillUpdate 里轮询会抢在开窗动画中间降级。
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(windowWillClose(_:)),
            name: NSWindow.willCloseNotification,
            object: nil
        )

        #if DEBUG
            if CommandLine.arguments.contains("--debug-pop-status-menu") {
                DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
                    StatusBarMenuController.shared.popUpForDebug()
                }
            }
        #endif
    }

    @MainActor
    func applicationWillTerminate(_ notification: Notification) {
        ConnectionManager.shared.flushStorage()
    }

    // willClose 触发时窗口尚未 orderOut，故把检查推到下一个主队列轮次
    @objc @MainActor func windowWillClose(_ notification: Notification) {
        DispatchQueue.main.async { [weak self] in
            self?.demoteToMenuBarIfIdle()
        }
    }

    // 只剩状态栏入口（无可见/未最小化窗口）时退回 accessory，Dock 图标随之消失。
    // 最小化窗口仍算存在：降级会让它无法从 Dock 恢复。
    @MainActor
    private func demoteToMenuBarIfIdle() {
        let hasWindow = NSApp.windows.contains { window in
            window.isVisible && !window.isMiniaturized
                && !window.className.contains("StatusBar")
                && !window.className.contains("Popover")
                && !window.className.contains("Menu")
        }
        if !hasWindow, NSApp.activationPolicy() != .accessory {
            NSApp.setActivationPolicy(.accessory)
        }
    }
}
