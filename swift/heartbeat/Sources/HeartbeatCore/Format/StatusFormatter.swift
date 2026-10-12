import Foundation

/// Text and JSON renderings of a `Snapshot` for `heartbeatctl` (and the app's header and row details).
/// JSON keys are part of the CLI's interface: the tests pin them.
public struct StatusFormatter: Sendable {
    public var calendar: Calendar
    public var home: String

    public init(calendar: Calendar = .autoupdatingCurrent, home: String = NSHomeDirectory()) {
        self.calendar = calendar
        self.home = home
    }

    // MARK: - Shared pieces

    /// "2 failing · 1 warning — checked 12:41", "All 14 agents ok — checked 12:41".
    public func headline(_ snapshot: Snapshot) -> String {
        let checked = "checked \(clock(snapshot.takenAt))"
        if let fatal = snapshot.fatalError { return "Cannot check: \(fatal) — \(checked)" }
        var parts: [String] = []
        let failing = snapshot.count(.failing), warning = snapshot.count(.warning)
        if failing > 0 { parts.append("\(failing) failing") }
        if warning > 0 { parts.append("\(warning) \(warning == 1 ? "warning" : "warnings")") }
        if snapshot.configError != nil { parts.append("config error") }
        let problems = snapshot.discoveryProblems.count
        if problems > 0 { parts.append("\(problems) plist \(problems == 1 ? "problem" : "problems")") }
        let visible = snapshot.agents.count { $0.severity != .hidden }
        let paused = snapshot.count(.paused), ok = snapshot.count(.ok)
        if snapshot.agents.isEmpty {
            parts.append("No agents found")
        } else if visible == 0 {
            let hidden = snapshot.count(.hidden)
            parts.append("\(hidden) \(hidden == 1 ? "agent" : "agents") hidden")
        } else if paused > 0 {
            if parts.isEmpty, ok > 0 { parts.append("\(ok) ok") }
            parts.append("\(paused) paused")
        } else if parts.isEmpty {
            parts.append("All \(visible) \(visible == 1 ? "agent" : "agents") ok")
        }
        return parts.joined(separator: " · ") + " — " + checked
    }

    /// One-line row detail: "Exited with status 1 · log 3 h ago", "every 15 min · log 4 min ago".
    public func detail(_ agent: AgentSnapshot, now: Date) -> String {
        var parts = [agent.verdict.summary ?? ScheduleDescription.describe(agent.agent)]
        if let latest = agent.evidence.latest, agent.severity != .hidden {
            let kind = agent.evidenceSources.first { $0.date == latest }?.kind
            let word = switch kind {
            case .log?: "log"
            case .runsLedger?: "ran"
            case .receipt?: "receipt"
            case .evidencePath?, nil: "updated"
            }
            parts.append("\(word) \(ScheduleDescription.age(Int(now.timeIntervalSince(latest)))) ago")
        }
        return parts.joined(separator: " · ")
    }

    // MARK: - status

    static let sections: [(Severity, String, String)] = [
        (.failing, "Failing", "✗"), (.warning, "Warning", "!"), (.paused, "Paused", "‖"), (.ok, "OK", "✓"),
        (.hidden, "Hidden", "-"),
    ]

    /// Header, then one section per severity. Without `all`, OK and hidden agents are left out. `pi` adds the Pi row,
    /// `backups` the Mac backups row.
    public func statusText(_ snapshot: Snapshot, all: Bool = false, pi: PiCheck? = nil,
                           backups: MacBackupsCheck? = nil) -> String {
        var lines = ["Heartbeat — " + headline(snapshot)]
        let shown = snapshot.agents.filter { all || ($0.severity != .ok && $0.severity != .hidden) }
        let width = shown.map(\.label.count).max() ?? 0
        for (severity, title, mark) in Self.sections {
            let members = shown.filter { $0.severity == severity }
            guard !members.isEmpty else { continue }
            lines.append("")
            lines.append("\(title) (\(members.count))")
            for agent in members {
                let label = agent.label.padding(toLength: width, withPad: " ", startingAt: 0)
                lines.append("  \(mark) \(label)  \(detail(agent, now: snapshot.takenAt))")
            }
        }
        if !all {
            let ok = snapshot.count(.ok), hidden = snapshot.count(.hidden)
            var more: [String] = []
            if ok > 0 { more.append("\(ok) ok") }
            if hidden > 0 { more.append("\(hidden) hidden") }
            if !more.isEmpty {
                lines.append("")
                lines.append("(\(more.joined(separator: ", ")) not shown; use --all)")
            }
        }
        if let pi {
            lines.append("")
            lines.append(piRow(pi))
            lines += piProblems(pi).map { "  \($0)" }
            lines.append(piJournalRow(pi))
            lines += piJournalEntries(pi).map { "  \($0)" }
        }
        if let backups {
            lines.append("")
            lines.append(macBackupsRow(backups, now: snapshot.takenAt))
            lines += macBackupsMenuInfo(backups, now: snapshot.takenAt).dropLast().flatMap { $0 }.map { "  \($0)" }
        }
        let notes = problemLines(snapshot)
        if !notes.isEmpty {
            lines.append("")
            lines += notes
        }
        return lines.joined(separator: "\n")
    }

