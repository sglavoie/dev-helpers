import AppKit
import HeartbeatCore

/// Per-agent menu commands: launchctl actions (Restart… and Unload… confirmed first, launchctl's stderr shown in
/// an alert on failure, then polls at 1 s and 5 s), the log window, Open Log and Reveal Plist. Menu items carry
/// a `Request` as `representedObject`; the agent is looked up again in the current snapshot when clicked.
@MainActor
final class AgentActions: NSObject {
    /// What a menu item asks for.
    final class Request: NSObject {
        let label: String
        let action: AgentAction?
        let path: String?

        init(label: String, action: AgentAction? = nil, path: String? = nil) {
            self.label = label
            self.action = action
            self.path = path
        }
    }

    private let monitor: Monitor
    private let launchctl = LaunchctlClient()
    private lazy var logWindows = LogWindowController(openLog: { [weak self] path in self?.openLog(path) })

    init(monitor: Monitor) {
        self.monitor = monitor
    }

    // MARK: Menu selectors

    @objc func performAction(_ sender: NSMenuItem) {
        guard let request = sender.representedObject as? Request, let action = request.action,
              let agent = monitor.snapshot?.agent(request.label) else { return }
        perform(action, on: agent)
    }

    @objc func viewLog(_ sender: NSMenuItem) {
        guard let request = sender.representedObject as? Request, let agent = monitor.snapshot?.agent(request.label),
              let snapshot = monitor.snapshot else { return }
        logWindows.show(label: agent.label, name: agent.name(labelPrefix: snapshot.config.labelPrefix),
                        paths: agent.agent.logPaths)
    }

    @objc func openLogItem(_ sender: NSMenuItem) {
        guard let path = (sender.representedObject as? Request)?.path else { return }
        openLog(path)
    }

    @objc func revealPlist(_ sender: NSMenuItem) {
        guard let path = (sender.representedObject as? Request)?.path else { return }
        NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
    }

    // MARK: launchctl

    func perform(_ action: AgentAction, on agent: AgentSnapshot) {
        let name = agent.name(labelPrefix: monitor.snapshot?.config.labelPrefix ?? "")
        if action.needsConfirmation, !confirm(action, name: name, agent: agent) { return }

        let label = agent.label, plistPath = agent.agent.plistPath, launchctl = launchctl
        let wasPaused = monitor.isPaused(label)
        // Pause before bootout so no poll in between shows the unload as an amber "Not loaded".
        if action == .unload { monitor.setPaused(label, true) }
        Task {
            let failure: String? = await Task.detached {
                do {
                    switch action {
                    case .runNow: try launchctl.kickstart(label)
                    case .restart: try launchctl.kickstart(label, restart: true)
                    case .unload: try launchctl.bootout(label)
                    case .load: try launchctl.bootstrap(plistPath: plistPath)
                    }
                    return nil
                } catch let error as LaunchctlError {
                    return Self.alertText(error)
                } catch {
                    return "\(error)"
                }
            }.value
            switch (action, failure) {
            case (.unload, .some): monitor.setPaused(label, wasPaused)
            case (.load, nil): monitor.setPaused(label, false)
            default: break
            }
            if let failure {
                let verb = action.title.replacingOccurrences(of: "…", with: "")
                showAlert("\(verb) failed for \(name)", failure)
            }
            monitor.refreshAfterAction()
        }
    }

    private func confirm(_ action: AgentAction, name: String, agent: AgentSnapshot) -> Bool {
        let alert = NSAlert()
        switch action {
        case .restart:
            let pid = agent.status.runtime?.pid.map { " (pid \($0))" } ?? ""
            alert.messageText = "Restart \(name)?"
            alert.informativeText = "launchctl kickstart -k kills the running process\(pid) and starts it again."
            alert.addButton(withTitle: "Restart")
        case .unload:
            alert.messageText = "Unload \(name)?"
            alert.informativeText = "launchctl bootout stops launchd from running it until you choose Load. "
                + "Heartbeat shows it as paused meanwhile."
            alert.addButton(withTitle: "Unload")
        case .runNow, .load:
            return true
        }
        alert.addButton(withTitle: "Cancel")
        NSApp.activate()
        return alert.runModal() == .alertFirstButtonReturn
    }

    nonisolated static func alertText(_ error: LaunchctlError) -> String {
        switch error {
        case .commandFailed(let argv, _, let message):
            let command = argv.joined(separator: " ")
            return message.isEmpty ? error.description : "\(message)\n\n\(command)"
        case .timedOut:
            return error.description
        }
    }

    // MARK: Logs

    /// Opens a log with `openLogCommand` when configured, otherwise with the default app for the file.
    func openLog(_ path: String) {
        guard FileManager.default.fileExists(atPath: path) else {
            showAlert("No log yet", "\(path) does not exist yet.")
            return
        }
        guard let argv = LogTail.openCommand(monitor.snapshot?.config.openLogCommand, path: path) else {
            if !NSWorkspace.shared.open(URL(fileURLWithPath: path)) {
                NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
            }
            return
        }
        launch(argv)
    }

    /// Starts `argv` without waiting for it (it may be an editor); a non-zero exit shows its stderr.
    private func launch(_ argv: [String]) {
        let process = Process()
        // Bare names resolve against the GUI PATH, not launchd's minimal one.
        if argv[0].contains("/") {
            process.executableURL = URL(fileURLWithPath: argv[0])
            process.arguments = Array(argv.dropFirst())
        } else {
            process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
            process.arguments = argv
        }
        var environment = ProcessInfo.processInfo.environment
        environment["PATH"] = CommandRunner.guiPath
        process.environment = environment
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        let stderr = Pipe()
        process.standardError = stderr
        let command = argv.joined(separator: " ")
        process.terminationHandler = { [weak self] process in
            let status = process.terminationStatus
            guard status != 0 else { return }
            let data = (try? stderr.fileHandleForReading.readToEnd()) ?? Data()
            let message = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
            Task { @MainActor in
                self?.showAlert("Open Log failed (exit \(status))", message.isEmpty ? command : "\(message)\n\n\(command)")
            }
        }
        do {
            try process.run()
        } catch {
            showAlert("Cannot run openLogCommand", "\(command)\n\n\(error.localizedDescription)")
        }
    }

    private func showAlert(_ message: String, _ info: String) {
        NSApp.activate()
        let alert = NSAlert()
        alert.alertStyle = .warning
        alert.messageText = message
        alert.informativeText = info
        alert.runModal()
    }
}
