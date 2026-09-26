import Foundation

/// Everything Heartbeat remembers between polls and relaunches (`state.json`).
public struct HeartbeatState: Equatable, Codable, Sendable {
    public static let currentVersion = 1

    public var version: Int
    /// `kern.boottime` when the ledger was last updated; a different value means a new launchd session.
    public var bootTime: Date?
    public var agents: [String: AgentState]

    public init(version: Int = HeartbeatState.currentVersion, bootTime: Date? = nil, agents: [String: AgentState] = [:]) {
        self.version = version
        self.bootTime = bootTime
        self.agents = agents
    }

    public static let empty = HeartbeatState()

    public subscript(label: String) -> AgentState {
        get { agents[label] ?? AgentState() }
        set { agents[label] = newValue }
    }

    /// `notifiedSeverity` per label, the input TransitionTracker expects.
    public var notifiedSeverities: [String: Severity] {
        agents.compactMapValues(\.notifiedSeverity)
    }

    /// Stores a TransitionTracker result: labels it evaluated get its value, others keep theirs.
    public mutating func applyNotified(_ notified: [String: Severity], evaluated labels: some Sequence<String>) {
        for label in labels { self[label].notifiedSeverity = notified[label] }
    }
}

/// What Heartbeat remembers about one agent.
public struct AgentState: Equatable, Codable, Sendable {
    /// `runs` from the last `launchctl print` while loaded.
    public var runs: Int?
    /// When `runs` was last seen going up: the ledger's evidence of a run.
    public var runsChangedAt: Date?
    /// When the service was first seen loaded in the current launchd session.
    public var loadedObservedAt: Date?
    /// When a running KeepAlive daemon's `runs` last went up, i.e. it restarted.
    public var lastRestartAt: Date?
    /// Unloaded from Heartbeat's menu; stays gray instead of amber.
    public var paused: Bool
    /// The severity a banner was last shown for (`failing`), so a relaunch doesn't repeat it.
    public var notifiedSeverity: Severity?
    /// Consecutive polls where a KeepAlive agent had no PID.
    public var notRunningStreak: Int
    public var lastHealthCheck: HealthCheckResult?

    public init(
        runs: Int? = nil, runsChangedAt: Date? = nil, loadedObservedAt: Date? = nil, lastRestartAt: Date? = nil,
        paused: Bool = false, notifiedSeverity: Severity? = nil, notRunningStreak: Int = 0,
        lastHealthCheck: HealthCheckResult? = nil
    ) {
        self.runs = runs
        self.runsChangedAt = runsChangedAt
        self.loadedObservedAt = loadedObservedAt
        self.lastRestartAt = lastRestartAt
        self.paused = paused
        self.notifiedSeverity = notifiedSeverity
        self.notRunningStreak = notRunningStreak
        self.lastHealthCheck = lastHealthCheck
    }

    enum CodingKeys: String, CodingKey {
        case runs, runsChangedAt, loadedObservedAt, lastRestartAt, paused, notifiedSeverity, notRunningStreak, lastHealthCheck
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        runs = try c.decodeIfPresent(Int.self, forKey: .runs)
        runsChangedAt = try c.decodeIfPresent(Date.self, forKey: .runsChangedAt)
        loadedObservedAt = try c.decodeIfPresent(Date.self, forKey: .loadedObservedAt)
        lastRestartAt = try c.decodeIfPresent(Date.self, forKey: .lastRestartAt)
        paused = try c.decodeIfPresent(Bool.self, forKey: .paused) ?? false
        notifiedSeverity = try c.decodeIfPresent(Severity.self, forKey: .notifiedSeverity)
        notRunningStreak = try c.decodeIfPresent(Int.self, forKey: .notRunningStreak) ?? 0
        lastHealthCheck = try c.decodeIfPresent(HealthCheckResult.self, forKey: .lastHealthCheck)
    }

    /// The history HealthEvaluator reads.
    public var history: AgentHistory {
        AgentHistory(previousNotRunningPolls: notRunningStreak, lastRestartAt: lastRestartAt, loadedObservedAt: loadedObservedAt)
    }
}
