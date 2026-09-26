import Foundation
import Testing
@testable import SloppyCore

@Suite struct ImportExportTests {
    let baseSnippet = Snippet(id: "snippet-1", title: "Title", content: "Content", createdAt: 1000, updatedAt: 1000)

    func payload(_ snippetsJSON: String, history: String = "{}") throws -> ImportPayload {
        let json = #"{"version": "1.0.0", "exportedAt": 2000, "snippets": \#(snippetsJSON), "tags": [], "placeholderHistory": \#(history)}"#
        return try ImportExport.decodeImport(Data(json.utf8))
    }

    @Test func replaceBackfillsLegacySnippetFields() throws {
        let imported = try payload(#"[{"id": "legacy-1", "title": "Legacy", "content": "Content", "category": " Work ", "createdAt": 1000, "updatedAt": 1000}]"#)

        let data = ImportExport.replace(with: imported)

        #expect(data == StorageData(
            version: StorageConstants.currentVersion,
            snippets: [
                Snippet(id: "legacy-1", title: "Legacy", content: "Content", description: "", tags: ["work"],
                        createdAt: 1000, updatedAt: 1000, useCount: 0),
            ],
            tags: [],
            placeholderHistory: [:]
        ))
    }

    @Test func mergePreservesExplicitFlagsAndNormalizesDuplicateTags() {
        let current = StorageData(snippets: [baseSnippet])
        var imported = baseSnippet
        imported.id = "snippet-2"
        imported.tags = ["Work", " work ", "Personal"]
        imported.isFavorite = true
        imported.isArchived = true
        imported.isPinned = true

        let data = ImportExport.merge(ImportPayload(snippets: [LegacySnippet(imported)]), into: current)

        #expect(data.snippets.count == 2)
        let merged = data.snippets[1]
        #expect(merged.id == "snippet-2")
        #expect(merged.tags == ["work", "personal"])
        #expect(merged.useCount == 0)
        #expect(merged.isFavorite && merged.isArchived && merged.isPinned)
    }

    @Test func mergingTheSameImportTwiceAddsNoDuplicates() throws {
        let current = StorageData(
            snippets: [baseSnippet],
            placeholderHistory: ["name": [PlaceholderHistoryValue(value: "Ada", useCount: 2, lastUsed: 5, createdAt: 1)]]
        )
        let imported = try payload(
            #"""
            [
              {"id": "snippet-1", "title": "Other title", "content": "Other", "createdAt": 1, "updatedAt": 1},
              {"id": "snippet-2", "title": "New", "content": "New", "tags": ["Work"], "createdAt": 2, "updatedAt": 2},
              {"id": "snippet-3", "title": "Newer", "content": "Newer", "createdAt": 3, "updatedAt": 3}
            ]
            """#,
            history: #"{"name": [{"value": "Ada", "useCount": 9, "lastUsed": 9, "createdAt": 9}, {"value": "Grace", "useCount": 1, "lastUsed": 3, "createdAt": 3}], "city": [{"value": "Oslo", "useCount": 1, "lastUsed": 1, "createdAt": 1}]}"#
        )

        let once = ImportExport.merge(imported, into: current)
        let twice = ImportExport.merge(imported, into: once)

        #expect(once.snippets.map(\.id) == ["snippet-1", "snippet-2", "snippet-3"])
        #expect(once.snippets[0] == baseSnippet)
        #expect(once.placeholderHistory["name"]?.map(\.value) == ["Ada", "Grace"])
        #expect(once.placeholderHistory["name"]?.first?.useCount == 2)
        #expect(once.placeholderHistory["city"]?.map(\.value) == ["Oslo"])
        #expect(twice == once)
    }

    @Test func mergeSkipsIdsRepeatedWithinOneImport() throws {
        let imported = try payload(#"[{"id": "dup", "title": "A", "content": "a", "createdAt": 1, "updatedAt": 1}, {"id": "dup", "title": "B", "content": "b", "createdAt": 1, "updatedAt": 1}]"#)
        let data = ImportExport.merge(imported, into: .empty)
        #expect(data.snippets.map(\.title) == ["A"])
    }

    @Test func exportThenReplaceImportRoundTrips() throws {
        let original = StorageData(
            snippets: [
                Snippet(id: "snippet-1700000000000-abc123xyz", title: "Greeting 👋", content: "Hi {{name}} — 你好",
                        description: "desc", tags: ["personal", "work/projects"], createdAt: 1_700_000_000_000,
                        updatedAt: 1_700_000_000_500, lastUsedAt: 1_700_000_000_400, useCount: 3,
                        isFavorite: true, isArchived: false, isPinned: true),
                Snippet(id: "snippet-2", title: "Never used", content: "x", createdAt: 1, updatedAt: 1),
            ],
            tags: [],
            placeholderHistory: ["name": [PlaceholderHistoryValue(value: "Ada", useCount: 1, lastUsed: 2, createdAt: 2)]]
        )

        let export = ImportExport.makeExport(original, now: 1_800_000_000_000)
        #expect(export.version == "7.0.0")
        #expect(export.exportedAt == 1_800_000_000_000)

        let json = try ImportExport.encodeExport(export)
        let restored = ImportExport.replace(with: try ImportExport.decodeImport(json))

        #expect(restored == original)
        #expect(try JSONDecoder().decode(ExportData.self, from: json) == export)
    }

    @Test func replaceResetsStoredTagList() {
        let payload = ImportPayload(ImportExport.makeExport(StorageData(tags: ["stale"]), now: 0))
        #expect(ImportExport.replace(with: payload).tags.isEmpty)
    }

    @Test func importRequiresSnippets() {
        #expect(throws: (any Error).self) { try ImportExport.decodeImport(Data(#"{"version": 7}"#.utf8)) }
    }

    @Test func mergedHistoryKeepsMostRecentPastLimit() {
        let limit = StorageConstants.maxStoredValuesPerKey
        let current: PlaceholderHistory = ["k": (0..<limit).map { PlaceholderHistoryValue(value: "old\($0)", useCount: 1, lastUsed: Int64($0), createdAt: 0) }]
        let imported: PlaceholderHistory = ["k": [PlaceholderHistoryValue(value: "new", useCount: 1, lastUsed: 10_000, createdAt: 0)]]

        let merged = PlaceholderHistoryMerge.merge(current, imported)["k"]!

        #expect(merged.count == limit)
        #expect(merged.first?.value == "new")
        #expect(!merged.contains { $0.value == "old0" })
    }

    @Test func exportFileNameUsesSafeUTCTimestamp() {
        #expect(ImportExport.exportFileName(now: 1_767_605_400_123) == "sloppy-paste-2026-01-05T09-30-00-123Z.json")
    }

    @Test func mergeSummaryCountsOnlyNewSnippets() throws {
        let imported = try payload(#"[{"id": "snippet-1", "title": "A", "content": "a", "createdAt": 1, "updatedAt": 1}, {"id": "snippet-2", "title": "B", "content": "b", "createdAt": 1, "updatedAt": 1}]"#)
        let current = StorageData(snippets: [baseSnippet])

        let merge = ImportSummary(imported, current: current, mode: .merge)
        #expect(merge.fileCount == 2)
        #expect(merge.importedCount == 1)
        #expect(merge.resultMessage == "Added 1 new snippet (1 already present)")

        let replace = ImportSummary(imported, current: current, mode: .replace)
        #expect(replace.importedCount == 2)
        #expect(replace.currentCount == 1)
        #expect(replace.resultMessage == "Imported 2 snippets")

        let again = ImportSummary(imported, current: ImportExport.merge(imported, into: current), mode: .merge)
        #expect(again.resultMessage == "Added 0 new snippets (2 already present)")
    }
}
