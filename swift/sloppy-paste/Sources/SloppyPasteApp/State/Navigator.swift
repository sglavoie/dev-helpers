import Observation
import SloppyCore

/// A screen inside the picker panel.
enum Route: Hashable {
    case root
    case placeholderForm(snippetID: String, mode: PlaceholderFormMode)
    case editor(EditorMode)
    case tagPicker(snippetID: String)
    case manageTags
    case renameTag(String)
    case mergeTags(String)
    case history(key: String?)
}

enum EditorMode: Hashable {
    case new
    case edit(snippetID: String)
    case fromClipboard
}

/// The picker's route stack. The root screen is always at the bottom.
@MainActor
@Observable
final class Navigator {
    private(set) var stack: [Route] = [.root]
    /// Runs before every stack change. The panel ends text editing here: a
    /// screen removed while its field is editing otherwise leaves an orphaned
    /// field editor as first responder, and the next screen cannot take focus.
    @ObservationIgnored var willChange: @MainActor () -> Void = {}

    var current: Route { stack[stack.count - 1] }
    var canPop: Bool { stack.count > 1 }

    func push(_ route: Route) {
        willChange()
        stack.append(route)
    }

    /// Pops one screen. Returns false when already at the root.
    @discardableResult
    func pop() -> Bool {
        guard canPop else { return false }
        willChange()
        stack.removeLast()
        return true
    }

    func popToRoot() {
        guard canPop else { return }
        willChange()
        stack = [.root]
    }
}
