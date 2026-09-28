import Foundation

/// How long a run may be missing before the overdue rule fires, for sizing a `maxAgeSeconds` override
/// to a new schedule.
public enum MaxAgeSuggestion {
    /// The allowed age of a `StartInterval` agent without an override: two intervals, or one plus 5 minutes.
    public static func defaultAllowedAge(interval seconds: Int) -> Int {
        max(2 * seconds, seconds + 300)
    }

    /// Slots looked at when measuring a calendar's longest gap; enough for "every minute" to stop early.
    static let maxSlots = 500
    /// How far ahead a calendar is measured: a month covers weekly and weekday-only schedules.
    static let horizon: TimeInterval = 32 * 86400

    /// A `maxAgeSeconds` that fits `schedule`, or `nil` for triggers without a cadence (watch paths, none).
    /// Calendars get 1.5 × their longest gap between slots, rounded up to the minute.
    public static func suggested(for schedule: Schedule, now: Date, calendar: Calendar) -> Int? {
        switch schedule {
        case .interval(let seconds):
            return defaultAllowedAge(interval: seconds)
        case .calendar(let entries):
            guard let gap = longestGap(entries, after: now, calendar: calendar) else { return nil }
            let allowed = Int((gap * 1.5).rounded(.up))
            return (allowed + 59) / 60 * 60
        case .watchPaths, .none:
            return nil
        }
    }

    /// The longest time between consecutive slots over the next month (at least two slots, so monthly and
    /// yearly schedules still get one gap).
    static func longestGap(_ entries: [CalendarEntry], after now: Date, calendar: Calendar) -> TimeInterval? {
        guard var previous = ScheduleMath.nextSlot(entries, after: now, calendar: calendar) else { return nil }
        let end = now.addingTimeInterval(horizon)
        var longest: TimeInterval?
        for _ in 0..<maxSlots {
            guard let next = ScheduleMath.nextSlot(entries, after: previous, calendar: calendar) else { break }
            longest = max(longest ?? 0, next.timeIntervalSince(previous))
            previous = next
            if next > end { break }
        }
        return longest
    }

    /// True when `current` is shorter than a normal gap (false overdue alarms) or over 4× the suggestion
    /// (the check barely fires).
    public static func needsUpdate(current: Int, suggested: Int) -> Bool {
        current < suggested || current > 4 * suggested
    }
}
