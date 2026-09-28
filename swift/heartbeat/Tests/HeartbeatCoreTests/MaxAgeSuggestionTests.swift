import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct MaxAgeSuggestionTests {
    static let now = TimeFixtures.local(2026, 9, 26, 12, 0)
    static func suggest(_ schedule: Schedule) -> Int? {
        MaxAgeSuggestion.suggested(for: schedule, now: now, calendar: TimeFixtures.montreal)
    }

    @Test func intervalMatchesTheEvaluatorDefault() {
        #expect(Self.suggest(.interval(seconds: 900)) == 1800)
        #expect(Self.suggest(.interval(seconds: 120)) == 420)
        #expect(Self.suggest(.interval(seconds: 1800)) == 3600)
    }

    @Test func calendarUsesOneAndAHalfLongestGaps() {
        #expect(Self.suggest(.calendar([CalendarEntry(minute: 0, hour: 10)])) == 36 * 3600)
        // 10:00 and 13:00: the longest gap is 13:00 -> 10:00, 21 h.
        #expect(Self.suggest(.calendar([CalendarEntry(minute: 0, hour: 10), CalendarEntry(minute: 0, hour: 13)]))
            == Int(21 * 3600 * 1.5))
        // Weekly: the last gap measured (Oct 28 -> Nov 4) spans the fall-back change, so it is 7 d + 1 h.
        #expect(Self.suggest(.calendar([CalendarEntry(minute: 0, hour: 12, weekday: 3)])) == Int((7 * 86400 + 3600) * 1.5))
        #expect(Self.suggest(.calendar([CalendarEntry(minute: 10)])) == 90 * 60)
    }

    @Test func noCadenceNoSuggestion() {
        #expect(Self.suggest(.watchPaths(["/tmp"], throttleSeconds: nil)) == nil)
        #expect(Self.suggest(.none) == nil)
    }

    @Test func needsUpdateThresholds() {
        #expect(MaxAgeSuggestion.needsUpdate(current: 2700, suggested: 3600))
        #expect(!MaxAgeSuggestion.needsUpdate(current: 2700, suggested: 1800))
        #expect(!MaxAgeSuggestion.needsUpdate(current: 7200, suggested: 1800))
        #expect(MaxAgeSuggestion.needsUpdate(current: 7201, suggested: 1800))
    }
}
