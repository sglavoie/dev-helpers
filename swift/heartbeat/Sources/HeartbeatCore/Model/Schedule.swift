/// What makes launchd start the job, besides `RunAtLoad` and `KeepAlive`.
public enum Schedule: Equatable, Sendable {
    /// `StartInterval` in seconds.
    case interval(seconds: Int)
    /// `StartCalendarInterval`, from either the array or the bare-dictionary form.
    case calendar([CalendarEntry])
    /// `WatchPaths` (and/or `QueueDirectories`); `throttleSeconds` is `ThrottleInterval` when set.
    case watchPaths([String], throttleSeconds: Int?)
    /// No timed or path trigger (run-at-load or KeepAlive-only jobs).
    case none
}
