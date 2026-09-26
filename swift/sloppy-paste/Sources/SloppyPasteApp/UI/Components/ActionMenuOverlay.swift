import SwiftUI

/// The ⌘K action menu: the current screen's catalog as a filterable list
/// with each action's shortcut. KeyRouter routes ↑/↓, ↵ and Esc while it
/// is open; typing goes to the filter field.
struct ActionMenuOverlay: View {
    let menu: ActionMenu
    @Environment(\.keyRouter) private var keyRouter
    @ViewState private var searchHandle = FocusHandle()

    var body: some View {
        let visible = menu.visibleActions
        let selection = menu.selection
        ZStack(alignment: .bottomTrailing) {
            Color.black.opacity(0.2)
                .onTapGesture { keyRouter?.closeActionMenu() }
            VStack(alignment: .leading, spacing: 0) {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 1) {
                            ForEach(visible) { binding in
                                row(binding, isSelected: binding.id == selection?.id)
                                    .id(binding.id)
                            }
                            if visible.isEmpty {
                                Text("No matching actions")
                                    .foregroundStyle(.secondary)
                                    .frame(maxWidth: .infinity)
                                    .padding(.vertical, 16)
                            }
                        }
                        .padding(6)
                    }
                    .onChange(of: selection?.id) {
                        guard let id = selection?.id else { return }
                        proxy.scrollTo(id)
                    }
                }
                .frame(maxHeight: 300)
                .fixedSize(horizontal: false, vertical: true)
                Divider()
                EditorTextField(
                    placeholder: "Search for actions…",
                    text: Binding(get: { menu.query }, set: { menu.setQuery($0) }),
                    handle: searchHandle)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(8)
            }
            .frame(width: 360)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 10))
            .shadow(radius: 12)
            .padding(.trailing, 12)
            .padding(.bottom, 36)
        }
        .onAppear {
            Task { @MainActor in searchHandle.focus() }
        }
    }

    private func row(_ binding: KeyBinding, isSelected: Bool) -> some View {
        HStack {
            Text(binding.title)
                .lineLimit(1)
            Spacer(minLength: 12)
            if let chord = binding.chord {
                Text(chord.label)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 5)
        .background(RoundedRectangle(cornerRadius: 6).fill(isSelected ? Color.accentColor.opacity(0.25) : .clear))
        .contentShape(Rectangle())
        .onTapGesture { keyRouter?.performFromMenu(binding) }
    }
}
