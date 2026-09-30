import AppKit
import SwiftUI

// 与应用程序图标共用同一个 ICNS 资源。
public struct TinkLogoView: View {
    public var size: CGFloat = 64
    public var showShadow: Bool = true
    public var theme: ColorScheme? = nil

    private static let appIcon: NSImage = {
        if let url = Bundle.main.url(forResource: "Tink", withExtension: "icns"),
           let image = NSImage(contentsOf: url) {
            return image
        }

        // swift run 未打包为 .app 时，从开发目录读取同一资源。
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Resources/Tink.icns")
        return NSImage(contentsOf: sourceURL) ?? NSImage(named: NSImage.applicationIconName) ?? NSImage()
    }()

    public init(size: CGFloat = 64, showShadow: Bool = true, theme: ColorScheme? = nil) {
        self.size = size
        self.showShadow = showShadow
        self.theme = theme
    }

    public var body: some View {
        Image(nsImage: Self.appIcon)
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
