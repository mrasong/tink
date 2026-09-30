import AppKit
import SwiftData
import SwiftUI

// 通知管理主视图：三栏 NavigationSplitView（分组 | 消息列表 | 内嵌详情预览），
// 列表为 macOS 原生常开多选（List(selection:)），单击选中并自动标记已读，
// 双击/Return 经 contextMenu primaryAction 打开独立详情窗口
public struct NotificationManagerView: View {
    @ObservedObject var connection = ConnectionManager.shared
    // 分组选择是窗口与开窗调用方（状态栏菜单）之间的唯一通道
    @ObservedObject private var coordinator = NotificationWindowCoordinator.shared
    @Query(sort: \StoredMessage.receivedAt, order: .reverse) private var storedMessages:
        [StoredMessage]
    @State private var selectedIDs = Set<UInt64>()
    @State private var searchText = ""
    @State private var sidebarSearchText = ""

    // 删除确认弹窗状态
    enum DeleteActionType {
        case single(UInt64)
        case selected(Int)
        case clearGroup(NotificationGroup, Int)
    }
    @State private var pendingDeleteAction: DeleteActionType?
    @State private var showDeleteConfirmation = false

    private let smartGroups: [NotificationGroup] = [.all, .unread, .starred, .general]

    // 窗口 minWidth 必须容纳三栏各自的最小宽度：窄于此值时 NavigationSplitView 会去压缩
    // 固定宽度的侧栏列，列表内容随之溢出窗口左边缘被裁切。余量留给分隔线。
    // 详情列的 min 还须低于 ≈220：搜索框占用的是详情列那段工具栏，实测该列窄于此值才会折叠成放大镜。
    private enum Column {
        static let sidebarWidth: CGFloat = 180
        static let listMinWidth: CGFloat = 200
        static let detailMinWidth: CGFloat = 160
        static let windowMinWidth: CGFloat = sidebarWidth + listMinWidth + detailMinWidth + 20
    }

    public init() {}

    private var selectedGroup: NotificationGroup { coordinator.selectedGroup }

    // 动态提取所有不重复的有效 Group
    private var allGroups: [String] {
        var set = Set<String>()
        for msg in storedMessages {
            if let g = msg.group?.trimmingCharacters(in: .whitespacesAndNewlines), !g.isEmpty {
                set.insert(g)
            }
        }
        return Array(set).sorted()
    }

    // 根据侧栏搜索关键词过滤 Group 列表
    private var displayedGroups: [String] {
        let q = sidebarSearchText.trimmingCharacters(in: .whitespacesAndNewlines)
        if q.isEmpty {
            return allGroups
        }
        return allGroups.filter { $0.localizedCaseInsensitiveContains(q) }
    }

    // 分组成员判定的唯一出处：侧栏计数、列表过滤、导出与清空都走这里
    private func messages(in group: NotificationGroup) -> [StoredMessage] {
        switch group {
        case .all:
            return storedMessages
        case .unread:
            return storedMessages.filter { !$0.isRead }
        case .starred:
            return storedMessages.filter { $0.isStarred }
        case .general:
            return storedMessages.filter {
                $0.group == nil || $0.group!.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            }
        case .custom(let name):
            return storedMessages.filter { $0.group == name }
        }
    }

    private func countForGroup(_ group: NotificationGroup) -> Int {
        messages(in: group).count
    }

    // 按当前选中的 Group 与搜索关键词组合过滤消息列表
    private var filteredMessages: [TinkMessage] {
        let groupFiltered = messages(in: selectedGroup).map { $0.message() }

        let query = searchText.trimmingCharacters(in: .whitespacesAndNewlines)
        if query.isEmpty {
            return groupFiltered
        }
        return groupFiltered.filter {
            $0.title.localizedCaseInsensitiveContains(query)
                || $0.body.localizedCaseInsensitiveContains(query)
                || ($0.group?.localizedCaseInsensitiveContains(query) ?? false)
        }
    }

