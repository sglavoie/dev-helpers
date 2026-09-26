import Foundation
import Testing
@testable import SloppyCore

/// The paste-back sequence with a fake target, focus and key tap: ⌘V is
/// posted only while the saved target is live and frontmost, exactly once.
@MainActor
@Suite struct PasteTargetSafetyTests {
    @Test func absentTargetOnlyCopies() async throws {
        let system = FakeSystem(frontmost: 7)
        let outcome = try await system.sequence().run("Hello", into: nil)
        #expect(outcome == .copiedOnly(.targetUnavailable))
        #expect(system.clipboard == "Hello")
        #expect(system.keyEvents.isEmpty)
    }

    @Test func terminatedTargetOnlyCopies() async throws {
        let system = FakeSystem(frontmost: 7)
        system.terminated = [1]
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .copiedOnly(.targetUnavailable))
        #expect(system.clipboard == "Hello")
        #expect(system.activations.isEmpty)
        #expect(system.keyEvents.isEmpty)
    }

    @Test func targetQuittingDuringTheDelayOnlyCopies() async throws {
        let system = FakeSystem(frontmost: 1)
        system.duringWait = { system.terminated = [1] }
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .copiedOnly(.targetUnavailable))
        #expect(system.keyEvents.isEmpty)
    }

    @Test func failedActivationOnlyCopies() async throws {
        let system = FakeSystem(frontmost: 7)
        system.activationSucceeds = false
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .copiedOnly(.targetUnavailable))
        #expect(system.clipboard == "Hello")
        #expect(system.activations == [1])
        #expect(system.keyEvents.isEmpty)
    }

    @Test func activationThatNeverTakesFocusOnlyCopies() async throws {
        let system = FakeSystem(frontmost: 7)
        system.activationTakesFocus = false
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .copiedOnly(.targetUnavailable))
        #expect(system.keyEvents.isEmpty)
    }

    @Test func focusSwitchDuringTheDelayOnlyCopies() async throws {
        let system = FakeSystem(frontmost: 1)
        system.duringWait = { system.frontmost = 9 }
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .copiedOnly(.targetUnavailable))
        #expect(system.clipboard == "Hello")
        #expect(system.keyEvents.isEmpty)
    }

    @Test func unchangedFrontmostTargetGetsExactlyOneKeyPair() async throws {
        let system = FakeSystem(frontmost: 1)
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .pasted)
        #expect(system.clipboard == "Hello")
        #expect(system.activations.isEmpty)
        #expect(system.waits == 1)
        #expect(system.keyEvents == [true, false])
    }

    @Test func reactivatedTargetGetsExactlyOneKeyPair() async throws {
        let system = FakeSystem(frontmost: 7)
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .pasted)
        #expect(system.activations == [1])
        #expect(system.keyEvents == [true, false])
    }

    @Test func secureInputAfterTheDelayOnlyCopies() async throws {
        let system = FakeSystem(frontmost: 1)
        system.duringWait = { system.secureInput = true }
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .copiedOnly(.secureInput))
        #expect(system.keyEvents.isEmpty)
    }

    @Test func missingPermissionSkipsTargetChecks() async throws {
        let system = FakeSystem(frontmost: 7)
        system.permission = .copiedOnly(.accessibilityMissing)
        let outcome = try await system.sequence().run("Hello", into: 1)
        #expect(outcome == .copiedOnly(.accessibilityMissing))
        #expect(system.clipboard == "Hello")
        #expect(system.activations.isEmpty)
        #expect(system.waits == 0)
        #expect(system.keyEvents.isEmpty)
    }

    @Test func targetUnavailableHUDExplainsTheFallback() throws {
        let hud = try #require(PasteOutcome.copiedOnly(.targetUnavailable).hud)
        #expect(hud.title == "Copied to Clipboard")
        #expect(hud.message?.contains("no longer in front") == true)
    }
}

/// Targets are process identifiers; `frontmost` is the app that owns focus.
@MainActor
private final class FakeSystem {
    var frontmost: Int32?
    var terminated: Set<Int32> = []
    var activationSucceeds = true
    var activationTakesFocus = true
    var secureInput = false
    var permission = PasteOutcome.pasted
    var duringWait: () -> Void = {}

    private(set) var clipboard: String?
    private(set) var activations: [Int32] = []
    private(set) var waits = 0
    private(set) var keyEvents: [Bool] = []

    init(frontmost: Int32?) {
        self.frontmost = frontmost
    }

    func sequence() -> PasteSequence<Int32> {
        PasteSequence(
            copy: { self.clipboard = $0 },
            permission: { self.permission },
            isSecureInputEnabled: { self.secureInput },
            isLive: { !self.terminated.contains($0) },
            isFrontmost: { self.frontmost == $0 },
            activate: { target in
                self.activations.append(target)
                guard self.activationSucceeds else { return false }
                if self.activationTakesFocus { self.frontmost = target }
                return true
            },
            wait: {
                self.waits += 1
                self.duringWait()
            },
            postPasteKey: { self.keyEvents.append($0) }
        )
    }
}
