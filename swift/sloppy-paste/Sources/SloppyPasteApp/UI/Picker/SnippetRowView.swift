import SwiftUI
import SloppyCore

/// One snippet in the root list: context badge, title, a content preview and
/// the stale, `⌨ n`, pin, star, tag and date accessories.
struct SnippetRowView: View {
    var snippet: Snippet
    var isSelected: Bool
    /// The detail pane is open, so the row drops its subtitle and accessories.
    var compact: Bool
    var showsStalenessReason: Bool
    var historyAvailable: Bool
    var now: Int64

    var body: some View {
        let titleContext = TitleContext.parse(snippet.title)
        let analytics = Staleness.analyze(snippet, now: now)

        HStack(spacing: 10) {
            leadingIcon(context: titleContext.context)
                .frame(width: 22, height: 22)

            HStack(spacing: 8) {
                Text(titleContext.displayTitle)
                    .lineLimit(1)
                    .layoutPriority(1)
                    .help(snippet.title)
                if !compact {
                    Text(subtitle(analytics))
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }

            Spacer(minLength: 8)

            if !compact {
                accessories(analytics)
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(
            RoundedRectangle(cornerRadius: 6)
                .fill(isSelected ? Color.accentColor.opacity(0.25) : .clear))
        .contentShape(Rectangle())
    }

    @ViewBuilder
    private func leadingIcon(context: String?) -> some View {
        if let context {
            ContextBadge(context: context)
        } else {
            Image(systemName: snippet.isPinned ? "pin" : snippet.isFavorite ? "star" : "doc.text")
                .foregroundStyle(.secondary)
        }
    }

    private func subtitle(_ analytics: SnippetAnalytics) -> String {
        let text = showsStalenessReason ? (analytics.stalenessReason ?? snippet.content) : snippet.content
        return text.split(whereSeparator: \.isNewline).first.map(String.init) ?? ""
    }

    @ViewBuilder
    private func accessories(_ analytics: SnippetAnalytics) -> some View {
        let requiredInputs = PickerListModel.requiredInputCount(snippet)
        HStack(spacing: 6) {
            if analytics.isStale && !snippet.isArchived {
                Text("stale")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .help(analytics.stalenessReason ?? "Stale snippet")
            }
            if requiredInputs > 0 {
                Text("⌨ \(requiredInputs)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(historyAvailable ? Color.green : Color.secondary)
                    .help(historyAvailable
                        ? "⇧⌘↵: paste with last values"
                        : "\(requiredInputs) required placeholder\(requiredInputs > 1 ? "s" : "")")
            }
            if snippet.isPinned {
                Image(systemName: "pin.fill").font(.caption).help("Pinned")
            }
            if snippet.isFavorite {
                Image(systemName: "star.fill").font(.caption).foregroundStyle(.yellow).help("Bookmarked")
            }
            if snippet.tags.isEmpty {
                Chip(text: "untagged", color: .secondary)
            } else {
                ForEach(snippet.tags.prefix(3), id: \.self) { Chip(text: $0) }
                if snippet.tags.count > 3 {
                    Chip(text: "+\(snippet.tags.count - 3)", color: .secondary)
                }
            }
            Text(Formatting.relativeTime(snippet.updatedAt, now: now))
                .font(.caption)
                .foregroundStyle(.secondary)
                .help("Updated: \(Formatting.absoluteDate(snippet.updatedAt))")
        }
        .fixedSize()
    }
}
