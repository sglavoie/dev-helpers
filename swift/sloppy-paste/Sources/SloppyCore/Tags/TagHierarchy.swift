import Foundation

/// A tag and its direct children, for hierarchical lists.
public struct TagNode: Sendable, Hashable {
    public var tag: String
    public var name: String
    public var depth: Int
    public var children: [TagNode]

    public init(tag: String, name: String, depth: Int, children: [TagNode]) {
        self.tag = tag
        self.name = name
        self.depth = depth
        self.children = children
    }
}

/// A tag tree row with its depth, for rendering an indented flat list.
public struct FlatTagNode: Sendable, Hashable {
    public var tag: String
    public var name: String
    public var depth: Int
    public var hasChildren: Bool

    public init(tag: String, name: String, depth: Int, hasChildren: Bool) {
        self.tag = tag
        self.name = name
        self.depth = depth
        self.hasChildren = hasChildren
    }
}

/// Hierarchy helpers for `/`-separated tags such as `work/projects/client-a`.
/// Normalisation itself lives in `TagNormalization`.
public enum TagHierarchy {
    /// Sentinel used to represent the "Untagged" pseudo-category.
    public static let untaggedSentinel = "__untagged__"

    /// `"work/projects/client-a"` → `["work", "projects", "client-a"]`; empty segments are dropped.
    public static func parseTagPath(_ tag: String) -> [String] {
        tag.split(separator: "/", omittingEmptySubsequences: true).map(String.init)
    }

    /// `"work/projects/client-a"` → `"work/projects"`; nil for a root tag.
    public static func parentTag(_ tag: String) -> String? {
        let segments = parseTagPath(tag)
        guard segments.count > 1 else { return nil }
        return segments.dropLast().joined(separator: "/")
    }

    /// `"work/projects/client-a"` → `["work", "work/projects"]`.
    public static func allParentTags(_ tag: String) -> [String] {
        let segments = parseTagPath(tag)
        guard segments.count > 1 else { return [] }
        return (1..<segments.count).map { segments[..<$0].joined(separator: "/") }
    }

    /// 0-indexed depth: `"work"` → 0, `"work/projects"` → 1.
    public static func tagDepth(_ tag: String) -> Int {
        parseTagPath(tag).count - 1
    }

    /// The last path segment: `"work/projects/client-a"` → `"client-a"`.
    public static func tagName(_ tag: String) -> String {
        parseTagPath(tag).last ?? tag
    }

    /// Direct children of `parent` in `tags` (case-insensitive).
    public static func childTags(_ tags: [String], parent: String) -> [String] {
        tags.filter { isDirectChildOf($0, parent) }
    }

    /// Children, grandchildren and so on of `parent` in `tags` (case-insensitive).
    public static func descendantTags(_ tags: [String], parent: String) -> [String] {
        let prefix = TagNormalization.normalizeTag(parent) + "/"
        return tags.filter { TagNormalization.normalizeTag($0).hasPrefix(prefix) }
    }

    /// Whether `tag` is a direct or indirect child of `parent`.
    public static func isChildOf(_ tag: String, _ parent: String) -> Bool {
        let normalizedTag = TagNormalization.normalizeTag(tag)
        let normalizedParent = TagNormalization.normalizeTag(parent)
        guard normalizedTag != normalizedParent else { return false }
        return normalizedTag.hasPrefix(normalizedParent + "/")
    }

    /// Whether `tag` is exactly one level below `parent`.
    public static func isDirectChildOf(_ tag: String, _ parent: String) -> Bool {
        let normalizedTag = TagNormalization.normalizeTag(tag)
        let prefix = TagNormalization.normalizeTag(parent) + "/"
        guard normalizedTag.hasPrefix(prefix) else { return false }
        return !normalizedTag.dropFirst(prefix.count).contains("/")
    }

    /// Tags with no parent segment.
    public static func rootTags(_ tags: [String]) -> [String] {
        tags.filter { !$0.contains("/") }
    }

    /// Builds a tree from a flat list. A nested tag whose parent is missing is
    /// shown at the root (keeping its depth) until the parent exists.
    public static func buildTagTree(_ tags: [String]) -> [TagNode] {
        let normalized = Array(Set(TagNormalization.normalizeTags(tags))).sorted()

        func buildNode(_ tag: String) -> TagNode {
            TagNode(
                tag: tag,
                name: tagName(tag),
                depth: tagDepth(tag),
                children: childTags(normalized, parent: tag).map(buildNode)
            )
        }

        var tree = rootTags(normalized).map(buildNode)

        var inTree = Set<String>()
        func collect(_ node: TagNode) {
            inTree.insert(node.tag)
            node.children.forEach(collect)
        }
        tree.forEach(collect)

        let orphaned = normalized.filter { $0.contains("/") && !inTree.contains($0) }
        let orphanedSet = Set(orphaned)
        // Only the shallowest orphans become roots; deeper ones nest under them.
        let orphanedRoots = orphaned.filter { tag in
            !allParentTags(tag).contains { orphanedSet.contains($0) || inTree.contains($0) }
        }
        tree += orphanedRoots.map(buildNode)
        return tree
    }

    /// Depth-first flattening of a tag tree.
    public static func flattenTagTree(_ tree: [TagNode]) -> [FlatTagNode] {
        var result: [FlatTagNode] = []
        func flatten(_ node: TagNode) {
            result.append(FlatTagNode(
                tag: node.tag, name: node.name, depth: node.depth, hasChildren: !node.children.isEmpty))
            node.children.forEach(flatten)
        }
        tree.forEach(flatten)
        return result
    }

    /// Adds every parent of every tag: `["work/projects"]` → `["work", "work/projects"]`.
    /// Normalised, deduplicated and sorted.
    public static func expandTagsWithParents(_ tags: [String]) -> [String] {
        var expanded = Set<String>()
        for tag in TagNormalization.normalizeTags(tags) {
            expanded.insert(tag)
            expanded.formUnion(allParentTags(tag))
        }
        return expanded.sorted(by: TagNormalization.localeLess)
    }

    /// Snippets carrying `tag` or one of its descendants; `untaggedSentinel` selects untagged snippets.
    public static func filterSnippets(_ snippets: [Snippet], byTag tag: String) -> [Snippet] {
        if tag == untaggedSentinel {
            return snippets.filter { $0.tags.isEmpty }
        }
        return snippets.filter { $0.tags.contains { $0 == tag || isChildOf($0, tag) } }
    }
}
