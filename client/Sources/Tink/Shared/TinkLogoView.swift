import AppKit
import SwiftUI

// 深浅模式各一张位图，跟随系统外观切换（与 Dock 图标同源设计）。
public struct TinkLogoView: View {
    public var size: CGFloat = 64
    public var showShadow: Bool = true
    public var theme: ColorScheme? = nil

    @Environment(\.colorScheme) private var environmentTheme

    private static func loadResource(_ name: String) -> NSImage {
        if let url = Bundle.main.url(forResource: name, withExtension: "png"),
           let image = NSImage(contentsOf: url) {
            return image
        }
        // swift run 未打包为 .app 时，从开发目录读取同一资源。
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Resources/\(name).png")
        return NSImage(contentsOf: sourceURL) ?? NSImage(named: NSImage.applicationIconName) ?? NSImage()
    }

    private static let darkLogo = loadResource("LogoDark")
    private static let lightLogo = loadResource("LogoLight")

    public init(size: CGFloat = 64, showShadow: Bool = true, theme: ColorScheme? = nil) {
        self.size = size
        self.showShadow = showShadow
        self.theme = theme
    }

    public var body: some View {
        let scheme = theme ?? environmentTheme
        Image(nsImage: scheme == .dark ? Self.darkLogo : Self.lightLogo)
            .resizable()
            .interpolation(.high)
            .scaledToFit()
            .frame(width: size, height: size)
            .shadow(color: showShadow ? Color.black.opacity(0.2) : .clear,
                    radius: size * 0.08, x: 0, y: size * 0.04)
            .accessibilityLabel("Tink")
    }
}


#Preview {
    TinkLogoView()
}
