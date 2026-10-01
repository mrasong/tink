import AppKit

enum AppIcons {
    static let defaultMenuBarSymbol = "bell.badge.fill"

    /// 菜单栏实际图标：无未读为 template 单色；有未读时角标染红。
    static func menuBarImage(symbolName: String, unread: Bool, appearance: NSAppearance) -> NSImage? {
        let config = NSImage.SymbolConfiguration(pointSize: 15, weight: .regular)
        guard let plain = NSImage(systemSymbolName: symbolName, accessibilityDescription: "Tink")?
            .withSymbolConfiguration(config) else { return nil }
        guard unread else {
            plain.isTemplate = true
            return plain
        }

        if symbolName.contains(".badge") {
            // palette 分层：layer1=角标染 systemRed，layer2=主体随深浅色自适应，几何由系统保证
            let palette = config.applying(
                NSImage.SymbolConfiguration(paletteColors: [.systemRed, .labelColor]))
            let image = NSImage(systemSymbolName: symbolName, accessibilityDescription: "Tink")?
                .withSymbolConfiguration(palette)
            image?.isTemplate = false
            return image ?? plain
        }

        // 无角标层的自定义符号：等比缩放居中后右上角叠加红点
        let canvas = NSSize(width: 18, height: 18)
        let image = NSImage(size: canvas)
        image.lockFocus()
        appearance.performAsCurrentDrawingAppearance {
            let s = plain.size
            let scale = min(canvas.width / s.width, canvas.height / s.height)
            let drawSize = NSSize(width: s.width * scale, height: s.height * scale)
            let rect = NSRect(
                x: (canvas.width - drawSize.width) / 2,
                y: (canvas.height - drawSize.height) / 2,
                width: drawSize.width, height: drawSize.height)
            plain.draw(in: rect, from: .zero, operation: .sourceOver, fraction: 1)
            NSColor.labelColor.set()
            rect.fill(using: .sourceAtop)
            NSColor.systemRed.set()
            NSBezierPath(ovalIn: NSRect(x: canvas.width - 7, y: canvas.height - 7, width: 6, height: 6)).fill()
        }
        image.unlockFocus()
        image.isTemplate = false
        return image
    }
}
