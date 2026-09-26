import SwiftUI

/// Renders the navigator's current screen inside the picker panel.
struct PickerContentView: View {
    @Environment(Navigator.self) private var navigator
    @Environment(ActionMenu.self) private var actionMenu

    var body: some View {
        Group {
            switch navigator.current {
            case .root:
                PickerRootView()
            case .placeholderForm(let snippetID, let mode):
                PlaceholderFormView(snippetID: snippetID, mode: mode)
                    .id(navigator.current)
            case .editor(let mode):
                SnippetEditorView(mode: mode)
                    .id(navigator.current)
            case .tagPicker(let snippetID):
                TagPickerScreen(snippetID: snippetID)
                    .id(navigator.current)
            case .manageTags:
                ManageTagsView()
            case .renameTag(let tag):
                RenameTagView(tag: tag)
                    .id(navigator.current)
            case .mergeTags(let tag):
                MergeTagsView(sourceTag: tag)
                    .id(navigator.current)
            case .history(nil):
                ManagePlaceholderHistoryView()
            case .history(let key?):
                PlaceholderHistoryDetailView(key: key)
                    .id(navigator.current)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .overlay {
            if actionMenu.isOpen {
                ActionMenuOverlay(menu: actionMenu)
            }
        }
        .overlay { ToastOverlay() }
        .background(.regularMaterial)
    }
}

extension View {
    /// Publishes a screen's key-binding catalog to the KeyRouter while the
    /// screen is on screen.
    func keyBindings(_ bindings: [KeyBinding], for route: Route) -> some View {
        modifier(KeyBindingsModifier(bindings: bindings, route: route))
    }
}

private struct KeyBindingsModifier: ViewModifier {
    @Environment(\.keyRouter) private var keyRouter
    var bindings: [KeyBinding]
    var route: Route

    /// Re-publishes when the set of bindings, their enabled state, their
    /// titles or their chords change, so the ⌘K menu never shows a stale
    /// "Pin"/"Unpin" and root ⌘C follows the search field.
    private var signature: [String] {
        bindings.map { "\($0.id):\($0.isEnabled):\($0.title):\(String(describing: $0.chord))" }
    }

    func body(content: Content) -> some View {
        content
            .onAppear { keyRouter?.setBindings(bindings, for: route) }
            .onChange(of: signature) { keyRouter?.setBindings(bindings, for: route) }
            .onDisappear { keyRouter?.removeBindings(for: route) }
    }
}
