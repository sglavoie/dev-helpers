import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct StateStoreTests {
    let directory: URL
    let store: StateStore

    init() {
        directory = FileManager.default.temporaryDirectory
            .appending(path: "HeartbeatCoreTests-\(UUID().uuidString)", directoryHint: .isDirectory)
        store = StateStore(url: directory.appending(path: "nested/state.json"))
    }

    static let sample = HeartbeatState(bootTime: Date(timeIntervalSince1970: 1_790_000_000), agents: [
        "com.sglavoie.sync-legacy": AgentState(
            runs: 4, runsChangedAt: Date(timeIntervalSince1970: 1_790_100_000.25),
            loadedObservedAt: Date(timeIntervalSince1970: 1_790_000_100), notifiedSeverity: .failing),
        "com.sglavoie.brainnotes-vault-guard": AgentState(
            paused: true, notRunningStreak: 2,
            lastHealthCheck: HealthCheckResult(finishedAt: Date(timeIntervalSince1970: 1_790_100_500),
                                               outcome: .warning(exitCode: 2), detail: "cannot check")),
    ])

    @Test func defaultLocationIsApplicationSupport() {
        #expect(StateStore.defaultURL.path.hasSuffix("Library/Application Support/Heartbeat/state.json"))
    }

    @Test func missingFileLoadsEmpty() throws {
        let result = try store.load()
        #expect(result.state == .empty)
        #expect(result.quarantinedURL == nil)
        #expect(store.read() == nil)
        #expect(!FileManager.default.fileExists(atPath: store.url.path))
    }

    @Test func saveRoundTripsExactly() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try store.save(Self.sample)
        #expect(try store.load().state == Self.sample)
        #expect(store.read() == Self.sample)
        let text = String(decoding: try Data(contentsOf: store.url), as: UTF8.self)
        #expect(text.contains("\"notifiedSeverity\" : \"failing\""))
        #expect(text.contains("\"runs\" : 4"))
    }

    @Test func saveReplacesAtomicallyAndLeavesNoTempFiles() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try store.save(.empty)
        try store.save(Self.sample)
        let files = try FileManager.default.contentsOfDirectory(atPath: store.url.deletingLastPathComponent().path)
        #expect(files == ["state.json"])
        #expect(store.read() == Self.sample)
    }

    @Test func missingFieldsDecodeToDefaults() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try FileManager.default.createDirectory(at: store.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data(#"{"version": 1, "agents": {"a": {}}}"#.utf8).write(to: store.url)
        let state = try store.load().state
        #expect(state["a"] == AgentState())
        #expect(state.bootTime == nil)
    }

    @Test func corruptFileIsQuarantinedAndStartsEmpty() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try FileManager.default.createDirectory(at: store.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{ nope".utf8).write(to: store.url)
        #expect(store.read() == nil)

        let result = try store.load(now: Date(timeIntervalSince1970: 1_790_000_000))
        #expect(result.state == .empty)
        let quarantined = try #require(result.quarantinedURL)
        #expect(quarantined.lastPathComponent == "state.corrupt-1790000000.json")
        #expect(FileManager.default.fileExists(atPath: quarantined.path))
        #expect(!FileManager.default.fileExists(atPath: store.url.path))

        try Data("{ again".utf8).write(to: store.url)
        let second = try store.load(now: Date(timeIntervalSince1970: 1_790_000_000))
        #expect(second.quarantinedURL?.lastPathComponent == "state.corrupt-1790000000-1.json")
    }

    @Test func notifiedHelpers() {
        var state = Self.sample
        #expect(state.notifiedSeverities == ["com.sglavoie.sync-legacy": .failing])
        state.applyNotified(["b": .failing], evaluated: ["com.sglavoie.sync-legacy", "b"])
        #expect(state.notifiedSeverities == ["b": .failing])
        #expect(state["com.sglavoie.sync-legacy"].runs == 4)
    }
}

@Suite struct LedgerTests {
    static let t0 = Date(timeIntervalSince1970: 1_790_000_000)
    static func at(_ minutes: Double) -> Date { t0.addingTimeInterval(minutes * 60) }

    static func loaded(runs: Int?, pid: Int32? = nil) -> ServiceStatus {
        .loaded(ServiceRuntime(state: pid == nil ? .notRunning : .running, pid: pid, runs: runs, lastExit: .exited(code: 0)))
    }

    @Test func firstObservationRecordsLoadButNoRun() {
        var agent = AgentState()
        Ledger.record(Self.loaded(runs: 5), keepAlive: false, now: Self.at(0), in: &agent)
        #expect(agent.runs == 5)
        #expect(agent.loadedObservedAt == Self.at(0))
        #expect(agent.runsChangedAt == nil)
        #expect(Ledger.evidenceDate(agent) == nil)
    }

    @Test func runsIncrementIsEvidence() {
        var agent = AgentState()
        Ledger.record(Self.loaded(runs: 5), keepAlive: false, now: Self.at(0), in: &agent)
        Ledger.record(Self.loaded(runs: 5), keepAlive: false, now: Self.at(1), in: &agent)
        #expect(agent.runsChangedAt == nil)
        Ledger.record(Self.loaded(runs: 6), keepAlive: false, now: Self.at(2), in: &agent)
        #expect(agent.runsChangedAt == Self.at(2))
        #expect(Ledger.evidenceDate(agent) == Self.at(2))
        #expect(agent.lastRestartAt == nil)
        #expect(agent.loadedObservedAt == Self.at(0))
    }

    @Test func runningKeepAliveIncrementIsARestart() {
        var agent = AgentState()
        Ledger.record(Self.loaded(runs: 1, pid: 10), keepAlive: true, now: Self.at(0), in: &agent)
        Ledger.record(Self.loaded(runs: 2, pid: 11), keepAlive: true, now: Self.at(5), in: &agent)
        #expect(agent.lastRestartAt == Self.at(5))
        #expect(agent.history.lastRestartAt == Self.at(5))

        // Between restarts with no PID: runs going up is a run, not a restart.
        Ledger.record(Self.loaded(runs: 3), keepAlive: true, now: Self.at(6), in: &agent)
        #expect(agent.lastRestartAt == Self.at(5))
        #expect(agent.runsChangedAt == Self.at(6))
    }

    @Test func seeingTheAgentLoadedEndsAPause() {
        var agent = AgentState(paused: true)
        Ledger.record(.notLoaded, keepAlive: false, now: Self.at(0), in: &agent)
        #expect(agent.paused)
        Ledger.record(.unknown(reason: "timeout"), keepAlive: false, now: Self.at(1), in: &agent)
        #expect(agent.paused)
        Ledger.record(Self.loaded(runs: 0), keepAlive: false, now: Self.at(2), in: &agent)
        #expect(!agent.paused)
    }

    @Test func unloadForgetsTheSessionAndReloadStartsOver() {
        var agent = AgentState()
        Ledger.record(Self.loaded(runs: 5), keepAlive: false, now: Self.at(0), in: &agent)
        Ledger.record(Self.loaded(runs: 6), keepAlive: false, now: Self.at(1), in: &agent)
        Ledger.record(.notLoaded, keepAlive: false, now: Self.at(2), in: &agent)
        #expect(agent.runs == nil)
        #expect(agent.loadedObservedAt == nil)
        #expect(agent.runsChangedAt == Self.at(1))
        Ledger.record(Self.loaded(runs: 1), keepAlive: false, now: Self.at(3), in: &agent)
        #expect(agent.loadedObservedAt == Self.at(3))
        #expect(agent.runsChangedAt == Self.at(1))
    }

    @Test func runsGoingDownMeansAnUnseenReload() {
        var agent = AgentState()
        Ledger.record(Self.loaded(runs: 9), keepAlive: false, now: Self.at(0), in: &agent)
        Ledger.record(Self.loaded(runs: 1), keepAlive: false, now: Self.at(5), in: &agent)
        #expect(agent.runs == 1)
        #expect(agent.loadedObservedAt == Self.at(5))
        #expect(agent.runsChangedAt == nil)
    }

    @Test func unreadableStatusChangesNothing() {
        var agent = AgentState()
        Ledger.record(Self.loaded(runs: 3), keepAlive: false, now: Self.at(0), in: &agent)
        let before = agent
        Ledger.record(.unknown(reason: "timed out"), keepAlive: false, now: Self.at(1), in: &agent)
        #expect(agent == before)
        Ledger.record(Self.loaded(runs: nil), keepAlive: false, now: Self.at(2), in: &agent)
        #expect(agent == before)
    }

    @Test func rebootResetsSessionFieldsButKeepsTheRest() {
        var state = HeartbeatState()
        Ledger.noteBoot(Self.at(0), in: &state)
        #expect(state.bootTime == Self.at(0))
        state["a"] = AgentState(runs: 40, runsChangedAt: Self.at(10), loadedObservedAt: Self.at(1), paused: true,
                                notifiedSeverity: .failing, notRunningStreak: 3)
        Ledger.noteBoot(Self.at(0), in: &state)
        #expect(state["a"].runs == 40)

        Ledger.noteBoot(Self.at(100), in: &state)
        #expect(state.bootTime == Self.at(100))
        #expect(state["a"] == AgentState(runsChangedAt: Self.at(10), paused: true, notifiedSeverity: .failing))

        // After the reboot a small runs count is a first observation, not a decrease.
        Ledger.record(Self.loaded(runs: 1), keepAlive: false, now: Self.at(101), in: &state["a"])
        #expect(state["a"].loadedObservedAt == Self.at(101))
        #expect(state["a"].runsChangedAt == Self.at(10))
    }

    @Test func verdictFeedsTheNotRunningStreak() {
        var agent = AgentState()
        let missing = HealthVerdict(label: "a", severity: .warning, keepAliveMissing: true)
        Ledger.record(missing, in: &agent)
        #expect(agent.history.previousNotRunningPolls == 1)
        Ledger.record(missing, in: &agent)
        #expect(agent.notRunningStreak == 2)
        Ledger.record(HealthVerdict(label: "a", severity: .ok), in: &agent)
        #expect(agent.notRunningStreak == 0)
    }

    /// Two polls through the evaluator: rule 5 turns red only on the second miss, via the ledger's streak.
    @Test func streakDrivesRuleFiveAcrossPolls() {
        let agent = AgentDefinition(label: "a", plistPath: "/tmp/a.plist", keepAlive: .always)
        let context = HealthContext(now: Self.at(60), power: PowerTimeline(bootTime: Self.t0), calendar: TimeFixtures.montreal)
        var state = AgentState()
        var severities: [Severity] = []
        for poll in 0..<2 {
            let status = Self.loaded(runs: 3)
            Ledger.record(status, keepAlive: true, now: Self.at(Double(poll)), in: &state)
            let verdict = HealthEvaluator.evaluate(HealthInput(agent: agent, status: status, history: state.history), context: context)
            Ledger.record(verdict, in: &state)
            severities.append(verdict.severity)
        }
        #expect(severities == [.warning, .failing])
    }

    @Test func pruneKeepsDiscoveredAndPaused() {
        var state = HeartbeatState(agents: ["a": AgentState(), "gone": AgentState(), "paused": AgentState(paused: true)])
        Ledger.prune(&state, keeping: ["a"])
        #expect(state.agents.keys.sorted() == ["a", "paused"])
    }
}
