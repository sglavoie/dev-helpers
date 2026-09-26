import Foundation

/// In-place edits of `StorageData.placeholderHistory`.
extension StorageData {
    public func placeholderHistory(forKey key: String) -> [PlaceholderHistoryValue] {
        placeholderHistory[key] ?? []
    }

    public var placeholderKeys: [String] {
        placeholderHistory.keys.sorted(by: TagNormalization.localeLess)
    }

    /// Records one use of `value` for `key`, evicting the least recently used
    /// value past `StorageConstants.maxStoredValuesPerKey`.
    /// Returns false (and changes nothing) for an empty key or blank value.
    @discardableResult
    public mutating func addPlaceholderValue(key: String, value: String, now: Int64) -> Bool {
        guard !key.isEmpty, !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return false }

        var values = placeholderHistory[key] ?? []
        if let index = values.firstIndex(where: { $0.value == value }) {
            values[index].useCount += 1
            values[index].lastUsed = now
        } else {
            values.append(PlaceholderHistoryValue(value: value, useCount: 1, lastUsed: now, createdAt: now))
            if values.count > StorageConstants.maxStoredValuesPerKey {
                values.sort { $0.lastUsed < $1.lastUsed }
                values.removeFirst()
            }
        }
        placeholderHistory[key] = values
        return true
    }

    /// Bumps usage of an existing value. Returns false if the key or value is unknown.
    @discardableResult
    public mutating func updatePlaceholderValueUsage(key: String, value: String, now: Int64) -> Bool {
        guard let index = placeholderHistory[key]?.firstIndex(where: { $0.value == value }) else { return false }
        placeholderHistory[key]![index].useCount += 1
        placeholderHistory[key]![index].lastUsed = now
        return true
    }

    /// Removes a value, and the key once it has no values left. Returns false if the key is unknown.
    @discardableResult
    public mutating func deletePlaceholderValue(key: String, value: String) -> Bool {
        guard var values = placeholderHistory[key] else { return false }
        values.removeAll { $0.value == value }
        placeholderHistory[key] = values.isEmpty ? nil : values
        return true
    }

    /// Renames a stored value in place.
    public mutating func updatePlaceholderValue(key: String, oldValue: String, newValue: String) throws {
        guard !newValue.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw StorageError.emptyPlaceholderValue
        }
        guard let values = placeholderHistory[key] else { throw StorageError.placeholderKeyNotFound }
        guard let index = values.firstIndex(where: { $0.value == oldValue }) else {
            throw StorageError.placeholderValueNotFound
        }
        if let duplicate = values.firstIndex(where: { $0.value == newValue }), duplicate != index {
            throw StorageError.duplicatePlaceholderValue
        }
        placeholderHistory[key]![index].value = newValue
    }

    public mutating func clearPlaceholderHistory(forKey key: String) {
        placeholderHistory[key] = nil
    }

    public mutating func clearAllPlaceholderHistory() {
        placeholderHistory = [:]
    }
}

public enum PlaceholderHistoryMerge {
    /// Adds imported values whose text is not already stored for the key,
    /// keeping the most recently used values past the per-key limit.
    public static func merge(_ current: PlaceholderHistory, _ imported: PlaceholderHistory) -> PlaceholderHistory {
        var merged = current
        for (key, values) in imported {
            guard let existing = merged[key] else {
                merged[key] = values
                continue
            }
            let existingValues = Set(existing.map(\.value))
            var combined = existing + values.filter { !existingValues.contains($0.value) }
            if combined.count > StorageConstants.maxStoredValuesPerKey {
                combined.sort { $0.lastUsed > $1.lastUsed }
                combined = Array(combined.prefix(StorageConstants.maxStoredValuesPerKey))
            }
            merged[key] = combined
        }
        return merged
    }
}
