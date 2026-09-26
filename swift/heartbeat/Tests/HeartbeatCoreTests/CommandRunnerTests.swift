import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct CommandRunnerTests {
    let runner = CommandRunner(environment: ["PATH": CommandRunner.guiPath])

    @Test func capturesStdoutStderrAndExitCode() throws {
        let result = try runner.run(["/bin/sh", "-c", "echo out; echo err >&2; exit 3"], timeout: 5)
        #expect(result.termination == .exited(3))
        #expect(result.exitCode == 3)
        #expect(result.stdoutText == "out\n")
        #expect(result.stderrText == "err\n")
        #expect(!result.timedOut)
        #expect(!result.succeeded)
    }

    @Test func resolvesBareNamesAgainstPath() throws {
        let result = try runner.run(["echo", "hi"], timeout: 5)
        #expect(result.succeeded)
        #expect(result.stdoutText == "hi\n")
    }

    @Test func passesArgumentsWithoutAShell() throws {
        let result = try runner.run(["/usr/bin/printf", "%s|", "a b", "$HOME", "*"], timeout: 5)
        #expect(result.stdoutText == "a b|$HOME|*|")
    }

    @Test func usesTheGivenEnvironmentAndNoStdin() throws {
        let custom = CommandRunner(environment: ["PATH": "/usr/bin:/bin", "HB_TEST": "yes"])
        let result = try custom.run(["/bin/sh", "-c", "echo $HB_TEST; cat"], timeout: 5)
        #expect(result.stdoutText == "yes\n")
        #expect(result.succeeded)
    }

    @Test func missingExecutableFailsToStart() {
        #expect(throws: CommandError.launchFailed(executable: "no-such-heartbeat-cmd", message: "not found in PATH")) {
            try runner.run(["no-such-heartbeat-cmd"], timeout: 5)
        }
        #expect(throws: CommandError.self) { try runner.run(["/nonexistent/tool"], timeout: 5) }
        #expect(throws: CommandError.emptyArguments) { try runner.run([], timeout: 5) }
    }

    @Test func signalTermination() throws {
        let result = try runner.run(["/bin/sh", "-c", "kill -TERM $$"], timeout: 5)
        #expect(result.termination == .signaled(SIGTERM))
        #expect(result.exitCode == nil)
    }

    @Test func outputIsCappedButDrained() throws {
        // 200 KB on stdout: the child must not block on a full pipe, and we keep 64 KB.
        let result = try runner.run(["/bin/sh", "-c", "head -c 204800 /dev/zero; echo done >&2"], timeout: 10)
        #expect(result.succeeded)
        #expect(result.stdout.count == CommandRunner.outputLimit)
        #expect(result.stdoutTruncated)
        #expect(result.stderrText == "done\n")
        #expect(!result.stderrTruncated)
    }

    @Test func timeoutKillsTheWholeProcessGroup() throws {
        // The shell starts a background `sleep` and waits; killing only the shell would orphan it.
        let started = Date()
        let result = try runner.run(["/bin/sh", "-c", "sleep 60 & echo $!; wait"], timeout: 0.5)
        #expect(result.timedOut)
        #expect(result.termination == .signaled(SIGKILL))
        #expect(Date().timeIntervalSince(started) < 5)

        let child = try #require(pid_t(result.stdoutText.trimmingCharacters(in: .whitespacesAndNewlines)))
        #expect(Self.waitUntilGone(child))
    }

    @Test func backgroundChildHoldingPipesDoesNotHang() throws {
        // The shell exits at once but its `sleep` keeps stdout open; the runner kills the group after the grace.
        let quick = CommandRunner(environment: ["PATH": CommandRunner.guiPath], drainGrace: 0.3)
        let started = Date()
        let result = try quick.run(["/bin/sh", "-c", "sleep 60 & echo $!; exit 0"], timeout: 30)
        #expect(result.termination == .exited(0))
        #expect(!result.timedOut)
        #expect(Date().timeIntervalSince(started) < 5)

        let child = try #require(pid_t(result.stdoutText.trimmingCharacters(in: .whitespacesAndNewlines)))
        #expect(Self.waitUntilGone(child))
    }

    /// Polls until `pid` no longer exists (killed children are reaped by launchd asynchronously).
    static func waitUntilGone(_ pid: pid_t) -> Bool {
        for _ in 0..<100 {
            if kill(pid, 0) == -1, errno == ESRCH { return true }
            usleep(20_000)
        }
        return false
    }
}
