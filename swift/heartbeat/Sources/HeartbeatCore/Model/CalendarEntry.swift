/// One `StartCalendarInterval` dictionary. A missing key is a wildcard ("every"), not zero.
public struct CalendarEntry: Equatable, Hashable, Sendable, Codable {
    public var minute: Int?
    public var hour: Int?
    public var day: Int?
    /// Raw plist value; launchd treats both 0 and 7 as Sunday. Use `normalizedWeekday` for math.
    public var weekday: Int?
    public var month: Int?

    public init(minute: Int? = nil, hour: Int? = nil, day: Int? = nil, weekday: Int? = nil, month: Int? = nil) {
        self.minute = minute
        self.hour = hour
        self.day = day
        self.weekday = weekday
        self.month = month
    }

    /// Weekday in 0...6 with Sunday as 0 (7 folds to 0).
    public var normalizedWeekday: Int? {
        weekday.map { $0 == 7 ? 0 : $0 }
    }

    /// True when every key is a wildcard, i.e. launchd fires every minute.
    public var isEveryMinute: Bool {
        minute == nil && hour == nil && day == nil && weekday == nil && month == nil
    }
}
