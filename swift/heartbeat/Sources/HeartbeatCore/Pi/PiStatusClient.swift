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
}

extension OverallStatus {
    /// The agents' overall with the Pi row folded in; the Pi can only turn ok into warning.
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

    static func firstLine(_ text: String) -> String {
        text.split(whereSeparator: \.isNewline).first.map { $0.trimmingCharacters(in: .whitespaces) } ?? ""
    }
}
