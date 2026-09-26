import Foundation

/// An import file as decoded leniently: snippets may be legacy-shaped and
/// only `snippets` and `placeholderHistory` are read.
public struct ImportPayload: Decodable, Sendable, Hashable {
    public var snippets: [LegacySnippet]
    public var placeholderHistory: PlaceholderHistory?

    public init(snippets: [LegacySnippet], placeholderHistory: PlaceholderHistory? = nil) {
        self.snippets = snippets
        self.placeholderHistory = placeholderHistory
    }

    public init(_ export: ExportData) {
        self.init(snippets: export.snippets.map(LegacySnippet.init), placeholderHistory: export.placeholderHistory)
    }

    private enum CodingKeys: String, CodingKey {
        case snippets, placeholderHistory
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        snippets = try c.decode([LegacySnippet].self, forKey: .snippets)
        placeholderHistory = try c.decodeIfPresent(PlaceholderHistory.self, forKey: .placeholderHistory)
    }
}

public enum ImportMode: String, Sendable, CaseIterable {
    /// Adds snippets whose id is not already stored.
    case merge
    /// Replaces all data with the import.
    case replace
}

public enum ImportExport {
    public static func makeExport(_ data: StorageData, now: Int64) -> ExportData {
        ExportData(
            version: "\(StorageConstants.currentVersion).0.0",
            exportedAt: now,
            snippets: data.snippets,
            tags: data.tags,
            placeholderHistory: data.placeholderHistory
        )
    }

    public static func encodeExport(_ export: ExportData) throws -> Data {
        try StorageCoding.encode(export)
    }

    public static func decodeImport(_ json: Data) throws -> ImportPayload {
        try JSONDecoder().decode(ImportPayload.self, from: json)
    }

    public static func apply(_ payload: ImportPayload, to current: StorageData, mode: ImportMode) -> StorageData {
        switch mode {
        case .merge: merge(payload, into: current)
        case .replace: replace(with: payload)
        }
    }

    /// Adds imported snippets whose id is not already stored (so importing the
    /// same file twice adds nothing) and merges placeholder history.
    public static func merge(_ payload: ImportPayload, into current: StorageData) -> StorageData {
        var ids = Set(current.snippets.map(\.id))
        let newSnippets = payload.snippets
            .filter { ids.insert($0.id).inserted }
            .map { $0.normalized() }

        var data = current
        data.snippets += newSnippets
        if let imported = payload.placeholderHistory {
            data.placeholderHistory = PlaceholderHistoryMerge.merge(current.placeholderHistory, imported)
        }
        return data
    }

    /// Replaces everything with the import; the stored `tags` list is reset.
    public static func replace(with payload: ImportPayload) -> StorageData {
        StorageData(
            version: StorageConstants.currentVersion,
            snippets: payload.snippets.map { $0.normalized() },
            tags: [],
            placeholderHistory: payload.placeholderHistory ?? [:]
        )
    }
}

/// What an import will do, for the confirmation and the result message.
public struct ImportSummary: Sendable, Hashable {
    public var mode: ImportMode
    /// Snippets in the file.
    public var fileCount: Int
    /// Snippets that end up added (merge) or stored (replace).
    public var importedCount: Int
    /// Snippets currently stored.
    public var currentCount: Int

    public init(_ payload: ImportPayload, current: StorageData, mode: ImportMode) {
        self.mode = mode
        fileCount = payload.snippets.count
        currentCount = current.snippets.count
        importedCount = ImportExport.apply(payload, to: current, mode: mode).snippets.count
            - (mode == .merge ? current.snippets.count : 0)
    }

    /// The success message, e.g. "Imported 3 snippets" or "Added 2 new snippets (1 already present)".
    public var resultMessage: String {
        switch mode {
        case .replace:
            return "Imported \(Self.snippets(importedCount))"
        case .merge:
            let skipped = fileCount - importedCount
            let added = "Added \(importedCount) new \(importedCount == 1 ? "snippet" : "snippets")"
            return skipped == 0 ? added : "\(added) (\(skipped) already present)"
        }
    }

    public static func snippets(_ count: Int) -> String {
        "\(count) \(count == 1 ? "snippet" : "snippets")"
    }
}

extension ImportExport {
    /// `sloppy-paste-2026-01-05T09-30-00-000Z.json`, the extension's ISO
    /// timestamp with `:` and `.` replaced so it is a safe file name.
    public static func exportFileName(now: Int64) -> String {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        formatter.timeZone = TimeZone(identifier: "UTC")
        let timestamp = formatter.string(from: Date(epochMilliseconds: now))
            .replacingOccurrences(of: ":", with: "-")
            .replacingOccurrences(of: ".", with: "-")
        return "sloppy-paste-\(timestamp).json"
    }
}

/// Shared JSON encoding: pretty-printed with sorted keys so files diff cleanly.
public enum StorageCoding {
    public static func encode(_ value: some Encodable) throws -> Data {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        return try encoder.encode(value)
    }
}