    func problemLines(_ snapshot: Snapshot) -> [String] {
        var lines: [String] = []
        if let error = snapshot.configError { lines.append("! config: \(error) (using \(snapshot.configSource.name) config)") }
        lines += snapshot.configWarnings.map { "! config: \($0)" }
        lines += snapshot.discoveryProblems.map { "! \($0)" }
        return lines
    }

    /// `pi` adds a `pi` object and folds the Pi row into `overall`; `backups` adds a `backups` object and folds a
    /// failing Mac backups row into `overall` as a warning.
    public func statusJSON(_ snapshot: Snapshot, all: Bool = false, pi: PiCheck? = nil,
                           backups: MacBackupsCheck? = nil) -> String {
        let agents = snapshot.agents.filter { all || $0.severity != .hidden }
        let counts = Dictionary(uniqueKeysWithValues: Severity.allCases.map { ($0.rawValue, snapshot.count($0)) })
        var object: [String: Any] = [
            "version": HeartbeatCore.version,
            "checkedAt": iso(snapshot.takenAt),
            "overall": snapshot.overall.including(pi).including(backups: backups, now: snapshot.takenAt).rawValue,
            "headline": headline(snapshot),
            "counts": counts,
            "agents": agents.map { agentJSON($0, snapshot: snapshot) },
            "problems": snapshot.discoveryProblems.map(\.description) + (snapshot.fatalError.map { [$0] } ?? []),
            "config": [
                "source": snapshot.configSource.key,
                "error": snapshot.configError ?? NSNull(),
                "warnings": snapshot.configWarnings,
            ] as [String: Any],
            "power": [
                "bootTime": iso(snapshot.power.bootTime),
                "lastWake": snapshot.power.lastWake.map(iso) ?? NSNull(),
            ] as [String: Any],
        ]
        if let pi { object["pi"] = piJSON(pi) }
        if let backups { object["backups"] = macBackupsJSON(backups, now: snapshot.takenAt) }
        return Self.serialize(object)
    }

    /// Keys of one agent object in `status --json` and `list --json`.
    public func agentJSON(_ agent: AgentSnapshot, snapshot: Snapshot) -> [String: Any] {
        let runtime = agent.status.runtime
        var lastExitCode: Any = NSNull(), lastSignal: Any = NSNull()
        switch runtime?.lastExit {
        case .exited(let code)?: lastExitCode = Int(code)
        case .signaled(let signal, _)?: lastSignal = Int(signal)
        case .neverExited?, nil: break
        }
        let reasons: [[String: Any]] = agent.verdict.reasons.map {
            ["rule": $0.rule, "code": $0.code.rawValue, "severity": $0.severity.rawValue, "message": $0.message]
        }
        return [
            "label": agent.label,
            "name": agent.name(labelPrefix: snapshot.config.labelPrefix),
            "severity": agent.severity.rawValue,
            "summary": agent.verdict.summary ?? NSNull(),
            "detail": detail(agent, now: snapshot.takenAt),
            "reasons": reasons,
            "schedule": ScheduleDescription.describe(agent.agent),
            "state": stateText(agent.status),
            "pid": runtime?.pid.map { Int($0) } ?? NSNull(),
            "runs": runtime?.runs ?? NSNull(),
            "lastExitCode": lastExitCode,
            "lastSignal": lastSignal,
            "lastEvidence": agent.evidence.latest.map(iso) ?? NSNull(),
            "evidenceOrigin": agent.evidence.origin ?? NSNull(),
            "nextExpected": agent.verdict.overdue?.nextExpected.map(iso) ?? NSNull(),
            "plistPath": agent.agent.plistPath,
            "logPaths": agent.agent.logPaths,
            "disabled": agent.agent.disabled,
            "notify": agent.notify,
        ]
    }

