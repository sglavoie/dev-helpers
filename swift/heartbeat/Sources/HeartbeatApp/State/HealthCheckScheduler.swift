import Foundation
import HeartbeatCore

/// Runs health commands (rule 7) on each agent's own interval, at most two at a time, with the fixed GUI PATH.
/// Monitor hands it the jobs after every poll and stores each result in state.json, then polls so the verdict
/// (and a banner) follows right away. Results survive a relaunch, so a restart doesn't rerun every command.
@MainActor
final class HealthCheckScheduler {
    static let maxConcurrent = 2

    /// Called on the main actor with each finished check.
    var onResult: ((String, HealthCheckResult) -> Void)?

    private var jobs: [HealthCheckJob] = []
    private var results: [String: HealthCheckResult] = [:]
    /// Waiting to start, in order; a label is never both queued and running.
    private var queue: [HealthCheckJob] = []
    private var running: Set<String> = []
    private var timer: DispatchWorkItem?
    private let runner: any CommandRunning = CommandRunner(environment: HealthCheck.environment())

    /// The current jobs and their last results (from state.json); queues whatever is due and sets the timer.
    func update(jobs: [HealthCheckJob], results: [String: HealthCheckResult]) {
        self.jobs = jobs
        self.results = results
        let labels = Set(jobs.map(\.label))
        queue.removeAll { !labels.contains($0.label) }
        enqueueDue()
    }

    /// "Run Health Check Now": runs it next regardless of its interval.
    func runNow(_ label: String) {
        guard let job = jobs.first(where: { $0.label == label }), !running.contains(label) else { return }
        queue.removeAll { $0.label == label }
        queue.insert(job, at: 0)
        startNext()
    }

    func isRunning(_ label: String) -> Bool {
        running.contains(label) || queue.contains { $0.label == label }
    }

    private func enqueueDue() {
        for job in HealthCheckPlan.due(jobs, results: results, now: Date())
        where !running.contains(job.label) && !queue.contains(where: { $0.label == job.label }) {
            queue.append(job)
        }
        startNext()
        scheduleTimer()
    }

    private func startNext() {
        while running.count < Self.maxConcurrent, !queue.isEmpty {
            let job = queue.removeFirst()
            running.insert(job.label)
            let runner = runner
            Task {
                let result = await Task.detached(priority: .utility) {
                    HealthCheck.run(job.config, runner: runner)
                }.value
                finish(job.label, result)
            }
        }
    }

    private func finish(_ label: String, _ result: HealthCheckResult) {
        running.remove(label)
        results[label] = result
        onResult?(label, result)
        enqueueDue()
    }

    /// Wakes up when the next job falls due. Polls call `update` too, which also catches up after sleep
    /// (this timer runs on uptime, which stops while the Mac sleeps).
    private func scheduleTimer() {
        timer?.cancel()
        let pending = jobs.filter { job in !running.contains(job.label) && !queue.contains { $0.label == job.label } }
        guard let next = HealthCheckPlan.nextDue(pending, results: results, now: Date()) else { return }
        let item = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.enqueueDue() }
        }
        timer = item
        DispatchQueue.main.asyncAfter(deadline: .now() + max(1, next.timeIntervalSinceNow), execute: item)
    }
}
