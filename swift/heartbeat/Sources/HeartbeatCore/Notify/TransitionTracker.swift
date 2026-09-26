import Foundation

/// A banner to show. The app maps these onto `UNUserNotificationCenter` requests.
public enum TransitionNotification: Equatable, Sendable {
    /// The agent turned red. `message` is the worst reason.
    case failing(label: String, name: String, message: String)
    /// The agent was red (and notified) and is ok again.
    case recovered(label: String, name: String)
    /// More than `TransitionTracker.batchLimit` banners in one poll, folded into one.
    case summary(failing: [String], recovered: [String])

    /// Request identifier: per-agent banners replace each other.
    public var identifier: String {
        switch self {
        case .failing(let label, _, _), .recovered(let label, _): "heartbeat.\(label)"
        case .summary: "heartbeat.summary"
        }
    }

    public var label: String? {
        switch self {
        case .failing(let label, _, _), .recovered(let label, _): label
        case .summary: nil
        }
    }
}

/// One agent's verdict this poll plus what the tracker needs to know about it.
public struct TransitionInput: Equatable, Sendable {
    public var verdict: HealthVerdict
    /// Display name for the banner.
    public var name: String
    /// Global notifications on and the agent's `notify` not false.
    public var notify: Bool

    public init(verdict: HealthVerdict, name: String, notify: Bool) {
        self.verdict = verdict
        self.name = name
        self.notify = notify
    }
}

public struct TransitionResult: Equatable, Sendable {
    public var notifications: [TransitionNotification]
    /// The new `notifiedSeverity` per label (`failing` while a red banner stands); persist it in state.json.
    public var notified: [String: Severity]
}

/// Decides which banners a poll produces. Pure: only red (already confirmed by the rules, e.g. rule 5's second
/// poll) and red → ok produce banners; amber is silent; a red agent whose reason changes stays quiet.
public enum TransitionTracker {
    public static let batchLimit = 3

    /// - Parameter previous: `notifiedSeverity` per label from state.json, so a relaunch is quiet.
    public static func update(_ inputs: [TransitionInput], previous: [String: Severity]) -> TransitionResult {
        var notifications: [TransitionNotification] = []
        var notified: [String: Severity] = [:]

        for input in inputs {
            let label = input.verdict.label
            let wasFailing = previous[label] == .failing
            switch input.verdict.severity {
            case .failing:
                // Remembered even when banners are off, so turning them on later doesn't replay old failures.
                notified[label] = .failing
                if !wasFailing && input.notify {
                    notifications.append(.failing(label: label, name: input.name, message: input.verdict.summary ?? "Failing"))
                }
            case .warning:
                // Red → amber isn't a recovery; keep the red marker so amber → red doesn't banner again.
                if wasFailing { notified[label] = .failing }
            case .ok:
                if wasFailing && input.notify {
                    notifications.append(.recovered(label: label, name: input.name))
                }
            case .paused, .hidden:
                // Unloaded or hidden on purpose: clear silently.
                break
            }
        }

        if notifications.count > batchLimit {
            notifications = [.summary(
                failing: notifications.compactMap { if case .failing(let label, _, _) = $0 { label } else { nil } },
                recovered: notifications.compactMap { if case .recovered(let label, _) = $0 { label } else { nil } })]
        }
        return TransitionResult(notifications: notifications, notified: notified)
    }
}
