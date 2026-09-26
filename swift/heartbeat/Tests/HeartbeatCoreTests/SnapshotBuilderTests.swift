import Foundation
import Testing
@testable import HeartbeatCore

/// Builders for snapshot tests: a fixed clock, boot time and file system, and launchctl answered by a fake runner.
enum SnapshotFixtures {
    static let now = TimeFixtures.local(2026, 9, 26, 12, 41)
    static let boot = TimeFixtures.local(2026, 9, 20, 8, 0)
    static let power = PowerTimeline(bootTime: boot)

    /// A minimal top-level `launchctl print` body.
    static func printOutput(_ label: String, state: String = "not running", pid: Int? = nil, runs: Int = 1,
                            lastExit: String = "0") -> String {
        var lines = ["gui/501/\(label) = {", "\tactive count = 0", "\tpath = /tmp/\(label).plist", "\tstate = \(state)"]
        if let pid { lines.append("\tpid = \(pid)") }
        lines += ["\truns = \(runs)", "\tlast exit code = \(lastExit)", "}"]
        return lines.joined(separator: "\n")
    }

    static func agent(_ name: String, schedule: Schedule = .interval(seconds: 900), keepAlive: KeepAlivePolicy = .none,
                      log: String? = nil, disabled: Bool = false) -> AgentDefinition {
        AgentDefinition(label: "com.sglavoie.\(name)", plistPath: "/LA/com.sglavoie.\(name).plist",
                        programArguments: ["/bin/\(name)"], schedule: schedule, keepAlive: keepAlive, disabled: disabled,
                        standardOutPath: log)
    }

    static func builder(
        _ agents: [AgentDefinition], prints: [String: CommandResult], files: [String: Date] = [:],
        problems: [DiscoveryProblem] = [], receipts: @escaping @Sendable (ReceiptConfig) -> ReceiptStatus = { .missing(path: $0.path) },
        healthRunner: any CommandRunning = FakeRunner(), power: PowerTimeline? = SnapshotFixtures.power
    ) -> SnapshotBuilder {
        SnapshotBuilder(
            discover: { _ in DiscoveryResult(agents: agents, problems: problems) },
            launchctl: LaunchctlClient(runner: FakeRunner(prints, fallback: result(113)), uid: 501),
            healthRunner: healthRunner,
            readReceipt: receipts,
            modificationDate: { files[$0] },
            power: { power },
            now: { SnapshotFixtures.now },
            calendar: TimeFixtures.montreal)
    }

    static func loaded(_ label: String, state: String = "not running", pid: Int? = nil, runs: Int = 1,
                       lastExit: String = "0") -> (String, CommandResult) {
        ("gui/501/\(label)", result(0, stdout: printOutput(label, state: state, pid: pid, runs: runs, lastExit: lastExit)))
    }

    static func config(_ agents: [String: AgentConfig] = [:], error: String? = nil) -> ConfigLoadResult {
        ConfigLoadResult(config: HeartbeatConfig(agents: agents), source: error == nil ? .file : .lastGood, error: error)
    }
}

/// Counts calls from concurrent closures.
final class Counter: @unchecked Sendable {
    private let lock = NSLock()
    private var count = 0
    var value: Int { lock.withLock { count } }
    func increment() { lock.withLock { count += 1 } }
}

@Suite struct SnapshotBuilderTests {
    typealias F = SnapshotFixtures

    @Test func failedLastExitMakesOverallFailing() {
        let sync = F.agent("sync-legacy", schedule: .watchPaths(["/a"], throttleSeconds: 30), log: "/logs/sync.log")
        let forgejo = F.agent("forgejo-sync", log: "/logs/forgejo.log")
        let builder = F.builder([forgejo, sync], prints: Dictionary(uniqueKeysWithValues: [
            F.loaded(sync.label, runs: 4, lastExit: "1"), F.loaded(forgejo.label, runs: 18),
        ]), files: ["/logs/sync.log": F.now.addingTimeInterval(-3 * 3600), "/logs/forgejo.log": F.now.addingTimeInterval(-240)])

        let snapshot = builder.buildSync(config: F.config(), state: .empty, ledger: .readOnly)

        #expect(snapshot.overall == .failing)
        #expect(snapshot.overall.exitCode == 2)
        #expect(snapshot.agents.map(\.label) == [forgejo.label, sync.label])
        let failing = try! #require(snapshot.agent(sync.label))
        #expect(failing.severity == .failing)
        #expect(failing.verdict.summary == "Exited with status 1")
        #expect(failing.evidence == Evidence(hasSource: true, latest: F.now.addingTimeInterval(-3 * 3600), origin: "/logs/sync.log"))
        #expect(snapshot.agent(forgejo.label)?.severity == .ok)
    }

