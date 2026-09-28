import Foundation

public enum PlistWriteError: Error, Equatable, Sendable, CustomStringConvertible {
    /// Binary plists and other non-XML files are never rewritten.
    case unsupportedFormat
    /// The XML has no top-level `<dict>` the scanner can follow.
    case malformed(String)
    /// The rewritten file did not parse back to the intended schedule, or changed another key.
    case verificationFailed(String)

    public var description: String {
        switch self {
        case .unsupportedFormat: "only XML plists can be edited"
        case .malformed(let reason): "cannot follow the plist's XML: \(reason)"
        case .verificationFailed(let reason): "the rewritten plist did not check out: \(reason)"
        }
    }
}

/// What to do with `ThrottleInterval`, which also paces KeepAlive restarts and so is never dropped implicitly.
public enum ThrottleUpdate: Equatable, Sendable {
    case keep
    case set(Int)
    case remove
}

/// Rewrites only the schedule keys of an XML LaunchAgent plist, keeping everything else byte for byte:
/// hand-written plists (4-space indents, custom key order, one-line calendar dicts) would get a whole-file diff
/// from `PropertyListSerialization`.
///
/// The top-level `StartInterval`, `StartCalendarInterval` and `WatchPaths` pairs are removed and the new block
/// goes where the first of them was, else before the closing `</dict>`, indented like the file's other keys.
/// `QueueDirectories` is left alone. The result is parsed back and must give the intended schedule with every
/// other key unchanged, or nothing is returned.
public enum PlistScheduleWriter {
    static let scheduleKeys: Set<String> = ["StartInterval", "StartCalendarInterval", "WatchPaths"]
    static let throttleKey = "ThrottleInterval"

    /// For `.watchPaths`, only the paths are written; `throttle` decides `ThrottleInterval`.
    public static func apply(_ schedule: Schedule, throttle: ThrottleUpdate = .keep, to data: Data) throws -> Data {
        guard !data.starts(with: Data("bplist".utf8)), let xml = String(data: data, encoding: .utf8) else {
            throw PlistWriteError.unsupportedFormat
        }
        let output = try apply(schedule, throttle: throttle, to: xml)
        let result = Data(output.utf8)
        try verify(original: data, result: result, schedule: schedule, throttle: throttle)
        return result
    }

    public static func apply(_ schedule: Schedule, throttle: ThrottleUpdate = .keep, to xml: String) throws -> String {
        let top = try TopLevelScanner(xml).scan()
        let newline = xml.contains("\r\n") ? "\r\n" : "\n"
        let base = top.entries.first { $0.key == "Label" }.map { indentation(of: $0.keyRange, in: xml) }
            ?? top.entries.first.map { indentation(of: $0.keyRange, in: xml) } ?? ""
        let unit = base.isEmpty ? "    " : base
        func lines(_ items: [(depth: Int, text: String)]) -> String {
            items.map { base + String(repeating: unit, count: $0.depth) + $0.text + newline }.joined()
        }

        var edits: [(range: Range<String.Index>, text: String)] = []
        let removed = top.entries.filter { scheduleKeys.contains($0.key) }
        for entry in removed {
            edits.append((lineRange(entry.keyRange.lowerBound..<entry.valueRange.upperBound, in: xml), ""))
        }

        let style = top.entries.first { $0.key == "StartCalendarInterval" }
            .map { CalendarStyle(existing: String(xml[$0.valueRange])) } ?? CalendarStyle()
        var block = lines(scheduleLines(schedule, calendarStyle: style))
        let throttleEntry = top.entries.last { $0.key == throttleKey }
        switch (throttle, throttleEntry) {
        case (.set(let seconds), let entry?):
            edits.append((entry.valueRange, "<integer>\(seconds)</integer>"))
        case (.set(let seconds), nil):
            block += lines([(0, "<key>\(throttleKey)</key>"), (0, "<integer>\(seconds)</integer>")])
        case (.remove, let entry?):
            edits.append((lineRange(entry.keyRange.lowerBound..<entry.valueRange.upperBound, in: xml), ""))
        case (.keep, _), (.remove, nil):
            break
        }

        if !block.isEmpty {
            if let first = removed.first {
                let range = lineRange(first.keyRange.lowerBound..<first.valueRange.upperBound, in: xml)
                let index = edits.firstIndex { $0.range == range }!
                edits[index].text = block
            } else {
                let close = top.closeRange.lowerBound
                let start = Self.lineStart(before: close, in: xml)
                if xml[start..<close].allSatisfy({ $0 == " " || $0 == "\t" }) {
                    edits.append((start..<start, block))
                } else {
                    edits.append((close..<close, newline + block))
                }
            }
        }

        var output = xml
        for edit in edits.sorted(by: { $0.range.lowerBound > $1.range.lowerBound }) {
            output.replaceSubrange(edit.range, with: edit.text)
        }
        return output
    }

    /// How the file already writes its calendar, so an edit keeps it: seneca-remind puts each entry on one line,
    /// log-rotate has a bare dict instead of an array.
    struct CalendarStyle: Equatable {
        var oneLineEntries = false
        var bareDict = false

