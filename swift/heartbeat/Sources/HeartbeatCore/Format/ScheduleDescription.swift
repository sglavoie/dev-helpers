/// Short human descriptions of launchd triggers, e.g. "daily 10:00, 13:00" or "Wed 12:00".
/// Missing calendar keys are wildcards, so "hourly at :10" is not "00:10".
public enum ScheduleDescription {
    static let weekdayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
    static let monthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]

    /// Describes the agent's trigger, falling back to KeepAlive / RunAtLoad when it has no schedule.
    public static func describe(_ agent: AgentDefinition) -> String {
        guard agent.schedule == .none else { return describe(agent.schedule) }
        if agent.keepAlive == .always { return "always running (KeepAlive)" }
        if agent.keepAlive.expectsRunning { return "running, restarted on failure (KeepAlive)" }
        return agent.runAtLoad ? "at load only" : "on demand"
    }

    public static func describe(_ schedule: Schedule) -> String {
        switch schedule {
        case .interval(let seconds):
            return "every \(duration(seconds))"
        case .calendar(let entries):
            return describe(entries)
        case .watchPaths(let paths, let throttle):
            let names = paths.map { $0.split(separator: "/").last.map(String.init) ?? $0 }
            var text = "on change: " + (names.first ?? "")
            if names.count > 1 { text += " +\(names.count - 1)" }
            if let throttle { text += " (throttle \(duration(throttle)))" }
            return text
        case .none:
            return "on demand"
        }
    }

    public static func describe(_ entries: [CalendarEntry]) -> String {
        guard !entries.isEmpty else { return "never (empty calendar)" }
        // The common "a few fixed times a day" shape reads best grouped.
        if entries.allSatisfy({ $0.day == nil && $0.weekday == nil && $0.month == nil && $0.hour != nil && $0.minute != nil }) {
            return "daily " + entries.map(time).joined(separator: ", ")
        }
        return entries.map(describe).joined(separator: ", ")
    }

    public static func describe(_ entry: CalendarEntry) -> String {
        var parts: [String] = []
        if let month = entry.month { parts.append(monthNames[month - 1]) }
        switch (entry.day, entry.normalizedWeekday) {
        // crontab semantics: with both set, either one matches.
        case (let day?, let weekday?): parts.append("day \(day) or \(weekdayNames[weekday])")
        case (let day?, nil): parts.append("day \(day)")
        case (nil, let weekday?): parts.append(weekdayNames[weekday])
        // "daily" only reads right in front of a fixed time; "hourly at :10" carries its own frequency.
        case (nil, nil): if entry.month == nil && entry.hour != nil && entry.minute != nil { parts.append("daily") }
        }
        parts.append(time(entry))
        return parts.joined(separator: " ")
    }

    static func time(_ entry: CalendarEntry) -> String {
        switch (entry.hour, entry.minute) {
        case (let hour?, let minute?): pad(hour) + ":" + pad(minute)
        case (let hour?, nil): "every minute of \(pad(hour)):xx"
        case (nil, let minute?): "hourly at :" + pad(minute)
        case (nil, nil): "every minute"
        }
    }

    static func pad(_ value: Int) -> String {
        value < 10 ? "0\(value)" : "\(value)"
    }

    /// "45 s", "15 min", "2 h", "1 h 30 min", "2 d".
    public static func duration(_ seconds: Int) -> String {
        if seconds < 60 || seconds % 60 != 0 { return "\(seconds) s" }
        let minutes = seconds / 60
        if minutes < 60 { return "\(minutes) min" }
        if minutes % (24 * 60) == 0 { return "\(minutes / (24 * 60)) d" }
        let hours = minutes / 60, rest = minutes % 60
        return rest == 0 ? "\(hours) h" : "\(hours) h \(rest) min"
    }
}
