import Foundation

/// The overall verdict across every agent; also `heartbeatctl status`'s exit code.
public enum OverallStatus: String, Codable, Sendable, CaseIterable {
    case ok
    case warning
    case failing
    /// Heartbeat couldn't look at all (LaunchAgents directory unreadable, no boot time).
    case unknown

    /// 0 ok / 1 warn / 2 failing / 3 unknown.
    public var exitCode: Int32 {
        switch self {
        case .ok: 0
        case .warning: 1
        case .failing: 2
        case .unknown: 3
        }
    }
}

/// One place an agent's evidence of a run can come from, for `explain`.
public struct EvidenceSource: Equatable, Sendable {
    public enum Kind: String, Codable, Sendable {
        case log
        case evidencePath
        case runsLedger
        case receipt
    }

    public var kind: Kind
    /// A file path, or a description for the ledger and receipt.
    public var name: String
    /// `nil` when the file doesn't exist (yet) or the source has no time.
    public var date: Date?

    public init(kind: Kind, name: String, date: Date?) {
        self.kind = kind
        self.name = name
        self.date = date
    }
}

/// Everything known about one agent at one poll.
public struct AgentSnapshot: Equatable, Sendable {
    public var agent: AgentDefinition
    public var config: AgentConfig
    public var status: ServiceStatus
    public var evidence: Evidence
    public var evidenceSources: [EvidenceSource]
    public var history: AgentHistory
    public var healthCheck: HealthCheckResult?
    public var receipt: ReceiptStatus?
    public var verdict: HealthVerdict
    /// Whether a banner may be shown (global `notifications` and the agent's `notify`).
    public var notify: Bool

    public var label: String { agent.label }
    public var severity: Severity { verdict.severity }

    /// `displayName` from config, else the label without the prefix.
    public func name(labelPrefix: String) -> String {
        if let displayName = config.displayName { return displayName }
        guard label.hasPrefix(labelPrefix), label.count > labelPrefix.count else { return label }
        return String(label.dropFirst(labelPrefix.count))
    }
}

/// One poll: every discovered agent evaluated, plus what went wrong around them.
public struct Snapshot: Equatable, Sendable {
    public var takenAt: Date
    public var power: PowerTimeline
    public var config: HeartbeatConfig
    public var configSource: ConfigLoadResult.Source
    public var configWarnings: [String]
    public var configError: String?
    /// Agents sorted by label, hidden ones included.
    public var agents: [AgentSnapshot]
    public var discoveryProblems: [DiscoveryProblem]
    /// Set when the LaunchAgents directory or the boot time couldn't be read; makes the overall `unknown`.
    public var fatalError: String?
    /// The state after this poll (the app saves it; the CLI throws it away).
    public var state: HeartbeatState

    public func agent(_ label: String) -> AgentSnapshot? {
        agents.first { $0.label == label }
    }

    public func count(_ severity: Severity) -> Int {
        agents.count { $0.severity == severity }
    }

    /// Red if any agent is red; amber if any is amber, the config is broken, a plist couldn't be read or
    /// nothing was discovered; else ok. Paused and hidden agents don't count.
    public var overall: OverallStatus {
        if fatalError != nil { return .unknown }
        let worst = agents.map(\.severity).max() ?? .ok
        if worst == .failing { return .failing }
        if worst == .warning || configError != nil || !discoveryProblems.isEmpty
            || !agents.contains(where: { $0.severity != .hidden }) {
            return .warning
        }
        return .ok
    }

    /// Exit status for one agent's severity (`heartbeatctl explain`): failing 2, warning 1, else 0.
    public static func exitCode(_ severity: Severity) -> Int32 {
        switch severity {
        case .failing: OverallStatus.failing.exitCode
        case .warning: OverallStatus.warning.exitCode
        case .hidden, .ok, .paused: OverallStatus.ok.exitCode
        }
    }
}

