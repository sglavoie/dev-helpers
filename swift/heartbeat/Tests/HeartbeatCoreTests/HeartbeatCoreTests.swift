import Testing
@testable import HeartbeatCore

@Test func versionIsSet() {
    #expect(!HeartbeatCore.version.isEmpty)
}
