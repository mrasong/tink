import AppKit
import SwiftUI

// 设置窗口场景标识，TinkApp 中以普通 Window 场景声明
public enum SettingsWindow {
    public static let id = "tink-settings"
}

public struct SettingsView: View {
    @State private var serverURL: String = ""
    @State private var apiToken: String = ""
    @State private var isTokenVisible: Bool = false
    @State private var deviceName: String = ""
    @State private var deviceID: String = ""
    @State private var lastEventID: UInt64 = 0
    @AppStorage(ClientConfiguration.StorageKeys.menuBarSymbolName) private var menuBarSymbolName =
        ClientConfiguration.StorageKeys.menuBarDefaultIcon

    private let menuBarSymbols = [
        "bell", "bell.fill",
        "bell.badge", "bell.badge.fill",
        "bubble.left", "bubble.left.fill",
        "bubble.right", "bubble.right.fill",
        "message", "message.fill",
        "tray", "tray.fill",
        "alarm", "alarm.fill",
        "star", "star.fill",
        "bolt", "bolt.fill",
        "bolt.circle", "bolt.circle.fill",
    ]

    // 状态与弹窗
    @State private var isSavedAlert = false
    @State private var isTestingPing = false
    @State private var pingResult: String? = nil
    @State private var pingSuccess: Bool? = nil
    @State private var isCopiedDeviceID = false
    @State private var testNotificationResult: String?
    @State private var isSendingTestNotification = false
    @State private var isShowingResetConfirmation = false
    @ObservedObject private var connection = ConnectionManager.shared

    public init() {}

