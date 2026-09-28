import Foundation

/// Per-agent settings from config that change how the rules judge it.
public struct AgentHealthOptions: Equatable, Sendable {
    public var hidden: Bool
    /// Heartbeat unloaded the agent on purpose (Unload… in the menu).
    public var pausedByHeartbeat: Bool
    /// Exit codes that count as success besides 0.
    public var ignoreExitCodes: Set<Int32>
    /// Overrides the interval-derived allowance and applies an age check to any schedule.
    public var maxAgeSeconds: Int?
    /// Overrides `KeepAlivePolicy.expectsRunning`.
    public var expectsRunning: Bool?
    /// How long after a calendar slot (or the wake that follows it) a run may still be missing.
    public var graceSeconds: Int
    /// Rule 7: the configured health command, if any (its interval decides when a result is stale).
    public var health: HealthCommandConfig?

    public init(hidden: Bool = false, pausedByHeartbeat: Bool = false, ignoreExitCodes: Set<Int32> = [],
                maxAgeSeconds: Int? = nil, expectsRunning: Bool? = nil,
                graceSeconds: Int = HealthEvaluator.defaultGraceSeconds, health: HealthCommandConfig? = nil) {
        self.hidden = hidden
        self.pausedByHeartbeat = pausedByHeartbeat
        self.ignoreExitCodes = ignoreExitCodes
        self.maxAgeSeconds = maxAgeSeconds
        self.expectsRunning = expectsRunning
        self.graceSeconds = graceSeconds
        self.health = health
    }
}

/// What earlier polls remembered about an agent (kept in state.json by a later session).
public struct AgentHistory: Equatable, Sendable {
    /// Consecutive earlier polls where a KeepAlive agent had no PID (this poll not included).
    public var previousNotRunningPolls: Int
    /// When `runs` was last seen going up for a running KeepAlive daemon, i.e. when it restarted.
    public var lastRestartAt: Date?
    /// When Heartbeat first saw the service loaded in its current launchd session.
    public var loadedObservedAt: Date?

    public init(previousNotRunningPolls: Int = 0, lastRestartAt: Date? = nil, loadedObservedAt: Date? = nil) {
        self.previousNotRunningPolls = previousNotRunningPolls
        self.lastRestartAt = lastRestartAt
        self.loadedObservedAt = loadedObservedAt
    }
}

/// Everything the rules look at for one agent.
public struct HealthInput: Equatable, Sendable {
    public var agent: AgentDefinition
    public var status: ServiceStatus
    public var evidence: Evidence
    public var options: AgentHealthOptions
    public var history: AgentHistory
    /// Rule 7: the newest health command result; `nil` until the first run finishes.
    public var healthCheck: HealthCheckResult?
    /// Rule 8: the receipt as read this poll; `nil` when no receipt is configured.
    public var receipt: ReceiptStatus?

    public init(agent: AgentDefinition, status: ServiceStatus, evidence: Evidence = .none,
                options: AgentHealthOptions = AgentHealthOptions(), history: AgentHistory = AgentHistory(),
                healthCheck: HealthCheckResult? = nil, receipt: ReceiptStatus? = nil) {
        self.agent = agent
        self.status = status
        self.evidence = evidence
        self.options = options
        self.history = history
        self.healthCheck = healthCheck
        self.receipt = receipt
    }
}

/// The clock, power history and calendar the rules run against; always injected, never read inside.
public struct HealthContext: Sendable {
    public var now: Date
    public var power: PowerTimeline
    public var calendar: Calendar

    public init(now: Date, power: PowerTimeline, calendar: Calendar) {
        self.now = now
        self.power = power
        self.calendar = calendar
    }
}

/// Health rules 1-8 from the plan. Pure: same input and context, same verdict.
public enum HealthEvaluator {
    public static let defaultGraceSeconds = 900
    /// A running KeepAlive daemon that restarted within this window is amber, not red.
    public static let restartWindow: TimeInterval = 3600
    /// Evidence up to this long before a calendar slot still counts for it (log mtime vs. launch jitter).
    public static let slotSlack: TimeInterval = 60

