import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct ScheduleDraftTests {
    static func agent(_ schedule: Schedule, throttle: Int? = nil, queue: [String] = []) -> AgentDefinition {
        AgentDefinition(label: "com.sglavoie.test", plistPath: "/tmp/test.plist", schedule: schedule,
                        queueDirectories: queue, throttleInterval: throttle)
    }

    @Test func intervalPicksTheLargestEvenUnit() {
        let hours = ScheduleDraft(agent: Self.agent(.interval(seconds: 7200)))
        #expect((hours.intervalValue, hours.intervalUnit) == (2, .hours))
        let minutes = ScheduleDraft(agent: Self.agent(.interval(seconds: 900)))
        #expect((minutes.intervalValue, minutes.intervalUnit) == (15, .minutes))
        let seconds = ScheduleDraft(agent: Self.agent(.interval(seconds: 90)))
        #expect((seconds.intervalValue, seconds.intervalUnit) == (90, .seconds))
    }

    @Test func untouchedDraftHasNoChanges() {
        for schedule: Schedule in [.interval(seconds: 900), .calendar([CalendarEntry(minute: 0, hour: 10)]),
                                   .watchPaths(["/tmp/a"], throttleSeconds: 10), .none] {
            #expect(!ScheduleDraft(agent: Self.agent(schedule, throttle: 10)).hasChanges)
        }
    }

    @Test func switchingKindsKeepsFieldsAndConvertsUnits() {
        var draft = ScheduleDraft(agent: Self.agent(.interval(seconds: 900)))
        draft.kind = .calendar
        #expect(draft.schedule == .calendar([CalendarEntry(minute: 0, hour: 9)]))
        #expect(draft.hasChanges)
        draft.kind = .interval
        #expect(!draft.hasChanges)
        draft.intervalValue = 1
        draft.intervalUnit = .hours
        #expect(draft.schedule == .interval(seconds: 3600))
    }

    @Test func validation() {
        var draft = ScheduleDraft(agent: Self.agent(.interval(seconds: 60)))
        draft.intervalValue = 0
        #expect(draft.schedule == nil)
        #expect(draft.errors == ["The interval must be at least 1 minute."])

        draft.kind = .calendar
        draft.calendarEntries = [CalendarEntry(minute: 60, hour: 24), CalendarEntry()]
        #expect(draft.errors.count == 3)
        draft.calendarEntries = []
        #expect(draft.errors == ["Add at least one time."])

        draft.kind = .watchPaths
        draft.watchPaths = ["  "]
        draft.throttleSeconds = 0
        #expect(draft.errors == ["Add at least one path to watch.", "Throttle must be at least 1 second."])
    }

    @Test func throttleOnlyChangesWithWatchPaths() {
        var draft = ScheduleDraft(agent: Self.agent(.watchPaths(["/tmp/a"], throttleSeconds: 10), throttle: 10))
        #expect(draft.throttleUpdate == .keep)
        draft.throttleSeconds = 30
        #expect(draft.throttleUpdate == .set(30))
        draft.throttleSeconds = nil
        #expect(draft.throttleUpdate == .remove)
        draft.kind = .interval
        #expect(draft.throttleUpdate == .keep)
    }

    @Test(arguments: [CalendarEntry(day: 30, month: 2), CalendarEntry(day: 31, month: 4),
                      CalendarEntry(day: 31, month: 6), CalendarEntry(day: 31, month: 9),
                      CalendarEntry(day: 31, month: 11)])
    func impossibleCalendarDatesCannotBeSaved(entry: CalendarEntry) {
        let draft = ScheduleDraft(agent: Self.agent(.calendar([CalendarEntry(hour: 9), entry])))
        #expect(!draft.isValid)
        #expect(draft.schedule == nil)
        #expect(draft.errors.count == 1)
        #expect(draft.errors.first?.hasPrefix("Time 2:") == true)
        #expect(draft.preview(now: TimeFixtures.local(2026, 9, 26, 12, 0), calendar: TimeFixtures.montreal) == nil)
    }

    @Test(arguments: [CalendarEntry(day: 29, month: 2), CalendarEntry(day: 31),
                      CalendarEntry(day: 30, weekday: 1, month: 2), CalendarEntry(day: 31, weekday: 7, month: 4)])
    func possibleCalendarDatesRemainValid(entry: CalendarEntry) {
        let draft = ScheduleDraft(agent: Self.agent(.calendar([entry])))
        #expect(draft.isValid)
        #expect(draft.schedule != nil)
        #expect(ScheduleMath.nextSlot(entry, after: TimeFixtures.local(2026, 9, 26, 12, 0),
                                     calendar: TimeFixtures.montreal) != nil)
    }

    @Test(arguments: ScheduleDraft.IntervalUnit.allCases)
    func oversizedIntervalsAreRejected(unit: ScheduleDraft.IntervalUnit) {
        var draft = ScheduleDraft(agent: Self.agent(.interval(seconds: 60)))
        draft.intervalUnit = unit
        draft.intervalValue = Int.max
        #expect(draft.errors == ["The interval is too large; enter a smaller value."])
        #expect(draft.schedule == nil)
        draft.intervalValue = (Int.max / 8) / unit.seconds
        #expect(draft.isValid)
        #expect(draft.schedule != nil)
        let suggested = MaxAgeSuggestion.defaultAllowedAge(interval: draft.intervalValue * unit.seconds)
        #expect(!MaxAgeSuggestion.needsUpdate(current: suggested, suggested: suggested))
    }

    @Test func queueDirectoriesAreNotEditablePaths() {
        let draft = ScheduleDraft(agent: Self.agent(.watchPaths(["/tmp/a", "/tmp/q"], throttleSeconds: nil), queue: ["/tmp/q"]))
        #expect(draft.watchPaths == ["/tmp/a"])
        #expect(!draft.hasChanges)
        let queueOnly = ScheduleDraft(agent: Self.agent(.watchPaths(["/tmp/q"], throttleSeconds: nil), queue: ["/tmp/q"]))
        #expect(queueOnly.kind == .none)
        #expect(!queueOnly.hasChanges)
    }

    @Test func calendarPreviewListsNextSlots() {
        var draft = ScheduleDraft(agent: Self.agent(.none))
        draft.kind = .calendar
        draft.calendarEntries = [CalendarEntry(minute: 0, hour: 10), CalendarEntry(minute: 0, hour: 13)]
        let preview = draft.preview(now: TimeFixtures.local(2026, 9, 26, 12, 0), calendar: TimeFixtures.montreal)
        #expect(preview?.description == "daily 10:00, 13:00")
        #expect(preview?.upcoming == [TimeFixtures.local(2026, 9, 26, 13, 0), TimeFixtures.local(2026, 9, 27, 10, 0),
                                      TimeFixtures.local(2026, 9, 27, 13, 0)])
    }
}
