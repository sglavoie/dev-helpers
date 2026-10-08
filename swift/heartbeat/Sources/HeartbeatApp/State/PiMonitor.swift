import Foundation
import HeartbeatCore

/// Checks the Pi every `piStatusSeconds` (and on menu open when older than that) on its own detached task, apart
/// from Monitor's agent poll, so a slow or unreachable Pi (ssh can take up to `PiStatusClient.timeout`) never
/// delays it. One check runs at a time; a host change checks again at once. The Pi row never produces banners.
/// Each check also reads the lan-devices list, so the LAN devices row follows the same timer.
@MainActor
final class PiMonitor {
    /// Called on the main actor after every check.
    var onUpdate: ((PiCheck) -> Void)?
    var onActivityChange: () -> Void = {}
    private(set) var check: PiCheck?
    /// The lan-devices list read with the last check.
    private(set) var devices: LanDevicesCheck?
    private(set) var isChecking = false

    private let client: PiStatusClient
    private let devicesClient: LanDevicesClient
    private(set) var host: String?
    private var interval: TimeInterval = TimeInterval(HeartbeatConfig.defaults.piStatusSeconds)
    private var checkQueued = false
    private var nextCheck: DispatchWorkItem?

    init(client: PiStatusClient = PiStatusClient(), devicesClient: LanDevicesClient = LanDevicesClient()) {
        self.client = client
        self.devicesClient = devicesClient
    }

    isolated deinit {
        nextCheck?.cancel()
    }

    /// Takes piHost and piStatusSeconds from each poll's config; the first call (or a new host) starts a check.
    func configure(_ config: HeartbeatConfig) {
        let interval = TimeInterval(max(config.piStatusSeconds, HeartbeatConfig.minimumPollSeconds))
        let hostChanged = config.piHost != host
        let intervalChanged = interval != self.interval
        host = config.piHost
        self.interval = interval
        if hostChanged {
            check = nil
            devices = nil
            refresh()
            onActivityChange()
        } else if intervalChanged, !isChecking {
            schedule(after: max(0, interval - age))
        }
    }

    /// Checks now, or right after the check in flight.
    func refresh() {
        guard let host else { return }
        guard !isChecking else {
            checkQueued = true
            return
        }
        isChecking = true
        onActivityChange()
        nextCheck?.cancel()
        let client = client, devicesClient = devicesClient
        Task { [weak self] in
            // An unreachable Pi fails the device list the same way, so it isn't asked a second time.
            let (check, devices) = await Task.detached(priority: .utility) { () -> (PiCheck, LanDevicesCheck) in
                let check = client.check(host: host)
                if case .unreachable(let detail) = check.status {
                    return (check, LanDevicesCheck(host: host, checkedAt: check.checkedAt, result: .unreachable(detail)))
                }
                return (check, devicesClient.check(host: host))
            }.value
            self?.finish(check, devices: devices)
        }
    }

    /// The menu is opening: check if the last result is older than `piStatusSeconds`.
    func refreshIfStale() {
        if age >= interval { refresh() }
    }

    private var age: TimeInterval {
        check.map { Date().timeIntervalSince($0.checkedAt) } ?? .infinity
    }

    private func finish(_ check: PiCheck, devices: LanDevicesCheck) {
        isChecking = false
        // A result for a host the config no longer names is dropped; the queued check asks the new one.
        if check.host == host {
            self.check = check
            self.devices = devices
            onUpdate?(check)
        }
        if checkQueued || check.host != host {
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
