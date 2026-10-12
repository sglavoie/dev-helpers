import Foundation

extension StatusFormatter {
    // MARK: - Pi

    /// "Pi — ok · 48/48 up", "Pi — warn: Kuma: Forgejo down", "Pi — unreachable over Tailscale".
    public func piRow(_ check: PiCheck?) -> String {
        guard let check else { return "Pi — checking…" }
        switch check.status {
        case .summary(let summary):
            guard summary.status == .ok else {
                return "Pi — \(summary.status.rawValue): " + (summary.problems.first ?? "no details")
            }
            if let up = summary.kumaUp, let total = summary.kumaTotal { return "Pi — ok · \(up)/\(total) up" }
            return "Pi — ok"
        case .unreachable: return "Pi — unreachable over Tailscale"
        case .unreadable: return "Pi — cannot read pi-status"
        }
    }

    /// Every problem line, or what went wrong reaching the Pi.
    public func piProblems(_ check: PiCheck) -> [String] {
        switch check.status {
        case .summary(let summary): summary.problems
        case .unreachable(let detail), .unreadable(let detail): [detail]
        }
    }

    /// The Pi row's submenu: problems (or "No problems"), then Kuma counts, host and times.
    public func piMenuInfo(_ check: PiCheck?, now: Date) -> [[String]] {
        guard let check else { return [["Not checked yet"]] }
        var problems = piProblems(check)
        if case .summary(let summary) = check.status, problems.isEmpty {
            problems = [summary.status == .ok ? "No problems" : "Status \(summary.status.rawValue)"]
        }
        var facts: [String] = []
        if case .summary(let summary) = check.status {
            if let up = summary.kumaUp, let total = summary.kumaTotal { facts.append("Kuma: \(up)/\(total) monitors up") }
            if let generated = summary.generated { facts.append("pi-status report: \(ago(generated, now: now))") }
        }
        facts.append("Host: \(check.host)")
        facts.append("Checked: \(ago(check.checkedAt, now: now))")
        return [problems, facts]
    }

    /// "Pi journal — no errors in the last hour", "Pi journal — 15+ errors, last 17:41", "Pi journal — unknown".
    public func piJournalRow(_ check: PiCheck?) -> String {
        guard let check else { return "Pi journal — checking…" }
        guard case .summary(let summary) = check.status else { return "Pi journal — unknown" }
        guard let journal = summary.journal else { return "Pi journal — not reported" }
        guard journal.reason == nil, journal.count > 0 || journal.status == .ok else { return "Pi journal — unknown" }
        guard journal.count > 0 else { return "Pi journal — no errors in the last hour" }
        let count = "\(journal.count)\(journal.truncated ? "+" : "") \(journal.count == 1 && !journal.truncated ? "error" : "errors")"
        return "Pi journal — \(count)" + (journal.last.map { ", last \(clock($0))" } ?? "")
    }

    /// "systemd: Failed to start x.service ×10 · 17:41", newest first; the reason when the journal couldn't be read.
    public func piJournalEntries(_ check: PiCheck) -> [String] {
        guard case .summary(let summary) = check.status, let journal = summary.journal else { return [] }
        if let reason = journal.reason { return [reason] }
        return journal.entries.map { entry in
            entry.message + (entry.count > 1 ? " ×\(entry.count)" : "") + (entry.last.map { " · \(clock($0))" } ?? "")
        }
    }

    /// The journal row's submenu: the entries (or why there are none), then the window and check time.
    public func piJournalMenuInfo(_ check: PiCheck?, now: Date) -> [[String]] {
        guard let check else { return [["Not checked yet"]] }
        var entries = piJournalEntries(check)
        if entries.isEmpty {
            entries = switch check.status {
            case .summary(let summary) where summary.journal == nil: ["pi-status sent no journal section"]
            case .summary: ["No errors in the last hour"]
            case .unreachable, .unreadable: ["The Pi couldn't be checked"]
            }
        }
        var facts = ["Window: last hour, journal priority err and above"]
        if case .summary(let summary) = check.status, let journal = summary.journal, journal.truncated {
            facts.append("Only the newest \(journal.count) lines were read")
        }
        facts.append("Checked: \(ago(check.checkedAt, now: now))")
        return [entries, facts]
    }

    /// The `journal` object inside `pi` in `status --pi --json`; `null` when pi-status sent no journal section.
    public func piJournalJSON(_ check: PiCheck) -> Any {
        guard case .summary(let summary) = check.status, let journal = summary.journal else { return NSNull() }
        return [
            "status": journal.status.rawValue,
            "severity": check.journalSeverity?.rawValue ?? "unknown",
            "row": piJournalRow(check),
            "count": journal.count,
            "truncated": journal.truncated,
            "reason": journal.reason ?? NSNull(),
            "entries": journal.entries.map { entry -> [String: Any] in
                ["message": entry.message, "count": entry.count, "last": entry.last.map(iso) ?? NSNull()]
            },
        ] as [String: Any]
    }

    /// The `pi` object in `status --pi --json`.
    public func piJSON(_ check: PiCheck) -> [String: Any] {
        var object: [String: Any] = [
            "host": check.host,
            "checkedAt": iso(check.checkedAt),
            "severity": check.severity.rawValue,
            "row": piRow(check),
            "problems": piProblems(check),
            "kumaUp": NSNull(),
            "kumaTotal": NSNull(),
            "generated": NSNull(),
            "journal": piJournalJSON(check),
        ]
        switch check.status {
        case .summary(let summary):
            object["status"] = summary.status.rawValue
            object["kumaUp"] = summary.kumaUp ?? NSNull()
            object["kumaTotal"] = summary.kumaTotal ?? NSNull()
            object["generated"] = summary.generated.map(iso) ?? NSNull()
        case .unreachable: object["status"] = "unreachable"
        case .unreadable: object["status"] = "unreadable"
        }
        return object
    }
}
