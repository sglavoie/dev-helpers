import Foundation

/// The schedule editor's form state. Each kind keeps its own fields, so switching kinds and back loses nothing.
public struct ScheduleDraft: Equatable, Sendable {
    public enum Kind: String, CaseIterable, Sendable {
        case interval, calendar, watchPaths, none

        public var title: String {
            switch self {
            case .interval: "Interval"
            case .calendar: "Calendar"
            case .watchPaths: "Watch paths"
            case .none: "None"
            }
        }
    }

    public enum IntervalUnit: String, CaseIterable, Sendable {
        case seconds, minutes, hours

        public var seconds: Int {
            switch self {
            case .seconds: 1
            case .minutes: 60
            case .hours: 3600
            }
        }

        public var title: String {
            switch self {
            case .seconds: "seconds"
            case .minutes: "minutes"
            case .hours: "hours"
            }
        }
    }

    public var kind: Kind
    public var intervalValue: Int
    public var intervalUnit: IntervalUnit
    public var calendarEntries: [CalendarEntry]
    public var watchPaths: [String]
    /// `ThrottleInterval`, edited with the watch paths; `nil` removes it.
    public var throttleSeconds: Int?

    /// What the plist has now, for `hasChanges` and the throttle update.
    public let original: Schedule
    public let originalThrottle: Int?

    public init(agent: AgentDefinition) {
        var ownPaths: [String] = []
        switch agent.schedule {
        case .interval: kind = .interval
        case .calendar: kind = .calendar
        case .watchPaths(let paths, _):
            kind = .watchPaths
            ownPaths = paths.filter { !agent.queueDirectories.contains($0) }
        case .none: kind = .none
        }
        // A QueueDirectories-only agent has no WatchPaths of its own to edit.
        if kind == .watchPaths, ownPaths.isEmpty { kind = .none }

        if case .interval(let seconds) = agent.schedule {
            intervalUnit = seconds % 3600 == 0 ? .hours : seconds % 60 == 0 ? .minutes : .seconds
            intervalValue = seconds / intervalUnit.seconds
        } else {
            intervalValue = 15
            intervalUnit = .minutes
        }
        if case .calendar(let entries) = agent.schedule {
            calendarEntries = entries
        } else {
            calendarEntries = [CalendarEntry(minute: 0, hour: 9)]
        }
        watchPaths = ownPaths
        throttleSeconds = agent.throttleInterval
        originalThrottle = agent.throttleInterval
        original = Self.ownSchedule(kind: kind, agent: agent, paths: ownPaths)
    }

    static func ownSchedule(kind: Kind, agent: AgentDefinition, paths: [String]) -> Schedule {
        switch agent.schedule {
        case .watchPaths: kind == .watchPaths ? .watchPaths(paths, throttleSeconds: nil) : .none
        default: agent.schedule
        }
    }

    // MARK: Validation

    public static let fieldRanges: [(key: WritableKeyPath<CalendarEntry, Int?> & Sendable, name: String, range: ClosedRange<Int>)] = [
        (\.minute, "Minute", 0...59), (\.hour, "Hour", 0...23), (\.day, "Day", 1...31),
        (\.weekday, "Weekday", 0...7), (\.month, "Month", 1...12),
    ]

    /// Why the draft can't be saved; empty when it can.
    public var errors: [String] {
        var errors: [String] = []
        switch kind {
        case .interval:
            if intervalValue < 1 { errors.append("The interval must be at least 1 \(intervalUnit.title.dropLast()).") }
            let seconds = intervalValue.multipliedReportingOverflow(by: intervalUnit.seconds)
            // Leave room for the overdue allowance (2×) and its update threshold (4×).
            if seconds.overflow || seconds.partialValue.multipliedReportingOverflow(by: 8).overflow {
                errors.append("The interval is too large; enter a smaller value.")
            }
        case .calendar:
            if calendarEntries.isEmpty { errors.append("Add at least one time.") }
            for (index, entry) in calendarEntries.enumerated() {
                let row = calendarEntries.count > 1 ? "Time \(index + 1): " : ""
                for field in Self.fieldRanges {
                    if let value = entry[keyPath: field.key], !field.range.contains(value) {
                        errors.append("\(row)\(field.name) must be \(field.range.lowerBound)–\(field.range.upperBound).")
                    }
                }
                if entry.isEveryMinute {
                    errors.append("\(row)every field is Any, which runs every minute; use a 1-minute interval instead.")
                }
                // Day and Weekday are alternatives in launchd. February 29 is valid in leap years.
                if entry.weekday == nil, let month = entry.month, (1...12).contains(month),
                   let day = entry.day, (1...31).contains(day) {
                    let maximumDays = [31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
                    if day > maximumDays[month - 1] {
                        errors.append("\(row)\(ScheduleDescription.monthNames[month - 1]) has no day \(day); choose another day or a weekday.")
                    }
                }
            }
        case .watchPaths:
            if cleanedPaths.isEmpty { errors.append("Add at least one path to watch.") }
            if let throttle = throttleSeconds, throttle < 1 { errors.append("Throttle must be at least 1 second.") }
        case .none:
            break
        }
        return errors
    }

    public var isValid: Bool { errors.isEmpty }

    var cleanedPaths: [String] {
        watchPaths.map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.filter { !$0.isEmpty }
    }

    /// The schedule to write, or `nil` while the draft is invalid. Watch paths carry no throttle here;
    /// see `throttleUpdate`.
    public var schedule: Schedule? {
        guard isValid else { return nil }
        switch kind {
        case .interval: return .interval(seconds: intervalValue * intervalUnit.seconds)
        case .calendar: return .calendar(calendarEntries)
        case .watchPaths: return .watchPaths(cleanedPaths, throttleSeconds: nil)
        case .none: return Schedule.none
        }
    }

    /// Only the watch-paths form edits `ThrottleInterval`; other kinds keep it, as it may pace KeepAlive.
    public var throttleUpdate: ThrottleUpdate {
        guard kind == .watchPaths, throttleSeconds != originalThrottle else { return .keep }
        return throttleSeconds.map(ThrottleUpdate.set) ?? .remove
    }

    public var hasChanges: Bool {
        guard let schedule else { return true }
        return schedule != original || throttleUpdate != .keep
    }

    // MARK: Preview

    /// "daily 10:00" plus when it would run next.
    public struct Preview: Equatable, Sendable {
        public var description: String
        public var upcoming: [Date]
        /// For intervals, which run relative to the last one rather than the clock.
        public var note: String?
    }

    public func preview(now: Date, calendar: Calendar, count: Int = 3) -> Preview? {
        guard let schedule else { return nil }
        var description = ScheduleDescription.describe(schedule)
        if kind == .watchPaths, let throttle = throttleSeconds {
            description += " (throttle \(ScheduleDescription.duration(throttle)))"
        }
        switch schedule {
        case .interval(let seconds):
            return Preview(description: description, upcoming: [],
                           note: "Runs \(ScheduleDescription.duration(seconds)) after the previous run, counted while the Mac is awake.")
        case .calendar(let entries):
            var upcoming: [Date] = []
            var after = now
            while upcoming.count < count, let next = ScheduleMath.nextSlot(entries, after: after, calendar: calendar) {
                upcoming.append(next)
                after = next
            }
            return Preview(description: description, upcoming: upcoming, note: nil)
        case .watchPaths:
            return Preview(description: description, upcoming: [], note: "Runs when a watched path changes.")
        case .none:
            return Preview(description: description, upcoming: [], note: "Only RunAtLoad, KeepAlive or Run Now start it.")
        }
    }
}
