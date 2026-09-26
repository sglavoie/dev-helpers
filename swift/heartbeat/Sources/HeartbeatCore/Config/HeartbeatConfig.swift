import Foundation

/// `~/.config/heartbeat/config.json`. Every key is optional; missing keys take the defaults below.
public struct HeartbeatConfig: Equatable, Codable, Sendable {
    public static let currentVersion = 1
    public static let minimumPollSeconds = 15

    public var version: Int
    public var labelPrefix: String
    public var pollSeconds: Int
    /// Global switch for macOS banners; per-agent `notify` can only turn them off further.
    public var notifications: Bool
    public var piHost: String
    public var piStatusSeconds: Int
    /// argv with `{path}` replaced by the log path; `nil` opens the log with the default app.
    public var openLogCommand: [String]?
    /// Per-label settings. When the file has an `agents` key it replaces the seeded defaults wholesale.
    public var agents: [String: AgentConfig]

    public init(
        version: Int = HeartbeatConfig.currentVersion, labelPrefix: String = "com.sglavoie.", pollSeconds: Int = 60,
        notifications: Bool = true, piHost: String = "pi.tailb5cfdf.ts.net", piStatusSeconds: Int = 300,
        openLogCommand: [String]? = nil, agents: [String: AgentConfig] = HeartbeatConfig.seededAgents
    ) {
        self.version = version
        self.labelPrefix = labelPrefix
        self.pollSeconds = pollSeconds
        self.notifications = notifications
        self.piHost = piHost
        self.piStatusSeconds = piStatusSeconds
        self.openLogCommand = openLogCommand
        self.agents = agents
    }

    public static let defaults = HeartbeatConfig()

    /// Settings that apply with no config file: Kuma already pages the phone for forgejo-sync and
    /// pi-backup-fetch, pi-backup-fetch leaves a receipt, and the vault guard has a health script.
    public static let seededAgents: [String: AgentConfig] = [
        "com.sglavoie.forgejo-sync": AgentConfig(displayName: "Forgejo sync", notify: false, maxAgeSeconds: 2700),
        "com.sglavoie.pi-backup-fetch": AgentConfig(
            notify: false,
            receipt: ReceiptConfig(path: "~/Library/Application Support/pi-backup-fetch/last-run.json")),
        "com.sglavoie.brainnotes-vault-guard": AgentConfig(
            health: HealthCommandConfig(command: ["~/.local/bin/check-brainnotes-vault-health.sh"], warningExitCodes: [2])),
    ]

    enum CodingKeys: String, CodingKey, CaseIterable {
        case version, labelPrefix, pollSeconds, notifications, piHost, piStatusSeconds, openLogCommand, agents
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        let d = HeartbeatConfig.defaults
        version = try c.decodeIfPresent(Int.self, forKey: .version) ?? d.version
        labelPrefix = try c.decodeIfPresent(String.self, forKey: .labelPrefix) ?? d.labelPrefix
        pollSeconds = try c.decodeIfPresent(Int.self, forKey: .pollSeconds) ?? d.pollSeconds
        notifications = try c.decodeIfPresent(Bool.self, forKey: .notifications) ?? d.notifications
        piHost = try c.decodeIfPresent(String.self, forKey: .piHost) ?? d.piHost
        piStatusSeconds = try c.decodeIfPresent(Int.self, forKey: .piStatusSeconds) ?? d.piStatusSeconds
        openLogCommand = try c.decodeIfPresent([String].self, forKey: .openLogCommand) ?? d.openLogCommand
        agents = try c.decodeIfPresent([String: AgentConfig].self, forKey: .agents) ?? d.agents
    }

    /// The settings for `label`, or the defaults when the config doesn't mention it.
    public func agent(_ label: String) -> AgentConfig {
        agents[label] ?? AgentConfig()
    }

    /// Whether a banner may be shown for `label`.
    public func notifies(_ label: String) -> Bool {
        notifications && (agent(label).notify ?? true)
    }

    /// A copy with `~` expanded in every path-like field.
    public func expandingTilde(home: String) -> HeartbeatConfig {
        var copy = self
        copy.agents = agents.mapValues { $0.expandingTilde(home: home) }
        return copy
    }
}

/// Settings for one agent.
public struct AgentConfig: Equatable, Codable, Sendable {
    public var displayName: String?
    public var hidden: Bool
    /// `false` keeps the agent in the menu but never shows a banner for it.
    public var notify: Bool?
    public var ignoreExitCodes: [Int32]
    public var maxAgeSeconds: Int?
    /// Overrides whether the plist's KeepAlive means "should always have a PID".
    public var expectsRunning: Bool?
    public var graceSeconds: Int?
    /// Extra files whose mtime counts as evidence of a run.
    public var evidencePaths: [String]
    public var health: HealthCommandConfig?
    public var receipt: ReceiptConfig?

    public init(
        displayName: String? = nil, hidden: Bool = false, notify: Bool? = nil, ignoreExitCodes: [Int32] = [],
        maxAgeSeconds: Int? = nil, expectsRunning: Bool? = nil, graceSeconds: Int? = nil, evidencePaths: [String] = [],
        health: HealthCommandConfig? = nil, receipt: ReceiptConfig? = nil
    ) {
        self.displayName = displayName
        self.hidden = hidden
        self.notify = notify
        self.ignoreExitCodes = ignoreExitCodes
        self.maxAgeSeconds = maxAgeSeconds
        self.expectsRunning = expectsRunning
        self.graceSeconds = graceSeconds
        self.evidencePaths = evidencePaths
        self.health = health
        self.receipt = receipt
    }

