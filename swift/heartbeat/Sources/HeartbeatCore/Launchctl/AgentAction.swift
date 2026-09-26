/// A launchctl command the menu offers for one agent.
public enum AgentAction: String, CaseIterable, Sendable {
    /// `kickstart`: start a loaded job that isn't running.
    case runNow
    /// `kickstart -k`: kill and restart a running job (confirmed).
    case restart
    /// `bootout`: unload and mark paused (confirmed).
    case unload
    /// `bootstrap gui/$UID <plist>`: load an unloaded agent.
    case load

    public var title: String {
        switch self {
        case .runNow: "Run Now"
        case .restart: "Restart…"
        case .unload: "Unload…"
        case .load: "Load"
        }
    }

    public var needsConfirmation: Bool { self == .restart || self == .unload }

    /// What makes sense for a launchd status: Run Now or Restart… plus Unload… while loaded, Load when not
    /// loaded, nothing when launchctl couldn't tell.
    public static func available(for status: ServiceStatus) -> [AgentAction] {
        switch status {
        case .loaded(let runtime):
            let running = runtime.pid != nil || runtime.state == .running
            return [running ? .restart : .runNow, .unload]
        case .notLoaded:
            return [.load]
        case .unknown:
            return []
        }
    }
}