        init(oneLineEntries: Bool = false, bareDict: Bool = false) {
            self.oneLineEntries = oneLineEntries
            self.bareDict = bareDict
        }

        init(existing value: String) {
            bareDict = value.hasPrefix("<dict")
            oneLineEntries = !bareDict && value.contains("<dict><key>")
        }
    }

    /// The new block's lines, relative to the top-level indentation.
    static func scheduleLines(_ schedule: Schedule, calendarStyle: CalendarStyle = CalendarStyle())
        -> [(depth: Int, text: String)] {
        switch schedule {
        case .interval(let seconds):
            return [(0, "<key>StartInterval</key>"), (0, "<integer>\(seconds)</integer>")]
        case .calendar(let entries):
            func fields(_ entry: CalendarEntry) -> [(key: String, value: Int)] {
                [("Month", entry.month), ("Day", entry.day), ("Weekday", entry.weekday), ("Hour", entry.hour),
                 ("Minute", entry.minute)].compactMap { key, value in value.map { (key, $0) } }
            }
            func pairs(_ entry: CalendarEntry, depth: Int) -> [(Int, String)] {
                fields(entry).flatMap { [(depth, "<key>\($0.key)</key>"), (depth, "<integer>\($0.value)</integer>")] }
            }
            var lines: [(Int, String)] = [(0, "<key>StartCalendarInterval</key>")]
            if calendarStyle.bareDict, entries.count == 1 {
                return lines + [(0, "<dict>")] + pairs(entries[0], depth: 1) + [(0, "</dict>")]
            }
            lines.append((0, "<array>"))
            for entry in entries {
                if calendarStyle.oneLineEntries {
                    let body = fields(entry).map { "<key>\($0.key)</key><integer>\($0.value)</integer>" }.joined()
                    lines.append((1, "<dict>\(body)</dict>"))
                } else {
                    lines.append((1, "<dict>"))
                    lines += pairs(entry, depth: 2)
                    lines.append((1, "</dict>"))
                }
            }
            lines.append((0, "</array>"))
            return lines
        case .watchPaths(let paths, _):
            return [(0, "<key>WatchPaths</key>"), (0, "<array>")]
                + paths.map { (1, "<string>\(escape($0))</string>") }
                + [(0, "</array>")]
        case .none:
            return []
        }
    }

    static func escape(_ text: String) -> String {
        text.replacingOccurrences(of: "&", with: "&amp;")
            .replacingOccurrences(of: "<", with: "&lt;")
            .replacingOccurrences(of: ">", with: "&gt;")
    }

    /// Start of the line holding `index` (Swift reads "\r\n" as one Character, so check both).
    static func lineStart(before index: String.Index, in xml: String) -> String.Index {
        xml[..<index].lastIndex { $0 == "\n" || $0 == "\r\n" }.map { xml.index(after: $0) } ?? xml.startIndex
    }

    /// Leading whitespace of the line `range` starts on, when only whitespace precedes it.
    static func indentation(of range: Range<String.Index>, in xml: String) -> String {
        let start = Self.lineStart(before: range.lowerBound, in: xml)
        let prefix = xml[start..<range.lowerBound]
        return prefix.allSatisfy({ $0 == " " || $0 == "\t" }) ? String(prefix) : ""
    }

    /// Widens `range` to whole lines when it sits alone on them, so removing it leaves no blank line.
    static func lineRange(_ range: Range<String.Index>, in xml: String) -> Range<String.Index> {
        let start = Self.lineStart(before: range.lowerBound, in: xml)
        guard xml[start..<range.lowerBound].allSatisfy({ $0 == " " || $0 == "\t" }) else { return range }
        var end = range.upperBound
        while end < xml.endIndex, xml[end] == " " || xml[end] == "\t" { end = xml.index(after: end) }
        guard end == xml.endIndex || xml[end] == "\n" || xml[end] == "\r\n" else { return range }
        if end < xml.endIndex { end = xml.index(after: end) }
        return start..<end
    }

    // MARK: Verification

    static func verify(original: Data, result: Data, schedule: Schedule, throttle: ThrottleUpdate) throws {
        func dictionary(_ data: Data) throws -> NSMutableDictionary {
            guard let dict = try? PropertyListSerialization.propertyList(from: data, format: nil) as? NSDictionary else {
                throw PlistWriteError.verificationFailed("not a dictionary plist")
            }
            return NSMutableDictionary(dictionary: dict)
        }
        let before = try dictionary(original), after = try dictionary(result)
        var managed = scheduleKeys
        if throttle != .keep { managed.insert(throttleKey) }
        before.removeObjects(forKeys: Array(managed))
        after.removeObjects(forKeys: Array(managed))
        guard before.isEqual(after) else { throw PlistWriteError.verificationFailed("another key changed") }

        let parsed: AgentDefinition
        do {
            parsed = try LaunchAgentPlistParser.parse(result, plistPath: "rewritten plist")
        } catch {
            throw PlistWriteError.verificationFailed("\(error)")
        }
        let expected: Schedule = switch schedule {
        case .interval, .calendar: schedule
        case .watchPaths(let paths, _): .watchPaths(paths + parsed.queueDirectories, throttleSeconds: parsed.throttleInterval)
        case .none: parsed.queueDirectories.isEmpty
            ? .none : .watchPaths(parsed.queueDirectories, throttleSeconds: parsed.throttleInterval)
        }
        guard parsed.schedule == expected else {
            throw PlistWriteError.verificationFailed("schedule reads back as \(ScheduleDescription.describe(parsed.schedule))")
        }
        let throttleApplied = switch throttle {
        case .keep: true
        case .set(let seconds): parsed.throttleInterval == seconds
        case .remove: parsed.throttleInterval == nil
        }
        guard throttleApplied else { throw PlistWriteError.verificationFailed("ThrottleInterval did not change") }
    }
}

