import AppKit
import Carbon.HIToolbox
import Observation

extension HotKey {
    /// Builds a hotkey from a recorded keyDown. Nil when it has no ⌘, ⌥ or ⌃
    /// (a bare or ⇧-only key would swallow typing everywhere), except for
    /// function keys, which may stand alone.
    init?(event: NSEvent) {
        let flags = event.modifierFlags.intersection([.command, .option, .control, .shift])
        var carbon = 0
        if flags.contains(.command) { carbon |= cmdKey }
        if flags.contains(.option) { carbon |= optionKey }
        if flags.contains(.control) { carbon |= controlKey }
        if flags.contains(.shift) { carbon |= shiftKey }
        let keyCode = Int(event.keyCode)
        let hasPrimaryModifier = !flags.intersection([.command, .option, .control]).isEmpty
        guard hasPrimaryModifier || Self.functionKeyNames[keyCode] != nil else { return nil }
        self.init(keyCode: UInt32(keyCode), carbonModifiers: UInt32(carbon))
    }

    /// "⌃⌥⌘V"-style label, with the key as the current layout types it.
    var displayString: String {
        let modifiers = Int(carbonModifiers)
        var text = ""
        if modifiers & controlKey != 0 { text += "⌃" }
        if modifiers & optionKey != 0 { text += "⌥" }
        if modifiers & shiftKey != 0 { text += "⇧" }
        if modifiers & cmdKey != 0 { text += "⌘" }
        return text + keyName
    }

    var keyName: String {
        let code = Int(keyCode)
        if let name = Self.specialKeyNames[code] ?? Self.functionKeyNames[code] { return name }
        return Self.character(for: code)?.uppercased() ?? "Key \(code)"
    }

    /// The NSMenuItem key equivalent and mask, so the menu shows the hotkey.
    var menuKeyEquivalent: (key: String, modifiers: NSEvent.ModifierFlags)? {
        guard Self.specialKeyNames[Int(keyCode)] == nil, Self.functionKeyNames[Int(keyCode)] == nil,
            let character = Self.character(for: Int(keyCode))?.lowercased()
        else { return nil }
        let modifiers = Int(carbonModifiers)
        var flags: NSEvent.ModifierFlags = []
        if modifiers & controlKey != 0 { flags.insert(.control) }
        if modifiers & optionKey != 0 { flags.insert(.option) }
        if modifiers & shiftKey != 0 { flags.insert(.shift) }
        if modifiers & cmdKey != 0 { flags.insert(.command) }
        return (character, flags)
    }

    private static let specialKeyNames: [Int: String] = [
        kVK_Space: "Space", kVK_Return: "↵", kVK_Tab: "⇥", kVK_Delete: "⌫", kVK_ForwardDelete: "⌦",
        kVK_Escape: "⎋", kVK_UpArrow: "↑", kVK_DownArrow: "↓", kVK_LeftArrow: "←", kVK_RightArrow: "→",
        kVK_Home: "↖", kVK_End: "↘", kVK_PageUp: "⇞", kVK_PageDown: "⇟", kVK_ANSI_KeypadEnter: "⌤",
    ]

    private static let functionKeyNames: [Int: String] = [
        kVK_F1: "F1", kVK_F2: "F2", kVK_F3: "F3", kVK_F4: "F4", kVK_F5: "F5", kVK_F6: "F6",
        kVK_F7: "F7", kVK_F8: "F8", kVK_F9: "F9", kVK_F10: "F10", kVK_F11: "F11", kVK_F12: "F12",
        kVK_F13: "F13", kVK_F14: "F14", kVK_F15: "F15", kVK_F16: "F16", kVK_F17: "F17", kVK_F18: "F18",
        kVK_F19: "F19", kVK_F20: "F20",
    ]

    /// The character `keyCode` types in the current keyboard layout, unmodified.
    private static func character(for keyCode: Int) -> String? {
        guard let source = TISCopyCurrentKeyboardLayoutInputSource()?.takeRetainedValue(),
            let rawLayout = TISGetInputSourceProperty(source, kTISPropertyUnicodeKeyLayoutData)
        else { return nil }
        let layoutData = Unmanaged<CFData>.fromOpaque(rawLayout).takeUnretainedValue() as Data
        return layoutData.withUnsafeBytes { buffer -> String? in
            guard let layout = buffer.baseAddress?.assumingMemoryBound(to: UCKeyboardLayout.self) else { return nil }
            var deadKeyState: UInt32 = 0
            var characters = [UniChar](repeating: 0, count: 4)
            var length = 0
            let status = UCKeyTranslate(
                layout, UInt16(keyCode), UInt16(kUCKeyActionDisplay), 0, UInt32(LMGetKbdType()),
                OptionBits(kUCKeyTranslateNoDeadKeysBit), &deadKeyState,
                characters.count, &length, &characters)
            guard status == noErr, length > 0 else { return nil }
            let text = String(utf16CodeUnits: characters, count: length)
            return text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? nil : text
        }
    }
}

/// The picker hotkey as stored in UserDefaults, registered with the
/// `HotKeyCenter`. A hotkey that fails to register (taken by another app)
/// is refused and the previous one stays active.
@MainActor
@Observable
final class HotKeySettings {
    static let defaultsKey = "hotKey.openPicker"

    private(set) var hotKey: HotKey
    /// Why the last change or the launch registration failed.
    private(set) var registrationError: String?

    @ObservationIgnored private let center: HotKeyCenter
    @ObservationIgnored private let handler: @MainActor () -> Void
    @ObservationIgnored private let defaults: UserDefaults

    init(center: HotKeyCenter, defaults: UserDefaults = .standard, handler: @escaping @MainActor () -> Void) {
        self.center = center
        self.defaults = defaults
        self.handler = handler
        hotKey = defaults.data(forKey: Self.defaultsKey)
            .flatMap { try? JSONDecoder().decode(HotKey.self, from: $0) } ?? .default
    }

    /// Registers the stored hotkey, falling back to the default if it fails.
    func registerStored() {
        if register(hotKey) { return }
        let failure = registrationError
        if hotKey != .default, register(.default) {
            hotKey = .default
        }
        registrationError = failure
    }

    /// Registers `next` and stores it. Returns false (keeping the old one) on failure.
    @discardableResult
    func update(_ next: HotKey) -> Bool {
        let previous = hotKey
        guard register(next) else {
            let failure = registrationError
            register(previous)
            registrationError = failure
            return false
        }
        hotKey = next
        if let data = try? JSONEncoder().encode(next) {
            defaults.set(data, forKey: Self.defaultsKey)
        }
        return true
    }

    /// Frees the hotkey while the recorder listens, so pressing the current
    /// combination reaches the recorder instead of opening the picker.
    func suspend() {
        center.unregister()
    }

    func resume() {
        register(hotKey)
    }

    func unregister() {
        center.unregister()
    }

    @discardableResult
    private func register(_ hotKey: HotKey) -> Bool {
        do {
            try center.register(hotKey, handler: handler)
            registrationError = nil
            return true
        } catch {
            NSLog("Sloppy Paste: could not register the global hotkey: \(error)")
            registrationError = "\(hotKey.displayString) is not available; another app may be using it."
            return false
        }
    }
}
