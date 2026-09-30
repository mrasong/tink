import AppKit
import Markdown
import SwiftUI

// 基于 Apple 官方 swift-markdown 解析的自适应富文本渲染组件
// 支持完整 GFM 语法：多级标题、图片(![alt](url))、代码块、引用块、无序/有序列表、任务列表、表格、行内样式（粗体、斜体、删除线、代码、链接）
public struct MarkdownContentView: View {
    public let content: String

    public init(_ content: String) {
        // 处理 JSON 或命令行参数传入时可能存在的字面量 "\n" 转义
        self.content = content.replacingOccurrences(of: "\\n", with: "\n")
    }

    public var body: some View {
        let document = Document(parsing: content)
        VStack(alignment: .leading, spacing: 14) {
            ForEach(Array(document.children.enumerated()), id: \.offset) { _, markup in
                MarkdownBlockView(markup: markup)
            }
        }
    }
}

// 独立的 Markdown 块级元素渲染器
struct MarkdownBlockView: View {
    let markup: Markup

    var body: some View {
        if let heading = markup as? Heading {
            HeadingView(heading: heading)
        } else if let paragraph = markup as? Paragraph {
            ParagraphView(paragraph: paragraph)
        } else if let codeBlock = markup as? CodeBlock {
            CodeBlockView(codeBlock: codeBlock)
        } else if let blockQuote = markup as? BlockQuote {
            BlockQuoteView(blockQuote: blockQuote)
        } else if let unorderedList = markup as? UnorderedList {
            UnorderedListView(list: unorderedList)
        } else if let orderedList = markup as? OrderedList {
            OrderedListView(list: orderedList)
        } else if markup is ThematicBreak {
            Divider()
                .padding(.vertical, 6)
        } else {
            InlineFlowView(inlines: Array(markup.children))
        }
    }
}

// 标题渲染
private struct HeadingView: View {
    let heading: Heading

    var body: some View {
        Text(inlineAttributedString(from: heading.children))
            .font(fontForLevel(heading.level))
            .fontWeight(.bold)
            .padding(.top, heading.level == 1 ? 6 : 3)
            .textSelection(.enabled)
    }

    private func fontForLevel(_ level: Int) -> Font {
        switch level {
        case 1: return .title
        case 2: return .title2
        case 3: return .title3
        case 4: return .headline
        default: return .subheadline
        }
    }
}

// 段落渲染：如果包含图片则按图文混合拆分流渲染，否则直接渲染为 Text
private struct ParagraphView: View {
    let paragraph: Paragraph

    var body: some View {
        let children = Array(paragraph.children)
        let containsImage = children.contains { $0 is Markdown.Image }

        if containsImage {
            InlineFlowView(inlines: children)
        } else {
            Text(inlineAttributedString(from: children))
                .font(.body)
                .lineSpacing(4)
                .textSelection(.enabled)
        }
    }
}

// 图文混排流：支持将行内普通文本与独立的 Markdown.Image 混合呈现
private struct InlineFlowView: View {
    let inlines: [Markup]

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            ForEach(Array(groupInlines().enumerated()), id: \.offset) { _, group in
                switch group {
                case .text(let items):
                    Text(inlineAttributedString(from: items))
                        .font(.body)
                        .lineSpacing(4)
                        .textSelection(.enabled)
                case .image(let img):
                    MarkdownImageView(image: img)
                }
            }
        }
    }

    private enum InlineGroup {
        case text([Markup])
        case image(Markdown.Image)
    }

    private func groupInlines() -> [InlineGroup] {
        var groups: [InlineGroup] = []
        var currentText: [Markup] = []

        for item in inlines {
            if let img = item as? Markdown.Image {
                if !currentText.isEmpty {
                    groups.append(.text(currentText))
                    currentText.removeAll()
                }
                groups.append(.image(img))
            } else {
                currentText.append(item)
            }
        }
        if !currentText.isEmpty {
            groups.append(.text(currentText))
        }
        return groups
    }
}

