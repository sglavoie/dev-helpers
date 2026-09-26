import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct AgentDiscoveryTests {
    let root: URL
    let launchAgents: URL
    let stow: URL

    init() throws {
        root = FileManager.default.temporaryDirectory
            .appending(path: "HeartbeatDiscovery-\(UUID().uuidString)", directoryHint: .isDirectory)
        launchAgents = root.appending(path: "Library/LaunchAgents", directoryHint: .isDirectory)
        stow = root.appending(path: "scripts/launchagents/Library/LaunchAgents", directoryHint: .isDirectory)
        try FileManager.default.createDirectory(at: launchAgents, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: stow, withIntermediateDirectories: true)
    }

    func cleanUp() {
        try? FileManager.default.removeItem(at: root)
    }

    func write(_ xml: String, to url: URL) throws {
        try Data(xml.utf8).write(to: url)
    }

    func discover() throws -> DiscoveryResult {
        try AgentDiscovery(directory: launchAgents, labelPrefix: "com.sglavoie.").discover()
    }

    @Test func regularFilesAreDiscoveredSortedByLabel() throws {
        defer { cleanUp() }
        try write(PlistFixtures.logRotate, to: launchAgents.appending(path: "com.sglavoie.log-rotate.plist"))
        try write(PlistFixtures.forgejoSync, to: launchAgents.appending(path: "com.sglavoie.forgejo-sync.plist"))

        let result = try discover()

        #expect(result.agents.map(\.label) == ["com.sglavoie.forgejo-sync", "com.sglavoie.log-rotate"])
        #expect(result.problems.isEmpty)
        let agent = result.agents[0]
        #expect(agent.plistPath == agent.resolvedPlistPath)
        #expect(agent.plistPath == launchAgents.appending(path: "com.sglavoie.forgejo-sync.plist").path)
    }

    @Test func prefixFilterSkipsOtherVendorsAndNonPlists() throws {
        defer { cleanUp() }
        try write(PlistFixtures.forgejoSync, to: launchAgents.appending(path: "com.sglavoie.forgejo-sync.plist"))
        try write(PlistFixtures.minimal(label: "com.setapp.DesktopClient.SetappAgent"),
                  to: launchAgents.appending(path: "com.setapp.DesktopClient.SetappAgent.plist"))
        try write("ignored", to: launchAgents.appending(path: "com.sglavoie.notes.txt"))
        try FileManager.default.createDirectory(at: launchAgents.appending(path: "com.sglavoie.dir.plist"),
                                                withIntermediateDirectories: true)

        let result = try discover()

        #expect(result.agents.map(\.label) == ["com.sglavoie.forgejo-sync"])
        #expect(result.problems.isEmpty)
    }

    @Test func relativeStowSymlinkIsResolved() throws {
        defer { cleanUp() }
        let target = stow.appending(path: "com.sglavoie.sync-legacy.plist")
        try write(PlistFixtures.syncLegacy, to: target)
        let link = launchAgents.appending(path: "com.sglavoie.sync-legacy.plist")
        try FileManager.default.createSymbolicLink(
            atPath: link.path,
            withDestinationPath: "../../scripts/launchagents/Library/LaunchAgents/com.sglavoie.sync-legacy.plist")

        let result = try discover()

        #expect(result.problems.isEmpty)
        let agent = try #require(result.agents.first)
        #expect(agent.label == "com.sglavoie.sync-legacy")
        #expect(agent.plistPath == link.path)
        #expect(agent.resolvedPlistPath == target.path)
    }

    @Test func symlinkChainIsFollowed() throws {
        defer { cleanUp() }
        let target = stow.appending(path: "real.plist")
        try write(PlistFixtures.forgejoSync, to: target)
        let middle = root.appending(path: "middle.plist")
        try FileManager.default.createSymbolicLink(atPath: middle.path, withDestinationPath: target.path)
        try FileManager.default.createSymbolicLink(
            atPath: launchAgents.appending(path: "com.sglavoie.forgejo-sync.plist").path,
            withDestinationPath: middle.path)

        let agent = try #require(try discover().agents.first)

        #expect(agent.resolvedPlistPath == target.path)
    }

    @Test func brokenSymlinkIsReportedAndOthersStillLoad() throws {
        defer { cleanUp() }
        try write(PlistFixtures.forgejoSync, to: launchAgents.appending(path: "com.sglavoie.forgejo-sync.plist"))
        let link = launchAgents.appending(path: "com.sglavoie.gone.plist")
        try FileManager.default.createSymbolicLink(atPath: link.path, withDestinationPath: "../../scripts/missing.plist")

        let result = try discover()

        #expect(result.agents.map(\.label) == ["com.sglavoie.forgejo-sync"])
        #expect(result.problems == [.brokenSymlink(path: link.path, destination: "../../scripts/missing.plist")])
    }

    @Test func symlinkLoopIsReportedAsBroken() throws {
        defer { cleanUp() }
        let a = launchAgents.appending(path: "com.sglavoie.a.plist")
        let b = launchAgents.appending(path: "com.sglavoie.b.plist")
        try FileManager.default.createSymbolicLink(atPath: a.path, withDestinationPath: b.path)
        try FileManager.default.createSymbolicLink(atPath: b.path, withDestinationPath: a.path)

        let result = try discover()

        #expect(result.agents.isEmpty)
        #expect(result.problems.map(\.path) == [a.path, b.path])
    }

    @Test func invalidPlistIsReported() throws {
        defer { cleanUp() }
        let bad = launchAgents.appending(path: "com.sglavoie.bad.plist")
        try write("not a plist <<<", to: bad)
        try write(PlistFixtures.wrap("<key>Program</key><string>/bin/true</string>"),
                  to: launchAgents.appending(path: "com.sglavoie.nolabel.plist"))

        let result = try discover()

        #expect(result.agents.isEmpty)
        #expect(result.problems.count == 2)
        #expect(result.problems.first == .invalidPlist(path: bad.path, message: "\(bad.path): not a property list"))
    }

    @Test func missingDirectoryThrows() {
        defer { cleanUp() }
        let discovery = AgentDiscovery(directory: root.appending(path: "nope"), labelPrefix: "com.sglavoie.")
        #expect(throws: (any Error).self) { try discovery.discover() }
    }

    @Test func defaultsPointAtUserLaunchAgents() {
        #expect(AgentDiscovery.defaultDirectory.path.hasSuffix("/Library/LaunchAgents"))
        #expect(AgentDiscovery.defaultLabelPrefix == "com.sglavoie.")
    }
}
