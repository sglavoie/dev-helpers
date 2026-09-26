import Foundation
import Testing
@testable import HeartbeatCore

/// Records argv and answers from a table keyed by the last argument.
final class FakeRunner: CommandRunning, @unchecked Sendable {
    private let lock = NSLock()
    private var calls: [[String]] = []
    let responses: [String: CommandResult]
    let fallback: CommandResult?

    init(_ responses: [String: CommandResult] = [:], fallback: CommandResult? = nil) {
        self.responses = responses
        self.fallback = fallback
    }

    var recorded: [[String]] { lock.withLock { calls } }

    func run(_ argv: [String], timeout: TimeInterval) throws -> CommandResult {
        lock.withLock { calls.append(argv) }
        guard let result = responses[argv.last ?? ""] ?? fallback else {
            throw CommandError.launchFailed(executable: argv[0], message: "no fake response")
        }
        var copy = result
        copy.argv = argv
        return copy
    }
}

func result(_ code: Int32, stdout: String = "", stderr: String = "", timedOut: Bool = false) -> CommandResult {
    CommandResult(argv: [], termination: .exited(code), stdout: Data(stdout.utf8), stderr: Data(stderr.utf8), timedOut: timedOut)
}

@Suite struct LaunchctlClientTests {
    @Test func printParsesLoadedService() {
        let runner = FakeRunner(["gui/501/com.sglavoie.sync-legacy": result(0, stdout: LaunchctlFixtures.syncLegacy)])
        let client = LaunchctlClient(runner: runner, uid: 501)

        let status = client.print("com.sglavoie.sync-legacy")

        #expect(runner.recorded == [["/bin/launchctl", "print", "gui/501/com.sglavoie.sync-legacy"]])
        #expect(status.runtime?.lastExit == .exited(code: 1))
    }

    @Test func exit113IsNotLoaded() {
        let runner = FakeRunner(fallback: result(113, stderr: LaunchctlFixtures.notFoundStderr))
        #expect(LaunchctlClient(runner: runner, uid: 501).print("com.sglavoie.nope") == .notLoaded)
    }

    @Test func unparseableOutputIsUnknown() {
        let runner = FakeRunner(fallback: result(0, stdout: "something else"))
        guard case .unknown(let reason) = LaunchctlClient(runner: runner, uid: 501).print("x") else {
            Issue.record("expected unknown")
            return
        }
        #expect(reason.contains("header"))
    }

    @Test func otherFailuresAreUnknown() {
        let failing = FakeRunner(fallback: result(5, stderr: "Operation not permitted"))
        #expect(LaunchctlClient(runner: failing, uid: 501).print("x")
            == .unknown(reason: "/bin/launchctl print gui/501/x failed (exit 5): Operation not permitted"))

        let slow = FakeRunner(fallback: result(0, timedOut: true))
        #expect(LaunchctlClient(runner: slow, uid: 501).print("x") == .unknown(reason: "launchctl print timed out"))

        let missing = FakeRunner()
        guard case .unknown = LaunchctlClient(runner: missing, uid: 501).print("x") else {
            Issue.record("expected unknown")
            return
        }
    }

    @Test func printManyKeepsOrder() {
        let runner = FakeRunner([
            "gui/501/a": result(0, stdout: LaunchctlFixtures.forgejoSync),
            "gui/501/b": result(113),
            "gui/501/c": result(0, stdout: LaunchctlFixtures.ddcBrightnessd),
        ])
        let statuses = LaunchctlClient(runner: runner, uid: 501).print(["a", "b", "c"], maxConcurrent: 2)
        #expect(statuses.count == 3)
        #expect(statuses[0].runtime?.runs == 18)
        #expect(statuses[1] == .notLoaded)
        #expect(statuses[2].runtime?.pid == 1076)
        #expect(LaunchctlClient(runner: runner, uid: 501).print([String]()).isEmpty)
    }

    @Test func actionsBuildTheRightArgv() throws {
        let runner = FakeRunner(fallback: result(0))
        let client = LaunchctlClient(runner: runner, uid: 501)

        try client.kickstart("com.sglavoie.forgejo-sync")
        try client.kickstart("com.sglavoie.ddc-brightnessd", restart: true)
        try client.bootstrap(plistPath: "/Users/me/Library/LaunchAgents/com.sglavoie.forgejo-sync.plist")
        try client.bootout("com.sglavoie.forgejo-sync")

        #expect(runner.recorded == [
            ["/bin/launchctl", "kickstart", "gui/501/com.sglavoie.forgejo-sync"],
            ["/bin/launchctl", "kickstart", "-k", "gui/501/com.sglavoie.ddc-brightnessd"],
            ["/bin/launchctl", "bootstrap", "gui/501", "/Users/me/Library/LaunchAgents/com.sglavoie.forgejo-sync.plist"],
            ["/bin/launchctl", "bootout", "gui/501/com.sglavoie.forgejo-sync"],
        ])
    }

    @Test func actionFailureCarriesStderr() {
        let runner = FakeRunner(fallback: result(5, stderr: "Bootstrap failed: 5: Input/output error\n"))
        let client = LaunchctlClient(runner: runner, uid: 501)
        #expect(throws: LaunchctlError.commandFailed(
            argv: ["/bin/launchctl", "bootstrap", "gui/501", "/p.plist"],
            status: .exited(5),
            message: "Bootstrap failed: 5: Input/output error"
        )) {
            try client.bootstrap(plistPath: "/p.plist")
        }
    }

    @Test func actionTimeoutThrows() {
        let runner = FakeRunner(fallback: result(0, timedOut: true))
        #expect(throws: LaunchctlError.timedOut(argv: ["/bin/launchctl", "bootout", "gui/501/x"])) {
            try LaunchctlClient(runner: runner, uid: 501).bootout("x")
        }
    }
}