    enum CodingKeys: String, CodingKey, CaseIterable {
        case displayName, hidden, notify, ignoreExitCodes, maxAgeSeconds, expectsRunning, graceSeconds, evidencePaths,
             health, receipt
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        displayName = try c.decodeIfPresent(String.self, forKey: .displayName)
        hidden = try c.decodeIfPresent(Bool.self, forKey: .hidden) ?? false
        notify = try c.decodeIfPresent(Bool.self, forKey: .notify)
        ignoreExitCodes = try c.decodeIfPresent([Int32].self, forKey: .ignoreExitCodes) ?? []
        maxAgeSeconds = try c.decodeIfPresent(Int.self, forKey: .maxAgeSeconds)
        expectsRunning = try c.decodeIfPresent(Bool.self, forKey: .expectsRunning)
        graceSeconds = try c.decodeIfPresent(Int.self, forKey: .graceSeconds)
        evidencePaths = try c.decodeIfPresent([String].self, forKey: .evidencePaths) ?? []
        health = try c.decodeIfPresent(HealthCommandConfig.self, forKey: .health)
        receipt = try c.decodeIfPresent(ReceiptConfig.self, forKey: .receipt)
    }

    /// The options HealthEvaluator reads.
    public func healthOptions(pausedByHeartbeat: Bool = false) -> AgentHealthOptions {
        AgentHealthOptions(
            hidden: hidden, pausedByHeartbeat: pausedByHeartbeat, ignoreExitCodes: Set(ignoreExitCodes),
            maxAgeSeconds: maxAgeSeconds, expectsRunning: expectsRunning,
            graceSeconds: graceSeconds ?? HealthEvaluator.defaultGraceSeconds, health: health)
    }

    func expandingTilde(home: String) -> AgentConfig {
        var copy = self
        copy.evidencePaths = evidencePaths.map { ConfigLoader.expandTilde($0, home: home) }
        copy.receipt?.path = ConfigLoader.expandTilde(receipt?.path ?? "", home: home)
        if let first = health?.command.first {
            copy.health?.command[0] = ConfigLoader.expandTilde(first, home: home)
        }
        return copy
    }
}

/// Rule 7: a command whose exit status says whether the agent's job is really healthy.
public struct HealthCommandConfig: Equatable, Codable, Sendable {
    public static let defaultIntervalSeconds = 300
    public static let defaultTimeoutSeconds = 30

    /// argv, run without a shell and with the fixed GUI PATH; `~` is expanded in the executable.
    public var command: [String]
    public var intervalSeconds: Int
    public var timeoutSeconds: Int
    /// Non-zero exit codes that mean "couldn't check" (amber) rather than "unhealthy" (red).
    public var warningExitCodes: [Int32]

    public init(command: [String], intervalSeconds: Int = HealthCommandConfig.defaultIntervalSeconds,
                timeoutSeconds: Int = HealthCommandConfig.defaultTimeoutSeconds, warningExitCodes: [Int32] = []) {
        self.command = command
        self.intervalSeconds = intervalSeconds
        self.timeoutSeconds = timeoutSeconds
        self.warningExitCodes = warningExitCodes
    }

    enum CodingKeys: String, CodingKey, CaseIterable {
        case command, intervalSeconds, timeoutSeconds, warningExitCodes
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        command = try c.decode([String].self, forKey: .command)
        intervalSeconds = try c.decodeIfPresent(Int.self, forKey: .intervalSeconds) ?? Self.defaultIntervalSeconds
        timeoutSeconds = try c.decodeIfPresent(Int.self, forKey: .timeoutSeconds) ?? Self.defaultTimeoutSeconds
        warningExitCodes = try c.decodeIfPresent([Int32].self, forKey: .warningExitCodes) ?? []
    }

    /// A result older than this is stale (3 intervals).
    public var staleAfter: TimeInterval { TimeInterval(3 * intervalSeconds) }
}

/// Rule 8: a JSON file the job writes after each run, e.g. pi-backup-fetch's `last-run.json`.
public struct ReceiptConfig: Equatable, Codable, Sendable {
    public var path: String
    /// Boolean key: did the job report its result to Uptime Kuma.
    public var reportedKey: String
    /// String key compared against `okValues`.
    public var statusKey: String
    public var okValues: [String]

    public init(path: String, reportedKey: String = "reported", statusKey: String = "status", okValues: [String] = ["up"]) {
        self.path = path
        self.reportedKey = reportedKey
        self.statusKey = statusKey
        self.okValues = okValues
    }

    enum CodingKeys: String, CodingKey, CaseIterable {
        case path, reportedKey, statusKey, okValues
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        path = try c.decode(String.self, forKey: .path)
        reportedKey = try c.decodeIfPresent(String.self, forKey: .reportedKey) ?? "reported"
        statusKey = try c.decodeIfPresent(String.self, forKey: .statusKey) ?? "status"
        okValues = try c.decodeIfPresent([String].self, forKey: .okValues) ?? ["up"]
    }
}
