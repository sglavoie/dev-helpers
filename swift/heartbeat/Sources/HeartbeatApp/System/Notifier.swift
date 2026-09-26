import AppKit
import HeartbeatCore
import OSLog
import UserNotifications

/// Shows TransitionTracker's banners through `UNUserNotificationCenter`. Per-agent banners use the id
/// `heartbeat.<label>`, so "recovered" replaces the "failing" banner it answers; agent banners carry View Log and
/// Run Now actions (clicking the banner itself opens the log). The menu's Notifications toggle is kept in
/// UserDefaults on top of the config's `notifications` key.
@MainActor
final class Notifier: NSObject {
    static let agentCategory = "heartbeat.agent"
    static let viewLogAction = "heartbeat.viewLog"
    static let runNowAction = "heartbeat.runNow"
    private static let enabledKey = "notificationsEnabled"
    /// `log show --predicate 'subsystem == "dev.sglavoie.Heartbeat"'` lists every banner requested.
    private nonisolated static let log = Logger(subsystem: "dev.sglavoie.Heartbeat", category: "notifications")

    /// Banner actions, with the agent's label.
    var onViewLog: ((String) -> Void)?
    var onRunNow: ((String) -> Void)?

    private(set) var authorization: UNAuthorizationStatus = .notDetermined
    /// Banners from polls that finished while the permission request was still open (the first launch's
    /// prompt); posting them then would drop them, so they wait for the answer.
    private var held: [(TransitionNotification, NotificationText)]? = []
    /// `UNUserNotificationCenter` needs a bundle; `swift run` (a bare binary) gets no banners.
    private let center: UNUserNotificationCenter? =
        Bundle.main.bundleURL.pathExtension == "app" ? UNUserNotificationCenter.current() : nil

    /// The menu's Notifications toggle (on by default).
    var isEnabled: Bool {
        get { UserDefaults.standard.object(forKey: Self.enabledKey) as? Bool ?? true }
        set { UserDefaults.standard.set(newValue, forKey: Self.enabledKey) }
    }

    var isAvailable: Bool { center != nil }

    func start() {
        guard let center else { return }
        center.delegate = self
        center.setNotificationCategories([UNNotificationCategory(
            identifier: Self.agentCategory,
            actions: [
                UNNotificationAction(identifier: Self.viewLogAction, title: "View Log", options: [.foreground]),
                UNNotificationAction(identifier: Self.runNowAction, title: "Run Now", options: []),
            ],
            intentIdentifiers: [])])
        center.requestAuthorization(options: [.alert, .sound]) { [weak self] granted, error in
            Self.log.notice("authorization granted=\(granted) error=\(String(describing: error), privacy: .public)")
            Task { @MainActor in self?.authorizationAnswered(granted: granted) }
        }
    }

    private func authorizationAnswered(granted: Bool) {
        let held = held ?? []
        self.held = nil
        refreshAuthorization()
        if granted {
            held.forEach(deliver)
        } else if !held.isEmpty {
            Self.log.notice("dropped \(held.count) banners: notifications are not allowed")
        }
    }

    /// Re-reads the permission (the menu shows a hint when banners are denied in System Settings).
    func refreshAuthorization() {
        center?.getNotificationSettings { [weak self] settings in
            let status = settings.authorizationStatus
            Task { @MainActor in self?.authorization = status }
        }
    }

    func openSystemSettings() {
        let bundleID = Bundle.main.bundleIdentifier ?? ""
        let url = URL(string: "x-apple.systempreferences:com.apple.Notifications-Settings.extension?id=\(bundleID)")
        if let url { NSWorkspace.shared.open(url) }
    }

    /// - Parameter name: display name per label, for the summary banner.
    func post(_ notifications: [TransitionNotification], name: (String) -> String) {
        guard center != nil else { return }
        for notification in notifications {
            let text = notification.text(name: name)
            if held != nil {
                held?.append((notification, text))
            } else {
                deliver(notification, text)
            }
        }
    }

    private func deliver(_ notification: TransitionNotification, _ text: NotificationText) {
        guard let center else { return }
        let content = UNMutableNotificationContent()
        content.title = text.title
        content.body = text.body
        if let label = notification.label {
            content.categoryIdentifier = Self.agentCategory
            content.userInfo = ["label": label]
            content.threadIdentifier = label
        }
        if case .failing = notification { content.sound = .default }
        if case .summary(let failing, let recovered) = notification {
            // The summary supersedes those agents' older banners.
            center.removeDeliveredNotifications(withIdentifiers: (failing + recovered).map { "heartbeat.\($0)" })
        }
        let identifier = notification.identifier, title = text.title
        Self.log.notice("banner \(identifier, privacy: .public): \(title, privacy: .public)")
        center.add(UNNotificationRequest(identifier: identifier, content: content, trigger: nil)) { error in
            if let error {
                Self.log.error("banner \(identifier, privacy: .public) failed: \(error.localizedDescription, privacy: .public)")
            }
        }
    }
}

extension Notifier: UNUserNotificationCenterDelegate {
    /// Heartbeat is always "frontmost" as far as the center knows (an accessory app), so ask for the banner.
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter, willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .list, .sound])
    }

    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let label = response.notification.request.content.userInfo["label"] as? String
        let action = response.actionIdentifier
        completionHandler()
        guard let label else { return }
        Task { @MainActor in
            switch action {
            case Self.runNowAction: self.onRunNow?(label)
            case Self.viewLogAction, UNNotificationDefaultActionIdentifier: self.onViewLog?(label)
            default: break
            }
        }
    }
}
