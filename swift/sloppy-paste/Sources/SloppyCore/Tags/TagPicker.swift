import Foundation

/// Logic behind the unified tag picker: row states, filtering and toggling.
public enum TagPicker {
    public enum RowState: String, Sendable, Hashable {
        case selected
        /// Not stored on the snippet but covered by a stored descendant
        /// (storage drops redundant parents).
        case implied
        case unselected
    }

    public struct Row: Sendable, Hashable {
        public var tag: String
        public var state: RowState
        public var impliedBy: String?

        public init(tag: String, state: RowState, impliedBy: String? = nil) {
            self.tag = tag
            self.state = state
            self.impliedBy = impliedBy
        }
    }

    public enum CreateCandidate: Sendable, Hashable {
        case tag(String)
        case error(String)
    }

    public struct ToggleResult: Sendable, Hashable {
        public var tags: [String]
        public var changed: Bool
        public var impliedBy: String?

        public init(tags: [String], changed: Bool, impliedBy: String? = nil) {
            self.tags = tags
            self.changed = changed
            self.impliedBy = impliedBy
        }
    }

    /// Known and selected tags expanded with their parents, deduplicated,
    /// sorted, and annotated with their selection state.
    public static func buildRows(knownTags: [String], selectedTags: [String]) -> [Row] {
        let selected = TagNormalization.removeRedundantParents(selectedTags)
        let selectedSet = Set(selected)

        var impliedBy: [String: String] = [:]
        for tag in selected {
            for parent in TagHierarchy.allParentTags(tag) where !selectedSet.contains(parent) && impliedBy[parent] == nil {
                impliedBy[parent] = tag
            }
        }

        return TagHierarchy.expandTagsWithParents(knownTags + selected).map { tag in
            if selectedSet.contains(tag) {
                return Row(tag: tag, state: .selected)
            }
            if let source = impliedBy[tag] {
                return Row(tag: tag, state: .implied, impliedBy: source)
            }
            return Row(tag: tag, state: .unselected)
        }
    }

    /// Case-insensitive substring match against the full tag path.
    public static func filterRows(_ rows: [Row], searchText: String) -> [Row] {
        let query = searchText.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard !query.isEmpty else { return rows }
        return rows.filter { $0.tag.lowercased().contains(query) }
    }

    /// The tag the search text would create, a validation error to show, or
    /// nil when the text is blank or already matches a known tag.
    public static func createCandidate(searchText: String, knownTags: [String]) -> CreateCandidate? {
        let trimmed = searchText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }

        let validation = Validation.validateTag(trimmed)
        guard validation.isValid else {
            return .error(validation.error ?? "Invalid tag name")
        }

        let tag = validation.normalizedValue ?? TagNormalization.normalizeTag(trimmed)
        if knownTags.contains(where: { TagNormalization.normalizeTag($0) == tag }) {
            return nil
        }
        return .tag(tag)
    }

    /// Adds or removes a tag with the same redundant-parent normalisation as
    /// storage. Toggling a tag that is only implied changes nothing and
    /// reports the descendant responsible.
    public static func toggle(selectedTags: [String], tag: String) -> ToggleResult {
        let normalized = TagNormalization.normalizeTag(tag)
        let current = TagNormalization.removeRedundantParents(selectedTags)

        if current.contains(normalized) {
            return ToggleResult(
                tags: TagNormalization.removeRedundantParents(current.filter { $0 != normalized }), changed: true)
        }
        if let descendant = current.first(where: { $0.hasPrefix(normalized + "/") }) {
            return ToggleResult(tags: current, changed: false, impliedBy: descendant)
        }
        return ToggleResult(tags: TagNormalization.removeRedundantParents(current + [normalized]), changed: true)
    }
}
