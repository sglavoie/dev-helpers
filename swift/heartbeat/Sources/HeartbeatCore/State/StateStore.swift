import Foundation

/// `state.json`: the app writes it atomically after each poll, the CLI only reads it.
public struct StateStore: Sendable, Hashable {
    public var url: URL

    public init(url: URL = StateStore.defaultURL) {
        self.url = url
    }

    /// `~/Library/Application Support/Heartbeat/state.json`
    public static var defaultURL: URL {
        URL.applicationSupportDirectory
            .appending(path: "Heartbeat", directoryHint: .isDirectory)
            .appending(path: "state.json", directoryHint: .notDirectory)
    }

    public struct LoadResult: Sendable, Equatable {
        public var state: HeartbeatState
        /// Set when the file failed to decode and was moved aside; `state` is then empty.
        public var quarantinedURL: URL?
    }

    /// A missing file is empty state. An undecodable one is renamed to `state.corrupt-<now>.json` and also gives
    /// empty state: losing history only costs a few quiet polls, while refusing to start costs monitoring.
    public func load(now: Date = Date()) throws -> LoadResult {
        let data: Data
        do {
            data = try Data(contentsOf: url)
        } catch CocoaError.fileReadNoSuchFile {
            return LoadResult(state: .empty)
        }
        do {
            return LoadResult(state: try Self.decoder.decode(HeartbeatState.self, from: data))
        } catch {
            let quarantined = quarantineURL(now: now)
            try FileManager.default.moveItem(at: url, to: quarantined)
            return LoadResult(state: .empty, quarantinedURL: quarantined)
        }
    }

    /// Reads without side effects (for `heartbeatctl`): `nil` when missing or undecodable.
    public func read() -> HeartbeatState? {
        guard let data = try? Data(contentsOf: url) else { return nil }
        return try? Self.decoder.decode(HeartbeatState.self, from: data)
    }

    /// Writes atomically, creating the directory if needed.
    public func save(_ state: HeartbeatState) throws {
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Self.encoder.encode(state).write(to: url, options: .atomic)
    }

    func quarantineURL(now: Date) -> URL {
        let directory = url.deletingLastPathComponent()
        let stem = "\(url.deletingPathExtension().lastPathComponent).corrupt-\(Int(now.timeIntervalSince1970))"
        var candidate = directory.appending(path: "\(stem).json")
        var suffix = 1
        while FileManager.default.fileExists(atPath: candidate.path) {
            candidate = directory.appending(path: "\(stem)-\(suffix).json")
            suffix += 1
        }
        return candidate
    }

    static var encoder: JSONEncoder {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        encoder.dateEncodingStrategy = .secondsSince1970
        return encoder
    }

    static var decoder: JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .secondsSince1970
        return decoder
    }
}
