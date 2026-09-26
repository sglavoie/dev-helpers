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
