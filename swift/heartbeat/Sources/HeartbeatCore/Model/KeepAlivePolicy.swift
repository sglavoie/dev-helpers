/// The plist's `KeepAlive` value.
public enum KeepAlivePolicy: Equatable, Sendable {
    /// Absent or `<false/>`.
    case none
    /// `<true/>`: launchd restarts the job whenever it exits.
    case always
    /// Dictionary form. `successfulExit` is the `SuccessfulExit` key when present;
    /// `otherKeys` lists the remaining conditions (NetworkState, PathState, Crashed, ...), sorted.
    case conditional(successfulExit: Bool?, otherKeys: [String])

    /// Whether Heartbeat should expect a live PID: `true`, or `SuccessfulExit: false`
    /// (the job is restarted until it exits cleanly, so a long-running daemon is expected).
    public var expectsRunning: Bool {
        switch self {
        case .none: false
        case .always: true
        case .conditional(let successfulExit, _): successfulExit == false
        }
    }
}
