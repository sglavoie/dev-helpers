import Foundation

/// Finds `StartCalendarInterval` slots in local wall-clock time.
///
/// Semantics follow launchd (crontab-like): a missing key is a wildcard, Weekday 0 and 7 are Sunday,
/// and when both Day and Weekday are set either one matches. Across DST changes:
/// - a wall time that doesn't exist (spring forward, 02:30 in America/Montreal) is not a slot, so a
///   skipped run is never expected;
/// - a wall time that happens twice (fall back, 01:30) is one slot, at its first occurrence.
/// Both choices err toward the earlier or missing slot, which can only make the overdue rule more lenient.
public enum ScheduleMath {
    /// How far to look for a slot. Four years plus a day covers Feb 29 entries.
    static let searchDays = 4 * 366 + 1

    /// The latest slot of any entry at or before `date`, or `nil` if none exists (e.g. Feb 30).
    public static func latestSlot(_ entries: [CalendarEntry], atOrBefore date: Date, calendar: Calendar) -> Date? {
        entries.compactMap { latestSlot($0, atOrBefore: date, calendar: calendar) }.max()
    }

    /// The earliest slot of any entry strictly after `date`.
    public static func nextSlot(_ entries: [CalendarEntry], after date: Date, calendar: Calendar) -> Date? {
        entries.compactMap { nextSlot($0, after: date, calendar: calendar) }.min()
    }

    public static func latestSlot(_ entry: CalendarEntry, atOrBefore date: Date, calendar: Calendar) -> Date? {
        search(entry, from: date, step: -1, calendar: calendar) { $0 <= date }
    }

    public static func nextSlot(_ entry: CalendarEntry, after date: Date, calendar: Calendar) -> Date? {
        search(entry, from: date, step: 1, calendar: calendar) { $0 > date }
    }

    /// Walks whole local days from `date` in `step` direction and returns the first matching slot,
    /// scanning times within each day in the same direction.
    static func search(
        _ entry: CalendarEntry, from date: Date, step: Int, calendar: Calendar, accept: (Date) -> Bool
    ) -> Date? {
        let hours = entry.hour.map { [$0] } ?? Array(0...23)
        let minutes = entry.minute.map { [$0] } ?? Array(0...59)
        let times = hours.flatMap { hour in minutes.map { (hour, $0) } }
        let orderedTimes = step < 0 ? Array(times.reversed()) : times

        // Noon is never skipped or repeated by a DST change, so it is a safe anchor for day arithmetic.
        let startDay = calendar.dateComponents([.year, .month, .day], from: date)
        guard var anchor = calendar.date(from: DateComponents(
            year: startDay.year, month: startDay.month, day: startDay.day, hour: 12)) else { return nil }

        for _ in 0..<searchDays {
            let day = calendar.dateComponents([.year, .month, .day, .weekday], from: anchor)
            if let year = day.year, let month = day.month, let dayOfMonth = day.day, let weekday = day.weekday,
               matchesDay(entry, month: month, day: dayOfMonth, weekday: weekday - 1) {
                for (hour, minute) in orderedTimes {
                    guard let slot = wallTime(year: year, month: month, day: dayOfMonth, hour: hour, minute: minute,
                                              calendar: calendar) else { continue }
                    if accept(slot) { return slot }
                }
            }
            guard let next = calendar.date(byAdding: .day, value: step, to: anchor) else { return nil }
            anchor = next
        }
        return nil
    }

    /// `weekday` is 0...6 with Sunday as 0.
    static func matchesDay(_ entry: CalendarEntry, month: Int, day: Int, weekday: Int) -> Bool {
        if let wanted = entry.month, wanted != month { return false }
        switch (entry.day, entry.normalizedWeekday) {
        case (let wantedDay?, let wantedWeekday?): return wantedDay == day || wantedWeekday == weekday
        case (let wantedDay?, nil): return wantedDay == day
        case (nil, let wantedWeekday?): return wantedWeekday == weekday
        case (nil, nil): return true
        }
    }

    /// The instant a local wall time happens, or `nil` when it doesn't exist (spring-forward gap).
    /// Ambiguous fall-back times resolve to their first occurrence.
    static func wallTime(year: Int, month: Int, day: Int, hour: Int, minute: Int, calendar: Calendar) -> Date? {
        let wanted = DateComponents(year: year, month: month, day: day, hour: hour, minute: minute)
        guard let date = calendar.date(from: wanted) else { return nil }
        let got = calendar.dateComponents([.year, .month, .day, .hour, .minute], from: date)
        guard got.year == year, got.month == month, got.day == day, got.hour == hour, got.minute == minute else {
            return nil
        }
        // Foundation may pick the second occurrence of a repeated hour; step back an hour if that
        // lands on the same wall time.
        let hourEarlier = date.addingTimeInterval(-3600)
        let earlier = calendar.dateComponents([.year, .month, .day, .hour, .minute], from: hourEarlier)
        if earlier.year == year, earlier.month == month, earlier.day == day, earlier.hour == hour, earlier.minute == minute {
            return hourEarlier
        }
        return date
    }
}
