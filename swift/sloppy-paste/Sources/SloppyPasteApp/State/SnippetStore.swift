import Foundation
import Observation
import SloppyCore

/// The app's single source of truth for snippet data, backed by the JSON file.
/// Mutations go through `SnippetRepository` and are saved immediately.
@MainActor
@Observable
final class SnippetStore {
    private(set) var data: StorageData = .empty
    /// Set when the last load moved an undecodable file aside.
    private(set) var quarantinedURL: URL?
    /// The last load or save error, for display.
    private(set) var lastError: String?

    @ObservationIgnored let file: StorageFile
    @ObservationIgnored var now: () -> Int64
    @ObservationIgnored private var loadedModificationDate: Date?
    @ObservationIgnored private var watcher: FileWatcher?
    /// Called when a load (at launch or after an external edit) moved an
    /// undecodable file aside.
    @ObservationIgnored var onQuarantine: ((URL) -> Void)?

    init(file: StorageFile = StorageFile(), now: @escaping () -> Int64 = { Date().epochMilliseconds }) {
        self.file = file
        self.now = now
    }

    var snippets: [Snippet] { data.snippets }

    /// Loads the file from disk, migrating and quarantining as needed.
    func load() {
        do {
            let result = try file.load(now: now())
            data = result.data
            if let quarantined = result.quarantinedURL {
                quarantinedURL = quarantined
                onQuarantine?(quarantined)
            }
            lastError = nil
        } catch {
            lastError = error.localizedDescription
        }
        loadedModificationDate = currentModificationDate()
    }

    /// Reloads as soon as another process (an editor, `sloppyctl`, a sync
    /// tool) changes the file, not only when the picker next opens.
    func startWatching() {
        guard watcher == nil else { return }
        watcher = FileWatcher(url: file.url) { [weak self] in
            self?.reloadIfChanged()
        }
    }

    /// Reloads when the file changed on disk since the last load or save.
    func reloadIfChanged() {
        guard currentModificationDate() != loadedModificationDate else { return }
        load()
    }

    /// Applies a repository mutation and saves the result.
    @discardableResult
    func mutate<T>(_ body: (inout SnippetRepository, Int64) throws -> T) throws -> T {
        var repository = SnippetRepository(data: data)
        let result = try body(&repository, now())
        try save(repository.data)
        return result
    }

    /// Bumps useCount and lastUsedAt, and saves the placeholder values to history.
    func recordUse(id: String, placeholderValues: [PlaceholderValueToRecord] = []) throws {
        reloadIfChanged()
        try mutate { repository, now in
            try repository.recordUse(id: id, placeholderValues: placeholderValues, now: now)
        }
    }

    /// Replaces the data wholesale (imports) and saves it.
    func save(_ next: StorageData) throws {
        do {
            try file.save(next)
            data = next
            lastError = nil
            loadedModificationDate = currentModificationDate()
        } catch {
            lastError = error.localizedDescription
            throw error
        }
    }

    /// Imports into the current data (reloaded first); a replace backs the
    /// file up to `data.json.bak` before writing.
    func importPayload(_ payload: ImportPayload, mode: ImportMode) throws {
        reloadIfChanged()
        do {
            data = try file.importPayload(payload, into: data, mode: mode)
            lastError = nil
            loadedModificationDate = currentModificationDate()
        } catch {
            lastError = error.localizedDescription
            throw error
        }
    }

    func clearQuarantineNotice() {
        quarantinedURL = nil
    }

    private func currentModificationDate() -> Date? {
        // FileManager rather than URL resource values, which NSURL caches.
        (try? FileManager.default.attributesOfItem(atPath: file.url.path))?[.modificationDate] as? Date
    }
}
