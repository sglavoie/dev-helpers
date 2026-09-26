import AppKit
import Carbon.HIToolbox
import Observation

/// A key plus modifiers, matched against keyDown events.
struct KeyChord: Hashable {
    enum Key: Hashable {
        case character(Character)
        case escape, `return`, tab, delete, upArrow, downArrow, leftArrow, rightArrow

        var keyCode: Int? {
            switch self {
            case .character: nil
            case .escape: kVK_Escape
            case .return: kVK_Return
            case .tab: kVK_Tab
            case .delete: kVK_Delete
            case .upArrow: kVK_UpArrow
            case .downArrow: kVK_DownArrow
            case .leftArrow: kVK_LeftArrow
            case .rightArrow: kVK_RightArrow
            }
        }
    }

    static let relevantModifiers: NSEvent.ModifierFlags = [.command, .option, .control, .shift]

    var key: Key
    var modifiers: NSEvent.ModifierFlags

    init(_ key: Key, _ modifiers: NSEvent.ModifierFlags = []) {
        self.key = key
        self.modifiers = modifiers.intersection(Self.relevantModifiers)
    }

    static func == (lhs: KeyChord, rhs: KeyChord) -> Bool {
        lhs.key == rhs.key && lhs.modifiers.rawValue == rhs.modifiers.rawValue
    }

    func hash(into hasher: inout Hasher) {
        hasher.combine(key)
        hasher.combine(modifiers.rawValue)
    }

    func matches(_ event: NSEvent) -> Bool {
        guard event.modifierFlags.intersection(Self.relevantModifiers) == modifiers else { return false }
        switch key {
        case .character(let character):
            // Shift is matched through the modifiers, so compare case-insensitively.
            guard let characters = event.charactersIgnoringModifiers?.lowercased() else { return false }
            return characters == String(character).lowercased()
        default:
            // Keypad Enter counts as Return.
            if key == .return, Int(event.keyCode) == kVK_ANSI_KeypadEnter { return true }
            return Int(event.keyCode) == key.keyCode
        }
    }

    /// "⇧⌘P"-style label for menus and footers.
    var label: String {
        var text = ""
        if modifiers.contains(.control) { text += "⌃" }
        if modifiers.contains(.option) { text += "⌥" }
        if modifiers.contains(.shift) { text += "⇧" }
        if modifiers.contains(.command) { text += "⌘" }
        switch key {
        case .character(let character): text += String(character).uppercased()
        case .escape: text += "⎋"
        case .return: text += "↵"
        case .tab: text += "⇥"
        case .delete: text += "⌫"
        case .upArrow: text += "↑"
        case .downArrow: text += "↓"
        case .leftArrow: text += "←"
        case .rightArrow: text += "→"
        }
        return text
    }
}

/// One shortcut in a screen's catalog. The ⌘K action menu is built from the
/// same catalog, so menu entries and shortcuts cannot drift apart.
struct KeyBinding: Identifiable {
    var id: String
    var title: String
    var chord: KeyChord?
    /// Hidden bindings route keys but are left out of the action menu.
    var showsInMenu: Bool = true
    var isEnabled: Bool = true
    var perform: @MainActor () -> Void
}

/// Routes keyDown events in the picker panel through a local NSEvent monitor:
/// the ⌘K action menu while it is open, then ⌘K itself, the current screen's
/// bindings, and finally Esc, which pops the navigation stack or hides the
/// panel at the root.
@MainActor
final class KeyRouter {
    static let actionMenuChord = KeyChord(.character("k"), .command)

    let actionMenu = ActionMenu()
    private let navigator: Navigator
    private var bindingsByRoute: [Route: [KeyBinding]] = [:]
    private var monitor: Any?
    private weak var window: NSWindow?
    private let onEscapeAtRoot: @MainActor () -> Void

    init(navigator: Navigator, onEscapeAtRoot: @escaping @MainActor () -> Void) {
        self.navigator = navigator
        self.onEscapeAtRoot = onEscapeAtRoot
    }

