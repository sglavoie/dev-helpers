// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "SloppyPaste",
    platforms: [.macOS(.v15)],
    products: [
        .library(name: "SloppyCore", targets: ["SloppyCore"]),
        .executable(name: "SloppyPaste", targets: ["SloppyPasteApp"]),
        .executable(name: "sloppyctl", targets: ["sloppyctl"]),
    ],
    targets: [
        .target(name: "SloppyCore"),
        .executableTarget(
            name: "SloppyPasteApp",
            dependencies: ["SloppyCore"]
        ),
        .executableTarget(
            name: "sloppyctl",
            dependencies: ["SloppyCore"]
        ),
        .testTarget(
            name: "SloppyCoreTests",
            dependencies: ["SloppyCore"],
            swiftSettings: [
                // The Command Line Tools keep the Swift Testing macro plugin in a
                // subdirectory that swift-build's explicit-module builds sometimes
                // miss, which fails with "plugin for module 'TestingMacros' not found".
                .unsafeFlags([
                    "-plugin-path",
                    "/Library/Developer/CommandLineTools/usr/lib/swift/host/plugins/testing",
                ])
            ]
        ),
    ],
    swiftLanguageModes: [.v6]
)
