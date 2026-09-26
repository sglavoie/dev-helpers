import Foundation

/// When the Mac last booted and last woke from sleep, from `sysctl kern.boottime` / `kern.waketime`.
/// launchd skips calendar slots while the Mac is off and coalesces the ones missed during sleep into
/// one run on wake, so the health rules measure deadlines from these times.
public struct PowerTimeline: Equatable, Sendable, Codable {
    public var bootTime: Date
    /// `nil` when the Mac has not slept since boot (`kern.waketime` is `{ sec = 0 }` then).
    public var lastWake: Date?

    public init(bootTime: Date, lastWake: Date? = nil) {
        self.bootTime = bootTime
        // A wake time before boot is left over from the previous boot session: ignore it.
        self.lastWake = lastWake.flatMap { $0 > bootTime ? $0 : nil }
    }

    /// The start of the current awake stretch: the last wake, or boot when the Mac has not slept.
    public var awakeSince: Date {
        lastWake ?? bootTime
    }

    /// Builds the timeline from raw sysctl `timeval`s; a zero wake time means "no wake since boot".
    public init(bootTime boot: timeval, wakeTime wake: timeval) {
        let wakeDate = wake.tv_sec == 0 && wake.tv_usec == 0 ? nil : Self.date(wake)
        self.init(bootTime: Self.date(boot), lastWake: wakeDate)
    }

    /// Reads the live values. Returns `nil` only if `kern.boottime` can't be read.
    public static func current() -> PowerTimeline? {
        guard let boot = readTimeval("kern.boottime") else { return nil }
        let wake = readTimeval("kern.waketime") ?? timeval(tv_sec: 0, tv_usec: 0)
        return PowerTimeline(bootTime: boot, wakeTime: wake)
    }

    static func date(_ value: timeval) -> Date {
        Date(timeIntervalSince1970: TimeInterval(value.tv_sec) + TimeInterval(value.tv_usec) / 1_000_000)
    }

    static func readTimeval(_ name: String) -> timeval? {
        var value = timeval()
        var size = MemoryLayout<timeval>.size
        guard sysctlbyname(name, &value, &size, nil, 0) == 0, size == MemoryLayout<timeval>.size else { return nil }
        return value
    }
}