    /// Starts routing keyDown events delivered to `window`.
    func start(for window: NSWindow) {
        self.window = window
        guard monitor == nil else { return }
        monitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            // NSEvent is not Sendable, so only a Bool crosses the isolation hop.
            nonisolated(unsafe) let event = event
            let consumed = MainActor.assumeIsolated {
                guard let self, event.window === self.window else { return false }
                return self.handle(event)
            }
            return consumed ? nil : event
        }
    }

    func stop() {
        if let monitor {
            NSEvent.removeMonitor(monitor)
        }
        monitor = nil
    }

    /// Screens publish their catalog here whenever it changes.
    func setBindings(_ bindings: [KeyBinding], for route: Route) {
        bindingsByRoute[route] = bindings
    }

    func removeBindings(for route: Route) {
        bindingsByRoute[route] = nil
    }

    func bindings(for route: Route) -> [KeyBinding] {
        bindingsByRoute[route] ?? []
    }

    /// Returns true when the event was consumed.
    func handle(_ event: NSEvent) -> Bool {
        if actionMenu.isOpen {
            return handleActionMenu(event)
        }
        if Self.actionMenuChord.matches(event) {
            openActionMenu()
            return true
        }
        let bindings = bindings(for: navigator.current)
        if let binding = bindings.first(where: { $0.isEnabled && $0.chord?.matches(event) == true }) {
            binding.perform()
            return true
        }
        if KeyChord(.escape).matches(event) {
            if !navigator.pop() {
                onEscapeAtRoot()
            }
            return true
        }
        return false
    }

    // MARK: Action menu

    /// Opens the ⌘K menu on the current screen's catalog. Does nothing when
    /// the screen has no menu actions.
    func openActionMenu() {
        let actions = bindings(for: navigator.current).filter { $0.showsInMenu && $0.isEnabled }
        guard !actions.isEmpty else { return }
        actionMenu.open(actions, returningFocusTo: focusedView())
    }

    /// Closes the menu and gives keyboard focus back to the field that had it.
    func closeActionMenu(restoringFocus: Bool = true) {
        guard actionMenu.isOpen else { return }
        let target = actionMenu.close()
        guard restoringFocus, let window, let target, target.window === window else { return }
        window.makeFirstResponder(target)
    }

    /// Closes the menu, then runs the action as if its shortcut was pressed.
    func performFromMenu(_ binding: KeyBinding) {
        closeActionMenu()
        binding.perform()
    }

    private func handleActionMenu(_ event: NSEvent) -> Bool {
        if KeyChord(.escape).matches(event) || Self.actionMenuChord.matches(event) {
            closeActionMenu()
        } else if KeyChord(.upArrow).matches(event) {
            actionMenu.moveSelection(by: -1)
        } else if KeyChord(.downArrow).matches(event) {
            actionMenu.moveSelection(by: 1)
        } else if KeyChord(.return).matches(event) || KeyChord(.return, .command).matches(event) {
            if let binding = actionMenu.selection {
                performFromMenu(binding)
            }
        } else {
            // Typing goes to the menu's filter field; screen shortcuts stay off.
            return false
        }
        return true
    }

    /// The view that owns keyboard focus: a text field rather than its
    /// shared field editor.
    private func focusedView() -> NSView? {
        guard let responder = window?.firstResponder as? NSView else { return nil }
        if let editor = responder as? NSTextView, editor.isFieldEditor, let field = editor.delegate as? NSView {
            return field
        }
        return responder
    }
}

/// The ⌘K menu's state: the actions captured from the screen's catalog, the
/// filter text and the selection.
@MainActor
@Observable
final class ActionMenu {
    private(set) var isOpen = false
    private(set) var actions: [KeyBinding] = []
    private(set) var query = ""
    private(set) var selectedID: String?
    @ObservationIgnored private weak var returnFocus: NSView?

    /// Actions whose title contains every word of the filter text.
    var visibleActions: [KeyBinding] {
        let words = query.lowercased().split(whereSeparator: \.isWhitespace)
        guard !words.isEmpty else { return actions }
        return actions.filter { binding in
            let title = binding.title.lowercased()
            return words.allSatisfy { title.contains($0) }
        }
    }

    var selection: KeyBinding? {
        let visible = visibleActions
        return visible.first { $0.id == selectedID } ?? visible.first
    }

    func open(_ actions: [KeyBinding], returningFocusTo view: NSView?) {
        self.actions = actions
        query = ""
        selectedID = nil
        returnFocus = view
        isOpen = true
    }

    /// Returns the view to give focus back to.
    func close() -> NSView? {
        isOpen = false
        actions = []
        query = ""
        selectedID = nil
        defer { returnFocus = nil }
        return returnFocus
    }

    func setQuery(_ text: String) {
        guard text != query else { return }
        query = text
        selectedID = nil
    }

    func select(_ id: String) {
        selectedID = id
    }

    func moveSelection(by offset: Int) {
        let visible = visibleActions
        guard !visible.isEmpty else { return }
        let current = visible.firstIndex { $0.id == selection?.id } ?? 0
        selectedID = visible[min(max(current + offset, 0), visible.count - 1)].id
    }
}
