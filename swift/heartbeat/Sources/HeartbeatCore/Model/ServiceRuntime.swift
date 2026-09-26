/// How a service last stopped, from `launchctl print`.
public enum LastExit: Equatable, Sendable {
    /// `last exit code = (never exited)`: a daemon still on its first run, or a job that never ran.
    case neverExited
    /// `last exit code = <n>`.
    case exited(code: Int32)
    /// `last terminating signal = <name>: <n>`.
    case signaled(signal: Int32, name: String)
}

/// The top-level `state` of a loaded service.
public enum ServiceState: Equatable, Sendable, CustomStringConvertible {
    case running
    case notRunning
    /// Any other launchd state (e.g. "spawn scheduled"), kept verbatim.
    case other(String)

    public init(_ text: String) {
        switch text {
        case "running": self = .running
        case "not running": self = .notRunning
        default: self = .other(text)
        }
    }

    public var description: String {
        switch self {
        case .running: "running"
        case .notRunning: "not running"
        case .other(let text): text
        }
    }
}

/// Runtime facts launchd reports for a loaded service.
public struct ServiceRuntime: Equatable, Sendable {
    public var state: ServiceState
    public var pid: Int32?
    /// How many times launchd has started the job since it was loaded.
    public var runs: Int?
    /// `nil` when launchd printed neither `last exit code` nor `last terminating signal`.
    public var lastExit: LastExit?
    /// The plist path launchd loaded the job from.
    public var path: String?

    public init(state: ServiceState, pid: Int32? = nil, runs: Int? = nil, lastExit: LastExit? = nil, path: String? = nil) {
        self.state = state
        self.pid = pid
        self.runs = runs
        self.lastExit = lastExit
        self.path = path
    }
}

/// What `launchctl print gui/$UID/<label>` says about a label.
public enum ServiceStatus: Equatable, Sendable {
    case loaded(ServiceRuntime)
    /// Exit 113: launchd has no such service in the domain.
    case notLoaded
    /// launchctl failed, timed out or printed something the parser does not understand.
    case unknown(reason: String)

    public var runtime: ServiceRuntime? {
        if case .loaded(let runtime) = self { return runtime }
        return nil
    }
}