    @Test func evidenceIsTheNewestSource() {
        let agent = F.agent("pi-backup-fetch", schedule: .calendar([CalendarEntry(minute: 10, hour: 6)]), log: "/logs/pi.log")
        let finished = F.now.addingTimeInterval(-600)
        var state = HeartbeatState(bootTime: F.boot)
        state[agent.label].runsChangedAt = F.now.addingTimeInterval(-1200)
        let config = F.config([agent.label: AgentConfig(
            evidencePaths: ["/data/marker"], receipt: ReceiptConfig(path: "/data/last-run.json"))])
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)]),
                                files: ["/logs/pi.log": F.now.addingTimeInterval(-3600)],
                                receipts: { _ in .ok(finished: finished) })

        let snapshot = builder.buildSync(config: config, state: state, ledger: .readOnly)
        let result = try! #require(snapshot.agent(agent.label))

        #expect(result.evidence == Evidence(hasSource: true, latest: finished, origin: "/data/last-run.json"))
        #expect(result.evidenceSources == [
            EvidenceSource(kind: .log, name: "/logs/pi.log", date: F.now.addingTimeInterval(-3600)),
            EvidenceSource(kind: .evidencePath, name: "/data/marker", date: nil),
            EvidenceSource(kind: .runsLedger, name: "runs ledger", date: F.now.addingTimeInterval(-1200)),
            EvidenceSource(kind: .receipt, name: "/data/last-run.json", date: finished),
        ])
        #expect(result.receipt == ReceiptStatus.ok(finished: finished))
    }

    @Test func declaredButMissingLogIsASourceWithoutADate() {
        let agent = F.agent("job", log: "/logs/never.log")
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)]))
        let snapshot = builder.buildSync(config: F.config(), state: .empty, ledger: .readOnly)
        #expect(snapshot.agents[0].evidence == Evidence(hasSource: true))
        // Awake for days with no run recorded and a 15 min interval: overdue.
        #expect(snapshot.agents[0].verdict.summary == "Overdue: no run recorded")
    }

    @Test func noEvidenceSourceIsCannotVerify() {
        let agent = F.agent("asl-companion")
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)]))
        let snapshot = builder.buildSync(config: F.config(), state: .empty, ledger: .readOnly)
        #expect(snapshot.agents[0].evidence == .none)
        #expect(snapshot.agents[0].severity == .warning)
        #expect(snapshot.overall == .warning)
    }

    @Test func readOnlyLeavesTheLedgerAlone() {
        let agent = F.agent("job", log: "/logs/job.log")
        var state = HeartbeatState(bootTime: F.boot)
        state[agent.label].runs = 3
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label, runs: 5)]),
                                files: ["/logs/job.log": F.now])

        let snapshot = builder.buildSync(config: F.config(), state: state, ledger: .readOnly)

        #expect(snapshot.state == state)
    }

    @Test func recordUpdatesTheLedgerAndPrunes() {
        let agent = F.agent("job", log: "/logs/job.log")
        var state = HeartbeatState(bootTime: F.boot)
        state[agent.label].runs = 3
        state["com.sglavoie.gone"].runs = 1
        state["com.sglavoie.paused"].paused = true
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label, runs: 5)]),
                                files: ["/logs/job.log": F.now.addingTimeInterval(-60)])

        let snapshot = builder.buildSync(config: F.config(), state: state, ledger: .record)

        let recorded = snapshot.state[agent.label]
        #expect(recorded.runs == 5)
        #expect(recorded.runsChangedAt == F.now)
        #expect(recorded.loadedObservedAt == F.now)
        #expect(snapshot.agents[0].evidence.latest == F.now)
        #expect(snapshot.agents[0].evidence.origin == "runs ledger")
        #expect(Set(snapshot.state.agents.keys) == [agent.label, "com.sglavoie.paused"])
    }

    @Test func recordCountsTheKeepAliveStreak() {
        let agent = F.agent("daemon", schedule: .none, keepAlive: .always, log: "/logs/d.log")
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)]),
                                files: ["/logs/d.log": F.now])

        let first = builder.buildSync(config: F.config(), state: .empty, ledger: .record)
        let second = builder.buildSync(config: F.config(), state: first.state, ledger: .record)

        #expect(first.agents[0].severity == .warning)
        #expect(second.agents[0].severity == .failing)
        #expect(second.state[agent.label].notRunningStreak == 2)
    }

    @Test func newBootResetsSavedHistoryEvenReadOnly() {
        let agent = F.agent("job", log: "/logs/job.log")
        var state = HeartbeatState(bootTime: F.boot.addingTimeInterval(-86400))
        state[agent.label].loadedObservedAt = F.now
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)]),
                                files: ["/logs/job.log": F.now])
        let snapshot = builder.buildSync(config: F.config(), state: state, ledger: .readOnly)
        #expect(snapshot.agents[0].history.loadedObservedAt == nil)
    }

    @Test func hiddenAndPausedAgentsReadNothing() {
        let hidden = F.agent("hidden", log: "/logs/h.log")
        let paused = F.agent("paused", log: "/logs/p.log")
        var state = HeartbeatState(bootTime: F.boot)
        state[paused.label].paused = true
        let receiptReads = Counter()
        let health = HealthCommandConfig(command: ["/bin/check"])
        let runner = FakeRunner(fallback: result(1))
        let config = F.config([
            hidden.label: AgentConfig(hidden: true, health: health, receipt: ReceiptConfig(path: "/r")),
            paused.label: AgentConfig(health: health, receipt: ReceiptConfig(path: "/r")),
        ])
        let builder = F.builder([hidden, paused], prints: Dictionary(uniqueKeysWithValues: [F.loaded(hidden.label)]),
                                receipts: { receiptReads.increment(); return .missing(path: $0.path) }, healthRunner: runner)

        let snapshot = builder.buildSync(config: config, state: state, ledger: .readOnly, runHealthChecks: true)

        #expect(snapshot.agents.map(\.severity) == [.hidden, .paused])
        #expect(receiptReads.value == 0)
        #expect(runner.recorded.isEmpty)
        // Paused agents are not "nothing discovered" and don't color the icon.
        #expect(snapshot.overall == .ok)
    }

    @Test func healthChecksRunOnlyWhenAsked() {
        let agent = F.agent("vault-guard", schedule: .none, keepAlive: .always, log: "/logs/v.log")
        let config = F.config([agent.label: AgentConfig(health: HealthCommandConfig(command: ["/bin/check"], warningExitCodes: [2]))])
        let runner = FakeRunner(["/bin/check": result(1, stderr: "vault missing\n")])
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label, state: "running", pid: 42)]),
                                files: ["/logs/v.log": F.now], healthRunner: runner)

        let cached = builder.buildSync(config: config, state: .empty, ledger: .readOnly)
        #expect(runner.recorded.isEmpty)
        #expect(cached.agents[0].severity == .ok)

        let live = builder.buildSync(config: config, state: .empty, ledger: .readOnly, runHealthChecks: true)
        #expect(runner.recorded == [["/bin/check"]])
        #expect(live.agents[0].healthCheck == HealthCheckResult(finishedAt: F.now, outcome: .failed(exitCode: 1), detail: "vault missing"))
        #expect(live.agents[0].verdict.summary == "Health check failed (exit 1): vault missing")
        #expect(live.state[agent.label].lastHealthCheck?.outcome == .failed(exitCode: 1))
    }

    @Test func savedHealthResultIsUsedWithoutRunning() {
        let agent = F.agent("vault-guard", schedule: .none, log: "/logs/v.log")
        var state = HeartbeatState(bootTime: F.boot)
        state[agent.label].lastHealthCheck = HealthCheckResult(finishedAt: F.now.addingTimeInterval(-60), outcome: .warning(exitCode: 2))
        let config = F.config([agent.label: AgentConfig(health: HealthCommandConfig(command: ["/bin/check"], warningExitCodes: [2]))])
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)]), files: ["/logs/v.log": F.now])
        let snapshot = builder.buildSync(config: config, state: state, ledger: .readOnly)
        #expect(snapshot.agents[0].severity == .warning)
    }

    @Test func overallRules() {
        let agent = F.agent("job", log: "/logs/job.log")
        let prints = Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)])
        let files = ["/logs/job.log": F.now]

        #expect(F.builder([agent], prints: prints, files: files).buildSync(config: F.config(), state: .empty, ledger: .readOnly).overall == .ok)
        #expect(F.builder([agent], prints: prints, files: files)
            .buildSync(config: F.config(error: "invalid JSON"), state: .empty, ledger: .readOnly).overall == .warning)
        #expect(F.builder([agent], prints: prints, files: files, problems: [.brokenSymlink(path: "/LA/x.plist", destination: "/gone")])
            .buildSync(config: F.config(), state: .empty, ledger: .readOnly).overall == .warning)
        #expect(F.builder([], prints: [:]).buildSync(config: F.config(), state: .empty, ledger: .readOnly).overall == .warning)
        #expect(F.builder([agent], prints: prints, files: files, power: nil)
            .buildSync(config: F.config(), state: .empty, ledger: .readOnly).overall == .unknown)

        var failing = F.builder([agent], prints: prints)
        failing.discover = { _ in throw CocoaError(.fileReadNoPermission) }
        let snapshot = failing.buildSync(config: F.config(), state: .empty, ledger: .record)
        #expect(snapshot.overall == .unknown)
        #expect(snapshot.overall.exitCode == 3)
        #expect(snapshot.fatalError?.hasPrefix("cannot list LaunchAgents") == true)
    }

    @Test func notLoadedIsWarning() {
        let agent = F.agent("job", log: "/logs/job.log")
        let snapshot = F.builder([agent], prints: [:]).buildSync(config: F.config(), state: .empty, ledger: .readOnly)
        #expect(snapshot.agents[0].status == .notLoaded)
        #expect(snapshot.agents[0].verdict.summary == "Not loaded")
    }

    @Test func asyncBuildMatchesSync() async {
        let agent = F.agent("job", log: "/logs/job.log")
        let builder = F.builder([agent], prints: Dictionary(uniqueKeysWithValues: [F.loaded(agent.label)]), files: ["/logs/job.log": F.now])
        let async = await builder.build(config: F.config(), state: .empty, ledger: .record)
        #expect(async == builder.buildSync(config: F.config(), state: .empty, ledger: .record))
    }

    @Test func nameUsesDisplayNameOrStripsPrefix() {
        let agent = F.agent("forgejo-sync")
        let snapshot = F.builder([agent], prints: [:])
            .buildSync(config: F.config([agent.label: AgentConfig(displayName: "Forgejo sync")]), state: .empty, ledger: .readOnly)
        #expect(snapshot.agents[0].name(labelPrefix: "com.sglavoie.") == "Forgejo sync")
        var plain = snapshot.agents[0]
        plain.config.displayName = nil
        #expect(plain.name(labelPrefix: "com.sglavoie.") == "forgejo-sync")
        #expect(plain.name(labelPrefix: "org.other.") == "com.sglavoie.forgejo-sync")
    }

    @Test func fileModificationDateFollowsSymlinks() throws {
        let directory = FileManager.default.temporaryDirectory.appending(path: "heartbeat-mtime-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let target = directory.appending(path: "target.log")
        try Data("x".utf8).write(to: target)
        let stamp = Date(timeIntervalSince1970: 1_700_000_000)
        try FileManager.default.setAttributes([.modificationDate: stamp], ofItemAtPath: target.path)
        let link = directory.appending(path: "link.log")
        try FileManager.default.createSymbolicLink(at: link, withDestinationURL: target)

        #expect(SnapshotBuilder.fileModificationDate(link.path) == stamp)
        #expect(SnapshotBuilder.fileModificationDate(directory.appending(path: "missing").path) == nil)
    }
}