// 图片预览窗口 (SwiftUI WindowGroup 场景) 的标识，TinkApp 中按 String 值打开
enum ImagePreviewWindow {
    static let id = "image-preview"
}

// 图片渲染组件：支持网络图片 (http/https) 与本地文件 (file:/// 或本地绝对路径)
private struct MarkdownImageView: View {
    let image: Markdown.Image
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        VStack(alignment: .center, spacing: 4) {
            if let source = image.source {
                if source.hasPrefix("data:image/") {
                    // Base64 Data URL
                    if let data = dataFromDataURL(source), let nsImg = NSImage(data: data) {
                        imageButton(source: source) {
                            Image(nsImage: nsImg)
                                .resizable()
                                .aspectRatio(contentMode: .fit)
                        }
                    } else {
                        fallbackPlaceholder(
                            dest: source, error: String(localized: "Cannot load base64 image"))
                    }
                } else if let url = URL(string: source) {
                    if url.scheme == "file" || (url.scheme == nil && source.hasPrefix("/")) {
                        // 本地图片文件
                        let localPath = localFilePath(from: source, url: url)
                        if let nsImg = NSImage(contentsOfFile: localPath) {
                            imageButton(source: source) {
                                Image(nsImage: nsImg)
                                    .resizable()
                                    .aspectRatio(contentMode: .fit)
                            }
                        } else {
                            fallbackPlaceholder(
                                dest: source, error: String(localized: "Cannot load local image"))
                        }
                    } else if url.scheme == "http" || url.scheme == "https" {
                        // 网络异步图片
                        AsyncImage(url: url) { phase in
                            switch phase {
                            case .empty:
                                HStack(spacing: 8) {
                                    ProgressView()
                                        .controlSize(.small)
                                    Text("Loading image...")
                                        .font(.caption)
                                        .foregroundColor(.secondary)
                                }
                                .frame(height: 60)
                                .padding(.horizontal, 12)
                                .background(Color(NSColor.controlBackgroundColor))
                                .cornerRadius(6)

                            case .success(let image):
                                imageButton(source: source) {
                                    image
                                        .resizable()
                                        .aspectRatio(contentMode: .fit)
                                }

                            case .failure:
                                fallbackPlaceholder(
                                    dest: source, error: String(localized: "Failed to load image"))

                            @unknown default:
                                EmptyView()
                            }
                        }
                    } else {
                        fallbackPlaceholder(
                            dest: source, error: String(localized: "Unsupported URL scheme"))
                    }
                } else {
                    fallbackPlaceholder(
                        dest: source, error: String(localized: "Invalid URL"))
                }
            } else {
                fallbackPlaceholder(
                    dest: "", error: String(localized: "Invalid URL"))
            }

            // 图片标题 / Alt 文本
            let altText = image.plainText.trimmingCharacters(in: .whitespacesAndNewlines)
            if !altText.isEmpty {
                Text(altText)
                    .font(.caption)
                    .foregroundColor(.secondary)
                    .italic()
                    .frame(maxWidth: 960)
                    .multilineTextAlignment(.center)
            }
        }
        .padding(.vertical, 4)
    }

    private func imageButton<Content: View>(source: String, @ViewBuilder content: () -> Content)
        -> some View
    {
        Button {
            openWindow(id: ImagePreviewWindow.id, value: source)
        } label: {
            HStack {
                Spacer(minLength: 0)
                content()
                    .frame(maxWidth: 960)
                    .cornerRadius(8)
                    .shadow(color: Color.black.opacity(0.08), radius: 3, x: 0, y: 1)
                Spacer(minLength: 0)
            }
        }
        .buttonStyle(.plain)
        .frame(maxWidth: .infinity)
        .help(String(localized: "Open Image"))
        .accessibilityLabel(String(localized: "Open Image"))
    }

    private func fallbackPlaceholder(dest: String, error: String) -> some View {
        HStack(spacing: 6) {
            Image(systemName: "photo.badge.exclamationmark")
                .foregroundColor(.orange)
            Text("\(error): \(dest)")
                .font(.caption)
                .foregroundColor(.secondary)
        }
        .padding(8)
        .background(Color.orange.opacity(0.08))
        .cornerRadius(6)
    }

    // dataFromDataURL 解析 data:image/...;base64,... 格式的 Data
    private func dataFromDataURL(_ source: String) -> Data? {
        guard let commaIndex = source.firstIndex(of: ",") else { return nil }
        let base64String = String(source[source.index(after: commaIndex)...])
        return Data(base64Encoded: base64String, options: [.ignoreUnknownCharacters])
    }

    // localFilePath 从 file:// URL 或本地路径中解析出标准文件系统绝对路径
    private func localFilePath(from source: String, url: URL) -> String {
        if url.scheme == "file" {
            let path = url.path
            return path.removingPercentEncoding ?? path
        }
        return source.removingPercentEncoding ?? source
    }
}

