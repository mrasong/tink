import AppKit
import SwiftUI

@MainActor
public final class AppWindowOpener {
    public static let shared = AppWindowOpener()

    private var action: OpenWindowAction?

    private init() {}

    var isInstalled: Bool { action != nil }

    func install(_ action: OpenWindowAction) {
        self.action = action
    }

    public func open(id: String) {
        if let action {
            action(id: id)
            return
        }
        openViaMainMenu(windowID: id)
    }

    // 值型 WindowGroup（如消息详情按 messageID 开窗）用
    public func open<Value: Hashable & Codable>(id: String, value: Value) {
        if let action {
            action(id: id, value: value)
            return
        }
        openNewWindowViaMainMenu(sceneID: id)
    }

    // openWindow 环境只在场景视图内可得；状态栏菜单等 AppKit 调用方在
    // 任何窗口尚未打开过时，退化为触发 SwiftUI 自动生成的主菜单项。
    private func openViaMainMenu(windowID: String) {
        guard let key = Self.menuItemTitles[windowID] else { return }
        attemptInvoke(matching: { $0 == NSLocalizedString(key, comment: "") })
    }

    // 值型场景由 SwiftUI 生成「New <场景标题> Window」菜单项，以 nil 值开窗；
    // 标题是系统拼的，只能按本地化后的场景标题做包含匹配。
    private func openNewWindowViaMainMenu(sceneID: String) {
        guard let key = Self.newItemTitles[sceneID] else { return }
        let title = NSLocalizedString(key, comment: "")
        attemptInvoke(matching: { $0.contains(title) })
    }

    private static let menuItemTitles: [String: String] = [
        NotificationWindowCoordinator.windowID: "Notification Manager",
        SettingsWindow.id: "Settings…",
        AboutWindow.id: "About Tink",
    ]

    private static let newItemTitles: [String: String] = [
        MessageDetailWindowCoordinator.windowID: "Message Detail",
    ]

    private func attemptInvoke(matching: @escaping (String) -> Bool, remaining: Int = 10) {
        func search(_ item: NSMenuItem) -> NSMenuItem? {
            if matching(item.title), let action = item.action { return item }
            for sub in item.submenu?.items ?? [] {
                if let hit = search(sub) { return hit }
            }
            return nil
        }
        for top in NSApp.mainMenu?.items ?? [] {
            guard let item = search(top), let action = item.action else { continue }
            if NSApp.sendAction(action, to: item.target, from: item) { return }
        }
        guard remaining > 0 else { return }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in
            self?.attemptInvoke(matching: matching, remaining: remaining - 1)
        }
    }
}

struct WindowOpenerInstaller: View {
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Color.clear
            .frame(width: 0, height: 0)
            .onAppear {
                AppWindowOpener.shared.install(openWindow)
            }
    }
}
