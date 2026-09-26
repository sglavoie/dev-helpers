import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct LaunchAgentPlistParserTests {
    func parse(_ xml: String, path: String = "/tmp/agent.plist") throws -> AgentDefinition {
        try LaunchAgentPlistParser.parse(Data(xml.utf8), plistPath: path)
    }

    func parseError(_ xml: String) -> LaunchAgentPlistError? {
        do {
            _ = try parse(xml)
            return nil
        } catch {
            return error as? LaunchAgentPlistError
        }
    }

    @Test func startInterval() throws {
        let agent = try parse(PlistFixtures.forgejoSync)
        #expect(agent.label == "com.sglavoie.forgejo-sync")
        #expect(agent.schedule == .interval(seconds: 900))
        #expect(agent.runAtLoad)
        #expect(agent.keepAlive == .none)
        #expect(agent.processType == "Background")
        #expect(agent.executable == "/Users/sglavoie/.local/bin/forgejo-sync.sh")
        #expect(agent.logPaths == ["/Users/sglavoie/Library/Logs/forgejo-sync.log"])
        #expect(!agent.disabled)
    }

    @Test func calendarArray() throws {
        let agent = try parse(PlistFixtures.backupLegacyRepo)
        #expect(agent.schedule == .calendar([
            CalendarEntry(minute: 0, hour: 10),
            CalendarEntry(minute: 0, hour: 13),
            CalendarEntry(minute: 0, hour: 21),
        ]))
        #expect(agent.runAtLoad)
    }

    @Test func calendarArrayWithWeekday() throws {
        let agent = try parse(PlistFixtures.brewMaintain)
        #expect(agent.schedule == .calendar([CalendarEntry(minute: 0, hour: 12, weekday: 3)]))
        #expect(!agent.runAtLoad)
    }

    @Test func calendarBareDictionary() throws {
        let agent = try parse(PlistFixtures.logRotate)
        #expect(agent.schedule == .calendar([CalendarEntry(minute: 0, hour: 11, weekday: 1)]))
        #expect(agent.logPaths == ["/tmp/log-rotate.log"])
    }

    @Test func missingCalendarKeysAreWildcardsNotZero() throws {
        let agent = try parse(PlistFixtures.wrap("""
            <key>Label</key><string>x</string>
            <key>StartCalendarInterval</key><dict><key>Minute</key><integer>10</integer></dict>
            """))
        let entry = CalendarEntry(minute: 10)
        #expect(agent.schedule == .calendar([entry]))
        #expect(entry.hour == nil && entry.day == nil && entry.weekday == nil && entry.month == nil)
    }

    @Test func emptyCalendarDictionaryIsEveryMinute() throws {
        let agent = try parse(PlistFixtures.wrap("""
            <key>Label</key><string>x</string><key>StartCalendarInterval</key><dict/>
            """))
        #expect(agent.schedule == .calendar([CalendarEntry()]))
        #expect(CalendarEntry().isEveryMinute)
    }

    @Test func weekdaySevenIsSunday() throws {
        let agent = try parse(PlistFixtures.wrap("""
            <key>Label</key><string>x</string>
            <key>StartCalendarInterval</key>
            <array><dict><key>Weekday</key><integer>7</integer></dict><dict><key>Weekday</key><integer>0</integer></dict></array>
            """))
        guard case .calendar(let entries) = agent.schedule else {
            Issue.record("expected calendar, got \(agent.schedule)")
            return
        }
        #expect(entries.map(\.weekday) == [7, 0])
        #expect(entries.map(\.normalizedWeekday) == [0, 0])
    }

    @Test func outOfRangeCalendarValueIsInvalid() {
        let xml = PlistFixtures.wrap("""
            <key>Label</key><string>x</string>
            <key>StartCalendarInterval</key><dict><key>Hour</key><integer>24</integer></dict>
            """)
        #expect(parseError(xml) == .invalidValue(path: "/tmp/agent.plist", key: "StartCalendarInterval.Hour"))
    }

    @Test func watchPathsWithThrottle() throws {
        let agent = try parse(PlistFixtures.syncLegacy)
        guard case .watchPaths(let paths, let throttle) = agent.schedule else {
            Issue.record("expected watchPaths, got \(agent.schedule)")
            return
        }
        #expect(paths.count == 4)
        #expect(paths.first == "/Users/sglavoie/1_dev_projects/sglavoie_life-trail/PARA_MARIANA.md")
        #expect(throttle == 30)
        #expect(agent.throttleInterval == 30)
    }

    @Test func keepAliveTrue() throws {
        let agent = try parse(PlistFixtures.ddcBrightnessd)
        #expect(agent.keepAlive == .always)
        #expect(agent.keepAlive.expectsRunning)
        #expect(agent.schedule == .none)
        #expect(agent.throttleInterval == 10)
    }

    @Test func keepAliveSuccessfulExitFalse() throws {
        let agent = try parse(PlistFixtures.brainnotesVaultGuard)
        #expect(agent.keepAlive == .conditional(successfulExit: false, otherKeys: []))
        #expect(agent.keepAlive.expectsRunning)
    }

    @Test func keepAliveOtherConditionsDoNotExpectRunning() throws {
        let agent = try parse(PlistFixtures.wrap("""
            <key>Label</key><string>x</string>
            <key>KeepAlive</key><dict><key>SuccessfulExit</key><true/><key>NetworkState</key><true/></dict>
            """))
        #expect(agent.keepAlive == .conditional(successfulExit: true, otherKeys: ["NetworkState"]))
        #expect(!agent.keepAlive.expectsRunning)
    }

    @Test func keepAliveFalseIsNone() throws {
        let agent = try parse(PlistFixtures.wrap("<key>Label</key><string>x</string><key>KeepAlive</key><false/>"))
        #expect(agent.keepAlive == .none)
    }

    @Test func disabledTrueAndExplicitFalse() throws {
        let disabled = try parse(PlistFixtures.wrap("<key>Label</key><string>x</string><key>Disabled</key><true/>"))
        #expect(disabled.disabled)
        let enabled = try parse(PlistFixtures.brainnotesSyncthing)
        #expect(!enabled.disabled)
        #expect(enabled.keepAlive.expectsRunning)
        #expect(enabled.logPaths.count == 2)
    }

    @Test func installerManagedTabIndentedPlist() throws {
        let agent = try parse(PlistFixtures.piBackupFetch)
        #expect(agent.label == "com.sglavoie.pi-backup-fetch")
        #expect(!agent.runAtLoad)
        #expect(agent.schedule == .calendar([CalendarEntry(minute: 10, hour: 6), CalendarEntry(minute: 10, hour: 12)]))
        #expect(agent.programArguments.count == 3)
        #expect(agent.logPaths == [
            "/Users/sglavoie/Library/Application Support/pi-backup-fetch/runner.log",
            "/Users/sglavoie/Library/Application Support/pi-backup-fetch/runner.err",
        ])
        #expect(agent.workingDirectory == "/Users/sglavoie/Library/Application Support/pi-backup-fetch/runner")
    }

    @Test func devNullLogsAreNotEvidence() throws {
        let agent = try parse(PlistFixtures.aslCompanion)
        #expect(agent.keepAlive == .always)
        #expect(agent.logPaths.isEmpty)
    }

    @Test func binaryPlist() throws {
        let object = try PropertyListSerialization.propertyList(from: Data(PlistFixtures.logRotate.utf8), format: nil)
        let binary = try PropertyListSerialization.data(fromPropertyList: object, format: .binary, options: 0)
        #expect(binary.starts(with: Array("bplist".utf8)))
        let agent = try LaunchAgentPlistParser.parse(binary, plistPath: "/tmp/bin.plist")
        #expect(agent == (try parse(PlistFixtures.logRotate, path: "/tmp/bin.plist")))
    }

    @Test func programKeyIsExecutable() throws {
        let agent = try parse(PlistFixtures.minimal(label: "x"))
        #expect(agent.executable == "/bin/true")
        #expect(agent.schedule == .none)
    }

    @Test func missingLabel() {
        #expect(parseError(PlistFixtures.wrap("<key>Program</key><string>/bin/true</string>"))
            == .missingLabel(path: "/tmp/agent.plist"))
        #expect(parseError(PlistFixtures.wrap("<key>Label</key><string></string>")) == .missingLabel(path: "/tmp/agent.plist"))
    }

    @Test func garbageAndNonDictionary() {
        #expect(parseError("not a plist <<<") == .notAPropertyList(path: "/tmp/agent.plist"))
        let array = #"<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><array/></plist>"#
        #expect(parseError(array) == .notADictionary(path: "/tmp/agent.plist"))
    }

    @Test func integerAndBooleanAreNotInterchangeable() {
        #expect(parseError(PlistFixtures.wrap("<key>Label</key><string>x</string><key>RunAtLoad</key><integer>1</integer>"))
            == .invalidValue(path: "/tmp/agent.plist", key: "RunAtLoad"))
        #expect(parseError(PlistFixtures.wrap("<key>Label</key><string>x</string><key>StartInterval</key><true/>"))
            == .invalidValue(path: "/tmp/agent.plist", key: "StartInterval"))
    }

    @Test func parseFromFileKeepsDiscoveredPath() throws {
        let url = FileManager.default.temporaryDirectory.appending(path: "HeartbeatParser-\(UUID().uuidString).plist")
        defer { try? FileManager.default.removeItem(at: url) }
        try Data(PlistFixtures.forgejoSync.utf8).write(to: url)
        let agent = try LaunchAgentPlistParser.parse(contentsOf: url, plistPath: "/link.plist")
        #expect(agent.plistPath == "/link.plist")
        #expect(agent.resolvedPlistPath == url.path)
    }
}
