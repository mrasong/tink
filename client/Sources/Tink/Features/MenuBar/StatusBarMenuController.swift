import AppKit
import SwiftUI

// 状态栏入口：NSStatusItem + 原生 NSMenu。
// 头部（logo/名称/版本）通过 NSMenuItem.view 嵌 SwiftUI，其余全部为原生菜单项，
// 快捷键提示由系统依据 keyEquivalent 自动渲染。
@MainActor
final class StatusBarMenuController: NSObject, NSMenuDelegate {
    static let shared = StatusBarMenuController()

    private var statusItem: NSStatusItem?
    private let menu = NSMenu()
    private var defaultsObserver: NSObjectProtocol?

    private override init() {
        super.init()
    }

    func install() {
        guard statusItem == nil else { return }
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        item.button?.image = Self.statusBarIcon()
        menu.delegate = self
        menu.autoenablesItems = true
        item.menu = menu
        statusItem = item

        defaultsObserver = NotificationCenter.default.addObserver(
            forName: UserDefaults.didChangeNotification, object: nil, queue: .main
        ) { [weak self] _ in
            MainActor.assumeIsolated {
                self?.statusItem?.button?.image = Self.statusBarIcon()
            }
        }
    }

    private static func statusBarIcon() -> NSImage? {
        let symbolName = UserDefaults.standard.string(
            forKey: ClientConfiguration.StorageKeys.menuBarSymbolName
        ) ?? ClientConfiguration.StorageKeys.menuBarDefaultIcon
        if symbolName == ClientConfiguration.StorageKeys.menuBarDefaultIcon {
            return AppIcons.menuBarTemplateImage()
        }
        let image = NSImage(systemSymbolName: symbolName, accessibilityDescription: "Tink")
        image?.size = NSSize(width: 18, height: 18)
        return image
    }

    // MARK: - NSMenuDelegate

    func menuNeedsUpdate(_ menu: NSMenu) {
        rebuild()
    }

    private func rebuild() {
        menu.removeAllItems()
        let connection = ConnectionManager.shared

        menu.addItem(headerItem())

        let status = NSMenuItem()
        status.attributedTitle = Self.statusAttributedTitle(connection.status)
        status.isEnabled = false
        menu.addItem(status)

        menu.addItem(item("Reconnect", action: #selector(reconnect)))
        menu.addItem(.separator())

        if let error = connection.storageError {
            let errorItem = NSMenuItem(title: error, action: nil, keyEquivalent: "")
            errorItem.isEnabled = false
            menu.addItem(errorItem)
            menu.addItem(item("Retry", action: #selector(retryStorage)))
            menu.addItem(.separator())
        }

        menu.addItem(
            item(
                Self.countedTitle("Unread", count: connection.unreadCount),
                action: #selector(showUnread)))
        menu.addItem(
            item(
                Self.countedTitle("Open History", count: connection.messageStore.count()),
                action: #selector(showHistory)))
        menu.addItem(.separator())

        let settings = item("Settings…", action: #selector(openSettings))
        settings.keyEquivalent = ","
        menu.addItem(settings)
        menu.addItem(item("About", action: #selector(showAbout)))
        menu.addItem(.separator())

        let quit = item("Quit Tink", action: #selector(quit))
        quit.keyEquivalent = "q"
        menu.addItem(quit)
    }

    private func item(_ titleKey: String, action: Selector) -> NSMenuItem {
        let menuItem = NSMenuItem(
            title: NSLocalizedString(titleKey, comment: ""), action: action, keyEquivalent: "")
        menuItem.target = self
        return menuItem
    }

    private func headerItem() -> NSMenuItem {
        let hosting = NSHostingView(rootView: MenuBarHeaderView())
        hosting.frame = NSRect(origin: .zero, size: hosting.fittingSize)
        let header = NSMenuItem()
        header.view = hosting
        return header
    }

    private static func countedTitle(_ baseKey: String, count: Int) -> String {
        let base = NSLocalizedString(baseKey, comment: "")
        return count > 0 ? "\(base) (\(count))" : base
    }

    private static func statusAttributedTitle(_ status: ConnectionStatus) -> NSAttributedString {
        let color: NSColor
        switch status {
        case .connected: color = .systemGreen
        case .connecting: color = .systemOrange
        case .disconnected: color = .secondaryLabelColor
        case .error: color = .systemRed
        }
        let result = NSMutableAttributedString(
            string: "● ", attributes: [.foregroundColor: color])
        result.append(
            NSAttributedString(
                string: status.title,
                attributes: [.foregroundColor: NSColor.secondaryLabelColor]))
        return result
    }

    // MARK: - Actions

    @objc private func reconnect() {
        ConnectionManager.shared.reconnect()
    }

    @objc private func retryStorage() {
        ConnectionManager.shared.retryStorage()
    }

    @objc private func showUnread() {
        WindowNavigation.showNotificationManager(group: .unread)
    }

    @objc private func showHistory() {
        WindowNavigation.showNotificationManager(group: nil)
    }

    @objc private func openSettings() {
        WindowNavigation.showSettings()
    }

    @objc private func showAbout() {
        WindowNavigation.showAbout()
    }

    @objc private func quit() {
        NSApplication.shared.terminate(nil)
    }

    #if DEBUG
        func popUpForDebug() {
            rebuild()
            for (index, menuItem) in menu.items.enumerated() {
                let kind = menuItem.isSeparatorItem ? "separator" : menuItem.view != nil ? "view" : "title"
                let shortcut = menuItem.keyEquivalent.isEmpty
                    ? "" : " key=\(menuItem.keyEquivalent)"
                let line =
                    "[DebugMenu] \(index) [\(kind)] \(menuItem.title) enabled=\(menuItem.isEnabled)\(shortcut)\n"
                FileHandle.standardError.write(line.data(using: .utf8)!)
            }
            statusItem?.button?.performClick(nil)
        }
    #endif
}

// 菜单头部：logo + 名称 + 小字版本号（Q1: 紧跟名称右侧；Q2: v<short> 格式）
struct MenuBarHeaderView: View {
    static let width: CGFloat = 220

    var body: some View {
        HStack(alignment: .center, spacing: 8) {
            TinkLogoView(size: 28, showShadow: false)
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text("Tink")
                    .font(.system(size: 20, weight: .semibold, design: .rounded))
                Text(AppVersion.display)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        .frame(width: Self.width)
    }
}