struct ImagePreviewView: View {
    let source: String

    var body: some View {
        Group {
            if source.hasPrefix("data:image/") {
                if let data = dataFromDataURL(source), let image = NSImage(data: data) {
                    Image(nsImage: image)
                        .resizable()
                        .scaledToFit()
                } else {
                    ContentUnavailableView("Image Unavailable", systemImage: "photo")
                }
            } else if let url = URL(string: source), url.scheme == "http" || url.scheme == "https" {
                AsyncImage(url: url) { phase in
                    switch phase {
                    case .empty:
                        ProgressView()
                    case .success(let image):
                        image
                            .resizable()
                            .scaledToFit()
                    case .failure:
                        ContentUnavailableView("Image Unavailable", systemImage: "photo")
                    @unknown default:
                        EmptyView()
                    }
                }
            } else {
                let path = urlPath(for: source)
                if let image = NSImage(contentsOfFile: path) {
                    Image(nsImage: image)
                        .resizable()
                        .scaledToFit()
                } else {
                    ContentUnavailableView("Image Unavailable", systemImage: "photo")
                }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .frame(minWidth: 420, minHeight: 320)
        .padding(16)
        .background(Color(NSColor.windowBackgroundColor))
    }

    // urlPath 从本地路径或 file:// URI 中提取真实文件路径（处理转义字符）
    private func urlPath(for source: String) -> String {
        guard let url = URL(string: source), url.scheme == "file" else {
            return source.removingPercentEncoding ?? source
        }
        let path = url.path
        return path.removingPercentEncoding ?? path
    }

    // dataFromDataURL 解析 data:image/...;base64,... 格式的 Data
    private func dataFromDataURL(_ source: String) -> Data? {
        guard let commaIndex = source.firstIndex(of: ",") else { return nil }
        let base64String = String(source[source.index(after: commaIndex)...])
        return Data(base64Encoded: base64String, options: [.ignoreUnknownCharacters])
    }
}

// 代码块渲染（带语言标记、背景框、横向滚动）
private struct CodeBlockView: View {
    let codeBlock: CodeBlock

    var body: some View {
        VStack(alignment: .trailing, spacing: 4) {
            if let lang = codeBlock.language, !lang.isEmpty {
                Text(lang.uppercased())
                    .font(.caption2)
                    .fontWeight(.semibold)
                    .foregroundColor(.secondary)
            }
            ScrollView(.horizontal, showsIndicators: false) {
                Text(codeBlock.code)
                    .font(.system(.body, design: .monospaced))
                    .padding(10)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .background(Color(NSColor.textBackgroundColor).opacity(0.6))
            .cornerRadius(8)
            .overlay(
                RoundedRectangle(cornerRadius: 8)
                    .stroke(Color.primary.opacity(0.1), lineWidth: 1)
            )
        }
        .textSelection(.enabled)
    }
}

// 引用块渲染
private struct BlockQuoteView: View {
    let blockQuote: BlockQuote

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Rectangle()
                .fill(Color.accentColor.opacity(0.7))
                .frame(width: 3)

            VStack(alignment: .leading, spacing: 8) {
                ForEach(Array(blockQuote.children.enumerated()), id: \.offset) { _, child in
                    MarkdownBlockView(markup: child)
                }
            }
        }
        .padding(.vertical, 2)
        .padding(.leading, 2)
    }
}

// 无序列表
private struct UnorderedListView: View {
    let list: UnorderedList

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            ForEach(Array(list.listItems.enumerated()), id: \.offset) { _, item in
                HStack(alignment: .top, spacing: 6) {
                    if let checkbox = item.checkbox {
                        Image(systemName: checkbox == .checked ? "checkmark.square.fill" : "square")
                            .font(.system(size: 12))
                            .foregroundColor(checkbox == .checked ? .accentColor : .secondary)
                    } else {
                        Text("•")
                            .foregroundColor(.accentColor)
                            .fontWeight(.bold)
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        ForEach(Array(item.children.enumerated()), id: \.offset) { _, child in
                            MarkdownBlockView(markup: child)
                        }
                    }
                }
                .padding(.leading, 4)
            }
        }
    }
}

