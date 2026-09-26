import AppKit
import HeartbeatCore

/// Owns the poll loop: loads the config, builds a snapshot with `ledger: .record`, saves state.json and hands the
/// snapshot to the UI. Polls every `pollSeconds`, 90 s after wake (launchd runs missed calendar jobs on wake),
/// when the menu opens on a stale snapshot, and 0.5 s after the LaunchAgents directory, a stowed plist target or
/// the config changes. Only one poll runs at a time; a trigger during a poll queues exactly one more.
@MainActor
final class Monitor {
    static let wakeDelay: TimeInterval = 90
    static let staleMenuAge: TimeInterval = 15
    /// launchd needs a moment to start or unload a job, so actions poll twice.
    static let actionFollowUps: [TimeInterval] = [1, 5]

    /// Called on the main actor after every poll.
    var onUpdate: ((Snapshot) -> Void)?
    private(set) var snapshot: Snapshot?
    /// Why state.json couldn't be read or written, shown in the menu.
    private(set) var stateProblem: String?

    private let builder = SnapshotBuilder()
    private let store = StateStore()
    private var loader = ConfigLoader()
    private var state = HeartbeatState.empty
    /// Pause changes made while a poll was running; they win over the state that poll started from.
    private var pausedDuringPoll: [String: Bool] = [:]
    private var isPolling = false
    private var pollQueued = false
    private var nextPoll: DispatchWorkItem?
    private var wakePoll: DispatchWorkItem?
    private var wakeObserver: NSObjectProtocol?
    private lazy var watcher = DirectoryWatcher { [weak self] in self?.refresh() }

    func start() {
        do {
            let loaded = try store.load()
            state = loaded.state
            if let quarantined = loaded.quarantinedURL {
                stateProblem = "state.json was unreadable; moved to \(quarantined.lastPathComponent)"
            }
        } catch {
            stateProblem = "cannot read state.json: \(error.localizedDescription)"
        }
        wakeObserver = NSWorkspace.shared.notificationCenter.addObserver(
            forName: NSWorkspace.didWakeNotification, object: nil, queue: .main
        ) { [weak self] _ in
            MainActor.assumeIsolated { self?.scheduleWakePoll() }
        }
        watcher.watch(watchedPaths(for: nil))
        refresh()
    }

    /// Polls now, or right after the poll in flight.
    func refresh() {
        guard !isPolling else {
            pollQueued = true
            return
        }
        isPolling = true
        nextPoll?.cancel()
        let config = loader.load()
        let builder = builder, state = state
        Task { [weak self] in
            let snapshot = await builder.build(config: config, state: state, ledger: .record)
            self?.finish(snapshot)
        }
    }

    /// After a launchctl action: poll 1 s and 5 s later.
    func refreshAfterAction() {
        for delay in Self.actionFollowUps {
            DispatchQueue.main.asyncAfter(deadline: .now() + delay) { [weak self] in
                MainActor.assumeIsolated { self?.refresh() }
            }
        }
    }

    /// Marks an agent unloaded from the menu (gray instead of amber) and saves state.json right away.
    func setPaused(_ label: String, _ paused: Bool) {
        state[label].paused = paused
        if isPolling { pausedDuringPoll[label] = paused }
        saveState()
    }

    func isPaused(_ label: String) -> Bool {
        state[label].paused
    }

    /// The menu is opening: poll if the snapshot is older than 15 s.
    func refreshIfStale() {
        guard let snapshot else { return refresh() }
        if Date().timeIntervalSince(snapshot.takenAt) > Self.staleMenuAge {
            refresh()
        }
    }

    private func finish(_ snapshot: Snapshot) {
        state = snapshot.state
        for (label, paused) in pausedDuringPoll { state[label].paused = paused }
        pausedDuringPoll = [:]
        saveState()
        self.snapshot = snapshot
        watcher.watch(watchedPaths(for: snapshot))
        isPolling = false
        onUpdate?(snapshot)

        if pollQueued {
            pollQueued = false
            refresh()
        } else {
            schedulePoll(after: TimeInterval(max(snapshot.config.pollSeconds, HeartbeatConfig.minimumPollSeconds)))
        }
    }

    private func saveState() {
        do {
            try store.save(state)
            if stateProblem?.hasPrefix("cannot write") == true { stateProblem = nil }
        } catch {
            stateProblem = "cannot write state.json: \(error.localizedDescription)"
        }
    }

    private func schedulePoll(after seconds: TimeInterval) {
        nextPoll?.cancel()
        let item = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.refresh() }
        }
        nextPoll = item
        DispatchQueue.main.asyncAfter(deadline: .now() + seconds, execute: item)
    }

    private func scheduleWakePoll() {
        wakePoll?.cancel()
        let item = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.refresh() }
        }
        wakePoll = item
        DispatchQueue.main.asyncAfter(deadline: .now() + Self.wakeDelay, execute: item)
    }

    /// The LaunchAgents directory (symlinks added or removed), each stowed plist target and its directory
    /// (edits in place or by rename), and the config file and its directory.
    private func watchedPaths(for snapshot: Snapshot?) -> Set<String> {
        let config = ConfigLoader.defaultURL
        let configDirectory = config.deletingLastPathComponent()
        var paths: Set<String> = [AgentDiscovery.defaultDirectory.path, config.path, configDirectory.path]
        // Until ~/.config/heartbeat exists, its parent is what sees it appear.
        if !FileManager.default.fileExists(atPath: configDirectory.path) {
            paths.insert(configDirectory.deletingLastPathComponent().path)
        }
        for agent in snapshot?.agents ?? [] where agent.agent.resolvedPlistPath != agent.agent.plistPath {
            let target = URL(fileURLWithPath: agent.agent.resolvedPlistPath)
            paths.insert(target.path)
            paths.insert(target.deletingLastPathComponent().path)
        }
        return paths
    }
}