    public var body: some View {
        VStack(spacing: 0) {
            settingsHeader
            Form {
                serverSection
                deviceSection
                menuBarIconSection
                actionsSection
            }
            .formStyle(.grouped)
        }
        .frame(width: 460)
        .fixedSize(horizontal: false, vertical: true)
        .onAppear {
            loadSettings()
        }
        .alert("Settings Saved", isPresented: $isSavedAlert) {
            Button("OK", role: .cancel) {}
        }
        .alert(
            "Reset All Data?",
            isPresented: $isShowingResetConfirmation
        ) {
            Button("Reset & Quit", role: .destructive) {
                LocalStorage.shared.resetAllData()
                NSApplication.shared.terminate(nil)
            }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text(
                "This will permanently remove all local configurations, device credentials, and message history database. Tink will quit so it can start fresh upon next launch."
            )
        }
        .alert(
            "Storage Error",
            isPresented: Binding(
                get: { connection.storageError != nil },
                set: { if !$0 { connection.storageError = nil } }
            )
        ) {
            Button("Retry") { connection.retryStorage() }
            Button("OK", role: .cancel) {}
        } message: {
            Text(connection.storageError ?? String(localized: "Unable to save message history"))
        }
        .alert(
            "Temporary Storage",
            isPresented: Binding(
                get: { connection.persistenceWarning != nil },
                set: { _ in }
            )
        ) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(
                connection.persistenceWarning
                    ?? String(
                        localized: "Message history may not survive restarting Tink."))
        }
    }

    // MARK: - Header

    private var settingsHeader: some View {
        HStack(spacing: 10) {
            TinkLogoView(size: 40, showShadow: false)
            VStack(alignment: .leading, spacing: 3) {
                HStack(alignment: .firstTextBaseline, spacing: 6) {
                    Text("Tink")
                        .font(.system(size: 15, weight: .semibold, design: .rounded))

                    Text(AppVersion.display)
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }

                Text("Your Notifications. Your Way.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 20)
        .padding(.vertical, 20)
    }

    // MARK: - Form Sections

    private var serverSection: some View {
        Section {
            TextField(
                "Server Gateway URL", text: $serverURL,
                prompt: Text(ClientConfiguration.defaultServerURL))

            HStack(spacing: 4) {
                if isTokenVisible {
                    TextField("API Secret Key", text: $apiToken)
                } else {
                    SecureField("API Secret Key", text: $apiToken)
                }
                Button(action: { isTokenVisible.toggle() }) {
                    Image(systemName: isTokenVisible ? "eye.slash" : "eye")
                }
                .buttonStyle(.borderless)
                .help(
                    isTokenVisible
                        ? String(localized: "Hide token") : String(localized: "Show token"))
            }

            LabeledContent("Connection Test") {
                HStack(spacing: 8) {
                    if isTestingPing {
                        ProgressView()
                            .controlSize(.small)
                    }
                    if let pingResult {
                        Text(pingResult)
                            .foregroundStyle(pingSuccess == true ? Color.green : Color.red)
                    }
                    Button("Test Connection", action: testConnection)
                        .disabled(isTestingPing || serverURL.isEmpty)
                }
            }
        } header: {
            Text("Server Connection")
        } footer: {
            if !apiToken.isEmpty && !apiToken.hasPrefix("sk-tink-") {
                Text("⚠️ Token format usually starts with sk-tink-")
                    .foregroundColor(.orange)
            } else {
                Text("Authentication token registered with your Tink server")
            }
        }
    }

    private var deviceSection: some View {
        Section {
            TextField("Device Display Name", text: $deviceName, prompt: Text("MacBook Pro"))

            LabeledContent("Device ID (Immutable)") {
                HStack(spacing: 8) {
                    Text(deviceID)
                        .font(.system(.body, design: .monospaced))
                        .lineLimit(1)
                        .truncationMode(.middle)
                        .textSelection(.enabled)
                    Button(action: copyDeviceID) {
                        Label(
                            isCopiedDeviceID
                                ? String(localized: "Copied!") : String(localized: "Copy"),
                            systemImage: isCopiedDeviceID ? "checkmark" : "doc.on.doc")
                    }
                }
            }

            LabeledContent("Last Replay Cursor (Last-Event-ID)") {
                HStack(spacing: 8) {
                    Text("\(lastEventID)")
                        .font(.system(.body, design: .monospaced))
                    Button("Reset Cursor") {
                        LocalStorage.shared.lastEventID = 0
                        lastEventID = 0
                    }
                }
            }
        } header: {
            Text("Device Identity")
        } footer: {
            Text("Clear event cursor to receive previous unacknowledged offline notifications")
        }
    }

    private var menuBarIconSection: some View {
        Section {
            // macOS 的 Picker(.menu) 底层是 NSPopUpButtonCell，会丢弃菜单项图片，
            // 因此改用 Menu(NSMenuItem) 渲染带图标的下拉列表。
            Menu {
                Button {
                    menuBarSymbolName = ClientConfiguration.StorageKeys.menuBarDefaultIcon
                } label: {
                    Label {
                        Text("Tink Default")
                    } icon: {
                        if let icon = AppIcons.menuBarTemplateImage() {
                            Image(nsImage: icon)
                        } else {
                            Image(systemName: "app.dashed")
                        }
                    }
                    .labelStyle(.titleAndIcon)
                }

                Divider()

                ForEach(menuBarSymbols, id: \.self) { symbol in
                    Button {
                        menuBarSymbolName = symbol
                    } label: {
                        // macOS 27 起 AppKit 默认隐藏菜单项图片（NSMenuItem.h），
                        // 图标即内容的选择器需显式 titleAndIcon 强制显示
                        Label(symbol, systemImage: symbol)
                            .labelStyle(.titleAndIcon)
                    }
                }
            } label: {
                HStack(spacing: 6) {
                    currentMenuBarIconLabel
                    Text(
                        menuBarSymbolName
                            == ClientConfiguration.StorageKeys.menuBarDefaultIcon
                            ? String(localized: "Tink Default") : menuBarSymbolName
                    )
                    Spacer(minLength: 4)
                    Image(systemName: "chevron.up.chevron.down")
                        .font(.system(size: 10, weight: .semibold))
                        .foregroundColor(.secondary)
                }
            }
            .menuStyle(.button)
        } header: {
            Text("Menu Bar Icon")
        } footer: {
            Text("Changes take effect immediately and is saved automatically.")
        }
    }

    private var actionsSection: some View {
        Section {
            LabeledContent("Test Notification") {
                HStack(spacing: 8) {
                    if let testNotificationResult {
                        Text(testNotificationResult)
                            .foregroundStyle(
                                testNotificationResult == String(localized: "Sent")
                                    ? Color.green : Color.secondary
                            )
                            .lineLimit(1)
                    }
                    Button("Send", action: sendTestNotification)
                        .disabled(isSendingTestNotification)
                }
            }

            LabeledContent("Local Data") {
                Button("Reset All Data...", role: .destructive) {
                    isShowingResetConfirmation = true
                }
            }

            LabeledContent("Changes") {
                Button("Save & Reconnect") {
                    saveSettings()
                }
                .keyboardShortcut(.defaultAction)
                .buttonStyle(.borderedProminent)
            }
        } header: {
            Text("Actions")
        }
    }

    private func copyDeviceID() {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(deviceID, forType: .string)
        isCopiedDeviceID = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
            isCopiedDeviceID = false
        }
    }

    private func sendTestNotification() {
        Task {
            await MainActor.run {
                isSendingTestNotification = true
                testNotificationResult = String(localized: "Sending…")
            }
            do {
                try await APIClient.shared.sendTestNotification()
                await MainActor.run {
                    isSendingTestNotification = false
                    testNotificationResult = String(localized: "Sent")
                }
            } catch {
                await MainActor.run {
                    isSendingTestNotification = false
                    testNotificationResult = error.localizedDescription
                }
            }
        }
    }

    private var currentMenuBarIconLabel: some View {
        Group {
            if menuBarSymbolName == ClientConfiguration.StorageKeys.menuBarDefaultIcon {
                if let icon = AppIcons.menuBarTemplateImage() {
                    Image(nsImage: icon)
                        .frame(width: 16, height: 16)
                } else {
                    Image(systemName: "app.dashed")
                }
            } else {
                Image(systemName: menuBarSymbolName)
            }
        }
        .frame(width: 16, height: 16)
    }

    private func loadSettings() {
        let storage = LocalStorage.shared
        serverURL = storage.serverURL
        apiToken = storage.apiToken
        deviceName = storage.deviceName
        deviceID = storage.deviceID
        lastEventID = storage.lastEventID
    }

    private func saveSettings() {
        let storage = LocalStorage.shared
        storage.serverURL = serverURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ "))
        storage.apiToken = apiToken.trimmingCharacters(in: .whitespacesAndNewlines)
        storage.deviceName = deviceName.trimmingCharacters(in: .whitespacesAndNewlines)

        isSavedAlert = true
        ConnectionManager.shared.reconnect()
    }

    private func testConnection() {
        isTestingPing = true
        pingResult = String(localized: "Pinging...")
        pingSuccess = nil

        Task {
            do {
                let res = try await APIClient.shared.ping(url: serverURL)
                await MainActor.run {
                    self.isTestingPing = false
                    self.pingSuccess = res.healthy
                    self.pingResult =
                        res.healthy
                        ? String(
                            format: String(localized: "%1$lldms (OK)"), res.latencyMs)
                        : String(localized: "Failed")
                }
            } catch {
                await MainActor.run {
                    self.isTestingPing = false
                    self.pingSuccess = false
                    self.pingResult = String(localized: "Offline")
                }
            }
        }
    }
}