// 有序列表
private struct OrderedListView: View {
    let list: OrderedList

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            ForEach(Array(list.listItems.enumerated()), id: \.offset) { index, item in
                HStack(alignment: .top, spacing: 6) {
                    Text("\(index + 1).")
                        .font(.system(size: 12, weight: .semibold, design: .monospaced))
                        .foregroundColor(.secondary)

                    VStack(alignment: .leading, spacing: 4) {
                        ForEach(Array(item.children.enumerated()), id: \.offset) { _, child in
                            MarkdownBlockView(markup: child)
                        }
                    }
                }
                .padding(.leading, 4)
            }
        }
    }
}

// 行内 AttributedString 渲染：遍历子节点将 Inline Markup 正确转换为带样式的 AttributedString
private func inlineAttributedString<S: Sequence>(from inlines: S) -> AttributedString
where S.Element == Markup {
    var result = AttributedString()
    for item in inlines {
        result.append(renderInline(item))
    }
    return result
}

private func renderInline(_ markup: Markup) -> AttributedString {
    if let text = markup as? Markdown.Text {
        return AttributedString(text.string)
    } else if let inlineCode = markup as? InlineCode {
        var attr = AttributedString(inlineCode.code)
        attr.font = .system(.body, design: .monospaced)
        attr.backgroundColor = Color(NSColor.textBackgroundColor).opacity(0.6)
        return attr
    } else if let strong = markup as? Strong {
        var attr = AttributedString()
        for child in strong.children {
            attr.append(renderInline(child))
        }
        attr.font = .system(.body).bold()
        return attr
    } else if let emphasis = markup as? Emphasis {
        var attr = AttributedString()
        for child in emphasis.children {
            attr.append(renderInline(child))
        }
        attr.font = .system(.body).italic()
        return attr
    } else if let strikethrough = markup as? Strikethrough {
        var attr = AttributedString()
        for child in strikethrough.children {
            attr.append(renderInline(child))
        }
        attr.strikethroughStyle = .single
        return attr
    } else if let link = markup as? Markdown.Link {
        var attr = AttributedString()
        for child in link.children {
            attr.append(renderInline(child))
        }
        if let dest = link.destination, let url = URL(string: dest) {
            attr.link = url
            attr.foregroundColor = .accentColor
            attr.underlineStyle = .single
        }
        return attr
    } else if let image = markup as? Markdown.Image {
        // 如果在富文本中遇到孤立图片，返回其替代文本
        return AttributedString(image.plainText.isEmpty ? "[Image]" : "[\(image.plainText)]")
    } else if markup is SoftBreak {
        return AttributedString(" ")
    } else if markup is LineBreak {
        return AttributedString("\n")
    } else {
        var attr = AttributedString()
        for child in markup.children {
            attr.append(renderInline(child))
        }
        return attr
    }
}
