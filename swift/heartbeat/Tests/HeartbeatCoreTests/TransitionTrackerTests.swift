import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct TransitionTrackerTests {
    static func input(_ label: String, _ severity: Severity, message: String = "Exited with status 1", notify: Bool = true)
        -> TransitionInput {
        let reasons = severity == .ok ? [] : [HealthReason(rule: 4, code: .exitCode, severity: severity, message: message)]
        return TransitionInput(verdict: HealthVerdict(label: label, severity: severity, reasons: reasons), name: label.uppercased(),
                               notify: notify)
    }

    @Test func redNotifiesOnce() {
        let first = TransitionTracker.update([Self.input("a", .failing)], previous: [:])
        #expect(first.notifications == [.failing(label: "a", name: "A", message: "Exited with status 1")])
        #expect(first.notifications.first?.identifier == "heartbeat.a")
        #expect(first.notified == ["a": .failing])

        let second = TransitionTracker.update([Self.input("a", .failing)], previous: first.notified)
        #expect(second.notifications.isEmpty)
        #expect(second.notified == ["a": .failing])
    }

    @Test func newReasonWhileRedIsSilent() {
        let result = TransitionTracker.update([Self.input("a", .failing, message: "Overdue: missed the Sep 26 10:00 run")],
                                              previous: ["a": .failing])
        #expect(result.notifications.isEmpty)
    }

    @Test func recoveryNotifiesAndClears() {
        let result = TransitionTracker.update([Self.input("a", .ok)], previous: ["a": .failing])
        #expect(result.notifications == [.recovered(label: "a", name: "A")])
        #expect(result.notified.isEmpty)
        #expect(TransitionTracker.update([Self.input("a", .ok)], previous: result.notified).notifications.isEmpty)
    }

    @Test func amberIsSilentBothWays() {
        #expect(TransitionTracker.update([Self.input("a", .warning)], previous: [:]).notifications.isEmpty)
        // Red → amber keeps the marker: no "recovered", and amber → red again stays quiet.
        let toAmber = TransitionTracker.update([Self.input("a", .warning)], previous: ["a": .failing])
        #expect(toAmber.notifications.isEmpty)
        #expect(toAmber.notified == ["a": .failing])
        #expect(TransitionTracker.update([Self.input("a", .failing)], previous: toAmber.notified).notifications.isEmpty)
        // Amber → ok after a red still counts as the recovery.
        #expect(TransitionTracker.update([Self.input("a", .ok)], previous: toAmber.notified).notifications
                == [.recovered(label: "a", name: "A")])
    }

    @Test func pausedOrHiddenClearsSilently() {
        for severity in [Severity.paused, .hidden] {
            let result = TransitionTracker.update([Self.input("a", severity)], previous: ["a": .failing])
            #expect(result.notifications.isEmpty)
            #expect(result.notified.isEmpty)
        }
    }

    @Test func notifyFalseIsTrackedButSilent() {
        let red = TransitionTracker.update([Self.input("a", .failing, notify: false)], previous: [:])
        #expect(red.notifications.isEmpty)
        #expect(red.notified == ["a": .failing])
        // Turning banners on later doesn't replay a failure that was already there.
        #expect(TransitionTracker.update([Self.input("a", .failing)], previous: red.notified).notifications.isEmpty)
        #expect(TransitionTracker.update([Self.input("a", .ok, notify: false)], previous: red.notified).notifications.isEmpty)
    }

    @Test func relaunchWithPersistedStateIsQuiet() throws {
        let directory = FileManager.default.temporaryDirectory.appending(path: "HeartbeatCoreTests-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: directory) }
        let store = StateStore(url: directory.appending(path: "state.json"))

        var state = HeartbeatState()
        let inputs = [Self.input("a", .failing), Self.input("b", .ok)]
        let first = TransitionTracker.update(inputs, previous: state.notifiedSeverities)
        #expect(first.notifications.count == 1)
        state.applyNotified(first.notified, evaluated: ["a", "b"])
        try store.save(state)

        let relaunched = try store.load().state
        #expect(TransitionTracker.update(inputs, previous: relaunched.notifiedSeverities).notifications.isEmpty)
    }

    @Test func threeBannersStaySeparate() {
        let inputs = ["a", "b", "c"].map { Self.input($0, .failing) }
        #expect(TransitionTracker.update(inputs, previous: [:]).notifications.count == 3)
    }

    @Test func moreThanThreeFoldIntoOneSummary() {
        let inputs = ["a", "b", "c"].map { Self.input($0, .failing) } + [Self.input("d", .ok), Self.input("e", .failing, notify: false)]
        let result = TransitionTracker.update(inputs, previous: ["d": .failing])
        #expect(result.notifications == [.summary(failing: ["a", "b", "c"], recovered: ["d"])])
        #expect(result.notifications.first?.identifier == "heartbeat.summary")
        #expect(result.notifications.first?.label == nil)
        #expect(result.notified == ["a": .failing, "b": .failing, "c": .failing, "e": .failing])
    }

    @Test func labelsNotEvaluatedAreDropped() {
        let result = TransitionTracker.update([Self.input("a", .ok)], previous: ["gone": .failing])
        #expect(result.notified.isEmpty)
        #expect(result.notifications.isEmpty)
    }
}