    public var body: some View {
        NavigationSplitView {
            sidebar
        } content: {
            listView
        } detail: {
            detailPane
        }
        // 新选中的未读条目自动标记已读（Mail 语义）；打开独立详情由双击/Return 触发
        .onChange(of: selectedIDs) { oldSelection, newSelection in
            for id in newSelection.subtracting(oldSelection) {
                connection.markAsRead(id)
            }
        }
        // 切换分组后列表内容变化，残留选择会指向不可见消息，直接清空
        .onChange(of: coordinator.selectedGroup) { _, _ in
            selectedIDs.removeAll()
        }
        .confirmationDialog(
            "Confirm Delete",
            isPresented: $showDeleteConfirmation,
            titleVisibility: .visible,
            presenting: pendingDeleteAction
        ) { action in
            Button("Delete", role: .destructive) {
                executeDelete(action)
            }
            Button("Cancel", role: .cancel) {
                pendingDeleteAction = nil
            }
        } message: { action in
            Text(deleteConfirmationMessage(for: action))
        }
        .background(WindowOpenerInstaller())
        .frame(minWidth: Column.windowMinWidth, minHeight: 520)
    }

    // MARK: - Sidebar

    private var sidebar: some View {
        List(selection: sidebarSelection) {
            Section {
                ForEach(smartGroups, id: \.self) { group in
                    sidebarRow(for: group)
                }
            }
            if !displayedGroups.isEmpty {
                Section("Groups") {
                    ForEach(displayedGroups, id: \.self) { name in
                        sidebarRow(for: .custom(name))
                    }
                }
            }
        }
        .searchable(
            text: $sidebarSearchText, placement: .sidebar,
            prompt: Text("Search Category")
        )
        .navigationSplitViewColumnWidth(Column.sidebarWidth)
        .safeAreaInset(edge: .bottom) {
            sidebarFooter
        }
    }

    // List 单选需要可选绑定，空选择回退到当前分组
    private var sidebarSelection: Binding<NotificationGroup?> {
        Binding(
            get: { selectedGroup },
            set: { if let group = $0 { selectGroup(group) } }
        )
    }

    private func displayName(for group: NotificationGroup) -> String {
        switch group {
        case .all: return String(localized: "All")
        case .unread: return String(localized: "Unread")
        case .starred: return String(localized: "Starred")
        case .general: return String(localized: "General")
        case .custom(let name): return name
        }
    }

    @ViewBuilder
    private func sidebarRow(for group: NotificationGroup) -> some View {
        let count = countForGroup(group)
        let title = displayName(for: group)
        HStack {
            Label(title, systemImage: group.icon)
            Spacer()
            if count > 0 {
                Text("\(count)")
                    .foregroundStyle(.secondary)
            }
        }
        .tag(group)
        .contextMenu { groupActions(for: group) }
    }

    // 分组级动作挂在源列表行上（Mail 的 "Export Mailbox…" 同处），
    // 侧栏底部不再常驻按钮：HIG 明确反对把动作放在会被窗口下缘遮住的位置。
    // 作用于右键那一行，而非当前选中分组。
    @ViewBuilder
    private func groupActions(for group: NotificationGroup) -> some View {
        let count = countForGroup(group)
        Button(
            group == .all ? String(localized: "Export All") : String(localized: "Export Group")
        ) {
            exportGroupAsJSON(group)
        }
        .disabled(count == 0)

        Divider()

        Button(role: .destructive) {
            confirmClearGroup(group)
        } label: {
            Text(group == .all ? String(localized: "Clear All") : String(localized: "Clear Group"))
        }
        .disabled(count == 0)
    }

    private var sidebarFooter: some View {
        HStack(spacing: 8) {
            Circle()
                .fill(connection.status == .connected ? Color.green : Color.orange)
                .frame(width: 8, height: 8)
            Text(connection.status.title)
                .font(.caption)
                .foregroundColor(.secondary)
            Spacer()
        }
        .padding(10)
        .background(.bar)
    }

