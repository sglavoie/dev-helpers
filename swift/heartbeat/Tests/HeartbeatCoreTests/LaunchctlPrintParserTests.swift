import Testing
@testable import HeartbeatCore

@Suite struct LaunchctlPrintParserTests {
    @Test func exitedJobReportsLastExitAndRuns() throws {
        let runtime = try LaunchctlPrintParser.parse(LaunchctlFixtures.syncLegacy)
        #expect(runtime == ServiceRuntime(
            state: .notRunning,
            pid: nil,
            runs: 4,
            lastExit: .exited(code: 1),
            path: "/Users/sglavoie/scripts/launchagents/Library/LaunchAgents/com.sglavoie.sync-legacy.plist"
        ))
    }

    @Test func nestedStateLinesAreIgnored() throws {
        // sync-legacy's coalition blocks say `state = active` at two tabs.
        #expect(LaunchctlFixtures.syncLegacy.contains("\t\tstate = active"))
        #expect(try LaunchctlPrintParser.parse(LaunchctlFixtures.syncLegacy).state == .notRunning)
    }

    @Test func runningDaemonHasPidAndNeverExited() throws {
        let runtime = try LaunchctlPrintParser.parse(LaunchctlFixtures.ddcBrightnessd)
        #expect(runtime.state == .running)
        #expect(runtime.pid == 1076)
        #expect(runtime.runs == 1)
        #expect(runtime.lastExit == .neverExited)
    }

    @Test func secondRunningDaemon() throws {
        let runtime = try LaunchctlPrintParser.parse(LaunchctlFixtures.brainnotesVaultGuard)
        #expect(runtime.state == .running)
        #expect(runtime.pid == 1082)
        #expect(runtime.lastExit == .neverExited)
    }

    @Test func successfulIntervalJob() throws {
        let runtime = try LaunchctlPrintParser.parse(LaunchctlFixtures.forgejoSync)
        #expect(runtime.state == .notRunning)
        #expect(runtime.pid == nil)
        #expect(runtime.runs == 18)
        #expect(runtime.lastExit == .exited(code: 0))
    }

    @Test func loadedButNeverRun() throws {
        let runtime = try LaunchctlPrintParser.parse(LaunchctlFixtures.brewMaintain)
        #expect(runtime.runs == 0)
        #expect(runtime.lastExit == .neverExited)
    }

    @Test func terminatingSignal() throws {
        let runtime = try LaunchctlPrintParser.parse(LaunchctlFixtures.forgejoSyncSignaled)
        #expect(runtime.lastExit == .signaled(signal: 15, name: "Terminated"))
    }

    @Test func signalWinsOverExitCode() throws {
        let output = "gui/501/x = {\n\tstate = not running\n\tlast exit code = 0\n\tlast terminating signal = Killed: 9\n}\n"
        #expect(try LaunchctlPrintParser.parse(output).lastExit == .signaled(signal: 9, name: "Killed"))
    }

    @Test func exitCodeWithDescription() throws {
        let output = "gui/501/x = {\n\tstate = not running\n\tlast exit code = 78: EX_CONFIG\n}\n"
        #expect(try LaunchctlPrintParser.parse(output).lastExit == .exited(code: 78))
    }

    @Test func otherStatesAreKeptVerbatim() throws {
        let output = "gui/501/x = {\n\tstate = spawn scheduled\n}\n"
        let runtime = try LaunchctlPrintParser.parse(output)
        #expect(runtime.state == .other("spawn scheduled"))
        #expect(runtime.lastExit == nil)
        #expect(runtime.runs == nil)
    }

    @Test func garbageIsRejected() {
        #expect(throws: LaunchctlPrintError.missingHeader) { try LaunchctlPrintParser.parse("") }
        #expect(throws: LaunchctlPrintError.missingHeader) { try LaunchctlPrintParser.parse("not launchctl output\n") }
        #expect(throws: LaunchctlPrintError.missingHeader) { try LaunchctlPrintParser.parse(LaunchctlFixtures.notFoundStderr) }
    }

    @Test func missingStateIsRejected() {
        #expect(throws: LaunchctlPrintError.missingState) {
            try LaunchctlPrintParser.parse("gui/501/x = {\n\truns = 1\n\t\tstate = active\n}\n")
        }
    }

    @Test func malformedNumbersAreRejected() {
        #expect(throws: LaunchctlPrintError.invalidValue(key: "pid", value: "abc")) {
            try LaunchctlPrintParser.parse("gui/501/x = {\n\tstate = running\n\tpid = abc\n}\n")
        }
        #expect(throws: LaunchctlPrintError.invalidValue(key: "last exit code", value: "oops")) {
            try LaunchctlPrintParser.parse("gui/501/x = {\n\tstate = running\n\tlast exit code = oops\n}\n")
        }
    }
}
