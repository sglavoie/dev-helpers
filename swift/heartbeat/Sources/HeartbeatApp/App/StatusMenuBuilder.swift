import AppKit
import HeartbeatCore

/// Fills the status menu from a snapshot: a header line, one section per severity (Failing, Warning, OK, Paused)
/// with a read-only info submenu per agent, problems, then the footer.
@MainActor
struct StatusMenuBuilder {
    /// Footer commands; `target` implements them.
    struct Actions {
        var target: AnyObject
        var refresh: Selector
        var openConfig: Selector
        var toggleLaunchAtLogin: Selector
    }

    var formatter = StatusFormatter()

    func populate(_ menu: NSMenu, snapshot: Snapshot?, stateProblem: String?, launchAtLogin: LaunchAtLogin,
                  actions: Actions) {
        menu.removeAllItems()
        menu.addItem(disabled(snapshot.map(formatter.headline) ?? "Checking agents…"))

        if let snapshot {
            let prefix = snapshot.config.labelPrefix
            for section in formatter.menuSections(snapshot) {
                menu.addItem(.separator())
                menu.addItem(.sectionHeader(title: "\(section.title) (\(section.agents.count))"))
                for agent in section.agents {
                    menu.addItem(row(agent, name: agent.name(labelPrefix: prefix), snapshot: snapshot))
                }
            }
        }

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
        menu.addItem(command("Refresh Now", actions.refresh, key: "r", target: actions.target))
        menu.addItem(command("Open Config…", actions.openConfig, target: actions.target))
        let login = command(launchAtLogin.needsApproval ? "Launch at Login (needs approval)…" : "Launch at Login",
                            actions.toggleLaunchAtLogin, target: actions.target)
        login.state = launchAtLogin.isEnabled ? .on : .off
        menu.addItem(login)
        menu.addItem(.separator())
        menu.addItem(disabled("Heartbeat \(HeartbeatCore.version)"))
        menu.addItem(NSMenuItem(title: "Quit Heartbeat", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"))
    }

    /// Dot + name + short detail; the submenu holds reasons, schedule, evidence, runs and paths.
    private func row(_ agent: AgentSnapshot, name: String, snapshot: Snapshot) -> NSMenuItem {
        let item = NSMenuItem(title: name, action: nil, keyEquivalent: "")
        let title = NSMutableAttributedString(string: name, attributes: [.font: NSFont.menuFont(ofSize: 0)])
        title.append(NSAttributedString(
            string: "  " + formatter.detail(agent, now: snapshot.takenAt),
            attributes: [.font: NSFont.menuFont(ofSize: NSFont.smallSystemFontSize),
                         .foregroundColor: NSColor.secondaryLabelColor]))
        item.attributedTitle = title
        item.image = dot(Self.color(agent.severity))
        item.toolTip = agent.label

        let submenu = NSMenu(title: name)
        for (index, group) in formatter.menuInfo(agent, snapshot: snapshot).enumerated() {
            if index > 0 { submenu.addItem(.separator()) }
            group.forEach { submenu.addItem(disabled($0)) }
        }
        item.submenu = submenu
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
