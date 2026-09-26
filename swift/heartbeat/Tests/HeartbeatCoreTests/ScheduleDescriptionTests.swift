import Testing
@testable import HeartbeatCore

@Suite struct ScheduleDescriptionTests {
    @Test func intervals() {
        #expect(ScheduleDescription.describe(.interval(seconds: 900)) == "every 15 min")
        #expect(ScheduleDescription.describe(.interval(seconds: 45)) == "every 45 s")
        #expect(ScheduleDescription.describe(.interval(seconds: 90)) == "every 90 s")
        #expect(ScheduleDescription.describe(.interval(seconds: 5400)) == "every 1 h 30 min")
        #expect(ScheduleDescription.describe(.interval(seconds: 21600)) == "every 6 h")
        #expect(ScheduleDescription.describe(.interval(seconds: 86400)) == "every 1 d")
    }

    @Test func dailyTimesAreGrouped() {
        let entries = [CalendarEntry(minute: 0, hour: 10), CalendarEntry(minute: 30, hour: 21)]
        #expect(ScheduleDescription.describe(.calendar(entries)) == "daily 10:00, 21:30")
    }

    @Test func weekdaysIncludingSevenAsSunday() {
        #expect(ScheduleDescription.describe(.calendar([CalendarEntry(minute: 0, hour: 12, weekday: 3)])) == "Wed 12:00")
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 5, hour: 9, weekday: 7)) == "Sun 09:05")
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 5, hour: 9, weekday: 0)) == "Sun 09:05")
    }

    /// launchagent-status defaulted missing keys to 0 and printed these as "00:10" / "10:00".
    @Test func wildcardsAreNotZero() {
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 10)) == "hourly at :10")
        #expect(ScheduleDescription.describe(CalendarEntry(hour: 10)) == "every minute of 10:xx")
        #expect(ScheduleDescription.describe(CalendarEntry()) == "every minute")
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 0, weekday: 1)) == "Mon hourly at :00")
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 0, hour: 9)) == "daily 09:00")
    }

    @Test func dayMonthAndEitherSemantics() {
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 0, hour: 3, day: 1)) == "day 1 03:00")
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 0, hour: 3, day: 1, month: 12)) == "Dec day 1 03:00")
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 0, hour: 3, month: 1)) == "Jan 03:00")
        #expect(ScheduleDescription.describe(CalendarEntry(minute: 0, hour: 3, day: 15, weekday: 5)) == "day 15 or Fri 03:00")
        #expect(ScheduleDescription.describe(.calendar([])) == "never (empty calendar)")
    }

    @Test func watchPaths() {
        #expect(ScheduleDescription.describe(.watchPaths(["/a/b/one.md", "/a/two.md"], throttleSeconds: 30))
            == "on change: one.md +1 (throttle 30 s)")
        #expect(ScheduleDescription.describe(.watchPaths(["/x/y.env"], throttleSeconds: nil)) == "on change: y.env")
    }

    @Test func agentsWithoutSchedule() {
        func agent(_ keepAlive: KeepAlivePolicy, runAtLoad: Bool) -> AgentDefinition {
            AgentDefinition(label: "x", plistPath: "/x", runAtLoad: runAtLoad, keepAlive: keepAlive)
        }
        #expect(ScheduleDescription.describe(agent(.always, runAtLoad: true)) == "always running (KeepAlive)")
        #expect(ScheduleDescription.describe(agent(.conditional(successfulExit: false, otherKeys: []), runAtLoad: true))
            == "running, restarted on failure (KeepAlive)")
        #expect(ScheduleDescription.describe(agent(.none, runAtLoad: true)) == "at load only")
        #expect(ScheduleDescription.describe(agent(.none, runAtLoad: false)) == "on demand")
        let scheduled = AgentDefinition(label: "x", plistPath: "/x", schedule: .interval(seconds: 60), keepAlive: .always)
        #expect(ScheduleDescription.describe(scheduled) == "every 1 min")
    }
}
