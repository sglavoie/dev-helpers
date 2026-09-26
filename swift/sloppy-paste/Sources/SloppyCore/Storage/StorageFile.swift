import Foundation

/// The JSON data file: atomic pretty-printed writes, a `.bak` copy before a
/// replace-import, and quarantine instead of overwrite when it fails to decode.
public struct StorageFile: Sendable, Hashable {
    public var url: URL

    public init(url: URL = StorageFile.defaultURL) {
        self.url = url
    }

    /// `~/Library/Application Support/SloppyPaste/data.json`
    public static var defaultURL: URL {
        URL.applicationSupportDirectory
            .appending(path: "SloppyPaste", directoryHint: .isDirectory)
            .appending(path: "data.json", directoryHint: .notDirectory)
    }

    public var backupURL: URL { url.appendingPathExtension("bak") }

    public struct LoadResult: Sendable, Hashable {
        public var data: StorageData
        /// The file was migrated to the current version and saved back.
        public var didMigrate: Bool
        /// Set when the file failed to decode and was moved aside; `data` is then empty.
        public var quarantinedURL: URL?
    }

    /// Loads and migrates the file, saving it back if it migrated. A missing
    /// file gives empty data; an undecodable one is renamed to
    /// `data.corrupt-<now>.json` and also gives empty data.
    public func load(now: Int64) throws -> LoadResult {
        let contents: Data
        do {
            contents = try Data(contentsOf: url)
        } catch CocoaError.fileReadNoSuchFile {
            return LoadResult(data: .empty, didMigrate: false, quarantinedURL: nil)
        }

        let result: StorageMigrations.Result
        do {
            result = try StorageMigrations.migrate(json: contents)
        } catch {
            let quarantined = quarantineURL(now: now)
            try FileManager.default.moveItem(at: url, to: quarantined)
            return LoadResult(data: .empty, didMigrate: false, quarantinedURL: quarantined)
        }

        if result.didMigrate {
            try save(result.data)
        }
        return LoadResult(data: result.data, didMigrate: result.didMigrate, quarantinedURL: nil)
    }

    /// `data.corrupt-<now>.json`, or `data.corrupt-<now>-<n>.json` when an
    /// earlier quarantine already took that name.
    func quarantineURL(now: Int64) -> URL {
        let directory = url.deletingLastPathComponent()
        let stem = "\(url.deletingPathExtension().lastPathComponent).corrupt-\(now)"
        var candidate = directory.appending(path: "\(stem).json")
        var suffix = 1
        while FileManager.default.fileExists(atPath: candidate.path) {
            candidate = directory.appending(path: "\(stem)-\(suffix).json")
            suffix += 1
        }
        return candidate
    }

    /// Writes the data atomically, creating the directory if needed.
    public func save(_ data: StorageData) throws {
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try StorageCoding.encode(data).write(to: url, options: .atomic)
    }

    /// Copies the current file to `backupURL`, replacing an older backup.
    /// Returns nil when there is no file to back up.
    @discardableResult
    public func writeBackup() throws -> URL? {
        guard FileManager.default.fileExists(atPath: url.path) else { return nil }
        try Data(contentsOf: url).write(to: backupURL, options: .atomic)
        return backupURL
    }

    /// Applies an import and saves it. A replace-import backs up the file first.
    public func importPayload(_ payload: ImportPayload, into current: StorageData, mode: ImportMode) throws -> StorageData {
        if mode == .replace {
            try writeBackup()
        }
        let next = ImportExport.apply(payload, to: current, mode: mode)
        try save(next)
        return next
    }

    /// Used to reload after external edits.
    public func modificationDate() -> Date? {
        try? FileManager.default.attributesOfItem(atPath: url.path)[.modificationDate] as? Date
    }

    /// Size of the data file in bytes, or 0 if it does not exist.
    public func size() -> Int {
        (try? FileManager.default.attributesOfItem(atPath: url.path)[.size] as? Int) ?? 0
    }
}
