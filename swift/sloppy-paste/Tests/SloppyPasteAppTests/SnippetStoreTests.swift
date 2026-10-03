import Foundation
import Testing
import SloppyCore
@testable import SloppyPasteApp

@Suite @MainActor struct SnippetStoreTests {
    let directory = FileManager.default.temporaryDirectory.appending(path: "SloppyStoreTests-\(UUID().uuidString)")

    @Test func failedReloadBlocksMutationsAndImportsUntilAReadSucceeds() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let file = StorageFile(url: directory.appending(path: "data.json"))
        let original = StorageData(snippets: [Snippet(id: "a", title: "A", content: "Original", createdAt: 1, updatedAt: 1)])
        try file.save(original)
        let store = SnippetStore(file: file, now: { 10 })
        store.load()
        let modified = try #require(file.modificationDate())

        // A directory at the file path reliably causes a read error on macOS.
        try FileManager.default.removeItem(at: file.url)
        try FileManager.default.createDirectory(at: file.url, withIntermediateDirectories: false)
        store.load()
        #expect(store.lastError != nil)
        #expect(store.data == original)
        try FileManager.default.setAttributes([.modificationDate: modified], ofItemAtPath: file.url.path)

        var ranMutation = false
        #expect(throws: (any Error).self) {
            try store.mutate { repository, _ in
                ranMutation = true
                repository.deleteSnippet(id: "a")
            }
        }
        #expect(!ranMutation)
        let payload = ImportPayload(snippets: [])
        #expect(throws: (any Error).self) { try store.importPayload(payload, mode: .replace) }
        #expect(!FileManager.default.fileExists(atPath: file.backupURL.path))
        #expect(store.data == original)

        try FileManager.default.removeItem(at: file.url)
        var external = original
        external.snippets[0].content = "External edit"
        try file.save(external)
        // A previous read error must trigger a retry even with the old timestamp.
        try FileManager.default.setAttributes([.modificationDate: modified], ofItemAtPath: file.url.path)
        try store.recordUse(id: "a")
        #expect(store.lastError == nil)
        #expect(store.snippets[0].content == "External edit")
        #expect(store.snippets[0].useCount == 1)
        #expect(try file.read() == store.data)
    }

    @Test func mutationLoadsExistingDataEvenBeforeInitialLoad() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let file = StorageFile(url: directory.appending(path: "data.json"))
        try file.save(StorageData(snippets: [Snippet(id: "a", title: "A", content: "Keep", createdAt: 1, updatedAt: 1)]))
        let store = SnippetStore(file: file, now: { 10 })
        try store.mutate { repository, now in
            repository.addSnippet(title: "B", content: "New", now: now, id: "b")
        }
        #expect(store.snippets.map(\.id) == ["a", "b"])
    }
}