/// Discovers agents, reads launchd in parallel, gathers evidence and evaluates the rules. Shared by the app
/// (which saves the returned state) and `heartbeatctl` (which reads state.json and changes nothing on disk).
public struct SnapshotBuilder: Sendable {
    public enum LedgerMode: Sendable {
        /// The app: record this poll into the ledger (runs increments, load times, streaks).
        case record
        /// The CLI: judge with the saved history as it is. A runs increment seen by a one-off CLI call
        /// would date a run at "now", which is only the time someone looked.
        case readOnly
    }

    public var discover: @Sendable (_ labelPrefix: String) throws -> DiscoveryResult
    public var launchctl: LaunchctlClient
    /// Runs health commands when `runHealthChecks` is set.
    public var healthRunner: any CommandRunning
    public var readReceipt: @Sendable (ReceiptConfig) -> ReceiptStatus
    /// mtime of a path (following symlinks), or `nil` if it doesn't exist.
    public var modificationDate: @Sendable (String) -> Date?
    public var power: @Sendable () -> PowerTimeline?
    public var now: @Sendable () -> Date
    public var calendar: Calendar
    public var maxConcurrentPrints: Int

    public init(
        discover: @escaping @Sendable (String) throws -> DiscoveryResult = { try AgentDiscovery(labelPrefix: $0).discover() },
        launchctl: LaunchctlClient = LaunchctlClient(),
        healthRunner: any CommandRunning = CommandRunner(environment: HealthCheck.environment()),
        readReceipt: @escaping @Sendable (ReceiptConfig) -> ReceiptStatus = ReceiptReader.read,
        modificationDate: @escaping @Sendable (String) -> Date? = SnapshotBuilder.fileModificationDate,
        power: @escaping @Sendable () -> PowerTimeline? = PowerTimeline.current,
        now: @escaping @Sendable () -> Date = Date.init,
        calendar: Calendar = .autoupdatingCurrent,
        maxConcurrentPrints: Int = 4
    ) {
        self.discover = discover
        self.launchctl = launchctl
        self.healthRunner = healthRunner
        self.readReceipt = readReceipt
        self.modificationDate = modificationDate
        self.power = power
        self.now = now
        self.calendar = calendar
        self.maxConcurrentPrints = maxConcurrentPrints
    }

    /// Builds a snapshot off the caller's thread (launchctl and health commands block).
    public func build(config: ConfigLoadResult, state: HeartbeatState, ledger: LedgerMode,
                      runHealthChecks: Bool = false) async -> Snapshot {
        await withCheckedContinuation { continuation in
            DispatchQueue.global(qos: .utility).async {
                continuation.resume(returning: buildSync(config: config, state: state, ledger: ledger,
                                                         runHealthChecks: runHealthChecks))
            }
        }
    }

