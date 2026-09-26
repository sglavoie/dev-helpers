import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct LogTailTests {
    let directory: URL

    init() throws {
        directory = FileManager.default.temporaryDirectory
            .appending(path: "HeartbeatLogTail-\(UUID().uuidString)", directoryHint: .isDirectory)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }

    func write(_ name: String, _ text: String) throws -> String {
        let url = directory.appending(path: name)
        try Data(text.utf8).write(to: url)
        return url.path
    }

    static func numbered(_ range: ClosedRange<Int>) -> String {
        range.map { "line \($0)\n" }.joined()
    }

    @Test func shortFileIsReturnedWhole() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let tail = try LogTail.read(path: write("short.log", "one\ntwo\nthree\n"))
        #expect(tail.lines == ["one", "two", "three"])
        #expect(!tail.truncated)
        #expect(tail.fileSize == 14)
        #expect(tail.modified != nil)
        #expect(tail.text == "one\ntwo\nthree")
    }

    @Test func keepsTheLastLines() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let tail = try LogTail.read(path: write("long.log", Self.numbered(1...500)), lineCount: 200)
        #expect(tail.lines.count == 200)
        #expect(tail.lines.first == "line 301")
        #expect(tail.lines.last == "line 500")
        #expect(tail.truncated)
    }

    @Test func readsAcrossChunkBoundaries() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        // ~100 KB of lines: more than one 64 KB chunk has to be joined without splitting a line wrongly.
        let text = (1...2000).map { "entry \($0) " + String(repeating: "x", count: 40) + "\n" }.joined()
        let tail = try LogTail.read(path: write("chunks.log", text), lineCount: 1500)
        #expect(tail.lines.count == 1500)
        #expect(tail.lines.first == "entry 501 " + String(repeating: "x", count: 40))
        #expect(tail.lines.last == "entry 2000 " + String(repeating: "x", count: 40))
    }

    @Test func lastLineWithoutNewlineAndCRLF() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let tail = try LogTail.read(path: write("crlf.log", "a\r\nb\r\n\r\nc"))
        #expect(tail.lines == ["a", "b", "", "c"])
    }

    @Test func byteCapDropsThePartialFirstLine() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let tail = try LogTail.read(path: write("cap.log", Self.numbered(1...100)), lineCount: 200, maxBytes: 30)
        // 30 bytes from the end of "...line 98\nline 99\nline 100\n" start mid-line.
        #expect(tail.lines == ["line 98", "line 99", "line 100"])
        #expect(tail.truncated)
    }

    @Test func oneHugeLineIsStillShown() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let tail = try LogTail.read(path: write("huge.log", String(repeating: "z", count: 5000)), maxBytes: 100)
        #expect(tail.lines == [String(repeating: "z", count: 100)])
        #expect(tail.truncated)
    }

    @Test func emptyFileHasNoLines() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let tail = try LogTail.read(path: write("empty.log", ""))
        #expect(tail.lines.isEmpty)
        #expect(!tail.truncated)
        #expect(tail.fileSize == 0)
    }

    @Test func invalidUTF8IsReplaced() {
        let tail = LogTail.tail(Data([0x6F, 0x6B, 0x0A, 0xFF, 0xFE, 0x21, 0x0A]), lineCount: 10)
        #expect(tail.lines == ["ok", "\u{FFFD}\u{FFFD}!"])
    }

    @Test func zeroLinesRequested() {
        let tail = LogTail.tail(Data("a\nb\n".utf8), lineCount: 0)
        #expect(tail.lines.isEmpty)
        #expect(tail.truncated)
    }

    @Test func missingFileAndDirectory() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let missing = directory.appending(path: "nope.log").path
        #expect(throws: LogTailError.notFound(path: missing)) { try LogTail.read(path: missing) }
        #expect(throws: LogTailError.unreadable(path: directory.path, reason: "is a directory")) {
            try LogTail.read(path: directory.path)
        }
        #expect(LogTailError.notFound(path: "/x.log").description == "/x.log does not exist yet")
    }

    @Test func openCommandSubstitutesPath() {
        let template = ["open", "-a", "Ghostty", "--args", "-e", "nvim", "{path}"]
        #expect(LogTail.openCommand(template, path: "/tmp/a b.log")
            == ["open", "-a", "Ghostty", "--args", "-e", "nvim", "/tmp/a b.log"])
        #expect(LogTail.openCommand(["sh", "-c", "tail -f '{path}'"], path: "/l.log") == ["sh", "-c", "tail -f '/l.log'"])
    }

    @Test func openCommandAppendsPathAndExpandsTilde() {
        #expect(LogTail.openCommand(["~/bin/view"], path: "/l.log", home: "/Users/me") == ["/Users/me/bin/view", "/l.log"])
        #expect(LogTail.openCommand(nil, path: "/l.log") == nil)
        #expect(LogTail.openCommand([], path: "/l.log") == nil)
        #expect(LogTail.openCommand([""], path: "/l.log") == nil)
    }
}

@Suite struct AgentActionTests {
    @Test func actionsFollowTheLaunchdStatus() {
        let idle = ServiceStatus.loaded(ServiceRuntime(state: .notRunning, runs: 3, lastExit: .exited(code: 1)))
        let running = ServiceStatus.loaded(ServiceRuntime(state: .running, pid: 42, runs: 1))
        let spawning = ServiceStatus.loaded(ServiceRuntime(state: .other("spawn scheduled")))
        #expect(AgentAction.available(for: idle) == [.runNow, .unload])
        #expect(AgentAction.available(for: running) == [.restart, .unload])
        #expect(AgentAction.available(for: spawning) == [.runNow, .unload])
        #expect(AgentAction.available(for: .notLoaded) == [.load])
        #expect(AgentAction.available(for: .unknown(reason: "timeout")).isEmpty)
    }

    @Test func onlyDisruptiveActionsAreConfirmed() {
        #expect(AgentAction.allCases.filter(\.needsConfirmation) == [.restart, .unload])
        #expect(AgentAction.allCases.map(\.title) == ["Run Now", "Restart…", "Unload…", "Load"])
    }
}
