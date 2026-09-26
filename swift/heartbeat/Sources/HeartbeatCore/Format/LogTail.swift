import Foundation

/// The last lines of a log file, read backwards from the end so a large log costs only the bytes shown.
public struct LogTail: Equatable, Sendable {
    public static let defaultLineCount = 200
    /// Never reads more than this from the end, however long the lines are.
    public static let defaultMaxBytes = 512 * 1024
    static let chunkSize = 64 * 1024

    /// Oldest first, without line terminators (`\n` or `\r\n`).
    public var lines: [String]
    /// Earlier content exists that isn't in `lines`.
    public var truncated: Bool
    public var fileSize: UInt64
    public var modified: Date?

    public init(lines: [String], truncated: Bool, fileSize: UInt64, modified: Date? = nil) {
        self.lines = lines
        self.truncated = truncated
        self.fileSize = fileSize
        self.modified = modified
    }

    public var text: String { lines.joined(separator: "\n") }

    /// Reads the last `lineCount` lines of the file at `path`.
    public static func read(path: String, lineCount: Int = defaultLineCount, maxBytes: Int = defaultMaxBytes) throws -> LogTail {
        let attributes: [FileAttributeKey: Any]
        do {
            attributes = try FileManager.default.attributesOfItem(atPath: path)
        } catch {
            throw LogTailError.notFound(path: path)
        }
        guard attributes[.type] as? FileAttributeType != .typeDirectory else {
            throw LogTailError.unreadable(path: path, reason: "is a directory")
        }
        guard let handle = FileHandle(forReadingAtPath: path) else {
            throw LogTailError.unreadable(path: path, reason: "permission denied")
        }
        defer { try? handle.close() }

        do {
            let size = try handle.seekToEnd()
            let limit = UInt64(max(1, maxBytes))
            var start = size
            var data = Data()
            // Read whole chunks backwards until they hold enough line breaks (one extra: the first line
            // read is usually partial) or the byte limit or the file start is reached.
            while start > 0, size - start < limit, newlineCount(data) <= lineCount {
                let length = min(UInt64(chunkSize), start, limit - (size - start))
                start -= length
                try handle.seek(toOffset: start)
                data = (try handle.read(upToCount: Int(length)) ?? Data()) + data
            }
            var result = tail(data, lineCount: lineCount, startsMidFile: start > 0)
            result.fileSize = size
            result.modified = attributes[.modificationDate] as? Date
            return result
        } catch {
            throw LogTailError.unreadable(path: path, reason: error.localizedDescription)
        }
    }

    /// The last `lineCount` lines of `data`. With `startsMidFile` the first line is dropped as possibly partial
    /// (unless it is the only one). Invalid UTF-8 is replaced rather than rejected.
    public static func tail(_ data: Data, lineCount: Int, startsMidFile: Bool = false) -> LogTail {
        // Split bytes, not Characters: Swift treats "\r\n" as one Character that "\n" wouldn't match.
        var lines = data.split(separator: UInt8(ascii: "\n"), omittingEmptySubsequences: false).map { line in
            String(decoding: line.last == UInt8(ascii: "\r") ? line.dropLast() : line, as: UTF8.self)
        }
        // A final newline terminates the last line rather than starting an empty one.
        if lines.last == "" { lines.removeLast() }
        if startsMidFile, lines.count > 1 { lines.removeFirst() }
        let count = max(0, lineCount)
        let truncated = startsMidFile || lines.count > count
        if lines.count > count { lines.removeFirst(lines.count - count) }
        return LogTail(lines: lines, truncated: truncated, fileSize: UInt64(data.count))
    }

    /// The argv that opens `path` with the configured `openLogCommand`: every `{path}` is replaced and `~` is
    /// expanded in the first element; without any `{path}` the path is appended. `nil` when no command is set.
    public static func openCommand(_ template: [String]?, path: String,
                                   home: String = FileManager.default.homeDirectoryForCurrentUser.path) -> [String]? {
        guard let template, let first = template.first, !first.isEmpty else { return nil }
        var argv = template.map { $0.replacingOccurrences(of: "{path}", with: path) }
        if first == "~" || first.hasPrefix("~/") {
            argv[0] = home + first.dropFirst()
        }
        if !template.contains(where: { $0.contains("{path}") }) { argv.append(path) }
        return argv
    }

    static func newlineCount(_ data: Data) -> Int {
        data.reduce(0) { $1 == UInt8(ascii: "\n") ? $0 + 1 : $0 }
    }
}

public enum LogTailError: Error, Equatable, Sendable, CustomStringConvertible {
    case notFound(path: String)
    case unreadable(path: String, reason: String)

    public var description: String {
        switch self {
        case .notFound(let path): "\(path) does not exist yet"
        case .unreadable(let path, let reason): "Cannot read \(path): \(reason)"
        }
    }
}