    /// Poll order: note the boot, print every agent, run health commands if asked, then per agent record the
    /// ledger, gather evidence and evaluate with the updated history, and feed the verdict back.
    public func buildSync(config loaded: ConfigLoadResult, state initial: HeartbeatState, ledger: LedgerMode,
                          runHealthChecks: Bool = false) -> Snapshot {
        let config = loaded.config
        var state = initial
        var fatal: String?

        let power: PowerTimeline
        if let current = self.power() {
            power = current
            Ledger.noteBoot(current.bootTime, in: &state)
        } else {
            power = PowerTimeline(bootTime: .distantPast)
            fatal = "cannot read kern.boottime"
        }

        let discovery: DiscoveryResult
        do {
            discovery = try discover(config.labelPrefix)
        } catch {
            discovery = DiscoveryResult()
            fatal = "cannot list LaunchAgents: \(error.localizedDescription)"
        }

        let statuses = launchctl.print(discovery.agents.map(\.label), maxConcurrent: maxConcurrentPrints)

        // Rules 7-8 are skipped for hidden and paused agents, so don't run or read anything for them.
        func isActive(_ agent: AgentDefinition, _ status: ServiceStatus) -> Bool {
            let options = config.agent(agent.label)
            let paused = status == .notLoaded && (agent.disabled || state[agent.label].paused)
            return !options.hidden && !paused
        }
        if runHealthChecks {
            for (agent, status) in zip(discovery.agents, statuses) where isActive(agent, status) {
                guard let health = config.agent(agent.label).health else { continue }
                state[agent.label].lastHealthCheck = HealthCheck.run(health, runner: healthRunner, now: self.now)
            }
        }

        // Taken after the health commands so their results are never newer than the snapshot.
        let now = self.now()
        let context = HealthContext(now: now, power: power, calendar: calendar)

        var agents: [AgentSnapshot] = []
        for (agent, status) in zip(discovery.agents, statuses) {
            let label = agent.label
            let agentConfig = config.agent(label)
            let expectsRunning = agentConfig.expectsRunning ?? agent.keepAlive.expectsRunning
            if ledger == .record {
                Ledger.record(status, keepAlive: expectsRunning, now: now, in: &state[label])
            }
            let options = agentConfig.healthOptions(pausedByHeartbeat: state[label].paused)
            let receipt = isActive(agent, status) ? agentConfig.receipt.map(readReceipt) : nil
            let (evidence, sources) = gatherEvidence(agent, config: agentConfig, state: state[label], receipt: receipt)
            let healthCheck = agentConfig.health == nil ? nil : state[label].lastHealthCheck

            let input = HealthInput(agent: agent, status: status, evidence: evidence, options: options,
                                    history: state[label].history, healthCheck: healthCheck, receipt: receipt)
            let verdict = HealthEvaluator.evaluate(input, context: context)
            if ledger == .record {
                Ledger.record(verdict, in: &state[label])
            }
            agents.append(AgentSnapshot(
                agent: agent, config: agentConfig, status: status, evidence: evidence, evidenceSources: sources,
                history: input.history, healthCheck: healthCheck, receipt: receipt, verdict: verdict,
                notify: config.notifies(label)))
        }
        if ledger == .record, fatal == nil {
            Ledger.prune(&state, keeping: Set(discovery.agents.map(\.label)))
        }

        return Snapshot(
            takenAt: now, power: power, config: config, configSource: loaded.source, configWarnings: loaded.warnings,
            configError: loaded.error, agents: agents, discoveryProblems: discovery.problems, fatalError: fatal,
            state: state)
    }

    /// Evidence = the newest of the stdout/stderr log mtimes, config `evidencePaths` mtimes, the ledger's runs
    /// increment and the receipt's `finished`. A declared log or evidence path counts as a source even before
    /// the file exists (launchd creates the log when it starts the job, so a missing one means no run yet).
    func gatherEvidence(_ agent: AgentDefinition, config: AgentConfig, state: AgentState, receipt: ReceiptStatus?)
        -> (Evidence, [EvidenceSource]) {
        var sources: [EvidenceSource] = []
        for path in agent.logPaths {
            sources.append(EvidenceSource(kind: .log, name: path, date: modificationDate(path)))
        }
        for path in config.evidencePaths {
            sources.append(EvidenceSource(kind: .evidencePath, name: path, date: modificationDate(path)))
        }
        if let date = Ledger.evidenceDate(state) {
            sources.append(EvidenceSource(kind: .runsLedger, name: "runs ledger", date: date))
        }
        if let receipt, let path = config.receipt?.path {
            sources.append(EvidenceSource(kind: .receipt, name: path, date: receipt.finished))
        }
        let newest = sources.filter { $0.date != nil }.max { $0.date! < $1.date! }
        let evidence = Evidence(hasSource: !sources.isEmpty, latest: newest?.date, origin: newest?.name)
        return (evidence, sources)
    }

    /// `stat` follows symlinks, so a stowed log's target mtime is used.
    public static let fileModificationDate: @Sendable (String) -> Date? = { path in
        var info = stat()
        guard stat(path, &info) == 0 else { return nil }
        let time = info.st_mtimespec
        return Date(timeIntervalSince1970: TimeInterval(time.tv_sec) + TimeInterval(time.tv_nsec) / 1_000_000_000)
    }
}
