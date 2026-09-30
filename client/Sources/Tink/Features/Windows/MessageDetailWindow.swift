import AppKit
import SwiftData
import SwiftUI

// 单独的消息详情视图，支持完整 Markdown 渲染与富文本操作
public struct MessageDetailView: View {
    public let messageID: UInt64
    private let showsToolbar: Bool
    @ObservedObject private var connection = ConnectionManager.shared
    @Query private var storedMessages: [StoredMessage]
    @State private var showCopiedToast = false
    @State private var copyToastDismissWorkItem: DispatchWorkItem?

    /// - Parameter showsToolbar: 独立窗口为 true（带标题栏工具栏与最小尺寸）；
    ///   内嵌到三栏布局第三栏时传 false，窗口级 chrome 由外层负责。
    public init(messageID: UInt64, showsToolbar: Bool = true) {
        self.messageID = messageID
        self.showsToolbar = showsToolbar
        _storedMessages = Query(filter: #Predicate<StoredMessage> { $0.id == messageID })
    }

    public var body: some View {
        Group {
            if let message = currentMessage {
                detail(message)
            } else {
                ContentUnavailableView(
                    "Message Unavailable", systemImage: "exclamationmark.triangle")
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(WindowOpenerInstaller())
    }

    private func detail(_ message: TinkMessage) -> some View {
        Group {
            if showsToolbar {
                NavigationStack {
                    content(message)
                        .navigationTitle(
                            Text(
                                message.title.isEmpty
                                    ? String(localized: "Notification") : message.title))
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .toolbar {
                    ToolbarItemGroup(placement: .primaryAction) {
                        if let urlStr = message.url, let url = URL(string: urlStr),
                            !urlStr.isEmpty
                        {
                            Button {
                                NSWorkspace.shared.open(url)
                            } label: {
                                Image(systemName: "link")
                            }
                            .help(String(localized: "Open Link"))
                        }

                        Button {
                            currentMessage?.isRead == true
                                ? connection.markAsUnread(message.id)
                                : connection.markAsRead(message.id)
                        } label: {
                            Image(
                                systemName: currentMessage?.isRead == true
                                    ? "envelope" : "envelope.badge")
                        }
                        .help(
                            currentMessage?.isRead == true
                                ? String(localized: "Mark as Unread")
                                : String(localized: "Mark as Read"))

                        Button {
                            connection.toggleStar(message.id)
                        } label: {
                            Image(
                                systemName: currentMessage?.isStarred == true
                                    ? "star.fill" : "star")
                        }
                        .help(
                            currentMessage?.isStarred == true
                                ? String(localized: "Unstar") : String(localized: "Star"))

                        Button {
                            copyBody(message)
                        } label: {
                            Image(systemName: "document.on.document")
                        }
                        .help(String(localized: "Copy Content"))
                    }
                }
                .overlay(alignment: .bottom) {
                    if showCopiedToast {
                        Text("Copied")
                            .font(.caption2.weight(.medium))
                            .foregroundStyle(.primary)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                            .background(.regularMaterial, in: Capsule())
                            .overlay(
                                Capsule()
                                    .stroke(Color.secondary.opacity(0.2), lineWidth: 0.5)
                            )
                            .offset(y: -20)
                            .transition(.opacity)
                            .allowsHitTesting(false)
                    }
                }
                .frame(minWidth: 640, minHeight: 480)
            } else {
                content(message)
            }
        }
    }

    @ViewBuilder
    private func content(_ message: TinkMessage) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 12) {
                // 标题 + 元信息紧挨成组，与正文之间留分组间隔
                VStack(alignment: .leading, spacing: 4) {
                    // macOS 上系统不提供吸顶大标题（toolbarTitleDisplayMode 官方文档：
                    // "has no effect on macOS"），只能在内容顶部自绘静态大标题
                    Text(
                        message.title.isEmpty
                            ? String(localized: "Notification") : message.title)
                        .font(.largeTitle)
                        .fontWeight(.bold)

                    // 大标题下方的元信息行：分组徽章 + 时间小字
                    HStack(spacing: 8) {
                        if let group = message.group,
                            !group.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
                        {
                            Text(group)
                                .font(.caption)
                                .padding(.horizontal, 6)
                                .padding(.vertical, 2)
                                .background(Color.accentColor.opacity(0.12))
                                .foregroundColor(.accentColor)
                                .cornerRadius(4)
                        }

                        if let date = TimeFormatting.string(from: message.createdAt) {
                            Text(date)
                                .font(.caption)
                                .foregroundColor(.secondary)
                        }
                    }
                }

                // 消息主体：完整 Markdown 语法渲染（标题、代码块、列表、引用、粗体、斜体、链接等）
                MarkdownContentView(bodyForRendering(message))
            }
            .frame(maxWidth: 960, alignment: .leading)
            .frame(maxWidth: .infinity)
            .padding(.top, 24)
            .padding(.bottom, 28)
            .padding(.horizontal, 20)
        }
    }

    // 正文首个 H1 与标题完全相同时剥掉该行，避免系统大标题下重复出现同样的大字
    private func bodyForRendering(_ message: TinkMessage) -> String {
        let title = message.title.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !title.isEmpty else { return message.body }
        var lines = message.body.components(separatedBy: "\n")
        guard let firstContent = lines.firstIndex(where: {
            !$0.trimmingCharacters(in: .whitespaces).isEmpty
        }) else { return message.body }
        guard lines[firstContent].trimmingCharacters(in: .whitespaces) == "# \(title)" else {
            return message.body
        }
        lines.remove(at: firstContent)
        return lines.joined(separator: "\n")
    }

    private func copyBody(_ message: TinkMessage) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        guard pasteboard.setString(message.body, forType: .string) else { return }
        withAnimation(.easeOut(duration: 0.15)) {
            showCopiedToast = true
        }
        copyToastDismissWorkItem?.cancel()
        let workItem = DispatchWorkItem {
            withAnimation(.easeIn(duration: 0.15)) {
                showCopiedToast = false
            }
        }
        copyToastDismissWorkItem = workItem
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5, execute: workItem)
    }

    private var currentMessage: TinkMessage? {
        storedMessages.first?.message()
    }
}

// 消息详情窗口 (WindowGroup(for: UInt64.self)) 的调用协调器：
// 值型窗口按 messageID 开窗——不同消息各开一窗，同一消息复用其窗口并置前。
@MainActor
public final class MessageDetailWindowCoordinator {
    public static let shared = MessageDetailWindowCoordinator()
    public static let windowID = "message-detail"

    // 冷启动兜底：菜单栏形态下还没有任何场景视图安装过 openWindow，
    // 先记下点击的消息，让主菜单回退打开的那扇 nil 值详情窗渲染它。
    public private(set) var bootstrapMessageID: UInt64?

    private init() {}

    public func requestShow(message: TinkMessage) {
        ConnectionManager.shared.markAsRead(message.id)
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
        if !AppWindowOpener.shared.isInstalled {
            bootstrapMessageID = message.id
        }
        AppWindowOpener.shared.open(id: Self.windowID, value: message.id)
    }
}
