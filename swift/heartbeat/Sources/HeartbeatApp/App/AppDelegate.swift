import AppKit
import HeartbeatCore

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem?
    private let menu = NSMenu()
    private let monitor = Monitor()
    private let launchAtLogin = LaunchAtLogin()
    private let menuBuilder = StatusMenuBuilder()
    private lazy var agentActions = AgentActions(monitor: monitor)
    private let notifier = Notifier()

    func applicationDidFinishLaunching(_ notification: Notification) {
        menu.delegate = self
        statusItem = makeStatusItem()
        monitor.onUpdate = { [weak self] _ in self?.render() }
        monitor.bannersEnabled = { [notifier] in notifier.isEnabled }
        monitor.onNotifications = { [notifier] notifications, snapshot in
            notifier.post(notifications) { label in
                snapshot.agent(label)?.name(labelPrefix: snapshot.config.labelPrefix) ?? label
            }
        }
        notifier.onViewLog = { [weak self] label in self?.agentActions.showLog(label: label) }
        notifier.onRunNow = { [weak self] label in self?.agentActions.runNow(label: label) }
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
        let snapshot = monitor.snapshot
        if let button = statusItem?.button {
            StatusIcon.apply(StatusIcon.appearance(snapshot?.overall, failing: snapshot?.count(.failing) ?? 0), to: button)
            button.toolTip = snapshot.map { "Heartbeat — " + menuBuilder.formatter.headline($0) } ?? "Heartbeat"
        }
        menuBuilder.populate(
            menu, snapshot: snapshot, stateProblem: monitor.stateProblem, launchAtLogin: launchAtLogin,
            notifications: notificationsItem(snapshot),
            actions: StatusMenuBuilder.Actions(
                target: self, refresh: #selector(refreshNow(_:)), openConfig: #selector(openConfig(_:)),
                toggleLaunchAtLogin: #selector(toggleLaunchAtLogin(_:)),
                toggleNotifications: #selector(toggleNotifications(_:)), agent: agentActions))
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
    }
}