    // MARK: - list

    /// Every discovered agent with its launchd runtime and schedule, then any problems.
    public func listText(_ snapshot: Snapshot) -> String {
        let header = ["LABEL", "STATE", "PID", "LAST EXIT", "RUNS", "SCHEDULE"]
        var rows = [header]
        for agent in snapshot.agents {
            let flags = agent.agent.disabled ? "  [Disabled]" : ""
            rows.append([agent.label] + runtimeColumns(agent.status) + [ScheduleDescription.describe(agent.agent) + flags])
        }
        let widths = header.indices.map { column in rows.map { $0[column].count }.max() ?? 0 }
        var lines = rows.map { row in
            row.enumerated().map { column, cell in
                column == row.count - 1 ? cell : cell.padding(toLength: widths[column], withPad: " ", startingAt: 0)
            }.joined(separator: "  ")
        }
        for agent in snapshot.agents {
            if case .unknown(let reason) = agent.status { lines.append("! \(agent.label): \(reason)") }
        }
        lines += snapshot.discoveryProblems.map { "! \($0)" }
        if let fatal = snapshot.fatalError { lines.append("! \(fatal)") }
        return lines.joined(separator: "\n")
    }

    public func listJSON(_ snapshot: Snapshot) -> String {
        Self.serialize([
            "agents": snapshot.agents.map { agentJSON($0, snapshot: snapshot) },
            "problems": snapshot.discoveryProblems.map(\.description),
        ])
    }

    func runtimeColumns(_ status: ServiceStatus) -> [String] {
        switch status {
        case .loaded(let runtime):
            let lastExit = switch runtime.lastExit {
            case .exited(let code): "\(code)"
            case .signaled(let signal, let name): "sig \(signal) (\(name))"
            case .neverExited, nil: "-"
            }
            return [runtime.state.description, runtime.pid.map(String.init) ?? "-", lastExit, runtime.runs.map(String.init) ?? "-"]
        case .notLoaded:
            return ["not loaded", "-", "-", "-"]
        case .unknown:
            return ["unknown", "-", "-", "-"]
        }
    }

    func stateText(_ status: ServiceStatus) -> String {
        switch status {
        case .loaded(let runtime): runtime.state.description
        case .notLoaded: "not loaded"
        case .unknown: "unknown"
        }
    }

    // MARK: - menu

    /// Menu sections in display order (Failing, Warning, OK, Paused); empty ones and hidden agents are left out.
    public func menuSections(_ snapshot: Snapshot) -> [(title: String, agents: [AgentSnapshot])] {
        [(Severity.failing, "Failing"), (.warning, "Warning"), (.ok, "OK"), (.paused, "Paused")].compactMap { severity, title in
            let members = snapshot.agents.filter { $0.severity == severity }
            return members.isEmpty ? nil : (title, members)
        }
    }

    /// The per-agent info submenu: reasons, then facts, then paths. Each inner array is one group.
    public func menuInfo(_ agent: AgentSnapshot, snapshot: Snapshot) -> [[String]] {
        let now = snapshot.takenAt
        let reasons = agent.verdict.reasons.isEmpty
            ? ["No problems"]
            : agent.verdict.reasons.map { reason in
                let mark = Self.sections.first { $0.0 == reason.severity }?.2 ?? "-"
                return "\(mark) \(reason.message)"
            }

        var facts = ["Label: \(agent.label)", "Schedule: \(ScheduleDescription.describe(agent.agent))"]
        let origin = agent.evidenceSources.first { $0.date != nil && $0.date == agent.evidence.latest }
        let evidence = agent.evidence.hasSource ? date(agent.evidence.latest, now: now, missing: "none yet") : "no source"
        facts.append("Last evidence: \(evidence)\(origin.map { " (\($0.kind.menuName))" } ?? "")")
        if let next = agent.verdict.overdue?.nextExpected {
            facts.append("Next expected: \(date(next, now: now))")
        }
        let columns = runtimeColumns(agent.status)
        var state = "State: \(columns[0])"
        if columns[1] != "-" { state += " · pid \(columns[1])" }
        if columns[3] != "-" { state += " · runs \(columns[3])" }
        if columns[2] != "-" { state += " · last exit \(columns[2])" }
        facts.append(state)
        if case .unknown(let reason) = agent.status { facts.append("launchctl: \(reason)") }
        if agent.config.health != nil {
            facts.append("Health check: " + (agent.healthCheck.map { result in
                "\(outcomeText(result.outcome)) · \(ago(result.finishedAt, now: now))"
            } ?? "not run yet"))
            if let result = agent.healthCheck, result.outcome != .ok, let detail = result.detail {
                facts.append("Health output: \(detail)")
            }
        }
        if agent.config.receipt != nil {
            var receipt = "Receipt: " + (agent.receipt.map(receiptText) ?? "not read while paused")
            if let finished = agent.receipt?.finished { receipt += " · finished \(ago(finished, now: now))" }
            facts.append(receipt)
        }

        var paths = ["Plist: \(tilde(agent.agent.plistPath))"]
        if agent.agent.resolvedPlistPath != agent.agent.plistPath {
            paths.append("Target: \(tilde(agent.agent.resolvedPlistPath))")
        }
        paths += agent.agent.logPaths.map { "Log: \(tilde($0))" }
        return [reasons, facts, paths]
    }

