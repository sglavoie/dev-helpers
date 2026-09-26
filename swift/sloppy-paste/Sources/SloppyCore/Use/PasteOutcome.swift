import Foundation

/// Why a paste fell back to copying only.
public enum CopyOnlyReason: Sendable, Hashable {
    /// The app has no Accessibility grant, so it cannot post ⌘V.
    case accessibilityMissing
    /// Secure Event Input is on (a password field), so a synthetic ⌘V would be dropped.
    case secureInput
    /// The app that was frontmost when the picker opened is gone or no longer
    /// in front, so ⌘V would land somewhere else.
    case targetUnavailable
}

/// What happened to a paste or copy action, and the HUD text that reports it.
public enum PasteOutcome: Sendable, Hashable {
    case pasted
    case copied
    case copiedOnly(CopyOnlyReason)

    /// Decides whether the app may post ⌘V. The clipboard is set either way;
    /// the clipboard is never restored and nothing is re-pasted later.
    public static func decide(accessibilityTrusted: Bool, secureInputEnabled: Bool) -> PasteOutcome {
        if !accessibilityTrusted { return .copiedOnly(.accessibilityMissing) }
        if secureInputEnabled { return .copiedOnly(.secureInput) }
        return .pasted
    }

    /// HUD title and message; nil when the paste itself is the feedback.
    public var hud: (title: String, message: String?)? {
        switch self {
        case .pasted:
            nil
        case .copied:
            ("Copied to Clipboard", nil)
        case .copiedOnly(.accessibilityMissing):
            ("Copied to Clipboard", "Grant Accessibility access to paste directly. Press ⌘V to paste.")
        case .copiedOnly(.secureInput):
            ("Copied to Clipboard", "Secure input is on, so Sloppy Paste cannot paste. Press ⌘V to paste.")
        case .copiedOnly(.targetUnavailable):
            ("Copied to Clipboard", "The previous app is no longer in front, so Sloppy Paste did not paste. Press ⌘V to paste.")
        }
    }
}
