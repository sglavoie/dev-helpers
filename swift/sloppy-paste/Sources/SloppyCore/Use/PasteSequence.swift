import Foundation

/// The paste-back steps with every system dependency injected, so the target
/// checks can be tested without a live session. The clipboard is set first
/// and kept on every path; ⌘V is posted at most once and never retried.
@MainActor
public struct PasteSequence<Target> {
    public var copy: @MainActor (String) throws -> Void
    /// Accessibility and Secure Input before anything else happens.
    public var permission: @MainActor () -> PasteOutcome
    public var isSecureInputEnabled: @MainActor () -> Bool
    /// Whether the target process is still running.
    public var isLive: @MainActor (Target) -> Bool
    /// Whether the target is the actual frontmost application right now.
    public var isFrontmost: @MainActor (Target) -> Bool
    /// Asks the target to come to the front; false when the request failed.
    public var activate: @MainActor (Target) -> Bool
    /// Time for the target to take key focus back.
    public var wait: @MainActor () async -> Void
    public var postPasteKey: @MainActor (_ keyDown: Bool) -> Void

    public init(
        copy: @escaping @MainActor (String) throws -> Void,
        permission: @escaping @MainActor () -> PasteOutcome,
        isSecureInputEnabled: @escaping @MainActor () -> Bool,
        isLive: @escaping @MainActor (Target) -> Bool,
        isFrontmost: @escaping @MainActor (Target) -> Bool,
        activate: @escaping @MainActor (Target) -> Bool,
        wait: @escaping @MainActor () async -> Void,
        postPasteKey: @escaping @MainActor (Bool) -> Void
    ) {
        self.copy = copy
        self.permission = permission
        self.isSecureInputEnabled = isSecureInputEnabled
        self.isLive = isLive
        self.isFrontmost = isFrontmost
        self.activate = activate
        self.wait = wait
        self.postPasteKey = postPasteKey
    }

    /// Copies `text`, then posts one ⌘V only if `target` is still live and
    /// frontmost right before the key events; otherwise reports copy-only.
    public func run(_ text: String, into target: Target?) async throws -> PasteOutcome {
        try copy(text)
        let decision = permission()
        guard decision == .pasted else { return decision }
        guard let target, isLive(target) else { return .copiedOnly(.targetUnavailable) }

        if !isFrontmost(target), !activate(target) {
            return .copiedOnly(.targetUnavailable)
        }
        await wait()
        // The target's focused field may have turned Secure Input on once it was active again.
        if isSecureInputEnabled() {
            return .copiedOnly(.secureInput)
        }
        // The target may have quit, or the user switched apps, during the wait.
        guard isLive(target), isFrontmost(target) else { return .copiedOnly(.targetUnavailable) }

        postPasteKey(true)
        postPasteKey(false)
        return .pasted
    }
}
