// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "Tink",
    platforms: [
        .macOS(.v15)  // 支持 macOS 15+ 及以上更高版本
    ],
    products: [
        .executable(name: "Tink", targets: ["Tink"])
    ],
    dependencies: [
        .package(url: "https://github.com/swiftlang/swift-markdown.git", from: "0.4.0")
    ],
    targets: [
        .executableTarget(
            name: "Tink",
            dependencies: [
                .product(name: "Markdown", package: "swift-markdown")
            ],
            resources: [
                .process("Resources")
            ]
        ),
        .testTarget(
            name: "TinkTests",
            dependencies: ["Tink"],
            path: "Tests/TinkTests"
        ),
    ]
)
