import AppKit
import Carbon.HIToolbox
import SloppyCore

/// Puts text on the clipboard and, when allowed, posts ⌘V into the target
/// app. It never restores the previous clipboard and never re-pastes.
@MainActor
final class Paster {
    static let delayDefaultsKey = "paste.delayMilliseconds"
    static let defaultDelayMilliseconds = 60

    private let accessibility: AccessibilityPermission

    init(accessibility: AccessibilityPermission) {
        self.accessibility = accessibility
    }

    /// Time for the target app to take key focus back before ⌘V; some apps need more.
    var delayMilliseconds: Int {
        let stored = UserDefaults.standard.object(forKey: Self.delayDefaultsKey) as? Int
        return max(0, stored ?? Self.defaultDelayMilliseconds)
    }

    /// Copies `text`, then pastes it into `target` unless Accessibility is
    /// missing, Secure Input is on, or `target` is no longer live and
    /// frontmost when ⌘V would be posted. The picker panel must already be hidden.
    func paste(_ text: String, into target: NSRunningApplication?) async throws -> PasteOutcome {
        let delay = delayMilliseconds
        let sequence = PasteSequence<NSRunningApplication>(
            copy: { try Pasteboard.copy($0) },
            permission: { [accessibility] in
                accessibility.refresh()
                return PasteOutcome.decide(
                    accessibilityTrusted: accessibility.isTrusted, secureInputEnabled: IsSecureEventInputEnabled())
            },
            isSecureInputEnabled: { IsSecureEventInputEnabled() },
            isLive: { !$0.isTerminated },
            isFrontmost: { target in
                NSWorkspace.shared.frontmostApplication?.processIdentifier == target.processIdentifier
            },
            activate: { $0.activate() },
            wait: { try? await Task.sleep(for: .milliseconds(delay)) },
            postPasteKey: Self.postCommandV(keyDown:)
        )
        return try await sequence.run(text, into: target)
    }

    private static func postCommandV(keyDown: Bool) {
        let source = CGEventSource(stateID: .combinedSessionState)
        let keyCode = keyCode(producing: "v") ?? CGKeyCode(kVK_ANSI_V)
        let event = CGEvent(keyboardEventSource: source, virtualKey: keyCode, keyDown: keyDown)
        event?.flags = .maskCommand
        event?.post(tap: .cghidEventTap)
    }

    /// The virtual key that types `character` in the current keyboard layout
    /// (Dvorak moves "v"), or nil when no key does (e.g. a Cyrillic layout).
    static func keyCode(producing character: String) -> CGKeyCode? {
        guard let source = TISCopyCurrentKeyboardLayoutInputSource()?.takeRetainedValue(),
            let rawLayout = TISGetInputSourceProperty(source, kTISPropertyUnicodeKeyLayoutData)
        else { return nil }
        let layoutData = Unmanaged<CFData>.fromOpaque(rawLayout).takeUnretainedValue() as Data
        let keyboardType = UInt32(LMGetKbdType())

        return layoutData.withUnsafeBytes { buffer -> CGKeyCode? in
            guard let layout = buffer.baseAddress?.assumingMemoryBound(to: UCKeyboardLayout.self) else { return nil }
            for keyCode in UInt16(0)..<128 {
                var deadKeyState: UInt32 = 0
                var characters = [UniChar](repeating: 0, count: 4)
                var length = 0
                let status = UCKeyTranslate(
                    layout, keyCode, UInt16(kUCKeyActionDisplay), 0, keyboardType,
                    OptionBits(kUCKeyTranslateNoDeadKeysBit), &deadKeyState,
                    characters.count, &length, &characters)
                if status == noErr, String(utf16CodeUnits: characters, count: length) == character {
                    return CGKeyCode(keyCode)
                }
            }
            return nil
        }
    }
}
