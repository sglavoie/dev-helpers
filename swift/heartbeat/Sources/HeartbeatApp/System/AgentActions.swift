import AppKit
import HeartbeatCore

/// Per-agent menu and banner commands: launchctl actions (Restart… and Unload… confirmed first, launchctl's stderr shown in
/// an alert on failure, then polls at 1 s and 5 s), the log window, Open Log, Run Health Check Now, Edit Schedule… and
/// Reveal Plist. Menu items carry
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
    private lazy var scheduleEditors = ScheduleEditorWindowController(
        save: { [weak self] model in await self?.saveSchedule(model) ?? .notSaved("Heartbeat is shutting down") },
        openConfig: { [weak self] in self?.openConfig() })
    /// Opens config.json (the footer's Open Config…); set by the app delegate.
    var openConfig: () -> Void = {}

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
        guard let request = sender.representedObject as? Request else { return }
        showLog(label: request.label)
    }

    @objc func runHealthCheck(_ sender: NSMenuItem) {
        guard let request = sender.representedObject as? Request else { return }
        monitor.healthChecks.runNow(request.label)
    }

    @objc func openLogItem(_ sender: NSMenuItem) {
        guard let path = (sender.representedObject as? Request)?.path else { return }
        openLog(path)
    }

    @objc func editSchedule(_ sender: NSMenuItem) {
        guard let request = sender.representedObject as? Request, let snapshot = monitor.snapshot,
              let agent = snapshot.agent(request.label) else { return }
        let name = agent.name(labelPrefix: snapshot.config.labelPrefix)
        let path = agent.agent.resolvedPlistPath
        guard let head = FileManager.default.contents(atPath: path)?.prefix(8) else {
            showAlert("Cannot read \(name)'s plist", path)
            return
        }
        guard !head.starts(with: Data("bplist".utf8)) else {
            showAlert("Cannot edit \(name)'s schedule", "\(path) is a binary plist; only XML plists can be edited.")
            return
        }
        scheduleEditors.show(ScheduleEditorModel.Context(
            label: agent.label, name: name, agent: agent.agent, isLoaded: agent.status.runtime != nil,
            isRunning: agent.status.runtime?.pid != nil, maxAge: Self.maxAgeContext(agent, snapshot: snapshot)))
    }

    @objc func revealPlist(_ sender: NSMenuItem) {
        guard let path = (sender.representedObject as? Request)?.path else { return }
        NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
    }

    // MARK: Banner actions

    func showLog(label: String) {
        guard let snapshot = monitor.snapshot, let agent = snapshot.agent(label) else { return }
        let name = agent.name(labelPrefix: snapshot.config.labelPrefix)
        guard !agent.agent.logPaths.isEmpty else {
            showAlert("No log for \(name)", "Its plist sends neither stdout nor stderr to a file.")
            return
        }
        logWindows.show(label: agent.label, name: name, paths: agent.agent.logPaths)
    }

    func isHealthCheckRunning(_ label: String) -> Bool {
        monitor.healthChecks.isRunning(label)
    }

    func runNow(label: String) {
        guard let agent = monitor.snapshot?.agent(label) else { return }
        perform(.runNow, on: agent)
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

    // MARK: Schedule

    static func maxAgeContext(_ agent: AgentSnapshot, snapshot: Snapshot) -> ScheduleEditorModel.Context.MaxAge {
        guard let maxAge = agent.config.maxAgeSeconds else { return .none }
        switch snapshot.configSource {
        case .file: return .inFile(maxAge)
        case .defaults: return .notEditable(maxAge, reason: "It comes from the built-in defaults; set it in config.json.")
        case .lastGood: return .notEditable(maxAge, reason: "config.json failed to load, so it can't be updated.")
        }
    }

    /// Writes the plist (and maxAgeSeconds), reloads the agent if it was loaded, then runs it if asked. The agent
    /// is marked paused across the reload so no poll shows the bootout as an amber "Not loaded".
    func saveSchedule(_ model: ScheduleEditorModel) async -> ScheduleEditorModel.SaveOutcome {
        guard let change = model.change else { return .notSaved("The schedule is not valid yet.") }
        let context = model.context, runNow = model.runNow && !context.agent.runAtLoad
        let label = context.label, agent = context.agent, launchctl = launchctl
        let wasPaused = monitor.isPaused(label)
        if context.isLoaded { monitor.setPaused(label, true) }

        enum Step: Sendable { case done, notWritten(String), notReloaded(String, stillLoaded: Bool), notRun(String) }
        let step: Step = await Task.detached {
            do {
                try ScheduleApplier().apply(change, to: agent)
            } catch {
                return .notWritten("\(error)")
            }
            guard context.isLoaded else { return .done }
            do {
                try launchctl.reload(label, plistPath: agent.plistPath)
            } catch {
                let text = (error as? LaunchctlError).map(Self.alertText) ?? "\(error)"
                return .notReloaded(text, stillLoaded: launchctl.print(label).runtime != nil)
            }
            guard runNow else { return .done }
            do {
                try launchctl.kickstart(label)
            } catch {
                return .notRun((error as? LaunchctlError).map(Self.alertText) ?? "\(error)")
            }
            return .done
        }.value

        let outcome: ScheduleEditorModel.SaveOutcome
        switch step {
        case .done:
            if context.isLoaded { monitor.setPaused(label, wasPaused) }
            outcome = .saved
        case .notWritten(let reason):
            if context.isLoaded { monitor.setPaused(label, wasPaused) }
            outcome = .notSaved(reason)
        case .notReloaded(let reason, let stillLoaded):
            // Left unloaded: keep it paused, so the menu offers Load instead of an amber "Not loaded".
            if stillLoaded { monitor.setPaused(label, wasPaused) }
            outcome = .savedWithProblem("The new schedule is saved in \(agent.resolvedPlistPath), but launchd did not "
                + "reload it\(stillLoaded ? " and still runs the old one" : "; choose Load to apply it").\n\n\(reason)")
        case .notRun(let reason):
            monitor.setPaused(label, wasPaused)
            outcome = .savedWithProblem("The new schedule is saved and loaded, but Run Now failed.\n\n\(reason)")
        }
        if case .savedWithProblem(let text) = outcome {
            showAlert("\(context.name): schedule saved with a problem", text)
        }
        if case .notSaved = outcome {} else { monitor.refreshAfterAction() }
        return outcome
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
