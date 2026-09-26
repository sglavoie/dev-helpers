import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct HealthCheckPlanTests {
    typealias F = SnapshotFixtures

    static let health = HealthCommandConfig(command: ["/bin/check"], intervalSeconds: 300)

    static func job(_ label: String, interval: Int = 300) -> HealthCheckJob {
        HealthCheckJob(label: label, config: HealthCommandConfig(command: ["/bin/check"], intervalSeconds: interval))
    }

    static func result(ago seconds: TimeInterval) -> HealthCheckResult {
        HealthCheckResult(finishedAt: F.now.addingTimeInterval(-seconds), outcome: .ok)
    }

    @Test func jobsSkipHiddenPausedAndAgentsWithoutACommand() {
        let active = F.agent("vault-guard"), hidden = F.agent("hidden"), paused = F.agent("paused"), plain = F.agent("plain")
        var state = HeartbeatState(bootTime: F.boot)
        state[paused.label].paused = true
        let config = F.config([
            active.label: AgentConfig(health: Self.health),
            hidden.label: AgentConfig(hidden: true, health: Self.health),
            paused.label: AgentConfig(health: Self.health),
        ])
        let snapshot = F.builder([active, hidden, paused, plain], prints: Dictionary(uniqueKeysWithValues: [
            F.loaded(active.label), F.loaded(hidden.label), F.loaded(plain.label),
        ])).buildSync(config: config, state: state, ledger: .readOnly)

        #expect(HealthCheckPlan.jobs(snapshot) == [HealthCheckJob(label: active.label, config: Self.health)])
    }

    @Test func dueWithoutAResultOrAfterTheInterval() {
        let jobs = [Self.job("fresh"), Self.job("old"), Self.job("never"), Self.job("exact")]
        let results = ["fresh": Self.result(ago: 60), "old": Self.result(ago: 900), "exact": Self.result(ago: 300)]
        // Never-run first, then the most overdue.
        #expect(HealthCheckPlan.due(jobs, results: results, now: F.now).map(\.label) == ["never", "old", "exact"])
    }

    @Test func nextDueIsTheEarliestFutureRun() {
        let jobs = [Self.job("a"), Self.job("b", interval: 120)]
        let results = ["a": Self.result(ago: 60), "b": Self.result(ago: 60)]
        #expect(HealthCheckPlan.nextDue(jobs, results: results, now: F.now) == F.now.addingTimeInterval(60))
        // Jobs already due don't count; with nothing left in the future there's no timer.
        #expect(HealthCheckPlan.nextDue([Self.job("c")], results: [:], now: F.now) == nil)
    }
}
