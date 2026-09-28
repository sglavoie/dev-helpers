import Foundation

/// The outcome of asking the Pi for `pi-status --json`.
public enum PiStatus: Equatable, Sendable {
    case summary(PiSummary)
    /// ssh couldn't reach the Pi (or timed out); the detail is ssh's own message.
    case unreachable(String)
    /// ssh worked but the output wasn't a pi-status report.
    case unreadable(String)
}

/// One Pi check, kept by the app between checks and printed by `heartbeatctl status --pi`.
public struct PiCheck: Equatable, Sendable {
    public var host: String
    public var checkedAt: Date
    public var status: PiStatus

    public init(host: String, checkedAt: Date, status: PiStatus) {
        self.host = host
        self.checkedAt = checkedAt
        self.status = status
    }

    /// Amber at most: Kuma already pages the phone for real Pi problems.
    public var severity: Severity {
        if case .summary(let summary) = status, summary.status == .ok { return .ok }
        return .warning
    }

    /// The journal row's dot: amber while the last hour has errors, green when it has none, gray (`nil`) when the
    /// journal couldn't be read. Never folded into the overall status.
    public var journalSeverity: Severity? {
        guard case .summary(let summary) = status, let journal = summary.journal, journal.reason == nil else { return nil }
        return journal.count > 0 ? .warning : journal.status == .ok ? .ok : nil
    }
}

extension OverallStatus {
    /// The agents' overall with the Pi row folded in; the Pi can only turn ok into warning. The journal row doesn't count.
    public func including(_ pi: PiCheck?) -> OverallStatus {
        self == .ok && pi?.severity == .warning ? .warning : self
    }
}

/// Runs `ssh -o BatchMode=yes -o ConnectTimeout=5 <piHost> ~/.local/bin/pi-status --json`. The full path is needed
/// because `~/.local/bin` isn't on the Pi's non-interactive PATH; the remote shell expands the `~`.
public struct PiStatusClient: Sendable {
    public static let sshPath = "/usr/bin/ssh"
    public static let remoteCommand = "~/.local/bin/pi-status --json"
    public static let kumaURL = URL(string: "https://uptime.sglavoie.com")!
    /// ssh's own exit status for connection and authentication failures.
    static let sshFailureExitCode: Int32 = 255

    public var runner: any CommandRunning
    /// Covers the 5 s connect plus pi-status itself (about 4 s over Tailscale, Kuma fetches time out at 5 s).
    public var timeout: TimeInterval

    public init(runner: any CommandRunning = CommandRunner(), timeout: TimeInterval = 30) {
        self.runner = runner
        self.timeout = timeout
    }

    /// `--` keeps a configured host from being read as an ssh option.
    public static func argv(host: String) -> [String] {
        [sshPath, "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "--", host, remoteCommand]
    }

    /// Blocks for up to `timeout`; call it off the main thread. Never throws.
    /// `checkedAt` is when the answer came back.
    public func check(host: String, now: () -> Date = Date.init) -> PiCheck {
        let status = fetch(host: host)
        return PiCheck(host: host, checkedAt: now(), status: status)
    }

    public func fetch(host: String) -> PiStatus {
        let result: CommandResult
        do {
            result = try runner.run(Self.argv(host: host), timeout: timeout)
        } catch {
            return .unreachable("\(error)")
        }
        if result.timedOut { return .unreachable("ssh timed out after \(Int(timeout)) s") }
        // pi-status exits 1 when a section is failing but still prints the report.
        let parsed = Result { try PiStatusParser.parse(result.stdout) }
        if case .success(let summary) = parsed { return .summary(summary) }
        let stderr = Self.firstLine(result.stderrText)
        switch result.termination {
        case .exited(Self.sshFailureExitCode):
            return .unreachable(stderr.isEmpty ? "ssh exited with status 255" : stderr)
        case .signaled(let signal):
            return .unreachable("ssh killed by signal \(signal)")
        case .exited(let code):
            if case .failure(let error) = parsed, code == 0 { return .unreadable("pi-status output: \(error)") }
            return .unreadable("pi-status exited with status \(code)" + (stderr.isEmpty ? "" : ": \(stderr)"))
        }
    }

    /// The query behind pi-status's journal section, without its 15-line limit. `-r` puts the newest first, so
    /// when the output hits `CommandRunner.outputLimit` it's the oldest lines that are cut.
    public static let journalCommand = "journalctl -p err --since -1h -n 300 -r --no-pager -o short-iso -q"

    public static func journalArgv(host: String) -> [String] {
        [sshPath, "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "--", host, journalCommand]
    }

    /// The last hour of journal errors on the Pi, oldest first. Blocks for up to `timeout`; call it off the main
    /// thread.
    public func journal(host: String) -> Result<PiJournalLog, PiJournalFetchError> {
        let result: CommandResult
        do {
            result = try runner.run(Self.journalArgv(host: host), timeout: timeout)
        } catch {
            return .failure(PiJournalFetchError(description: "\(error)"))
        }
        if result.timedOut { return .failure(PiJournalFetchError(description: "ssh timed out after \(Int(timeout)) s")) }
        let stderr = Self.firstLine(result.stderrText)
        switch result.termination {
        case .exited(0):
            var lines = result.stdoutText.split(whereSeparator: \.isNewline).map(String.init).filter {
                !$0.trimmingCharacters(in: .whitespaces).isEmpty
            }
            // A cut-off output ends mid-line, and that line is the oldest one.
            if result.stdoutTruncated, !lines.isEmpty { lines.removeLast() }
            return .success(PiJournalLog(lines: lines.reversed(), truncated: result.stdoutTruncated))
        case .exited(Self.sshFailureExitCode):
            return .failure(PiJournalFetchError(description: stderr.isEmpty ? "ssh exited with status 255" : stderr))
        case .exited(let code):
            return .failure(PiJournalFetchError(
                description: "journalctl exited with status \(code)" + (stderr.isEmpty ? "" : ": \(stderr)")))
        case .signaled(let signal):
            return .failure(PiJournalFetchError(description: "ssh killed by signal \(signal)"))
        }
    }

    static func firstLine(_ text: String) -> String {
        text.split(whereSeparator: \.isNewline).first.map { $0.trimmingCharacters(in: .whitespaces) } ?? ""
    }
}

/// The Pi's journal errors as the View Journal window shows them.
public struct PiJournalLog: Equatable, Sendable {
    /// Oldest first.
    public var lines: [String]
    /// The output hit the size limit, so older lines are missing.
    public var truncated: Bool

    public init(lines: [String], truncated: Bool = false) {
        self.lines = lines
        self.truncated = truncated
    }
}

public struct PiJournalFetchError: Error, Equatable, Sendable, CustomStringConvertible {
    public var description: String

    public init(description: String) {
        self.description = description
    }
}
