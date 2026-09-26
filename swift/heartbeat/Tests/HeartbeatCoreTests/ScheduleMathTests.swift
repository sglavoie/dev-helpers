import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct ScheduleMathTests {
    static let calendar = TimeFixtures.montreal
    static func local(_ year: Int, _ month: Int, _ day: Int, _ hour: Int, _ minute: Int, _ second: Int = 0) -> Date {
        TimeFixtures.local(year, month, day, hour, minute, second)
    }
    static func utc(_ text: String) -> Date { TimeFixtures.utc(text) }

    struct Case: CustomTestStringConvertible, Sendable {
        var name: String
        var entries: [CalendarEntry]
        var now: Date
        var latest: Date?
        var next: Date?
        var testDescription: String { name }
    }

    // 2026-09-26 is a Saturday. Montreal DST: 2026-03-08 02:00 EST -> 03:00 EDT; 2026-11-01 02:00 EDT -> 01:00 EST.
    static let cases: [Case] = [
        Case(name: "daily, later the same day", entries: [CalendarEntry(minute: 0, hour: 10)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 9, 26, 10, 0), next: local(2026, 9, 27, 10, 0)),
        Case(name: "daily, just before the slot", entries: [CalendarEntry(minute: 0, hour: 10)],
             now: local(2026, 9, 26, 9, 59), latest: local(2026, 9, 25, 10, 0), next: local(2026, 9, 26, 10, 0)),
        Case(name: "exactly on the slot is the latest, not the next", entries: [CalendarEntry(minute: 0, hour: 10)],
             now: local(2026, 9, 26, 10, 0), latest: local(2026, 9, 26, 10, 0), next: local(2026, 9, 27, 10, 0)),
        Case(name: "several entries: max past, min future",
             entries: [CalendarEntry(minute: 0, hour: 10), CalendarEntry(minute: 0, hour: 13), CalendarEntry(minute: 30, hour: 21)],
             now: local(2026, 9, 26, 14, 0), latest: local(2026, 9, 26, 13, 0), next: local(2026, 9, 26, 21, 30)),
        Case(name: "wildcard hour: hourly at :10", entries: [CalendarEntry(minute: 10)],
             now: local(2026, 9, 26, 12, 5), latest: local(2026, 9, 26, 11, 10), next: local(2026, 9, 26, 12, 10)),
        Case(name: "wildcard minute: every minute of 10:xx", entries: [CalendarEntry(hour: 10)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 9, 26, 10, 59), next: local(2026, 9, 27, 10, 0)),
        Case(name: "wildcard minute inside the hour", entries: [CalendarEntry(hour: 10)],
             now: local(2026, 9, 26, 10, 30, 20), latest: local(2026, 9, 26, 10, 30), next: local(2026, 9, 26, 10, 31)),
        Case(name: "all wildcards: every minute", entries: [CalendarEntry()],
             now: local(2026, 9, 26, 12, 0, 30), latest: local(2026, 9, 26, 12, 0), next: local(2026, 9, 26, 12, 1)),
        Case(name: "Weekday 7 is Sunday", entries: [CalendarEntry(minute: 5, hour: 9, weekday: 7)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 9, 20, 9, 5), next: local(2026, 9, 27, 9, 5)),
        Case(name: "Weekday 0 is Sunday", entries: [CalendarEntry(minute: 5, hour: 9, weekday: 0)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 9, 20, 9, 5), next: local(2026, 9, 27, 9, 5)),
        Case(name: "Weekday 3 is Wednesday", entries: [CalendarEntry(minute: 0, hour: 12, weekday: 3)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 9, 23, 12, 0), next: local(2026, 9, 30, 12, 0)),
        Case(name: "Day and Weekday: either matches", entries: [CalendarEntry(minute: 0, hour: 8, day: 1, weekday: 1)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 9, 21, 8, 0), next: local(2026, 9, 28, 8, 0)),
        Case(name: "Day of month", entries: [CalendarEntry(minute: 0, hour: 8, day: 1)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 9, 1, 8, 0), next: local(2026, 10, 1, 8, 0)),
        Case(name: "Day 31 skips short months", entries: [CalendarEntry(minute: 0, hour: 0, day: 31)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 8, 31, 0, 0), next: local(2026, 10, 31, 0, 0)),
        Case(name: "Feb 29 only in leap years", entries: [CalendarEntry(minute: 0, hour: 0, day: 29, month: 2)],
             now: local(2026, 9, 26, 12, 0), latest: local(2024, 2, 29, 0, 0), next: local(2028, 2, 29, 0, 0)),
        Case(name: "Feb 30 never happens", entries: [CalendarEntry(minute: 0, hour: 0, day: 30, month: 2)],
             now: local(2026, 9, 26, 12, 0), latest: nil, next: nil),
        Case(name: "Month with wildcard day", entries: [CalendarEntry(minute: 0, hour: 6, month: 1)],
             now: local(2026, 9, 26, 12, 0), latest: local(2026, 1, 31, 6, 0), next: local(2027, 1, 1, 6, 0)),
        Case(name: "empty calendar", entries: [],
             now: local(2026, 9, 26, 12, 0), latest: nil, next: nil),

        // Spring forward: 02:30 doesn't exist on 2026-03-08, so that day has no 02:30 slot.
        Case(name: "DST spring: nonexistent 02:30 is skipped (latest)", entries: [CalendarEntry(minute: 30, hour: 2)],
             now: local(2026, 3, 8, 4, 0), latest: utc("2026-03-07T07:30:00Z"), next: utc("2026-03-09T06:30:00Z")),
        Case(name: "DST spring: nonexistent 02:30 is skipped (next)", entries: [CalendarEntry(minute: 30, hour: 2)],
             now: utc("2026-03-08T06:59:00Z"), latest: utc("2026-03-07T07:30:00Z"), next: utc("2026-03-09T06:30:00Z")),
        Case(name: "DST spring: hourly jumps the gap", entries: [CalendarEntry(minute: 30)],
             now: utc("2026-03-08T07:10:00Z"), latest: utc("2026-03-08T06:30:00Z"), next: utc("2026-03-08T07:30:00Z")),
        Case(name: "DST spring: daily 10:00 is 23 h after the day before", entries: [CalendarEntry(minute: 0, hour: 10)],
             now: local(2026, 3, 8, 11, 0), latest: utc("2026-03-08T14:00:00Z"), next: utc("2026-03-09T14:00:00Z")),

        // Fall back: 01:30 happens at 05:30Z (EDT) and again at 06:30Z (EST); only the first is a slot.
        Case(name: "DST fall: repeated 01:30 is its first occurrence", entries: [CalendarEntry(minute: 30, hour: 1)],
             now: utc("2026-11-01T06:45:00Z"), latest: utc("2026-11-01T05:30:00Z"), next: utc("2026-11-02T06:30:00Z")),
        Case(name: "DST fall: next from before the repeat", entries: [CalendarEntry(minute: 30, hour: 1)],
             now: utc("2026-11-01T04:00:00Z"), latest: utc("2026-10-31T05:30:00Z"), next: utc("2026-11-01T05:30:00Z")),
        Case(name: "DST fall: daily 10:00 is 25 h after the day before", entries: [CalendarEntry(minute: 0, hour: 10)],
             now: local(2026, 11, 1, 11, 0), latest: utc("2026-11-01T15:00:00Z"), next: utc("2026-11-02T15:00:00Z")),
    ]

    @Test(arguments: cases)
    func slots(_ c: Case) {
        #expect(ScheduleMath.latestSlot(c.entries, atOrBefore: c.now, calendar: Self.calendar) == c.latest)
        #expect(ScheduleMath.nextSlot(c.entries, after: c.now, calendar: Self.calendar) == c.next)
    }

    @Test func dstDaysHaveTheExpectedLength() {
        let entry = [CalendarEntry(minute: 0, hour: 10)]
        let springDay = ScheduleMath.latestSlot(entry, atOrBefore: Self.local(2026, 3, 8, 11, 0), calendar: Self.calendar)!
        let before = ScheduleMath.latestSlot(entry, atOrBefore: springDay.addingTimeInterval(-1), calendar: Self.calendar)!
        #expect(springDay.timeIntervalSince(before) == 23 * 3600)
        let fallDay = ScheduleMath.latestSlot(entry, atOrBefore: Self.local(2026, 11, 1, 11, 0), calendar: Self.calendar)!
        let beforeFall = ScheduleMath.latestSlot(entry, atOrBefore: fallDay.addingTimeInterval(-1), calendar: Self.calendar)!
        #expect(fallDay.timeIntervalSince(beforeFall) == 25 * 3600)
    }
}
