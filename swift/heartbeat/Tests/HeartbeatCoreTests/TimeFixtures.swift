import Foundation

/// Fixed clocks for the time-based tests: nothing reads the real clock or the machine's time zone.
enum TimeFixtures {
    static let montreal: Calendar = {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "America/Montreal")!
        return calendar
    }()

    /// A Montreal wall time that is not ambiguous (don't use for 01:xx on the fall-back day).
    static func local(_ year: Int, _ month: Int, _ day: Int, _ hour: Int, _ minute: Int, _ second: Int = 0) -> Date {
        montreal.date(from: DateComponents(year: year, month: month, day: day, hour: hour, minute: minute, second: second))!
    }

    /// An exact instant, for DST cases where the wall time alone is ambiguous.
    static func utc(_ text: String) -> Date {
        ISO8601DateFormatter().date(from: text)!
    }
}
