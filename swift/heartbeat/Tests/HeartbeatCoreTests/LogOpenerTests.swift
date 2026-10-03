import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct LogOpenerTests {
    @Test func noisyFailureIsDrainedAndCapped() throws {
        // More than a pipe's capacity: waiting for termination before reading would deadlock.
        let result = try LogOpener.run(["/bin/sh", "-c", "head -c 204800 /dev/zero >&2; exit 3"])
        #expect(result.termination == .exited(3))
        #expect(result.stderr.count == CommandRunner.outputLimit)
        #expect(result.stderrTruncated)
        #expect(!result.timedOut)
    }

    @Test func editorGetsLiteralArgumentsGuiPathAndNoStdin() throws {
        let result = try LogOpener.run([
            "sh", "-c", "cat; printf '%s|%s' \"$PATH\" \"$1\" >&2", "editor", "a b $HOME *",
        ])
        #expect(result.succeeded)
        #expect(result.stderrText == CommandRunner.guiPath + "|a b $HOME *")
        #expect(!result.stderrTruncated)
    }

    @Test func closingStderrDoesNotEndTheEditor() throws {
        let result = try LogOpener.run(["/bin/sh", "-c", "exec 2>&-; sleep 0.2; exit 7"])
        #expect(result.termination == .exited(7))
        #expect(result.duration >= 0.2)
    }

    @Test func backgroundEditorIsNotKilledOrAwaited() throws {
        let result = try LogOpener.run(["/bin/sh", "-c", "sleep 10 & echo $! >&2; exit 0"])
        let child = try #require(pid_t(result.stderrText.trimmingCharacters(in: .whitespacesAndNewlines)))
        defer { kill(child, SIGKILL) }
        #expect(result.succeeded)
        #expect(result.duration < 5)
        #expect(kill(child, 0) == 0)
    }

    @Test func backgroundEditorCanWriteAfterLauncherExits() throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let marker = directory.appendingPathComponent("finished").path
        let result = try LogOpener.run([
            "/bin/sh", "-c", "(sleep 0.5; printf late >&2 && touch \"$1\") & echo $! >&2; exit 0", "editor", marker,
        ])
        #expect(result.succeeded)
        for _ in 0..<100 {
            if FileManager.default.fileExists(atPath: marker) { break }
            usleep(20_000)
        }
        #expect(FileManager.default.fileExists(atPath: marker))
    }

    @Test func launchFailureAndSignalAreReported() throws {
        #expect(throws: CommandError.emptyArguments) { try LogOpener.run([]) }
        #expect(throws: CommandError.self) { try LogOpener.run(["/nonexistent/editor"]) }
        let result = try LogOpener.run(["/bin/sh", "-c", "echo interrupted >&2; kill -TERM $$"])
        #expect(result.termination == .signaled(SIGTERM))
        #expect(result.stderrText == "interrupted\n")
    }
}
