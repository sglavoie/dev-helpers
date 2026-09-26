import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct HealthCheckTests {
    static let config = HealthCommandConfig(command: ["/bin/check"], warningExitCodes: [2])
    static let finished = Date(timeIntervalSince1970: 1_790_000_000)

    static func classify(_ termination: Termination, stdout: String = "", stderr: String = "", timedOut: Bool = false)
        -> HealthCheckResult {
        let result = CommandResult(argv: ["/bin/check"], termination: termination, stdout: Data(stdout.utf8),
                                   stderr: Data(stderr.utf8), timedOut: timedOut)
        return HealthCheckResult(result, config: config, finishedAt: finished)
    }

    @Test func classifiesExitStatuses() {
        #expect(Self.classify(.exited(0)).outcome == .ok)
        #expect(Self.classify(.exited(1)).outcome == .failed(exitCode: 1))
        #expect(Self.classify(.exited(2)).outcome == .warning(exitCode: 2))
        #expect(Self.classify(.exited(3)).outcome == .failed(exitCode: 3))
        #expect(Self.classify(.signaled(15)).outcome == .killed(signal: 15))
        #expect(Self.classify(.signaled(9), timedOut: true).outcome == .timedOut)
        #expect(Self.classify(.exited(0)).finishedAt == Self.finished)
    }

    @Test func detailIsFirstNonEmptyLineStderrFirst() {
        #expect(Self.classify(.exited(1), stdout: "out\n", stderr: "\n  err line  \nmore").detail == "err line")
        #expect(Self.classify(.exited(1), stdout: "\nstatus: dirty\n").detail == "status: dirty")
        #expect(Self.classify(.exited(0)).detail == nil)
        let long = Self.classify(.exited(1), stdout: String(repeating: "x", count: 300)).detail
        #expect(long?.count == 201)
    }

    @Test func runsRealCommandsWithGuiPath() throws {
        let runner = CommandRunner(environment: HealthCheck.environment(home: "/tmp"))
        let ok = HealthCheck.run(HealthCommandConfig(command: ["/bin/sh", "-c", "test \"$PATH\" = '\(CommandRunner.guiPath)' && test \"$HOME\" = /tmp"]),
                                 runner: runner)
        #expect(ok.outcome == .ok)
        let warn = HealthCheck.run(HealthCommandConfig(command: ["/bin/sh", "-c", "echo cannot check >&2; exit 2"], warningExitCodes: [2]),
                                   runner: runner)
        #expect(warn.outcome == .warning(exitCode: 2))
        #expect(warn.detail == "cannot check")
        let timeout = HealthCheck.run(HealthCommandConfig(command: ["/bin/sleep", "5"], timeoutSeconds: 1), runner: runner)
        #expect(timeout.outcome == .timedOut)
        let missing = HealthCheck.run(HealthCommandConfig(command: ["/nonexistent/check"]), runner: runner,
                                      now: { Self.finished })
        #expect(missing.finishedAt == Self.finished)
        guard case .couldNotStart = missing.outcome else {
            Issue.record("expected couldNotStart, got \(missing.outcome)")
            return
        }
    }

    @Test func resultRoundTripsThroughJSON() throws {
        for outcome: HealthCheckResult.Outcome in [.ok, .failed(exitCode: 1), .warning(exitCode: 2), .killed(signal: 9), .timedOut,
                                                   .couldNotStart(message: "nope")] {
            let result = HealthCheckResult(finishedAt: Self.finished, outcome: outcome, detail: "d")
            #expect(try JSONDecoder().decode(HealthCheckResult.self, from: JSONEncoder().encode(result)) == result)
        }
    }
}
