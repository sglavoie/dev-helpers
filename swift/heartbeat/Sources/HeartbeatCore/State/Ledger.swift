import Foundation

/// Turns successive `launchctl print` observations into history: launchd keeps a `runs` counter but no
/// "last run" time, so the time Heartbeat first sees `runs` go up is the ledger's evidence of a run.
public enum Ledger {
    /// Starts a new launchd session when the Mac rebooted: `runs` counters and load times reset.
    public static func noteBoot(_ bootTime: Date, in state: inout HeartbeatState) {
        guard state.bootTime != bootTime else { return }
        if state.bootTime != nil {
            for label in state.agents.keys {
                state.agents[label]?.runs = nil
                state.agents[label]?.loadedObservedAt = nil
                state.agents[label]?.notRunningStreak = 0
            }
        }
        state.bootTime = bootTime
    }

    /// Records one poll's status for an agent. Call before evaluating so the verdict sees the new history.
    public static func record(_ status: ServiceStatus, keepAlive: Bool, now: Date, in agent: inout AgentState) {
        switch status {
        case .notLoaded:
            agent.runs = nil
            agent.loadedObservedAt = nil
        case .unknown:
            // Nothing trustworthy to compare; keep the old values for the next readable poll.
            break
        case .loaded(let runtime):
            // Loaded again (from the menu or the shell): a later unload is no longer Heartbeat's pause.
            agent.paused = false
            if agent.loadedObservedAt == nil { agent.loadedObservedAt = now }
            guard let runs = runtime.runs else { return }
            defer { agent.runs = runs }
            guard let previous = agent.runs else { return }
            if runs > previous {
                agent.runsChangedAt = now
                let isRunning = runtime.pid != nil || runtime.state == .running
                if keepAlive && isRunning { agent.lastRestartAt = now }
            } else if runs < previous {
                // The service was reloaded (bootout + bootstrap) without Heartbeat seeing it unloaded.
                agent.loadedObservedAt = now
            }
        }
    }

    /// Feeds a verdict back: the KeepAlive not-running streak that rule 5 reads on the next poll.
    public static func record(_ verdict: HealthVerdict, in agent: inout AgentState) {
        agent.notRunningStreak = verdict.keepAliveMissing ? agent.notRunningStreak + 1 : 0
    }

    /// The ledger's contribution to an agent's evidence.
    public static func evidenceDate(_ agent: AgentState) -> Date? {
        agent.runsChangedAt
    }

    /// Drops agents that are no longer discovered, keeping paused ones so Load still knows them.
    public static func prune(_ state: inout HeartbeatState, keeping labels: Set<String>) {
        state.agents = state.agents.filter { labels.contains($0.key) || $0.value.paused }
    }
}
