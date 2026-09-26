import SwiftUI

/// Header for the list-style tool screens (Manage Tags, placeholder
/// history): a title, a filter field focused through a `FocusHandle`, and an
/// optional trailing accessory such as a sort menu.
struct ManagementHeader<Accessory: View>: View {
    var title: String
    var placeholder: String
    @Binding var text: String
    var handle: FocusHandle
    @ViewBuilder var accessory: () -> Accessory

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.headline)
                .lineLimit(1)
            HStack(spacing: 8) {
                EditorTextField(placeholder: placeholder, text: $text, handle: handle)
                    .fixedSize(horizontal: false, vertical: true)
                accessory()
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
    }
}

extension ManagementHeader where Accessory == EmptyView {
    init(title: String, placeholder: String, text: Binding<String>, handle: FocusHandle) {
        self.init(title: title, placeholder: placeholder, text: text, handle: handle) { EmptyView() }
    }
}

/// A footer of key hints; the first hint is emphasised as the ↵ action.
struct ManagementFooter: View {
    var summary: String
    var primary: String?
    var hints: [String]

    var body: some View {
        HStack(spacing: 14) {
            Text(summary)
            Spacer()
            if let primary {
                Text(primary).fontWeight(.semibold)
            }
            ForEach(hints, id: \.self) { Text($0) }
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }
}

/// A capsule accessory ("3 snippets", "12 uses").
struct CountChip: View {
    var text: String
    var tint: Color = .secondary

    var body: some View {
        Text(text)
            .font(.caption)
            .foregroundStyle(tint)
            .padding(.horizontal, 6)
            .padding(.vertical, 1)
            .background(tint.opacity(0.15), in: Capsule())
    }
}

/// A centred empty-state message.
struct EmptyListMessage: View {
    var title: String
    var message: String

    var body: some View {
        VStack(spacing: 6) {
            Text(title).font(.headline)
            Text(message).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, 60)
    }
}

extension View {
    /// Row padding and the selection highlight shared by the tool lists.
    func selectableRow(isSelected: Bool) -> some View {
        padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(RoundedRectangle(cornerRadius: 6).fill(isSelected ? Color.accentColor.opacity(0.25) : .clear))
            .contentShape(Rectangle())
    }
}

enum ListSelection {
    /// The id `offset` rows away from `current` (or the first row), without wrapping.
    static func moved(_ ids: [String], from current: String?, by offset: Int) -> String? {
        guard !ids.isEmpty else { return nil }
        let index = ids.firstIndex { $0 == current } ?? 0
        return ids[min(max(index + offset, 0), ids.count - 1)]
    }

    /// `current` when it is still listed, else the first id.
    static func effective(_ ids: [String], _ current: String?) -> String? {
        if let current, ids.contains(current) { return current }
        return ids.first
    }

    /// The neighbour to select once `removed` leaves the list.
    static func neighbour(of removed: String, in ids: [String]) -> String? {
        guard let index = ids.firstIndex(of: removed) else { return nil }
        let next = ids.indices.contains(index + 1) ? index + 1 : index - 1
        return ids.indices.contains(next) ? ids[next] : nil
    }

    /// Standard ↑/↓ bindings for a list.
    @MainActor
    static func arrowBindings(prefix: String, move: @escaping @MainActor (Int) -> Void) -> [KeyBinding] {
        [
            KeyBinding(id: "\(prefix)Up", title: "Previous", chord: KeyChord(.upArrow), showsInMenu: false) {
                move(-1)
            },
            KeyBinding(id: "\(prefix)Down", title: "Next", chord: KeyChord(.downArrow), showsInMenu: false) {
                move(1)
            },
        ]
    }
}