@Suite struct TransitionSnapshotTests {
    typealias F = SnapshotFixtures

    /// sync-legacy red (exit `exit`), forgejo-sync red with notify:false.
    static func snapshot(exit: String, notifications: Bool = true) -> Snapshot {
        let sync = F.agent("sync-legacy", schedule: .watchPaths(["/a"], throttleSeconds: 30), log: "/logs/sync.log")
        let forgejo = F.agent("forgejo-sync", log: "/logs/forgejo.log")
        let config = ConfigLoadResult(config: HeartbeatConfig(notifications: notifications, agents: [
            forgejo.label: AgentConfig(displayName: "Forgejo sync", notify: false),
        ]), source: .file)
        return F.builder([forgejo, sync], prints: Dictionary(uniqueKeysWithValues: [
            F.loaded(sync.label, lastExit: exit), F.loaded(forgejo.label, lastExit: exit),
        ]), files: ["/logs/sync.log": F.now, "/logs/forgejo.log": F.now])
            .buildSync(config: config, state: .empty, ledger: .readOnly)
    }

    @Test func oneBannerThenQuietAcrossARelaunch() throws {
        let directory = FileManager.default.temporaryDirectory.appending(path: "HeartbeatCoreTests-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: directory) }
        let store = StateStore(url: directory.appending(path: "state.json"))

        var state = HeartbeatState()
        let red = Self.snapshot(exit: "1")
        #expect(TransitionTracker.apply(red, to: &state)
                == [.failing(label: "com.sglavoie.sync-legacy", name: "sync-legacy", message: "Exited with status 1")])
        // notify:false agents are still marked, so enabling banners later doesn't replay them.
        #expect(state.notifiedSeverities == ["com.sglavoie.sync-legacy": .failing, "com.sglavoie.forgejo-sync": .failing])
        #expect(TransitionTracker.apply(red, to: &state).isEmpty)
        try store.save(state)

        var relaunched = try store.load().state
        #expect(TransitionTracker.apply(red, to: &relaunched).isEmpty)
        #expect(TransitionTracker.apply(Self.snapshot(exit: "0"), to: &relaunched)
                == [.recovered(label: "com.sglavoie.sync-legacy", name: "sync-legacy")])
        #expect(relaunched.notifiedSeverities.isEmpty)
    }

    @Test func togglesSilenceBannersButKeepTheMarkers() {
        for (config, toggle) in [(false, true), (true, false)] {
            var state = HeartbeatState()
            #expect(TransitionTracker.apply(Self.snapshot(exit: "1", notifications: config), to: &state, bannersEnabled: toggle).isEmpty)
            #expect(state.notifiedSeverities.count == 2)
            #expect(TransitionTracker.apply(Self.snapshot(exit: "1"), to: &state).isEmpty)
        }
    }

    @Test func bannerText() {
        let failing = TransitionNotification.failing(label: "l", name: "sync-legacy", message: "Exited with status 1")
        #expect(failing.text { $0 } == NotificationText(title: "sync-legacy is failing", body: "Exited with status 1"))
        #expect(TransitionNotification.recovered(label: "l", name: "Forgejo sync").text { $0 }
                == NotificationText(title: "Forgejo sync recovered", body: "Back to OK."))
        let summary = TransitionNotification.summary(failing: ["a", "b", "c"], recovered: ["d"]).text { $0.uppercased() }
        #expect(summary == NotificationText(title: "Heartbeat: 3 failing, 1 recovered", body: "Failing: A, B, C\nRecovered: D"))
        #expect(TransitionNotification.summary(failing: [], recovered: ["a", "b", "c", "d"]).text { $0 }.title
                == "Heartbeat: 4 recovered")
    }
}
