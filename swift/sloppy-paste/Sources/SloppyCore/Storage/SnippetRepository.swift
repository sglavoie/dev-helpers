import Foundation

/// A placeholder value submitted with a snippet use.
public struct PlaceholderValueToRecord: Sendable, Hashable {
    public var key: String
    public var value: String
    public var isSaved: Bool

    public init(key: String, value: String, isSaved: Bool = true) {
        self.key = key
        self.value = value
        self.isSaved = isSaved
    }
}

/// Snippet and tag operations on an in-memory `StorageData`. Callers own
/// persistence (see `StorageFile`) and pass the current time in.
public struct SnippetRepository: Sendable, Hashable {
    public var data: StorageData

    public init(data: StorageData = .empty) {
        self.data = data
    }

    public var snippets: [Snippet] { data.snippets }

    public func snippet(id: String) -> Snippet? {
        data.snippets.first { $0.id == id }
    }

    /// Unique tags across all snippets, sorted.
    public var tags: [String] {
        Array(Set(data.snippets.flatMap(\.tags))).sorted(by: TagNormalization.localeLess)
    }

    // MARK: Snippets

    @discardableResult
    public mutating func addSnippet(
        title: String,
        content: String,
        description: String = "",
        tags: [String] = [],
        now: Int64,
        id: String? = nil
    ) -> Snippet {
        let snippet = Snippet(
            id: id ?? SnippetID.make(now: now),
            title: title,
            content: content,
            description: description,
            tags: TagNormalization.removeRedundantParents(tags),
            createdAt: now,
            updatedAt: now
        )
        data.snippets.append(snippet)
        return snippet
    }

    /// Applies `changes` to the snippet and bumps `updatedAt`. Changed tags
    /// are normalised and lose redundant parents.
    @discardableResult
    public mutating func updateSnippet(id: String, now: Int64, _ changes: (inout Snippet) -> Void) throws -> Snippet {
        let index = try index(of: id)
        var snippet = data.snippets[index]
        let originalTags = snippet.tags
        changes(&snippet)
        snippet.id = id
        snippet.createdAt = data.snippets[index].createdAt
        if snippet.tags != originalTags {
            snippet.tags = TagNormalization.removeRedundantParents(snippet.tags)
        }
        snippet.updatedAt = now
        data.snippets[index] = snippet
        return snippet
    }

    public mutating func deleteSnippet(id: String) {
        data.snippets.removeAll { $0.id == id }
    }

    @discardableResult
    public mutating func duplicateSnippet(id: String, now: Int64, newID: String? = nil) throws -> Snippet {
        var copy = data.snippets[try index(of: id)]
        copy.id = newID ?? SnippetID.make(now: now)
        copy.title += " (Copy)"
        copy.createdAt = now
        copy.updatedAt = now
        copy.lastUsedAt = nil
        copy.useCount = 0
        copy.isFavorite = false
        copy.isArchived = false
        copy.isPinned = false
        data.snippets.append(copy)
        return copy
    }

    /// Toggles a flag, returning its new value.
    @discardableResult
    public mutating func toggle(_ flag: WritableKeyPath<Snippet, Bool>, id: String, now: Int64) throws -> Bool {
        let index = try index(of: id)
        data.snippets[index][keyPath: flag].toggle()
        data.snippets[index].updatedAt = now
        return data.snippets[index][keyPath: flag]
    }

    /// Counts a use and saves eligible placeholder values, all with one timestamp.
    /// Blank or opted-out values are skipped, and repeated key/value pairs count once.
    public mutating func recordUse(id: String, placeholderValues: [PlaceholderValueToRecord] = [], now: Int64) throws {
        let index = try index(of: id)
        data.snippets[index].useCount += 1
        data.snippets[index].lastUsedAt = now
        data.snippets[index].updatedAt = now

        var recorded = Set<[String]>()
        for item in placeholderValues where item.isSaved {
            guard !item.value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
                recorded.insert([item.key, item.value]).inserted
            else { continue }
            data.addPlaceholderValue(key: item.key, value: item.value, now: now)
        }
    }

    // MARK: Tags

    /// Removes `tag` (exact match) from every snippet that has it.
    public mutating func deleteTag(_ tag: String, now: Int64) {
        for index in data.snippets.indices where data.snippets[index].tags.contains(tag) {
            data.snippets[index].tags.removeAll { $0 == tag }
            data.snippets[index].updatedAt = now
        }
    }

    /// Renames `oldTag` and its descendants. Returns the number of snippets changed.
    @discardableResult
    public mutating func renameTag(_ oldTag: String, to newTag: String, now: Int64) -> Int {
        retag(from: oldTag, to: newTag, now: now)
    }

    /// Merges `sourceTag` and its descendants into `targetTag`. Returns the number of snippets changed.
    @discardableResult
    public mutating func mergeTags(_ sourceTag: String, into targetTag: String, now: Int64) throws -> Int {
        guard sourceTag != targetTag else { throw StorageError.cannotMergeTagWithItself }
        return retag(from: sourceTag, to: targetTag, now: now)
    }

    private mutating func retag(from source: String, to target: String, now: Int64) -> Int {
        var affected = 0
        for index in data.snippets.indices {
            let tags = data.snippets[index].tags
            guard tags.contains(where: { $0 == source || $0.hasPrefix(source + "/") }) else { continue }
            var seen = Set<String>()
            data.snippets[index].tags = tags
                .map { Self.replacePrefix($0, source: source, target: target).lowercased() }
                .filter { seen.insert($0).inserted }
            data.snippets[index].updatedAt = now
            affected += 1
        }
        return affected
    }

    private static func replacePrefix(_ tag: String, source: String, target: String) -> String {
        if tag == source { return target }
        guard tag.hasPrefix(source + "/") else { return tag }
        return target + tag.dropFirst(source.count)
    }

    private func index(of id: String) throws -> Int {
        guard let index = data.snippets.firstIndex(where: { $0.id == id }) else { throw StorageError.snippetNotFound }
        return index
    }
}