    public static func evaluate(_ input: HealthInput, context: HealthContext) -> HealthVerdict {
        let label = input.agent.label

        // Rule 1: hidden agents are excluded.
        if input.options.hidden {
            return HealthVerdict(label: label, severity: .hidden,
                                 reasons: [HealthReason(rule: 1, code: .hidden, severity: .hidden, message: "Hidden in config")])
        }

        // Rules 7-8 judge the job's output, not launchd, so they apply whenever the agent isn't paused.
        let outputReasons = [healthCheckReason(input, context: context), receiptReason(input.receipt)].compactMap { $0 }

        let runtime: ServiceRuntime
        switch input.status {
        case .notLoaded:
            // Rule 2: unloaded on purpose.
            if input.agent.disabled || input.options.pausedByHeartbeat {
                let message = input.options.pausedByHeartbeat ? "Paused (unloaded by Heartbeat)" : "Disabled in plist"
                return verdict(label, [HealthReason(rule: 2, code: .paused, severity: .paused, message: message)])
            }
            // Rule 3: unloaded otherwise.
            return verdict(label, [HealthReason(rule: 3, code: .notLoaded, severity: .warning, message: "Not loaded")]
                + outputReasons)
        case .unknown(let reason):
            return verdict(label, [HealthReason(rule: 3, code: .unreadableState, severity: .warning,
                                                message: "Cannot read launchctl state: \(reason)")] + outputReasons)
        case .loaded(let loaded):
            runtime = loaded
        }

        var reasons: [HealthReason] = []
        let isRunning = runtime.pid != nil || runtime.state == .running
        let expectsRunning = input.options.expectsRunning ?? input.agent.keepAlive.expectsRunning

        // Rule 4: last exit.
        if let failure = failureDescription(runtime.lastExit, ignoring: input.options.ignoreExitCodes) {
            if expectsRunning && isRunning {
                if let restart = input.history.lastRestartAt, context.now.timeIntervalSince(restart) <= restartWindow {
                    reasons.append(HealthReason(rule: 4, code: .restarted, severity: .warning,
                                                message: "Restarted after \(failure.short)"))
                }
            } else {
                reasons.append(HealthReason(rule: 4, code: failure.code, severity: .failing, message: failure.message))
            }
        }

        // Rule 5: a KeepAlive daemon should have a PID; one miss may be a ThrottleInterval respawn.
        let keepAliveMissing = expectsRunning && runtime.pid == nil
        if keepAliveMissing {
            let severity: Severity = input.history.previousNotRunningPolls >= 1 ? .failing : .warning
            reasons.append(HealthReason(rule: 5, code: .notRunning, severity: severity, message: "Not running (KeepAlive)"))
        }

        // Rule 6: overdue.
        let overdue = overdueCheck(input, isRunning: isRunning, context: context)
        if let reason = overdue.reason { reasons.append(reason) }

        reasons += outputReasons

        var result = verdict(label, reasons)
        result.keepAliveMissing = keepAliveMissing
        result.overdue = overdue.trace
        return result
    }

    static func verdict(_ label: String, _ reasons: [HealthReason]) -> HealthVerdict {
        // Stable sort: equal severities keep rule order.
        let sorted = reasons.enumerated().sorted {
            $0.element.severity != $1.element.severity ? $0.element.severity > $1.element.severity : $0.offset < $1.offset
        }.map(\.element)
        return HealthVerdict(label: label, severity: sorted.first?.severity ?? .ok, reasons: sorted)
    }

    struct Failure {
        var code: HealthReasonCode
        var message: String
        var short: String
    }

    static func failureDescription(_ lastExit: LastExit?, ignoring ignored: Set<Int32>) -> Failure? {
        switch lastExit {
        case nil, .neverExited:
            return nil
        case .exited(let code):
            guard code != 0, !ignored.contains(code) else { return nil }
            return Failure(code: .exitCode, message: "Exited with status \(code)", short: "exit \(code)")
        case .signaled(let signal, let name):
            let text = name.isEmpty ? "signal \(signal)" : "signal \(name) (\(signal))"
            return Failure(code: .signal, message: "Killed by \(text)", short: text)
        }
    }

    static func overdueCheck(_ input: HealthInput, isRunning: Bool, context: HealthContext)
        -> (reason: HealthReason?, trace: OverdueTrace?) {
        let now = context.now
        let evidence = input.evidence
        let maxAge = input.options.maxAgeSeconds

        var trace: OverdueTrace
        switch (input.agent.schedule, maxAge) {
        case (.calendar(let entries), nil):
            trace = OverdueTrace(kind: .calendar, evidence: evidence.latest,
                                 nextExpected: ScheduleMath.nextSlot(entries, after: now, calendar: context.calendar))
        case (.interval(let seconds), _):
            let allowed = maxAge ?? MaxAgeSuggestion.defaultAllowedAge(interval: seconds)
            trace = OverdueTrace(kind: .maxAge, evidence: evidence.latest, allowedAge: TimeInterval(allowed),
                                 nextExpected: evidence.latest?.addingTimeInterval(TimeInterval(seconds)))
        case (let schedule, let maxAge?):
            var nextExpected: Date?
            if case .calendar(let entries) = schedule {
                nextExpected = ScheduleMath.nextSlot(entries, after: now, calendar: context.calendar)
            }
            trace = OverdueTrace(kind: .maxAge, evidence: evidence.latest, allowedAge: TimeInterval(maxAge),
                                 nextExpected: nextExpected)
        default:
            return (nil, nil)
        }

        if isRunning {
            trace.skipped = "running now"
            return (nil, trace)
        }
        guard evidence.hasSource else {
            trace.skipped = "no evidence source"
            return (HealthReason(rule: 6, code: .cannotVerify, severity: .warning, message: "Cannot verify last run"), trace)
        }

        switch trace.kind {
        case .maxAge:
            let allowed = trace.allowedAge ?? 0
            let awake = context.power.awakeSince
            let t0 = max(awake, input.history.loadedObservedAt ?? awake)
            trace.t0 = t0
            guard now.timeIntervalSince(t0) > allowed else {
                trace.skipped = "awake or loaded for less than the allowed age"
                return (nil, trace)
            }
            guard let latest = evidence.latest else {
                return (overdue("Overdue: no run recorded"), trace)
            }
            let age = now.timeIntervalSince(latest)
            guard age > allowed else { return (nil, trace) }
            return (overdue("Overdue: last run \(ScheduleDescription.age(Int(age))) ago (allowed \(ScheduleDescription.duration(Int(allowed))))"), trace)

        case .calendar:
            guard case .calendar(let entries) = input.agent.schedule,
                  let slot = ScheduleMath.latestSlot(entries, atOrBefore: now, calendar: context.calendar) else {
                trace.skipped = "no past slot"
                return (nil, trace)
            }
            trace.latestSlot = slot
            let boot = context.power.bootTime
            let floor = max(boot, input.history.loadedObservedAt ?? boot)
            guard slot >= floor else {
                trace.skipped = "latest slot is before boot or load"
                return (nil, trace)
            }
            // launchd runs a slot missed during sleep once, on wake.
            let effective = max(slot, context.power.lastWake ?? slot)
            let deadline = effective.addingTimeInterval(TimeInterval(input.options.graceSeconds))
            trace.deadline = deadline
            if let latest = evidence.latest, latest >= slot.addingTimeInterval(-slotSlack) { return (nil, trace) }
            guard now > deadline else { return (nil, trace) }
            return (overdue("Overdue: missed the \(slotText(slot, calendar: context.calendar)) run"), trace)
        }
    }

