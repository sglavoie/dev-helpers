import AppKit
import HeartbeatCore

/// Fills the status menu from a snapshot: a header line, one section per severity (Failing, Warning, OK, Paused)
/// with a submenu per agent (read-only info, then log and launchctl actions), the Pi row, problems, then the footer.
@MainActor
struct StatusMenuBuilder {
    /// Footer commands; `target` implements them.
    struct Actions {
        var target: AnyObject
        var refresh: Selector
        var openConfig: Selector
        var toggleLaunchAtLogin: Selector
        var toggleNotifications: Selector
        var checkPi: Selector
        var openKuma: Selector
        var viewPiJournal: Selector
        /// Implements the per-agent items.
        var agent: AgentActions
    }

    /// The footer's Notifications item.
    struct NotificationsItem {
        var title: String
        var isOn: Bool
        var isEnabled: Bool
    }

    /// The Pi row's last check and whether one is running.
    struct PiItem {
        var check: PiCheck?
        var isChecking: Bool
        /// The configured `piHost`, which View Journal… asks.
        var host: String?
    }

    var formatter = StatusFormatter()

    func populate(_ menu: NSMenu, snapshot: Snapshot?, pi: PiItem, stateProblem: String?, launchAtLogin: LaunchAtLogin,
                  notifications: NotificationsItem, isRefreshing: Bool, actions: Actions) {
        menu.autoenablesItems = false
        menu.removeAllItems()
        menu.addItem(disabled(snapshot.map(formatter.headline) ?? "Checking agents…"))

        if let snapshot {
            let prefix = snapshot.config.labelPrefix
            for section in formatter.menuSections(snapshot) {
                menu.addItem(.separator())
                menu.addItem(.sectionHeader(title: "\(section.title) (\(section.agents.count))"))
                for agent in section.agents {
                    menu.addItem(row(agent, name: agent.name(labelPrefix: prefix), snapshot: snapshot,
                                     actions: actions.agent))
                }
            }
        }

        menu.addItem(.separator())
        menu.addItem(piRow(pi, now: Date(), actions: actions))
        menu.addItem(piJournalRow(pi, now: Date(), actions: actions))

        let problems = problemLines(snapshot, stateProblem: stateProblem)
        if !problems.isEmpty {
            menu.addItem(.separator())
            menu.addItem(.sectionHeader(title: "Problems"))
            for line in problems {
                let item = disabled(line)
                item.image = dot(.systemOrange)
                menu.addItem(item)
            }
        }

        menu.addItem(.separator())
        let refresh = command(isRefreshing ? "Refreshing…" : "Refresh Now", actions.refresh, key: "r", target: actions.target)
        refresh.isEnabled = !isRefreshing
        menu.addItem(refresh)
        menu.addItem(command("Open Config…", actions.openConfig, target: actions.target))
        let login = command(launchAtLogin.needsApproval ? "Launch at Login (needs approval)…" : "Launch at Login",
                            actions.toggleLaunchAtLogin, target: actions.target)
        login.state = launchAtLogin.isEnabled ? .on : .off
        let banners = command(notifications.title, actions.toggleNotifications, target: actions.target)
        banners.state = notifications.isOn ? .on : .off
        banners.isEnabled = notifications.isEnabled
        menu.addItem(banners)
        menu.addItem(login)
        menu.addItem(.separator())
        menu.addItem(disabled("Heartbeat \(HeartbeatCore.version)"))
        menu.addItem(NSMenuItem(title: "Quit Heartbeat", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"))
    }

    /// Dot + name + short detail; the submenu holds reasons, schedule, evidence, runs and paths, then actions.
    private func row(_ agent: AgentSnapshot, name: String, snapshot: Snapshot, actions: AgentActions) -> NSMenuItem {
        let item = NSMenuItem(title: name, action: nil, keyEquivalent: "")
        let title = NSMutableAttributedString(string: name, attributes: [.font: NSFont.menuFont(ofSize: 0)])
        title.append(NSAttributedString(
            string: "  " + (actions.activityTitle(agent.label) ?? formatter.detail(agent, now: snapshot.takenAt)),
            attributes: [.font: NSFont.menuFont(ofSize: NSFont.smallSystemFontSize),
                         .foregroundColor: NSColor.secondaryLabelColor]))
        item.attributedTitle = title
        item.image = dot(Self.color(agent.severity))
        item.toolTip = agent.label

        let submenu = NSMenu(title: name)
        submenu.autoenablesItems = false
        for (index, group) in formatter.menuInfo(agent, snapshot: snapshot).enumerated() {
            if index > 0 { submenu.addItem(.separator()) }
            group.forEach { submenu.addItem(disabled($0)) }
        }
        addActions(for: agent, to: submenu, actions: actions)
        item.submenu = submenu
        return item
    }

    /// Dot + "Pi — ok · 48/48 up"; the submenu lists problems and check details, then Check Now and Open Uptime Kuma.
    private func piRow(_ pi: PiItem, now: Date, actions: Actions) -> NSMenuItem {
        let title = pi.check == nil && pi.isChecking
            ? "Pi — Checking \(pi.host ?? "Pi")…" : formatter.piRow(pi.check)
        let item = NSMenuItem(title: title, action: nil, keyEquivalent: "")
        item.image = dot(pi.check.map { Self.color($0.severity) } ?? .systemGray)
        item.toolTip = pi.host.map { "ssh \($0) \(PiStatusClient.remoteCommand)" }
        let submenu = NSMenu(title: "Pi")
        submenu.autoenablesItems = false
        for (index, group) in formatter.piMenuInfo(pi.check, now: now).enumerated() {
            if index > 0 { submenu.addItem(.separator()) }
            group.forEach { submenu.addItem(disabled($0)) }
        }
        submenu.addItem(.separator())
        let check = command(pi.isChecking ? "Checking Pi…" : "Check Pi Now", actions.checkPi, target: actions.target)
        check.isEnabled = !pi.isChecking
        submenu.addItem(check)
        let kuma = command("Open Uptime Kuma", actions.openKuma, target: actions.target)
        kuma.toolTip = PiStatusClient.kumaURL.absoluteString
        submenu.addItem(kuma)
        item.submenu = submenu
        return item
    }

    /// Dot + "Pi journal — 15+ errors, last 17:41"; amber while the last hour has errors, but it never colors the icon.
    private func piJournalRow(_ pi: PiItem, now: Date, actions: Actions) -> NSMenuItem {
        let item = NSMenuItem(title: formatter.piJournalRow(pi.check), action: nil, keyEquivalent: "")
        item.image = dot(pi.check?.journalSeverity.map(Self.color) ?? .systemGray)
        let submenu = NSMenu(title: "Pi journal")
        submenu.autoenablesItems = false
        for (index, group) in formatter.piJournalMenuInfo(pi.check, now: now).enumerated() {
            if index > 0 { submenu.addItem(.separator()) }
            group.forEach { submenu.addItem(disabled($0)) }
        }
        submenu.addItem(.separator())
        let check = command(pi.isChecking ? "Checking Pi…" : "Check Pi Now", actions.checkPi, target: actions.target)
        check.isEnabled = !pi.isChecking
        submenu.addItem(check)
        let view = command("View Journal…", actions.viewPiJournal, target: actions.target)
        view.isEnabled = pi.host != nil
        view.toolTip = pi.host.map { "ssh \($0) \(PiStatusClient.journalCommand)" }
        submenu.addItem(view)
        item.submenu = submenu
        return item
    }

    /// View Log… and Open Log per log file, the launchctl actions that fit the agent's state, Run Health Check Now
    /// for agents with a health command, then Edit Schedule… and Reveal Plist.
    private func addActions(for agent: AgentSnapshot, to submenu: NSMenu, actions: AgentActions) {
        let label = agent.label
        let activity = actions.activityTitle(label)
        submenu.addItem(.separator())
        submenu.addItem(agentCommand("Copy Diagnostics", #selector(AgentActions.copyDiagnostics(_:)), actions,
                                     AgentActions.Request(label: label)))
        if let activity { submenu.addItem(disabled(activity)) }
        let logs = agent.agent.logPaths
        if !logs.isEmpty {
            submenu.addItem(.separator())
            submenu.addItem(agentCommand("View Log…", #selector(AgentActions.viewLog(_:)), actions,
                                         AgentActions.Request(label: label)))
            for path in logs {
                let title = logs.count > 1 && path == agent.agent.standardErrorPath ? "Open Error Log" : "Open Log"
                let item = agentCommand(title, #selector(AgentActions.openLogItem(_:)), actions,
                                        AgentActions.Request(label: label, path: path))
                item.toolTip = path
                submenu.addItem(item)
            }
        }
        let launchctlActions = AgentAction.available(for: agent.status)
        if !launchctlActions.isEmpty {
            submenu.addItem(.separator())
            for action in launchctlActions {
                let item = agentCommand(action.title, #selector(AgentActions.performAction(_:)), actions,
                                        AgentActions.Request(label: label, action: action))
                item.isEnabled = activity == nil
                submenu.addItem(item)
            }
        }
        if agent.config.health != nil, agent.severity != .hidden, agent.severity != .paused {
            submenu.addItem(.separator())
            let running = actions.isHealthCheckRunning(label)
            let item = agentCommand(running ? "Health Check Running…" : "Run Health Check Now",
                                    #selector(AgentActions.runHealthCheck(_:)), actions, AgentActions.Request(label: label))
            item.isEnabled = !running && activity == nil
            item.toolTip = agent.config.health?.command.joined(separator: " ")
            submenu.addItem(item)
        }
        submenu.addItem(.separator())
        let edit = agentCommand("Edit Schedule…", #selector(AgentActions.editSchedule(_:)), actions,
                                AgentActions.Request(label: label))
        edit.toolTip = "Change the schedule in \(agent.agent.resolvedPlistPath) and reload the agent"
        edit.isEnabled = activity == nil
        submenu.addItem(edit)
        let reveal = agentCommand("Reveal Plist", #selector(AgentActions.revealPlist(_:)), actions,
                                  AgentActions.Request(label: label, path: agent.agent.resolvedPlistPath))
        reveal.toolTip = agent.agent.resolvedPlistPath
        submenu.addItem(reveal)
    }

    private func agentCommand(_ title: String, _ action: Selector, _ target: AgentActions,
                              _ request: AgentActions.Request) -> NSMenuItem {
        let item = command(title, action, target: target)
        item.representedObject = request
        return item
    }

    private func problemLines(_ snapshot: Snapshot?, stateProblem: String?) -> [String] {
        var lines: [String] = []
        if let snapshot {
            if let fatal = snapshot.fatalError { lines.append(fatal) }
            if let error = snapshot.configError {
                lines.append("Config: \(error) (using \(snapshot.configSource.name) config)")
            }
            lines += snapshot.configWarnings.map { "Config: \($0)" }
            lines += snapshot.discoveryProblems.map(\.description)
        }
        if let stateProblem { lines.append(stateProblem) }
        return lines
    }

    static func color(_ severity: Severity) -> NSColor {
        switch severity {
        case .failing: .systemRed
        case .warning: .systemOrange
        case .ok: .systemGreen
        case .paused, .hidden: .systemGray
        }
    }

    private func dot(_ color: NSColor) -> NSImage? {
        let configuration = NSImage.SymbolConfiguration(pointSize: 9, weight: .regular)
            .applying(NSImage.SymbolConfiguration(paletteColors: [color]))
        return NSImage(systemSymbolName: "circle.fill", accessibilityDescription: nil)?
            .withSymbolConfiguration(configuration)
    }

    private func disabled(_ title: String) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: nil, keyEquivalent: "")
        item.isEnabled = false
        return item
    }

    private func command(_ title: String, _ action: Selector, key: String = "", target: AnyObject) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: key)
        item.target = target
        return item
    }
}
