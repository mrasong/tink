import SwiftUI

// “关于 Tink”窗口场景标识，TinkApp 中以 Window 场景声明
public enum AboutWindow {
    public static let id = "about-tink"
}

// 关于 Tink 视图：宿主窗口为 .hiddenTitleBar 风格。
// 间距取自参考截图实测值，但 .padding 量的是文字行框（cap 上方还有 ≈5pt），故数值比视觉间距小 5-8pt。
public struct AboutView: View {
    public init() {}

    public var body: some View {
        VStack(spacing: 0) {
            TinkLogoView(size: 52, showShadow: false)
                .padding(.top, 46)

            Text("Tink")
                .font(.system(size: 15, weight: .bold, design: .rounded))
                .padding(.top, 13)

            Text(String(format: String(localized: "Version %@"), "\(AppVersion.short) (\(commit))"))
                .font(.system(size: 11))
                .foregroundColor(.secondary)
                .padding(.top, 6)

            HStack(spacing: 4) {
                Text("by mrasong with ❤️")

                Image(systemName: "cube")

                Link(destination: ClientConfiguration.githubURL) {
                    Text("GitHub")
                }
                .buttonStyle(.plain)
            }
            .font(.system(size: 11))
            .foregroundColor(.secondary)
            .padding(.vertical, 16)

        }
        // 窗口高度 = 这里的高 + 32（标题栏安全区会计入 .windowResizability(.contentSize) 的尺寸，
        // .ignoresSafeArea 只让内容铺到窗口顶端，不减少窗口高），故 206 得到 284×238。
        .frame(width: 284, height: 180, alignment: .top)
        .ignoresSafeArea()
        .background(WindowOpenerInstaller())
    }

    private var commit: String {
        Bundle.main.object(forInfoDictionaryKey: "TinkGitCommit") as? String ?? "Unknown"
    }
}