    // MARK: - Message list column

    private var listView: some View {
        VStack(spacing: 0) {
            if let error = connection.storageError {
                HStack {
                    Label(error, systemImage: "exclamationmark.triangle")
                        .font(.caption)
                        .foregroundStyle(.orange)
                    Spacer()
                    Button("Retry") { connection.retryStorage() }
                        .buttonStyle(.plain)
                        .font(.caption)
                }
                .padding(.horizontal, 14)
                .padding(.vertical, 6)
            }

            if filteredMessages.isEmpty {
                emptyState
            } else {
                notificationList
            }
        }
        .navigationTitle("\(currentGroupTitle) (\(filteredMessages.count))")
        .navigationSplitViewColumnWidth(
            min: Column.listMinWidth, ideal: 300, max: 400
        )
        // 挂在列表列而非 SplitView 根视图：窄窗口下搜索框才会折叠成放大镜
        .searchable(
            text: $searchText, placement: .toolbar,
            prompt: Text("Search Content...")
        )
        .toolbar {
            ToolbarItemGroup(placement: .primaryAction) {
                primaryToolbarItems
            }
        }
    }

    @ViewBuilder
    private var primaryToolbarItems: some View {
        // 原生多选常开，无需选择模式开关；全选走系统 Edit 菜单的 ⌘A
        Button(action: toggleSelectedReadStatus) {
            Image(systemName: hasSelectedUnread ? "envelope" : "envelope.badge")
        }
        .disabled(selectedIDs.isEmpty)
        .help(
            hasSelectedUnread
                ? String(localized: "Mark selected notifications as read")
                : String(localized: "Mark selected notifications as unread"))

        Button(action: confirmDeleteSelected) {
            Image(systemName: "trash")
        }
        .disabled(selectedIDs.isEmpty)
        .tint(.red)
        .help(String(localized: "Delete selected notifications"))

        Button(action: exportSelectedAsJSON) {
            Image(systemName: "arrow.down.document")
        }
        .disabled(selectedIDs.isEmpty)
        .help(String(localized: "Export selected notifications as JSON"))
    }

    private var notificationList: some View {
        List(filteredMessages, selection: $selectedIDs) { message in
            MessageRowView(
                message: message,
                onToggleStar: {
                    connection.toggleStar(message.id)
                }
            )
        }
        // macOS 13+ 官方选择型菜单：单击=选择，双击/Return=primaryAction 打开独立详情窗口
        .contextMenu(
            forSelectionType: UInt64.self,
            menu: { ids in selectionMenu(for: ids) },
            primaryAction: { ids in
                openDetailWindows(for: ids)
            }
        )
    }

    // 行/选择右键菜单：单选时提供打开、链接、复制与读/星/删除；多选时提供批量读与删除
    @ViewBuilder
    private func selectionMenu(for ids: Set<UInt64>) -> some View {
        if ids.count == 1, let id = ids.first, let message = message(with: id) {
            Button(String(localized: "Open")) {
                openDetail(message)
            }

            if let urlStr = message.url, !urlStr.isEmpty, let url = URL(string: urlStr) {
                Button(String(localized: "Open Link")) {
                    NSWorkspace.shared.open(url)
                }
            }

            Button(String(localized: "Copy Content")) {
                copyMessageBody(message)
            }

            Divider()

            Button(
                message.isRead
                    ? String(localized: "Mark as Unread") : String(localized: "Mark as Read"),
                systemImage: message.isRead ? "envelope.badge" : "envelope"
            ) {
                message.isRead
                    ? connection.markAsUnread(message.id) : connection.markAsRead(message.id)
            }

            Button(
                message.isStarred
                    ? String(localized: "Remove Star") : String(localized: "Add Star"),
                systemImage: message.isStarred ? "star.slash" : "star"
            ) {
                connection.toggleStar(message.id)
            }

            Divider()

            Button("Delete", systemImage: "trash", role: .destructive) {
                confirmDeleteSingle(id)
            }
        } else {
            Button(
                hasSelectedUnread
                    ? String(localized: "Mark selected notifications as read")
                    : String(localized: "Mark selected notifications as unread")
            ) {
                toggleSelectedReadStatus()
            }

            Button("Delete", systemImage: "trash", role: .destructive) {
                confirmDeleteSelected()
            }
        }
    }

