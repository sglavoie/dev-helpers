import SwiftUI
import SloppyCore

/// The ⌘D side pane: the full content, then context, tags, description and usage.
struct SnippetDetailView: View {
    var snippet: Snippet

    var body: some View {
        let context = TitleContext.parse(snippet.title).context

        ScrollView {
            VStack(alignment: .leading, spacing: 12) {
                Text(snippet.content)
                    .font(.system(.body, design: .monospaced))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)

                Divider()

                if let context {
                    row("Context") {
                        Chip(text: context, color: Color(TitleContext.color(context)))
                    }
                }
                row("Tags") {
                    if snippet.tags.isEmpty {
                        Chip(text: "untagged", color: .secondary)
                    } else {
                        FlowTags(tags: snippet.tags)
                    }
                }
                if !snippet.description.isEmpty {
                    row("Description") { Text(snippet.description) }
                }
                row("Created") { Text(Formatting.absoluteDate(snippet.createdAt)) }
                row("Updated") { Text(Formatting.absoluteDate(snippet.updatedAt)) }
                if let lastUsedAt = snippet.lastUsedAt {
                    row("Last Used") { Text(Formatting.absoluteDate(lastUsedAt)) }
                }
                row("Use Count") { Text("\(snippet.useCount) time\(snippet.useCount == 1 ? "" : "s")") }
            }
            .font(.callout)
            .padding(14)
        }
    }

    private func row<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
            Text(title)
                .foregroundStyle(.secondary)
                .frame(width: 82, alignment: .leading)
            content()
            Spacer(minLength: 0)
        }
    }
}

/// Tag chips that wrap onto several lines.
private struct FlowTags: View {
    var tags: [String]

    var body: some View {
        FlowLayout(spacing: 4) {
            ForEach(tags, id: \.self) { Chip(text: $0) }
        }
    }
}

/// A minimal left-to-right wrapping layout.
private struct FlowLayout: Layout {
    var spacing: CGFloat

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = arrange(subviews, width: proposal.width ?? .infinity)
        let height = rows.last.map { $0.y + $0.height } ?? 0
        let width = rows.map(\.width).max() ?? 0
        return CGSize(width: width, height: height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        for row in arrange(subviews, width: bounds.width) {
            var x = bounds.minX
            for index in row.indices {
                let size = subviews[index].sizeThatFits(.unspecified)
                subviews[index].place(at: CGPoint(x: x, y: bounds.minY + row.y), proposal: ProposedViewSize(size))
                x += size.width + spacing
            }
        }
    }

    private struct Row {
        var indices: [Int] = []
        var y: CGFloat = 0
        var width: CGFloat = 0
        var height: CGFloat = 0
    }

    private func arrange(_ subviews: Subviews, width: CGFloat) -> [Row] {
        var rows: [Row] = [Row()]
        for index in subviews.indices {
            let size = subviews[index].sizeThatFits(.unspecified)
            var row = rows[rows.count - 1]
            if !row.indices.isEmpty && row.width + spacing + size.width > width {
                let y = row.y + row.height + spacing
                rows.append(Row(y: y))
                row = rows[rows.count - 1]
            }
            row.width += (row.indices.isEmpty ? 0 : spacing) + size.width
            row.height = max(row.height, size.height)
            row.indices.append(index)
            rows[rows.count - 1] = row
        }
        return rows
    }
}
