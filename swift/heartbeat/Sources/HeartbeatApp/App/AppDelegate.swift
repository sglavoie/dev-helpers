import AppKit
import HeartbeatCore

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem?
    private let menu = NSMenu()
    private let monitor = Monitor()
    private let piMonitor = PiMonitor()
    private let launchAtLogin = LaunchAtLogin()
    private let menuBuilder = StatusMenuBuilder()
    private lazy var agentActions = AgentActions(monitor: monitor)
    private lazy var piJournalWindow = PiJournalWindowController()
    private let notifier = Notifier()

    func applicationDidFinishLaunching(_ notification: Notification) {
        menu.delegate = self
        statusItem = makeStatusItem()
        monitor.onUpdate = { [weak self] snapshot in
            self?.piMonitor.configure(snapshot.config)
            self?.render()
        }
        piMonitor.onUpdate = { [weak self] _ in self?.render() }
        monitor.onActivityChange = { [weak self] in self?.render() }
        piMonitor.onActivityChange = { [weak self] in self?.render() }
        monitor.healthChecks.onActivityChange = { [weak self] in self?.render() }
        agentActions.onActivityChange = { [weak self] in self?.render() }
        monitor.bannersEnabled = { [notifier] in notifier.isEnabled }
        monitor.onNotifications = { [notifier] notifications, snapshot in
            notifier.post(notifications) { label in
                snapshot.agent(label)?.name(labelPrefix: snapshot.config.labelPrefix) ?? label
            }
        }
        notifier.onViewLog = { [weak self] label in self?.agentActions.showLog(label: label) }
        notifier.onRunNow = { [weak self] label in self?.agentActions.runNow(label: label) }
        agentActions.openConfig = { [weak self] in self?.openConfig(nil) }
        notifier.start()
        render()
        monitor.start()
    }

    private func makeStatusItem() -> NSStatusItem {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        item.menu = menu
        return item
    }

    /// Redraws the icon and the menu (also while the menu is open).
    private func render() {
        let snapshot = monitor.snapshot, pi = piMonitor.check
        if let button = statusItem?.button {
            let overall = snapshot?.overall.including(pi)
            StatusIcon.apply(StatusIcon.appearance(overall, failing: snapshot?.count(.failing) ?? 0), to: button)
            let formatter = menuBuilder.formatter
            button.toolTip = snapshot.map { "Heartbeat — " + formatter.headline($0) + "\n" + formatter.piRow(pi) + "\n" + formatter.piJournalRow(pi) } ?? "Heartbeat"
        }
        menuBuilder.populate(
            menu, snapshot: snapshot, pi: StatusMenuBuilder.PiItem(check: pi, isChecking: piMonitor.isChecking,
                                                                 host: piMonitor.host),
            stateProblem: monitor.stateProblem, launchAtLogin: launchAtLogin,
            notifications: notificationsItem(snapshot), isRefreshing: monitor.isPolling || piMonitor.isChecking,
            actions: StatusMenuBuilder.Actions(
                target: self, refresh: #selector(refreshNow(_:)), openConfig: #selector(openConfig(_:)),
                toggleLaunchAtLogin: #selector(toggleLaunchAtLogin(_:)),
                toggleNotifications: #selector(toggleNotifications(_:)), checkPi: #selector(checkPiNow(_:)),
                openKuma: #selector(openUptimeKuma(_:)), viewPiJournal: #selector(viewPiJournal(_:)),
                agent: agentActions))
    }

    /// Off in the config wins; a denied permission turns the item into a link to System Settings.
    private func notificationsItem(_ snapshot: Snapshot?) -> StatusMenuBuilder.NotificationsItem {
        if !notifier.isAvailable {
            return .init(title: "Notifications (needs the app bundle)", isOn: false, isEnabled: false)
        }
        if snapshot?.config.notifications == false {
            return .init(title: "Notifications (off in config)", isOn: false, isEnabled: false)
        }
        if notifier.authorization == .denied {
            return .init(title: "Notifications (allow in System Settings)…", isOn: false, isEnabled: true)
        }
        return .init(title: "Notifications", isOn: notifier.isEnabled, isEnabled: true)
    }

    @objc private func toggleNotifications(_ sender: Any?) {
        if notifier.authorization == .denied {
            notifier.openSystemSettings()
            return
        }
        notifier.isEnabled.toggle()
        render()
    }

    @objc private func refreshNow(_ sender: Any?) {
        monitor.refresh()
        piMonitor.refresh()
    }

    @objc private func checkPiNow(_ sender: Any?) {
        piMonitor.refresh()
        render()
    }

    @objc private func viewPiJournal(_ sender: Any?) {
        guard let host = piMonitor.host else { return }
        piJournalWindow.show(host: host)
    }

    @objc private func openUptimeKuma(_ sender: Any?) {
        NSWorkspace.shared.open(PiStatusClient.kumaURL)
    }

    /// Opens `~/.config/heartbeat/config.json`, writing the commented example first if there is none.
    @objc private func openConfig(_ sender: Any?) {
        let url = ConfigLoader.defaultURL
        do {
            if !FileManager.default.fileExists(atPath: url.path) {
                try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
                try Data(HeartbeatConfig.exampleText.utf8).write(to: url, options: .withoutOverwriting)
            }
        } catch {
            showAlert("Cannot create \(url.path)", error.localizedDescription)
            return
        }
        if !NSWorkspace.shared.open(url) {
            NSWorkspace.shared.activateFileViewerSelecting([url])
        }
    }

    @objc private func toggleLaunchAtLogin(_ sender: Any?) {
        if launchAtLogin.needsApproval {
            launchAtLogin.openSystemSettings()
            return
        }
        launchAtLogin.setEnabled(!launchAtLogin.isEnabled)
        if let error = launchAtLogin.lastError {
            showAlert("Cannot change Launch at Login", error)
        }
        render()
    }

    private func showAlert(_ message: String, _ info: String) {
        NSApp.activate()
        let alert = NSAlert()
        alert.messageText = message
        alert.informativeText = info
        alert.runModal()
    }
}

extension AppDelegate: NSMenuDelegate {
    func menuWillOpen(_ menu: NSMenu) {
        launchAtLogin.refresh()
        notifier.refreshAuthorization()
        render()
        monitor.refreshIfStale()
        piMonitor.refreshIfStale()
    }
}