    private func message(with id: UInt64) -> TinkMessage? {
        storedMessages.first { $0.id == id }?.message()
    }

    private func openDetail(_ message: TinkMessage) {
        MessageDetailWindowCoordinator.shared.requestShow(message: message)
    }

    private func openDetailWindows(for ids: Set<UInt64>) {
        // WindowGroup 值型窗口：每条消息各开一窗，同一条复用其窗口
        for id in ids.sorted() {
            if let message = message(with: id) {
                openDetail(message)
            }
        }
    }

    private func copyMessageBody(_ message: TinkMessage) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(message.body, forType: .string)
    }

    // MARK: - Detail preview column

    @ViewBuilder
    private var detailPane: some View {
        Group {
            if selectedIDs.count == 1, let id = selectedIDs.first {
                MessageDetailView(messageID: id, showsToolbar: false)
            } else {
                ContentUnavailableView(
                    "Select a notification", systemImage: "envelope"
                )
            }
        }
        .navigationSplitViewColumnWidth(min: Column.detailMinWidth, ideal: 500)
    }

    private var emptyState: some View {
        Group {
            if searchText.isEmpty {
                ContentUnavailableView(
                    "No notifications in this category", systemImage: "tray")
            } else {
                ContentUnavailableView {
                    Label("No results", systemImage: "magnifyingglass")
                } description: {
                    Text("No notifications matching \"\(searchText)\"")
                }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    // MARK: - Actions & Helpers

    private func selectGroup(_ group: NotificationGroup) {
        coordinator.selectedGroup = group
    }

    private var hasSelectedUnread: Bool {
        let selectedMessages = filteredMessages.filter { selectedIDs.contains($0.id) }
        return selectedMessages.contains { !$0.isRead }
    }

    private func toggleSelectedReadStatus() {
        guard !selectedIDs.isEmpty else { return }
        let shouldMarkRead = hasSelectedUnread
        for id in selectedIDs {
            if shouldMarkRead {
                connection.markAsRead(id)
            } else {
                connection.markAsUnread(id)
            }
        }
    }

    private func confirmDeleteSelected() {
        guard !selectedIDs.isEmpty else { return }
        pendingDeleteAction = .selected(selectedIDs.count)
        showDeleteConfirmation = true
    }

    private func confirmDeleteSingle(_ id: UInt64) {
        pendingDeleteAction = .single(id)
        showDeleteConfirmation = true
    }

    private func confirmClearGroup(_ group: NotificationGroup) {
        let count = countForGroup(group)
        guard count > 0 else { return }
        pendingDeleteAction = .clearGroup(group, count)
        showDeleteConfirmation = true
    }

    private func executeDelete(_ action: DeleteActionType) {
        switch action {
        case .single(let id):
            connection.deleteMessage(id: id)
            selectedIDs.remove(id)
        case .selected:
            connection.deleteMessages(ids: selectedIDs)
            selectedIDs.removeAll()
        case .clearGroup(let group, _):
            connection.deleteMessages(ids: Set(messages(in: group).map { $0.id }))
            selectedIDs.removeAll()
        }
        pendingDeleteAction = nil
    }

    private func deleteConfirmationMessage(for action: DeleteActionType) -> String {
        switch action {
        case .single:
            return String(
                localized:
                    "Are you sure you want to delete this notification? \nThis action cannot be undone."
            )
        case .selected(let count):
            return String(
                format:
                    String(
                        localized:
                            "Are you sure you want to delete %1$lld selected notification(s)? \nThis action cannot be undone."
                    ),
                count
            )
        case .clearGroup(let group, let count):
            if group == .all {
                return String(
                    localized:
                        "Are you sure you want to clear all notifications? \nThis action cannot be undone."
                )
            }
            return String(
                format:
                    String(
                        localized:
                            "Are you sure you want to clear all %1$lld notification(s) in '%2$@'? \nThis action cannot be undone."
                    ),
                count, group.title
            )
        }
    }

    private func exportSelectedAsJSON() {
        let list = filteredMessages.filter { selectedIDs.contains($0.id) }
        runExportPanel(list, suggestedName: "tink_selected.json")
    }

    // 侧栏是分组级操作：导出该分组的完整列表，与列表列的搜索过滤无关
    private func exportGroupAsJSON(_ group: NotificationGroup) {
        let safeGroupName = group.rawValue.replacingOccurrences(of: "__", with: "")
        runExportPanel(
            messages(in: group).map { $0.message() },
            suggestedName: "tink_messages_\(safeGroupName.isEmpty ? "all" : safeGroupName).json")
    }

    private func runExportPanel(_ exportList: [TinkMessage], suggestedName: String) {
        guard !exportList.isEmpty else { return }

        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        guard let data = try? encoder.encode(exportList) else { return }

        let savePanel = NSSavePanel()
        savePanel.canCreateDirectories = true
        savePanel.nameFieldStringValue = suggestedName
        savePanel.allowedContentTypes = [.json]

        if savePanel.runModal() == .OK, let url = savePanel.url {
            try? data.write(to: url)
        }
    }

    private var currentGroupTitle: String {
        selectedGroup.title
    }
}

// 原生 List 行：双行布局（star/未读点 + 标题 + 时间 / 摘要 + 分组标签）。
// 行内不挂点击手势、无行级 contextMenu：选择与选择型右键菜单由 List 层统一管理
private struct MessageRowView: View {
    let message: TinkMessage
    let onToggleStar: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 8) {
                Button(action: onToggleStar) {
                    Image(systemName: message.isStarred ? "star.fill" : "star")
                        .foregroundStyle(message.isStarred ? .yellow : .secondary)
                }
                .buttonStyle(.plain)

                if !message.isRead {
                    Circle()
                        .fill(Color.accentColor)
                        .frame(width: 7, height: 7)
                }

                Text(message.title.isEmpty ? String(localized: "Notification") : message.title)
                    .font(.system(size: 13, weight: message.isRead ? .regular : .semibold))
                    .lineLimit(1)

                Spacer(minLength: 8)

                if let date = TimeFormatting.string(from: message.createdAt) {
                    Text(date)
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                }
            }

            HStack(spacing: 8) {
                Text(MarkdownHelper.toSingleLineSummary(message.body))
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
                    .lineLimit(1)

                Spacer(minLength: 8)

                if let group = message.group,
                    !group.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
                {
                    Text(group)
                        .font(.system(size: 11, weight: .medium, design: .monospaced))
                        .foregroundStyle(Color.accentColor)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(Color.accentColor.opacity(0.1))
                        .clipShape(Capsule())
                }
            }
            .padding(.leading, 22)
        }
        .padding(.vertical, 4)
    }
}

// 通知管理窗口 (SwiftUI Window 场景) 的协调器：窗口本体在 TinkApp 中声明，
// 这里持有"当前分组"作为唯一数据源——视图与开窗调用方（状态栏菜单等）
// 都直接读写它，窗口已打开时改值即刻切换分组，未打开时开窗后即为目标分组。
@MainActor
public final class NotificationWindowCoordinator: ObservableObject {
    public static let shared = NotificationWindowCoordinator()
    public static let windowID = "notification-manager"

    @Published public var selectedGroup: NotificationGroup = .all

    private init() {}
}
