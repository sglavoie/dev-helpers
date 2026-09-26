import Foundation

/// Display formatting shared by the tag, history and storage views.
public enum Formatting {
    /// `"just now"`, `"5m ago"`, `"3h ago"`, `"2d ago"`, `"4w ago"`, `"2mo ago"`, `"1y ago"`.
    /// Future timestamps read as `"just now"`.
    public static func relativeTime(_ timestamp: Int64, now: Int64) -> String {
        let seconds = floorDiv(now - timestamp, 1000)
        let minutes = floorDiv(seconds, 60)
        let hours = floorDiv(minutes, 60)
        let days = floorDiv(hours, 24)
        let weeks = floorDiv(days, 7)
        let months = floorDiv(days, 30)
        let years = floorDiv(days, 365)

        if seconds < 60 { return "just now" }
        if minutes < 60 { return "\(minutes)m ago" }
        if hours < 24 { return "\(hours)h ago" }
        if days < 7 { return "\(days)d ago" }
        if days < 30 { return "\(weeks)w ago" }
        if days < 365 { return "\(months)mo ago" }
        return "\(years)y ago"
    }

    /// `1234567` → `"1,234,567"`.
    public static func number(_ value: Int) -> String {
        value.formatted(.number.grouping(.automatic).locale(Locale(identifier: "en_US")))
    }

    /// `"512 bytes"`, `"1.5 KB"`, `"2.25 MB"` (decimal units).
    public static func size(_ bytes: Int) -> String {
        if bytes < 1000 { return "\(bytes) bytes" }
        if bytes < 1_000_000 { return String(format: "%.1f KB", Double(bytes) / 1000) }
        return String(format: "%.2f MB", Double(bytes) / 1_000_000)
    }

    /// e.g. `"Jan 5, 2026, 09:30 AM"` in the given time zone.
    public static func absoluteDate(_ timestamp: Int64, timeZone: TimeZone = .current) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US")
        formatter.timeZone = timeZone
        formatter.dateFormat = "MMM d, yyyy, hh:mm a"
        return formatter.string(from: Date(epochMilliseconds: timestamp))
    }

    /// Floored integer division, matching JS `Math.floor(a / b)` for negative values.
    static func floorDiv(_ a: Int64, _ b: Int64) -> Int64 {
        let quotient = a / b
        return (a % b != 0 && (a < 0) != (b < 0)) ? quotient - 1 : quotient
    }
}

extension Date {
    public init(epochMilliseconds: Int64) {
        self.init(timeIntervalSince1970: Double(epochMilliseconds) / 1000)
    }

    public var epochMilliseconds: Int64 {
        Int64((timeIntervalSince1970 * 1000).rounded(.down))
    }
}

extension Array {
    /// Sort that keeps equal elements in their original order, like JS `Array.prototype.sort`.
    func stableSorted(by areInIncreasingOrder: (Element, Element) -> Bool) -> [Element] {
        enumerated()
            .sorted { lhs, rhs in
                if areInIncreasingOrder(lhs.element, rhs.element) { return true }
                if areInIncreasingOrder(rhs.element, lhs.element) { return false }
                return lhs.offset < rhs.offset
            }
            .map(\.element)
    }
}
