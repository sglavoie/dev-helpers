import Testing
@testable import SloppyCore

@Test func versionIsSet() {
    #expect(!SloppyCore.version.isEmpty)
}