    /// Rule 7: exit ≠ 0 → red; a `warningExitCodes` exit, a timeout or a failed start → amber. A result older
    /// than 3 intervals is stale (amber) once the Mac has been awake that long, i.e. the checks stopped running.
    static func healthCheckReason(_ input: HealthInput, context: HealthContext) -> HealthReason? {
        guard let config = input.options.health, let result = input.healthCheck else { return nil }
        let age = context.now.timeIntervalSince(result.finishedAt)
        if age > config.staleAfter && context.now.timeIntervalSince(context.power.awakeSince) > config.staleAfter {
            return HealthReason(rule: 7, code: .healthCheckStale, severity: .warning,
                                message: "Health check result is stale (last ran \(ScheduleDescription.age(Int(age))) ago)")
        }
        let suffix = result.detail.map { ": \($0)" } ?? ""
        switch result.outcome {
        case .ok:
            return nil
        case .failed(let code):
            return HealthReason(rule: 7, code: .healthCheckFailed, severity: .failing,
                                message: "Health check failed (exit \(code))\(suffix)")
        case .killed(let signal):
            return HealthReason(rule: 7, code: .healthCheckFailed, severity: .failing,
                                message: "Health check killed by signal \(signal)\(suffix)")
        case .warning(let code):
            return HealthReason(rule: 7, code: .healthCheckWarning, severity: .warning,
                                message: "Health check could not check (exit \(code))\(suffix)")
        case .timedOut:
            return HealthReason(rule: 7, code: .healthCheckWarning, severity: .warning,
                                message: "Health check timed out after \(ScheduleDescription.duration(config.timeoutSeconds))")
        case .couldNotStart(let message):
            return HealthReason(rule: 7, code: .healthCheckWarning, severity: .warning,
                                message: "Health check could not start: \(message)")
        }
    }

    /// Rule 8: status outside `okValues` → red; ran but not reported to Kuma, or no usable receipt → amber.
    static func receiptReason(_ receipt: ReceiptStatus?) -> HealthReason? {
        switch receipt {
        case nil, .ok:
            return nil
        case .badStatus(let status, _):
            return HealthReason(rule: 8, code: .receiptStatus, severity: .failing, message: "Receipt status is \(status)")
        case .notReported:
            return HealthReason(rule: 8, code: .receiptNotReported, severity: .warning,
                                message: "Ran locally but not reported to Kuma")
        case .missing(let path):
            return HealthReason(rule: 8, code: .receiptUnavailable, severity: .warning,
                                message: "No receipt at \((path as NSString).abbreviatingWithTildeInPath)")
        case .unreadable(let reason):
            return HealthReason(rule: 8, code: .receiptUnavailable, severity: .warning, message: "Unreadable receipt: \(reason)")
        }
    }

    static func overdue(_ message: String) -> HealthReason {
        HealthReason(rule: 6, code: .overdue, severity: .failing, message: message)
    }

    static func slotText(_ date: Date, calendar: Calendar) -> String {
        let parts = calendar.dateComponents([.month, .day, .hour, .minute], from: date)
        let month = ScheduleDescription.monthNames[(parts.month ?? 1) - 1]
        return "\(month) \(parts.day ?? 0) \(ScheduleDescription.pad(parts.hour ?? 0)):\(ScheduleDescription.pad(parts.minute ?? 0))"
    }
}
