import Testing
@testable import SloppyCore

@Suite struct TagHierarchyTests {
    typealias H = TagHierarchy
    typealias N = TagNormalization

    @Test(arguments: [
        ("Work", "work"), ("PROJECTS", "projects"), ("  work  ", "work"),
        ("\tprojects\n", "projects"), ("  Work/Projects  ", "work/projects"),
    ])
    func normalizeTag(input: String, expected: String) {
        #expect(N.normalizeTag(input) == expected)
    }

    @Test func normalizeTags() {
        #expect(N.normalizeTags(["Work", "PROJECTS", "  personal  "]) == ["work", "projects", "personal"])
        #expect(N.normalizeTags(["work", "  ", "", "projects"]) == ["work", "projects"])
    }

    @Test(arguments: [
        ("work/projects/client-a", ["work", "projects", "client-a"]),
        ("work", ["work"]),
        ("work//projects", ["work", "projects"]),
    ])
    func parseTagPath(tag: String, expected: [String]) {
        #expect(H.parseTagPath(tag) == expected)
    }

    @Test(arguments: [
        ("work/projects/client-a", "work/projects" as String?),
        ("work/projects", "work"),
        ("work", nil),
    ])
    func parentTag(tag: String, expected: String?) {
        #expect(H.parentTag(tag) == expected)
    }

    @Test(arguments: [
        ("work/projects/client-a", ["work", "work/projects"]),
        ("work/projects", ["work"]),
        ("work", []),
    ])
    func allParentTags(tag: String, expected: [String]) {
        #expect(H.allParentTags(tag) == expected)
    }

    @Test(arguments: [("work", 0), ("work/projects", 1), ("work/projects/client-a", 2)])
    func tagDepth(tag: String, expected: Int) {
        #expect(H.tagDepth(tag) == expected)
    }

    @Test(arguments: [("work/projects/client-a", "client-a"), ("work/projects", "projects"), ("work", "work")])
    func tagName(tag: String, expected: String) {
        #expect(H.tagName(tag) == expected)
    }

    let tags = ["work", "work/projects", "work/personal", "work/projects/client-a", "personal"]

    @Test func childTags() {
        #expect(H.childTags(tags, parent: "work") == ["work/projects", "work/personal"])
        #expect(!H.childTags(tags, parent: "work").contains("work/projects/client-a"))
        #expect(H.childTags(tags, parent: "personal") == [])
        #expect(H.childTags(tags, parent: "WORK") == ["work/projects", "work/personal"])
    }

    @Test func descendantTags() {
        #expect(H.descendantTags(tags, parent: "work") == ["work/projects", "work/personal", "work/projects/client-a"])
        #expect(H.descendantTags(tags, parent: "personal") == [])
    }

    @Test(arguments: [
        ("work/projects", "work", true),
        ("work/projects/client-a", "work", true),
        ("work", "work", false),
        ("personal", "work", false),
        ("WORK/PROJECTS", "work", true),
    ])
    func isChildOf(tag: String, parent: String, expected: Bool) {
        #expect(H.isChildOf(tag, parent) == expected)
    }

    @Test(arguments: [
        ("work/projects", "work", true),
        ("work/projects/client-a", "work", false),
        ("work", "work", false),
    ])
    func isDirectChildOf(tag: String, parent: String, expected: Bool) {
        #expect(H.isDirectChildOf(tag, parent) == expected)
    }

    @Test func rootTags() {
        #expect(H.rootTags(["work", "work/projects", "personal", "dev/backend"]) == ["work", "personal"])
    }
}

@Suite struct TagTreeTests {
    typealias H = TagHierarchy

    @Test func buildsHierarchy() {
        let tree = H.buildTagTree(["work", "work/projects", "work/personal", "personal"])
        #expect(tree.count == 2)
        #expect(tree[0].tag == "personal")
        #expect(tree[0].children.isEmpty)
        #expect(tree[1].tag == "work")
        #expect(tree[1].children.count == 2)
    }

    @Test func normalizesCase() {
        let tree = H.buildTagTree(["Work", "work/Projects", "PERSONAL"])
        #expect(tree.map(\.tag) == ["personal", "work"])
    }

    @Test func deduplicates() {
        #expect(H.buildTagTree(["work", "Work", "WORK"]).count == 1)
    }

    @Test func orphanedNestedTagAtRoot() {
        let tree = H.buildTagTree(["work/projects", "personal"])
        #expect(tree.count == 2)
        #expect(tree[0].tag == "personal")
        #expect(tree[0].depth == 0)
        #expect(tree[1].tag == "work/projects")
        #expect(tree[1].depth == 1)
        #expect(tree[1].children.isEmpty)
    }

