import Foundation

extension StatusFormatter {

    /// Everything that went into one agent's verdict, rule by rule.
    public func explain(_ agent: AgentSnapshot, snapshot: Snapshot) -> String {
        let now = snapshot.takenAt
        let definition = agent.agent
        var out: [String] = []
        func section(_ title: String, _ rows: [(String, String)]) {
            out.append("")
            out.append(title)
            let width = rows.map(\.0.count).max() ?? 0
            out += rows.map { "  \($0.0.padding(toLength: width, withPad: " ", startingAt: 0))  \($0.1)" }
        }

        out.append("\(agent.label) — \(agent.severity.rawValue.uppercased())")
        out += agent.verdict.reasons.map { "  \($0.message) (rule \($0.rule))" }

        var plist = tilde(definition.plistPath)
        if definition.resolvedPlistPath != definition.plistPath { plist += " -> \(tilde(definition.resolvedPlistPath))" }
        section("Definition", [
            ("plist", plist),
            ("program", ([definition.program].compactMap { $0 } + definition.programArguments).map(tilde).joined(separator: " ")),
            ("schedule", ScheduleDescription.describe(definition)),
            ("keepAlive", keepAliveText(definition.keepAlive)),
            ("run at load", yesNo(definition.runAtLoad)),
            ("disabled", yesNo(definition.disabled)),
            ("logs", definition.logPaths.isEmpty ? "none" : definition.logPaths.map(tilde).joined(separator: ", ")),
        ])

        var launchd: [(String, String)] = [("state", stateText(agent.status))]
        if case .loaded(let runtime) = agent.status {
            launchd.append(("pid", runtime.pid.map(String.init) ?? "-"))
            launchd.append(("runs", runtime.runs.map(String.init) ?? "-"))
            launchd.append(("last exit", runtimeColumns(agent.status)[2]))
        } else if case .unknown(let reason) = agent.status {
            launchd.append(("reason", reason))
        }
        section("launchd", launchd)

        section("Config (\(snapshot.configSource.name))", configRows(agent))

        let history = agent.history
        section("History (state.json)", [
            ("loaded since", date(history.loadedObservedAt, now: now)),
            ("last restart", date(history.lastRestartAt, now: now)),
            ("not-running streak", "\(history.previousNotRunningPolls) earlier polls"),
        ])

        var evidence = agent.evidenceSources.map { source in
            let newest = source.date != nil && source.date == agent.evidence.latest ? "  <- newest" : ""
            let when = source.date.map { date($0, now: now) } ?? "missing"
            return ("\(source.kind.rawValue)", "\(tilde(source.name)): \(when)\(newest)")
        }
        if evidence.isEmpty { evidence = [("none", "no log, evidence path, receipt or runs ledger")] }
        section("Evidence", evidence)

        section("Power", [
            ("boot", date(snapshot.power.bootTime, now: now)),
            ("last wake", date(snapshot.power.lastWake, now: now, missing: "no sleep since boot")),
            ("awake since", date(snapshot.power.awakeSince, now: now)),
            ("now", stamp(now)),
        ])

        if let trace = agent.verdict.overdue {
            var rows: [(String, String)] = [("kind", trace.kind == .calendar ? "calendar slot" : "max age")]
            rows.append(("evidence", date(trace.evidence, now: now)))
            if let allowed = trace.allowedAge { rows.append(("allowed age", ScheduleDescription.duration(Int(allowed)))) }
            if let t0 = trace.t0 { rows.append(("T0 (awake/loaded)", date(t0, now: now))) }
            if let slot = trace.latestSlot { rows.append(("S (latest slot)", date(slot, now: now))) }
            if let deadline = trace.deadline { rows.append(("E + grace (deadline)", date(deadline, now: now))) }
            if let skipped = trace.skipped { rows.append(("skipped", skipped)) }
            rows.append(("next expected", date(trace.nextExpected, now: now)))
            section("Overdue check (rule 6)", rows)
        }

        if let result = agent.healthCheck {
            var rows = [("finished", date(result.finishedAt, now: now)), ("outcome", outcomeText(result.outcome))]
            if let detail = result.detail { rows.append(("output", detail)) }
            section("Health command (rule 7)", rows)
        }

        section("Rules", ruleTrace(agent))
        out.append("")
        out.append("Verdict: \(agent.severity.rawValue)")
        return out.joined(separator: "\n")
    }

