import Foundation

/// The last run of an agent's health command (rule 7), kept in state.json.
public struct HealthCheckResult: Equatable, Codable, Sendable {
    public enum Outcome: Equatable, Codable, Sendable {
        case ok
        /// Non-zero exit not listed in `warningExitCodes`: unhealthy.
        case failed(exitCode: Int32)
        /// A `warningExitCodes` exit, e.g. 2 = "cannot check".
        case warning(exitCode: Int32)
        case killed(signal: Int32)
        case timedOut
        case couldNotStart(message: String)
    }

    public var finishedAt: Date
    public var outcome: Outcome
    /// The first non-empty output line (stderr before stdout), for the menu and banners.
    public var detail: String?

    public init(finishedAt: Date, outcome: Outcome, detail: String? = nil) {
        self.finishedAt = finishedAt
        self.outcome = outcome
        self.detail = detail
    }

    /// Classifies a finished command using the config's `warningExitCodes`.
    public init(_ result: CommandResult, config: HealthCommandConfig, finishedAt: Date) {
        self.finishedAt = finishedAt
        detail = Self.firstLine(result.stderrText) ?? Self.firstLine(result.stdoutText)
        if result.timedOut {
            outcome = .timedOut
            return
        }
        switch result.termination {
        case .exited(0): outcome = .ok
        case .exited(let code) where config.warningExitCodes.contains(code): outcome = .warning(exitCode: code)
        case .exited(let code): outcome = .failed(exitCode: code)
        case .signaled(let signal): outcome = .killed(signal: signal)
        }
    }

    static func firstLine(_ text: String) -> String? {
        let line = text.split(whereSeparator: \.isNewline).lazy
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .first { !$0.isEmpty }
        guard let line else { return nil }
        return line.count > 200 ? String(line.prefix(200)) + "…" : line
    }
}

/// Runs one health command with the fixed GUI PATH (GUI apps don't get a login shell's PATH).
public enum HealthCheck {
    /// Environment for health commands: GUI PATH, HOME, and a UTF-8 locale.
    public static func environment(home: String = NSHomeDirectory()) -> [String: String] {
        ["PATH": CommandRunner.guiPath, "HOME": home, "LANG": "en_US.UTF-8"]
    }

    public static func run(_ config: HealthCommandConfig, runner: CommandRunning = CommandRunner(environment: environment()),
                           now: () -> Date = Date.init) -> HealthCheckResult {
        do {
            let result = try runner.run(config.command, timeout: TimeInterval(config.timeoutSeconds))
            return HealthCheckResult(result, config: config, finishedAt: now())
        } catch {
            return HealthCheckResult(finishedAt: now(), outcome: .couldNotStart(message: "\(error)"))
        }
    }
}
