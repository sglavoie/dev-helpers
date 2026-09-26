import Foundation
import Testing
@testable import SloppyCore

@Suite struct StorageFileTests {
    let directory: URL
    let file: StorageFile

    init() throws {
        directory = FileManager.default.temporaryDirectory
            .appending(path: "SloppyCoreTests-\(UUID().uuidString)", directoryHint: .isDirectory)
        file = StorageFile(url: directory.appending(path: "nested/data.json"))
    }

    func contents() throws -> String {
        String(decoding: try Data(contentsOf: file.url), as: UTF8.self)
    }

    @Test func defaultLocationIsApplicationSupport() {
        #expect(StorageFile.defaultURL.path.hasSuffix("Library/Application Support/SloppyPaste/data.json"))
    }

    @Test func missingFileLoadsEmpty() throws {
        let result = try file.load(now: 1)
        #expect(result.data == .empty)
        #expect(!result.didMigrate)
        #expect(result.quarantinedURL == nil)
        #expect(!FileManager.default.fileExists(atPath: file.url.path))
    }

    @Test func saveWritesPrettyPrintedJSONAndLoadsBack() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let data = StorageData(snippets: [Snippet(id: "a", title: "A", content: "x/y", createdAt: 1, updatedAt: 1)])

        try file.save(data)

        #expect(try contents().contains("\n  \"placeholderHistory\""))
        #expect(try contents().contains("\"x/y\""))
        #expect(try file.load(now: 2).data == data)
        #expect(file.modificationDate() != nil)
        #expect(file.size() > 0)
    }

    @Test func legacyFileIsMigratedAndSavedBack() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try FileManager.default.createDirectory(at: file.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        let legacy = #"{"version": 1, "snippets": [{"id": "a", "title": "A", "content": "x", "category": "Work", "createdAt": 1, "updatedAt": 1}], "tags": []}"#
        try Data(legacy.utf8).write(to: file.url)

        let result = try file.load(now: 2)

        #expect(result.didMigrate)
        #expect(result.data.snippets.first?.tags == ["work"])
        #expect(try contents().contains("\"version\" : 7"))
        #expect(try !contents().contains("category"))
    }

    @Test func corruptFileIsQuarantinedNotOverwritten() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try FileManager.default.createDirectory(at: file.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{broken".utf8).write(to: file.url)

        let result = try file.load(now: 1234)

        let quarantined = try #require(result.quarantinedURL)
        #expect(quarantined.lastPathComponent == "data.corrupt-1234.json")
        #expect(String(decoding: try Data(contentsOf: quarantined), as: UTF8.self) == "{broken")
        #expect(!FileManager.default.fileExists(atPath: file.url.path))
        #expect(result.data == .empty)
    }

    @Test func secondQuarantineInTheSameMillisecondKeepsTheFirst() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try FileManager.default.createDirectory(at: file.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{first".utf8).write(to: file.url)
        let first = try #require(try file.load(now: 1234).quarantinedURL)
        try Data("{second".utf8).write(to: file.url)
        let second = try #require(try file.load(now: 1234).quarantinedURL)

        #expect(second.lastPathComponent == "data.corrupt-1234-1.json")
        #expect(String(decoding: try Data(contentsOf: first), as: UTF8.self) == "{first")
        #expect(String(decoding: try Data(contentsOf: second), as: UTF8.self) == "{second")
    }

    @Test func emptyFileIsQuarantined() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try FileManager.default.createDirectory(at: file.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data().write(to: file.url)

        let result = try file.load(now: 5)

        #expect(result.quarantinedURL?.lastPathComponent == "data.corrupt-5.json")
        #expect(result.data == .empty)
    }

    @Test func modificationDateChangesWhenTheFileIsReplaced() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        #expect(file.modificationDate() == nil)
        try file.save(.empty)
        let first = try #require(file.modificationDate())
        try FileManager.default.setAttributes(
            [.modificationDate: first.addingTimeInterval(-60)], ofItemAtPath: file.url.path)
        #expect(file.modificationDate() != first)
    }

    @Test func replaceImportWritesBackupFirst() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        let current = StorageData(snippets: [Snippet(id: "old", title: "Old", content: "x", createdAt: 1, updatedAt: 1)])
        try file.save(current)
        let before = try contents()
        let payload = ImportPayload(snippets: [LegacySnippet(id: "new", title: "New", content: "y", createdAt: 2, updatedAt: 2)])

        let next = try file.importPayload(payload, into: current, mode: .replace)

        #expect(next.snippets.map(\.id) == ["new"])
        #expect(String(decoding: try Data(contentsOf: file.backupURL), as: UTF8.self) == before)
        #expect(file.backupURL.lastPathComponent == "data.json.bak")
        #expect(try file.load(now: 3).data == next)
    }

    @Test func mergeImportDoesNotWriteBackup() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try file.save(.empty)
        let payload = ImportPayload(snippets: [LegacySnippet(id: "new", title: "New", content: "y", createdAt: 2, updatedAt: 2)])

        let next = try file.importPayload(payload, into: .empty, mode: .merge)

        #expect(next.snippets.count == 1)
        #expect(!FileManager.default.fileExists(atPath: file.backupURL.path))
    }
}
