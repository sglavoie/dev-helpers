import Foundation
import HeartbeatCore

/// Runs `goback status --json` every `MacBackupsClient.interval` (and on menu open when older than that) on its own
/// detached task, apart from Monitor's agent poll. goback is run by hand, so the history changes rarely; one check
/// runs at a time and a refresh during one runs again right after it.
@MainActor
final class BackupsMonitor {
    /// Called on the main actor after every check.
    var onUpdate: (() -> Void)?
    var onActivityChange: () -> Void = {}
    private(set) var check: MacBackupsCheck?
    private(set) var isChecking = false

    private let client: MacBackupsClient
    private let interval: TimeInterval
    private var checkQueued = false
    private var nextCheck: DispatchWorkItem?

    init(client: MacBackupsClient = MacBackupsClient(), interval: TimeInterval = MacBackupsClient.interval) {
        self.client = client
        self.interval = interval
    }

    isolated deinit {
        nextCheck?.cancel()
    }

    /// Checks now, or right after the check in flight.
    func refresh() {
        guard !isChecking else {
            checkQueued = true
            return
        }
        isChecking = true
        onActivityChange()
        nextCheck?.cancel()
        let client = client
        Task { [weak self] in
            let check = await Task.detached(priority: .utility) { client.check() }.value
            self?.finish(check)
        }
    }

    /// The menu is opening: check if the last result is older than the interval.
    func refreshIfStale() {
        let age = check.map { Date().timeIntervalSince($0.checkedAt) } ?? .infinity
        if age >= interval { refresh() }
    }

    private func finish(_ check: MacBackupsCheck) {
        isChecking = false
        self.check = check
        onUpdate?()
        if checkQueued {
            checkQueued = false
            refresh()
        } else {
            schedule(after: interval)
        }
        onActivityChange()
    }

    private func schedule(after seconds: TimeInterval) {
        nextCheck?.cancel()
        let item = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.refresh() }
        }
        nextCheck = item
        DispatchQueue.main.asyncAfter(deadline: .now() + seconds, execute: item)
    }
}