    /// One line per rule: the reasons it added, or why it passed or didn't apply.
    func ruleTrace(_ agent: AgentSnapshot) -> [(String, String)] {
        let fired = Dictionary(grouping: agent.verdict.reasons, by: \.rule)
        func line(_ rule: Int, _ passed: String) -> (String, String) {
            let key = "\(rule)"
            guard let reasons = fired[rule] else { return (key, "pass: \(passed)") }
            return (key, reasons.map { "\($0.severity.rawValue): \($0.message)" }.joined(separator: "; "))
        }
        if agent.severity == .hidden { return [line(1, "")] }
        let runtime = agent.status.runtime
        let expects = agent.config.expectsRunning ?? agent.agent.keepAlive.expectsRunning
        var trace = [
            ("1", "pass: not hidden"),
            line(2, runtime == nil ? "not disabled or paused" : "loaded"),
            line(3, runtime == nil ? "n/a" : "loaded and readable"),
        ]
        if agent.severity == .paused { return trace }
        guard runtime != nil else {
            return trace + [("4-6", "n/a: not loaded"), line(7, healthPass(agent)), line(8, receiptPass(agent))]
        }
        let exitText: String = switch runtime?.lastExit {
        case .exited(let code)?: "last exit \(code)"
        case .neverExited?: "never exited"
        default: "no exit recorded"
        }
        trace.append(line(4, exitText))
        trace.append(line(5, expects ? "running (pid \(runtime?.pid.map(String.init) ?? "-"))" : "KeepAlive not expected"))
        let overdue = agent.verdict.overdue
        trace.append(line(6, overdue == nil ? "no schedule to check" : overdue?.skipped.map { "skipped: \($0)" } ?? "on time"))
        trace.append(line(7, healthPass(agent)))
        trace.append(line(8, receiptPass(agent)))
        return trace
    }

    func healthPass(_ agent: AgentSnapshot) -> String {
        guard agent.config.health != nil else { return "no health command" }
        return agent.healthCheck == nil ? "no result yet (use --health to run it)" : "health command ok"
    }

    func receiptPass(_ agent: AgentSnapshot) -> String {
        agent.config.receipt == nil ? "no receipt" : "receipt ok"
    }

    func configRows(_ agent: AgentSnapshot) -> [(String, String)] {
        let config = agent.config
        var rows: [(String, String)] = []
        if let name = config.displayName { rows.append(("displayName", name)) }
        if config.hidden { rows.append(("hidden", "yes")) }
        rows.append(("notify", yesNo(agent.notify)))
        if !config.ignoreExitCodes.isEmpty { rows.append(("ignoreExitCodes", config.ignoreExitCodes.map(String.init).joined(separator: ", "))) }
        if let maxAge = config.maxAgeSeconds { rows.append(("maxAgeSeconds", "\(maxAge) (\(ScheduleDescription.duration(maxAge)))")) }
        if let expects = config.expectsRunning { rows.append(("expectsRunning", yesNo(expects))) }
        rows.append(("graceSeconds", "\(config.graceSeconds ?? HealthEvaluator.defaultGraceSeconds)"))
        if !config.evidencePaths.isEmpty { rows.append(("evidencePaths", config.evidencePaths.map(tilde).joined(separator: ", "))) }
        if let health = config.health {
            rows.append(("health", "\(health.command.map(tilde).joined(separator: " ")) every \(ScheduleDescription.duration(health.intervalSeconds)), timeout \(ScheduleDescription.duration(health.timeoutSeconds))"))
        }
        if let receipt = config.receipt { rows.append(("receipt", tilde(receipt.path))) }
        return rows
    }

    func outcomeText(_ outcome: HealthCheckResult.Outcome) -> String {
        switch outcome {
        case .ok: "ok"
        case .failed(let code): "failed (exit \(code))"
        case .warning(let code): "warning (exit \(code))"
        case .killed(let signal): "killed by signal \(signal)"
        case .timedOut: "timed out"
        case .couldNotStart(let message): "could not start: \(message)"
        }
    }

    func receiptText(_ status: ReceiptStatus) -> String {
        switch status {
        case .ok: "reported to Kuma"
        case .notReported: "not reported to Kuma"
        case .badStatus(let status, _): "status \(status)"
        case .missing(let path): "missing (\(tilde(path)))"
        case .unreadable(let reason): "unreadable: \(reason)"
        }
    }

    /// "3 min ago", or "in 3 min" for a time after `now`.
    func ago(_ date: Date, now: Date) -> String {
        let delta = Int(now.timeIntervalSince(date))
        return delta >= 0 ? "\(ScheduleDescription.age(delta)) ago" : "in \(ScheduleDescription.age(-delta))"
    }

    func keepAliveText(_ policy: KeepAlivePolicy) -> String {
        switch policy {
        case .none: "no"
        case .always: "always"
        case .conditional(let successfulExit, let other):
            ((successfulExit.map { ["SuccessfulExit: \($0)"] } ?? []) + other).joined(separator: ", ")
        }
    }
}