    // MARK: - check-config

    /// The loaded config, its problems, and config entries that match no discovered agent.
    public func configReport(_ result: ConfigLoadResult, path: String, discovered: [String]?) -> String {
        var lines = ["Config: \(tilde(path)) (\(result.source.name))"]
        if let error = result.error { lines.append("error: \(error)") }
        lines += result.warnings.map { "warning: \($0)" }
        lines += unknownLabels(result.config, discovered: discovered).map { "warning: agents.\($0) matches no discovered agent" }
        let config = result.config
        lines.append("labelPrefix \(config.labelPrefix) · poll \(config.pollSeconds) s · notifications \(config.notifications ? "on" : "off")")
        lines.append("piHost \(config.piHost) · every \(config.piStatusSeconds) s")
        for label in config.agents.keys.sorted() {
            lines.append("  \(label)")
        }
        if result.error == nil && result.warnings.isEmpty && unknownLabels(config, discovered: discovered).isEmpty {
            lines.append("OK")
        }
        return lines.joined(separator: "\n")
    }

    public func unknownLabels(_ config: HeartbeatConfig, discovered: [String]?) -> [String] {
        guard let discovered else { return [] }
        return config.agents.keys.filter { !discovered.contains($0) }.sorted()
    }

    // MARK: - Helpers

    static func serialize(_ object: Any) -> String {
        let data = (try? JSONSerialization.data(withJSONObject: object,
                                                options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])) ?? Data()
        return String(decoding: data, as: UTF8.self)
    }

    func iso(_ date: Date) -> String {
        let formatter = ISO8601DateFormatter()
        formatter.timeZone = calendar.timeZone
        return formatter.string(from: date)
    }

    func clock(_ date: Date) -> String {
        let parts = calendar.dateComponents([.hour, .minute], from: date)
        return "\(ScheduleDescription.pad(parts.hour ?? 0)):\(ScheduleDescription.pad(parts.minute ?? 0))"
    }

    func stamp(_ date: Date) -> String {
        let p = calendar.dateComponents([.year, .month, .day, .hour, .minute, .second], from: date)
        let pad = ScheduleDescription.pad
        return "\(p.year ?? 0)-\(pad(p.month ?? 0))-\(pad(p.day ?? 0)) \(pad(p.hour ?? 0)):\(pad(p.minute ?? 0)):\(pad(p.second ?? 0))"
    }

    /// "2026-09-26 12:41:03 (3 h ago)" / "(in 5 min)".
    func date(_ date: Date?, now: Date, missing: String = "-") -> String {
        guard let date else { return missing }
        if date == .distantPast { return "unknown" }
        return "\(stamp(date)) (\(ago(date, now: now)))"
    }

    func tilde(_ path: String) -> String {
        guard !home.isEmpty, path == home || path.hasPrefix(home + "/") else { return path }
        return "~" + path.dropFirst(home.count)
    }

    func yesNo(_ value: Bool) -> String { value ? "yes" : "no" }
}

extension ConfigLoadResult.Source {
    /// "defaults", "file", "last good".
    public var name: String {
        switch self {
        case .defaults: "defaults"
        case .file: "file"
        case .lastGood: "last good"
        }
    }

    /// The JSON value: "defaults", "file", "lastGood".
    public var key: String {
        switch self {
        case .defaults: "defaults"
        case .file: "file"
        case .lastGood: "lastGood"
        }
    }
}

extension EvidenceSource.Kind {
    /// "log", "runs ledger", ... for the menu.
    var menuName: String {
        switch self {
        case .log: "log"
        case .evidencePath: "evidence path"
        case .runsLedger: "runs ledger"
        case .receipt: "receipt"
        }
    }
}