/// Finds the top-level `<key>`/value pairs of a plist's root `<dict>` by tag, tracking nesting so keys inside
/// `EnvironmentVariables` or calendar entries are never mistaken for top-level ones.
struct TopLevelScanner {
    struct Entry {
        var key: String
        /// From `<key>` to `</key>`.
        var keyRange: Range<String.Index>
        /// The whole value element, e.g. `<array>…</array>` or `<true/>`.
        var valueRange: Range<String.Index>
    }

    struct Result {
        var entries: [Entry]
        /// The root dict's `</dict>`.
        var closeRange: Range<String.Index>
    }

    struct Tag {
        enum Kind { case open, close, empty }
        var kind: Kind
        var name: String
        var range: Range<String.Index>
    }

    let xml: String
    init(_ xml: String) { self.xml = xml }

    func scan() throws -> Result {
        var position = xml.startIndex
        // Skip to the root <dict>.
        while true {
            guard let tag = try nextTag(from: position) else { throw PlistWriteError.malformed("no root <dict>") }
            position = tag.range.upperBound
            if tag.name == "dict" {
                guard tag.kind == .open else { throw PlistWriteError.malformed("empty root <dict/>") }
                break
            }
        }

        var entries: [Entry] = []
        while true {
            guard let tag = try nextTag(from: position) else { throw PlistWriteError.malformed("unclosed root <dict>") }
            if tag.kind == .close, tag.name == "dict" {
                return Result(entries: entries, closeRange: tag.range)
            }
            guard tag.kind == .open, tag.name == "key" else {
                throw PlistWriteError.malformed("expected <key>, found <\(tag.name)>")
            }
            guard let keyClose = try nextTag(from: tag.range.upperBound), keyClose.kind == .close, keyClose.name == "key" else {
                throw PlistWriteError.malformed("unclosed <key>")
            }
            let key = String(xml[tag.range.upperBound..<keyClose.range.lowerBound])
            guard let value = try nextTag(from: keyClose.range.upperBound), value.kind != .close else {
                throw PlistWriteError.malformed("no value for \(key)")
            }
            let valueEnd = value.kind == .empty ? value.range.upperBound : try closing(value).upperBound
            entries.append(Entry(key: key, keyRange: tag.range.lowerBound..<keyClose.range.upperBound,
                                 valueRange: value.range.lowerBound..<valueEnd))
            position = valueEnd
        }
    }

    /// The range of the tag closing `open`, counting nested tags of the same name.
    func closing(_ open: Tag) throws -> Range<String.Index> {
        var depth = 1, position = open.range.upperBound
        while let tag = try nextTag(from: position) {
            position = tag.range.upperBound
            guard tag.name == open.name else { continue }
            switch tag.kind {
            case .open: depth += 1
            case .close: depth -= 1
            case .empty: break
            }
            if depth == 0 { return tag.range }
        }
        throw PlistWriteError.malformed("unclosed <\(open.name)>")
    }

    /// The next element tag at or after `position`, skipping comments, `<?…?>` and `<!…>`.
    func nextTag(from position: String.Index) throws -> Tag? {
        var position = position
        while let start = xml[position...].firstIndex(of: "<") {
            let rest = xml[start...]
            if rest.hasPrefix("<!--") {
                guard let end = rest.range(of: "-->") else { throw PlistWriteError.malformed("unclosed comment") }
                position = end.upperBound
                continue
            }
            guard let close = rest.firstIndex(of: ">") else { throw PlistWriteError.malformed("unclosed tag") }
            let end = xml.index(after: close)
            if rest.hasPrefix("<?") || rest.hasPrefix("<!") {
                position = end
                continue
            }
            var inner = xml[xml.index(after: start)..<close]
            var kind = Tag.Kind.open
            if inner.hasPrefix("/") {
                kind = .close
                inner = inner.dropFirst()
            } else if inner.hasSuffix("/") {
                kind = .empty
                inner = inner.dropLast()
            }
            let name = inner.prefix { !$0.isWhitespace }
            return Tag(kind: kind, name: String(name), range: start..<end)
        }
        return nil
    }
}
