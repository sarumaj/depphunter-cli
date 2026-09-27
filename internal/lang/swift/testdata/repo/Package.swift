// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "Demo",
    dependencies: [
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.60.0"),
        .package(url: "https://github.com/apple/swift-log", .upToNextMajor(from: "1.5.0")),
        .package(url: "git@github.com:apple/swift-argument-parser.git", exact: "1.3.0"),
        .package(url: "https://github.com/apple/swift-collections", "1.0.0"..<"2.0.0"),
        .package(url: "https://github.com/swiftlang/swift-markdown.git", branch: "main"),
        // .package(url: "https://github.com/commented/out", from: "1.0.0"),
        .package(url: "https://github.com/acme/private-kit", revision: "0123456789abcdef0123456789abcdef01234567"),
        .package(path: "Local/LocalKit"),
        .package(id: "mona.LinkedList", from: "1.0.0"),
    ],
    targets: [
        .target(
            name: "Demo",
            dependencies: [
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "Logging", package: "swift-log"),
                .product(name: "Markdown", package: "swift-markdown"),
                "Util",
            ]
        ),
        .target(name: "Util", dependencies: [.product(name: "Collections", package: "swift-collections")]),
        .executableTarget(name: "demo-cli", dependencies: [.product(name: "ArgumentParser", package: "swift-argument-parser"), "Demo"]),
        .testTarget(name: "DemoTests", dependencies: ["Demo"]),
    ]
)