    @Test func orphanNestsOnceParentAdded() {
        #expect(H.buildTagTree(["work/projects"]).map(\.tag) == ["work/projects"])
        let tree = H.buildTagTree(["work", "work/projects"])
        #expect(tree.count == 1)
        #expect(tree[0].tag == "work")
        #expect(tree[0].children.map(\.tag) == ["work/projects"])
    }

    @Test func multipleOrphans() {
        #expect(H.buildTagTree(["work/projects", "work/admin", "personal"]).map(\.tag)
            == ["personal", "work/admin", "work/projects"])
    }

    @Test func deeplyNestedOrphan() {
        let tree = H.buildTagTree(["work/projects/client-a"])
        #expect(tree.count == 1)
        #expect(tree[0].tag == "work/projects/client-a")
        #expect(tree[0].depth == 2)
        #expect(tree[0].children.isEmpty)
    }

    @Test func orphansWithCommonParent() {
        let tree = H.buildTagTree(["work/projects/client-a", "work/projects/client-b"])
        #expect(tree.map(\.tag) == ["work/projects/client-a", "work/projects/client-b"])
        #expect(tree.allSatisfy { $0.children.isEmpty })

        let withParent = H.buildTagTree(["work/projects", "work/projects/client-a", "work/projects/client-b"])
        #expect(withParent.count == 1)
        #expect(withParent[0].tag == "work/projects")
        #expect(withParent[0].children.count == 2)
    }

    @Test func flatten() {
        let flat = H.flattenTagTree(H.buildTagTree(["work", "work/projects", "work/personal", "personal"]))
        #expect(flat.count == 4)
        #expect(flat[0] == FlatTagNode(tag: "personal", name: "personal", depth: 0, hasChildren: false))
        #expect(flat[1] == FlatTagNode(tag: "work", name: "work", depth: 0, hasChildren: true))
    }
}

@Suite struct TagListTests {
    @Test(arguments: [
        (["work", "Work", "WORK"], ["work"]),
        (["Work", "PROJECTS", "work"], ["work", "projects"]),
        (["work/projects", "Work/Projects", "personal"], ["work/projects", "personal"]),
    ])
    func deduplicateTags(input: [String], expected: [String]) {
        #expect(TagNormalization.deduplicateTags(input) == expected)
    }

    @Test(arguments: [
        (["work/projects"], ["work", "work/projects"]),
        (["work/projects/client-a"], ["work", "work/projects", "work/projects/client-a"]),
        (["work", "work/projects"], ["work", "work/projects"]),
        (["work", "personal"], ["personal", "work"]),
        (["personal", "work/projects/client-a"], ["personal", "work", "work/projects", "work/projects/client-a"]),
        (["Work/Projects", "work", "WORK/PROJECTS"], ["work", "work/projects"]),
        ([], []),
        (["work/projects", "personal/notes"], ["personal", "personal/notes", "work", "work/projects"]),
    ])
    func expandTagsWithParents(input: [String], expected: [String]) {
        #expect(TagHierarchy.expandTagsWithParents(input) == expected)
    }

    @Test(arguments: [
        (["work", "work/projects"], ["work/projects"]),
        (["work", "work/projects", "work/projects/client-a"], ["work/projects/client-a"]),
        (["work", "personal"], ["personal", "work"]),
        (["work", "work/projects", "personal", "personal/notes"], ["personal/notes", "work/projects"]),
        (["work", "work/projects", "work/admin"], ["work/admin", "work/projects"]),
        (["Work", "work/Projects", "WORK/PROJECTS"], ["work/projects"]),
        ([], []),
        (["work/projects"], ["work/projects"]),
        (["root", "root/level1", "root/level1/level2", "root/level1/level2/level3", "other"],
         ["other", "root/level1/level2/level3"]),
    ])
    func removeRedundantParents(input: [String], expected: [String]) {
        #expect(TagNormalization.removeRedundantParents(input) == expected)
    }
}

@Suite struct FilterSnippetsByTagTests {
    static func snippet(_ id: String, _ tags: [String]) -> Snippet {
        Snippet(id: id, title: "Snippet \(id)", content: "Content \(id)", tags: tags,
                createdAt: 1000, updatedAt: 2000, lastUsedAt: 5000, useCount: 1)
    }

    let snippets = [
        snippet("1", ["work"]),
        snippet("2", ["work/projects"]),
        snippet("3", ["work/projects/client-a"]),
        snippet("4", ["personal"]),
        snippet("5", []),
        snippet("6", []),
        snippet("7", ["personal", "work"]),
    ]

    @Test(arguments: [
        ("personal", ["4", "7"]),
        ("work", ["1", "2", "3", "7"]),
        (TagHierarchy.untaggedSentinel, ["5", "6"]),
        ("nonexistent", []),
        ("work/projects", ["2", "3"]),
    ])
    func filters(tag: String, expected: [String]) {
        #expect(TagHierarchy.filterSnippets(snippets, byTag: tag).map(\.id).sorted() == expected)
    }
}
