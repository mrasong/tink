import AppKit

enum AppIcons {
    /// 菜单栏默认模板图标。打包为 .app 时资源位于 Contents/Resources；swift run 时回退到源码目录读取。
    static func menuBarTemplateImage() -> NSImage? {
        let url = Bundle.main.url(forResource: "MenuBarIcon", withExtension: "png")
            ?? URL(fileURLWithPath: #filePath)
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .appendingPathComponent("Resources/MenuBarIcon.png")
        guard let image = NSImage(contentsOf: url) else { return nil }
        image.isTemplate = true
        // 菜单栏尺寸以 points 为单位；NSImage 会按 Retina backing scale 渲染。
        image.size = NSSize(width: 18, height: 18)
        return image
    }
}
