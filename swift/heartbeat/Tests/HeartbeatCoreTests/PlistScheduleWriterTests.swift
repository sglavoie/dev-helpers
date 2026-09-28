import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct PlistScheduleWriterTests {
    static let header = """
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">

        """

    /// forgejo-sync as stowed: 4-space indents, StartInterval between ProgramArguments and RunAtLoad.
    static let spaces = header + """
        <dict>
            <key>Label</key>
            <string>com.sglavoie.forgejo-sync</string>
            <key>ProgramArguments</key>
            <array>
                <string>/Users/sglavoie/.local/bin/forgejo-sync.sh</string>
            </array>
            <key>StartInterval</key>
            <integer>900</integer>
            <key>RunAtLoad</key>
            <true/>
            <key>StandardOutPath</key>
            <string>/Users/sglavoie/Library/Logs/forgejo-sync.log</string>
        </dict>
        </plist>

        """

    /// Installer-written: tabs, sorted keys, a decoy StartInterval inside EnvironmentVariables.
    static let tabs = header + """
        <dict>
        \t<key>EnvironmentVariables</key>
        \t<dict>
        \t\t<key>StartInterval</key>
        \t\t<string>decoy</string>
        \t</dict>
        \t<key>KeepAlive</key>
        \t<true/>
        \t<key>Label</key>
        \t<string>com.sglavoie.asl-companion</string>
        \t<key>ProgramArguments</key>
        \t<array>
        \t\t<string>/opt/homebrew/bin/python3</string>
        \t</array>
        \t<key>ThrottleInterval</key>
        \t<integer>10</integer>
        </dict>
        </plist>

        """

    /// backup-legacy-repo: one-line calendar dicts.
    static let compact = header + """
        <dict>
            <key>Label</key>
            <string>com.sglavoie.backup-legacy-repo</string>
            <key>StartCalendarInterval</key>
            <array>
                <dict><key>Hour</key><integer>10</integer><key>Minute</key><integer>0</integer></dict>
                <dict><key>Hour</key><integer>13</integer><key>Minute</key><integer>0</integer></dict>
            </array>
            <key>RunAtLoad</key>
            <true/>
        </dict>
        </plist>

        """

    static func write(_ schedule: Schedule, throttle: ThrottleUpdate = .keep, to xml: String) throws -> String {
        String(decoding: try PlistScheduleWriter.apply(schedule, throttle: throttle, to: Data(xml.utf8)), as: UTF8.self)
    }

    static func parse(_ xml: String) throws -> AgentDefinition {
        try LaunchAgentPlistParser.parse(Data(xml.utf8), plistPath: "test.plist")
    }

    /// Lines only in `before`, and lines only in `after` (as multisets are overkill here, order-free sets).
    static func diff(_ before: String, _ after: String) -> (removed: Set<String>, added: Set<String>) {
        let old = Set(before.components(separatedBy: "\n")), new = Set(after.components(separatedBy: "\n"))
        return (old.subtracting(new), new.subtracting(old))
    }

    @Test func changingAnIntervalTouchesOnlyItsLine() throws {
        let output = try Self.write(.interval(seconds: 1800), to: Self.spaces)
        #expect(output == Self.spaces.replacingOccurrences(of: "<integer>900</integer>", with: "<integer>1800</integer>"))
    }

    @Test func intervalToCalendarKeepsPlaceAndIndent() throws {
        let entries = [CalendarEntry(minute: 0, hour: 9), CalendarEntry(minute: 30, hour: 17, weekday: 5)]
        let output = try Self.write(.calendar(entries), to: Self.spaces)
        #expect(output.contains("""
                <string>/Users/sglavoie/.local/bin/forgejo-sync.sh</string>
            </array>
            <key>StartCalendarInterval</key>
            <array>
                <dict>
                    <key>Hour</key>
                    <integer>9</integer>
                    <key>Minute</key>
                    <integer>0</integer>
                </dict>
                <dict>
                    <key>Weekday</key>
                    <integer>5</integer>
                    <key>Hour</key>
                    <integer>17</integer>
                    <key>Minute</key>
                    <integer>30</integer>
                </dict>
            </array>
            <key>RunAtLoad</key>
        """))
        #expect(!output.contains("StartInterval"))
        #expect(try Self.parse(output).schedule == .calendar(entries))
    }

    @Test func calendarKeepsTheFileStyle() throws {
        let entries = [CalendarEntry(minute: 0, hour: 11), CalendarEntry(minute: 30, hour: 14)]
        let output = try Self.write(.calendar(entries), to: Self.compact)
        #expect(Self.diff(Self.compact, output) == (
            ["        <dict><key>Hour</key><integer>10</integer><key>Minute</key><integer>0</integer></dict>",
             "        <dict><key>Hour</key><integer>13</integer><key>Minute</key><integer>0</integer></dict>"],
            ["        <dict><key>Hour</key><integer>11</integer><key>Minute</key><integer>0</integer></dict>",
             "        <dict><key>Hour</key><integer>14</integer><key>Minute</key><integer>30</integer></dict>"]))

        let bare = Self.compact.replacingOccurrences(of: """
                <array>
                    <dict><key>Hour</key><integer>10</integer><key>Minute</key><integer>0</integer></dict>
                    <dict><key>Hour</key><integer>13</integer><key>Minute</key><integer>0</integer></dict>
                </array>
            """, with: """
                <dict>
                    <key>Hour</key>
                    <integer>10</integer>
                </dict>
            """)
        let one = try Self.write(.calendar([CalendarEntry(hour: 12)]), to: bare)
        #expect(one == bare.replacingOccurrences(of: "<integer>10</integer>", with: "<integer>12</integer>"))
        let two = try Self.write(.calendar(entries), to: bare)
        #expect(two.contains("    <array>\n        <dict>\n            <key>Hour</key>"))
    }

    @Test func compactCalendarToInterval() throws {
        let output = try Self.write(.interval(seconds: 600), to: Self.compact)
        let (removed, added) = Self.diff(Self.compact, output)
        #expect(added == ["    <key>StartInterval</key>", "    <integer>600</integer>"])
        #expect(removed.allSatisfy { $0.contains("StartCalendarInterval") || $0.contains("<dict><key>Hour") || $0.contains("array>") })
        #expect(try Self.parse(output).schedule == .interval(seconds: 600))
    }

    @Test func tabsAndDecoyKeyInNestedDict() throws {
        let output = try Self.write(.interval(seconds: 300), to: Self.tabs)
        #expect(output.contains("\t\t<key>StartInterval</key>\n\t\t<string>decoy</string>"))
        #expect(output.contains("\t<integer>10</integer>\n\t<key>StartInterval</key>\n\t<integer>300</integer>\n</dict>"))
        #expect(try Self.parse(output).schedule == .interval(seconds: 300))
    }

    @Test func watchPathsWithNewThrottle() throws {
        let output = try Self.write(.watchPaths(["/tmp/a & b.md", "/tmp/c.md"], throttleSeconds: nil),
                                    throttle: .set(20), to: Self.spaces)
        #expect(output.contains("<string>/tmp/a &amp; b.md</string>"))
        #expect(output.contains("    <key>ThrottleInterval</key>\n    <integer>20</integer>\n    <key>RunAtLoad</key>"))
        let agent = try Self.parse(output)
        #expect(agent.schedule == .watchPaths(["/tmp/a & b.md", "/tmp/c.md"], throttleSeconds: 20))
    }

    @Test func throttleIsReplacedInPlaceOrRemoved() throws {
        let set = try Self.write(.none, throttle: .set(45), to: Self.tabs)
        #expect(set == Self.tabs.replacingOccurrences(of: "<integer>10</integer>", with: "<integer>45</integer>"))
        let removed = try Self.write(.none, throttle: .remove, to: Self.tabs)
        #expect(!removed.contains("ThrottleInterval"))
        #expect(try Self.parse(removed).throttleInterval == nil)
    }

    @Test func noneRemovesTheSchedule() throws {
        let output = try Self.write(.none, to: Self.spaces)
        #expect(Self.diff(Self.spaces, output) == (["    <key>StartInterval</key>", "    <integer>900</integer>"], []))
        #expect(try Self.parse(output).schedule == .none)
    }

    @Test func realFixturesRoundTrip() throws {
        let fixtures = [PlistFixtures.backupLegacyRepo, PlistFixtures.brainnotesVaultGuard, PlistFixtures.brewMaintain,
                        PlistFixtures.forgejoSync, PlistFixtures.logRotate, PlistFixtures.piBackupFetch]
        let schedules: [Schedule] = [.interval(seconds: 120), .calendar([CalendarEntry(minute: 5, hour: 4, day: 1)]),
                                     .watchPaths(["/tmp/x"], throttleSeconds: nil), .none]
        for fixture in fixtures {
            let before = try Self.parse(fixture)
            for schedule in schedules {
                let after = try Self.parse(try Self.write(schedule, to: fixture))
                if case .watchPaths(let paths, _) = schedule {
                    #expect(after.schedule == .watchPaths(paths, throttleSeconds: before.throttleInterval))
                } else {
                    #expect(after.schedule == schedule)
                }
                #expect(after.label == before.label)
                #expect(after.keepAlive == before.keepAlive)
                #expect(after.programArguments == before.programArguments)
            }
        }
    }

    @Test func queueDirectoriesAreKept() throws {
        let xml = Self.spaces.replacingOccurrences(of: "    <key>RunAtLoad</key>", with: """
                <key>QueueDirectories</key>
                <array>
                    <string>/tmp/queue</string>
                </array>
                <key>RunAtLoad</key>
            """)
        let output = try Self.write(.watchPaths(["/tmp/watch"], throttleSeconds: nil), to: xml)
        let agent = try Self.parse(output)
        #expect(agent.queueDirectories == ["/tmp/queue"])
        #expect(agent.schedule == .watchPaths(["/tmp/watch", "/tmp/queue"], throttleSeconds: nil))
    }

    @Test func binaryPlistsAreRefused() throws {
        let binary = try PropertyListSerialization.data(fromPropertyList: ["Label": "x", "StartInterval": 60],
                                                        format: .binary, options: 0)
        #expect(throws: PlistWriteError.unsupportedFormat) {
            try PlistScheduleWriter.apply(.interval(seconds: 30), to: binary)
        }
    }

    @Test func commentsAreSkipped() throws {
        let xml = Self.spaces.replacingOccurrences(of: "    <key>StartInterval</key>",
                                                   with: "    <!-- <key>Label</key> -->\n    <key>StartInterval</key>")
        let output = try Self.write(.interval(seconds: 60), to: xml)
        #expect(output.contains("<!-- <key>Label</key> -->"))
        #expect(try Self.parse(output).schedule == .interval(seconds: 60))
    }
}
