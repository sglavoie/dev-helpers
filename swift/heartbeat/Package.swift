// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "Heartbeat",
    platforms: [.macOS(.v15)],
    products: [
        .library(name: "HeartbeatCore", targets: ["HeartbeatCore"]),
        .executable(name: "Heartbeat", targets: ["HeartbeatApp"]),
        .executable(name: "heartbeatctl", targets: ["heartbeatctl"]),
    ],
    targets: [
        .target(name: "HeartbeatCore"),
        .executableTarget(
            name: "HeartbeatApp",
            dependencies: ["HeartbeatCore"]
        ),
        .executableTarget(
            name: "heartbeatctl",
            dependencies: ["HeartbeatCore"]
        ),
        .testTarget(
            name: "HeartbeatCoreTests",
            dependencies: ["HeartbeatCore"],
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
        .testTarget(
            name: "HeartbeatAppTests",
            dependencies: ["HeartbeatApp", "HeartbeatCore"],
            swiftSettings: [
                .unsafeFlags([
                    "-plugin-path",
                    "/Library/Developer/CommandLineTools/usr/lib/swift/host/plugins/testing",
                ])
            ]
        ),
    ],
    swiftLanguageModes: [.v6]
)
