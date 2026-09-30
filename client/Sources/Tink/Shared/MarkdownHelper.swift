import Foundation
import Markdown

// Markdown 工具类：基于 Apple 官方 swift-markdown 提供纯文本剥离与结构化提取
public enum MarkdownHelper {
    
    // 纯文本提取器：用于系统通知 Banner、Alert 弹窗等空间有限且需要 1 秒扫视的场景
    // 自动剥离 #, **, *, `, ```, >, -, [title](url) 等排版符号，只保留纯文本内容
    public static func toPlainText(_ markdown: String) -> String {
        let clean = markdown.replacingOccurrences(of: "\\n", with: "\n")
        let document = Document(parsing: clean)
        var extractor = PlainTextExtractor()
        extractor.visit(document)
        let result = extractor.text.trimmingCharacters(in: .whitespacesAndNewlines)
        return result.isEmpty ? clean : result
    }

    // 单行/双行紧凑预览文本：用于通知卡片列表摘要
    public static func toSingleLineSummary(_ markdown: String) -> String {
        let plain = toPlainText(markdown)
        return plain
            .components(separatedBy: .newlines)
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
            .joined(separator: " ")
    }
}

// 内部 AST 遍历器：安全提取 Markdown 中的纯文本内容
private struct PlainTextExtractor: MarkupWalker {
    var text: String = ""

    mutating func defaultVisit(_ markup: Markup) {
        for child in markup.children {
            visit(child)
        }
    }

    mutating func visitText(_ text: Text) {
        self.text += text.string
    }

    mutating func visitInlineCode(_ inlineCode: InlineCode) {
        self.text += inlineCode.code
    }

    mutating func visitCodeBlock(_ codeBlock: CodeBlock) {
        if !text.isEmpty && !text.hasSuffix("\n") {
            text += "\n"
        }
        text += codeBlock.code
        if !text.hasSuffix("\n") {
            text += "\n"
        }
    }

    mutating func visitParagraph(_ paragraph: Paragraph) {
        for child in paragraph.children {
            visit(child)
        }
        text += "\n"
    }

    mutating func visitHeading(_ heading: Heading) {
        for child in heading.children {
            visit(child)
        }
        text += "\n"
    }

    mutating func visitBlockQuote(_ blockQuote: BlockQuote) {
        for child in blockQuote.children {
            visit(child)
        }
    }

    mutating func visitListItem(_ listItem: ListItem) {
        for child in listItem.children {
            visit(child)
        }
    }

    mutating func visitLink(_ link: Link) {
        for child in link.children {
            visit(child)
        }
    }

    mutating func visitSoftBreak(_ softBreak: SoftBreak) {
        text += " "
    }

    mutating func visitLineBreak(_ lineBreak: LineBreak) {
        text += "\n"
    }
}
