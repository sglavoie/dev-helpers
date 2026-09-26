import Foundation

/// One agent's health command, as the app's scheduler runs it.
public struct HealthCheckJob: Equatable, Sendable {
    public var label: String
    public var config: HealthCommandConfig

    public init(label: String, config: HealthCommandConfig) {
        self.label = label
        self.config = config
    }
}

/// When health commands (rule 7) are due. Pure: the app's HealthCheckScheduler runs what this picks.
public enum HealthCheckPlan {
    /// Agents with a health command that rule 7 applies to: not hidden and not paused.
    public static func jobs(_ snapshot: Snapshot) -> [HealthCheckJob] {
        snapshot.agents.compactMap { agent in
            guard let health = agent.config.health, agent.severity != .hidden, agent.severity != .paused else { return nil }
            return HealthCheckJob(label: agent.label, config: health)
        }
    }

    /// When a job should run next: right away without a result, else one interval after the last one finished.
    public static func dueDate(_ job: HealthCheckJob, last: HealthCheckResult?) -> Date {
        guard let last else { return .distantPast }
        return last.finishedAt.addingTimeInterval(TimeInterval(job.config.intervalSeconds))
    }

    /// Jobs whose interval has passed, most overdue first.
    public static func due(_ jobs: [HealthCheckJob], results: [String: HealthCheckResult], now: Date) -> [HealthCheckJob] {
        jobs.map { ($0, dueDate($0, last: results[$0.label])) }
            .filter { $0.1 <= now }
            .sorted { $0.1 < $1.1 }
            .map(\.0)
    }

    /// The earliest time a job becomes due after `now`, for the scheduler's timer.
    public static func nextDue(_ jobs: [HealthCheckJob], results: [String: HealthCheckResult], now: Date) -> Date? {
        jobs.map { dueDate($0, last: results[$0.label]) }.filter { $0 > now }.min()
    }
}
