import Foundation
import Synchronization
import Testing
import HeartbeatCore
@testable import HeartbeatApp

/// Scripted SSH responses, with an optional gate to exercise a host change during a request.
private final class ScriptedRunner: CommandRunning, Sendable {
    struct Step: Sendable {
        var text: String = ""
        var exit: Int32 = 0
        var gate: DispatchSemaphore? = nil
    }

    private let steps: Mutex<[Step]>

    init(_ steps: [Step]) { self.steps = Mutex(steps) }

    func run(_ argv: [String], timeout: TimeInterval) throws -> CommandResult {
        let step = steps.withLock { $0.isEmpty ? Step(exit: 255) : $0.removeFirst() }
        if let gate = step.gate { _ = gate.wait(timeout: .now() + 5) }
        return CommandResult(argv: argv, termination: .exited(step.exit), stdout: Data(step.text.utf8),
                             stderr: step.exit == 0 ? Data() : Data("Connection failed".utf8))
    }
}

@MainActor
private func waitUntil(_ predicate: () -> Bool) async throws {
    let deadline = ContinuousClock.now.advanced(by: .seconds(3))
    while !predicate(), ContinuousClock.now < deadline {
        try await Task.sleep(for: .milliseconds(5))
    }
    try #require(predicate(), "Timed out waiting for the model to finish refreshing")
}

@Suite @MainActor struct RefreshTests {
    @Test(arguments: [false, true]) func changingPiHostClearsOldStatus(duringRequest: Bool) async throws {
        let gate = DispatchSemaphore(value: 0)
        defer { gate.signal() }
        var steps: [ScriptedRunner.Step] = [.init(exit: 255)]
        if duringRequest { steps.append(.init(exit: 255, gate: gate)) }
        steps.append(.init(text: #"{"status":"ok","sections":{}}"#))
        let monitor = PiMonitor(client: PiStatusClient(runner: ScriptedRunner(steps)))
        var publishedHosts: [String] = []
        monitor.onUpdate = { publishedHosts.append($0.host) }
        monitor.configure(HeartbeatConfig(piHost: "old-host"))
        try await waitUntil { !monitor.isChecking }
        #expect(monitor.check?.host == "old-host")
        #expect(OverallStatus.ok.including(monitor.check) == .warning)

        if duringRequest { monitor.refresh() }
        monitor.configure(HeartbeatConfig(piHost: "new-host"))
        #expect(monitor.check == nil)
        #expect(monitor.isChecking)
        #expect(OverallStatus.ok.including(monitor.check) == .ok)
        gate.signal()
        try await waitUntil { !monitor.isChecking }
        #expect(monitor.check?.host == "new-host")
        #expect(monitor.check?.severity == .ok)
        #expect(publishedHosts == ["old-host", "new-host"])
    }

    @Test(arguments: ["", "latest error\nearlier error\n"])
    func journalRetainsLastSuccessfulFetch(text: String) async throws {
        let runner = ScriptedRunner([.init(text: text), .init(exit: 255), .init(text: "recovered\n"), .init(exit: 255)])
        let model = PiJournalModel(host: "old-host", client: PiStatusClient(runner: runner))
        model.reload()
        try await waitUntil { !model.isLoading }
        let log = try #require(model.log)
        let fetchedAt = try #require(model.fetchedAt)
        model.reload()
        try await waitUntil { !model.isLoading }
        #expect(model.log == log)
        #expect(model.fetchedAt == fetchedAt)
        #expect(model.error == "Connection failed")

        model.reload()
        try await waitUntil { !model.isLoading }
        #expect(model.log?.lines == ["recovered"])
        #expect(model.error == nil)

        model.update(host: "new-host")
        #expect(model.log == nil)
        #expect(model.fetchedAt == nil)
        try await waitUntil { !model.isLoading }
        #expect(model.log == nil)
        #expect(model.fetchedAt == nil)
        #expect(model.error != nil)
    }

    @Test func logRetainsOutputUntilSourceChanges() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let file = directory.appendingPathComponent("out.log")
        let missing = directory.appendingPathComponent("err.log").path
        try Data("ready\nERROR: disk full\nerror: retrying\n".utf8).write(to: file)
        let model = LogTailModel(paths: [file.path, missing], standardErrorPath: missing)
        model.reload()
        try await waitUntil { !model.isLoading }
        let tail = try #require(model.tail)
        let fetchedAt = try #require(model.fetchedAt)
        #expect(model.title(for: file.path) == "Standard output")
        #expect(model.title(for: missing) == "Standard error")
        model.filter = "error:"
        #expect(model.displayedLines == ["ERROR: disk full", "error: retrying"])
        #expect(model.displayedText == "ERROR: disk full\nerror: retrying")
        model.filter = "no match"
        #expect(model.displayedLines.isEmpty)
        model.filter = ""
        #expect(model.displayedText == tail.text)

        try FileManager.default.removeItem(at: file)
        model.reload()
        try await waitUntil { !model.isLoading }
        #expect(model.tail == tail)
        #expect(model.fetchedAt == fetchedAt)
        #expect(model.error != nil)

        try Data("rotated log\n".utf8).write(to: file)
        model.reload()
        try await waitUntil { !model.isLoading }
        #expect(model.tail?.lines == ["rotated log"])
        #expect(model.error == nil)

        model.selectedPath = missing
        #expect(model.tail == nil)
        #expect(model.fetchedAt == nil)
        try await waitUntil { !model.isLoading }
        #expect(model.tail == nil)
        #expect(model.error != nil)
    }
}
