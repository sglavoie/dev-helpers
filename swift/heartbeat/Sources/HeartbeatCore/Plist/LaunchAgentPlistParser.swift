import Foundation

public enum LaunchAgentPlistError: Error, Equatable, CustomStringConvertible {
    case unreadable(path: String, reason: String)
    case notAPropertyList(path: String)
    case notADictionary(path: String)
    case missingLabel(path: String)
    case invalidValue(path: String, key: String)

    public var description: String {
        switch self {
        case .unreadable(let path, let reason): "\(path): cannot read (\(reason))"
        case .notAPropertyList(let path): "\(path): not a property list"
        case .notADictionary(let path): "\(path): top level is not a dictionary"
        case .missingLabel(let path): "\(path): missing Label"
        case .invalidValue(let path, let key): "\(path): invalid value for \(key)"
        }
    }
}

/// Reads LaunchAgent plists (XML or binary) with `PropertyListSerialization`.
public enum LaunchAgentPlistParser {
    public static func parse(contentsOf url: URL, plistPath: String? = nil) throws -> AgentDefinition {
        let data: Data
        do {
            data = try Data(contentsOf: url)
        } catch {
            throw LaunchAgentPlistError.unreadable(path: url.path, reason: error.localizedDescription)
        }
        return try parse(data, plistPath: plistPath ?? url.path, resolvedPlistPath: url.path)
    }

    public static func parse(_ data: Data, plistPath: String, resolvedPlistPath: String? = nil) throws -> AgentDefinition {
        let object: Any
        do {
            object = try PropertyListSerialization.propertyList(from: data, format: nil)
        } catch {
            throw LaunchAgentPlistError.notAPropertyList(path: plistPath)
        }
        guard let dict = object as? [String: Any] else {
            throw LaunchAgentPlistError.notADictionary(path: plistPath)
        }
        let reader = Reader(dict: dict, path: plistPath)

        guard let label = try reader.string("Label"), !label.isEmpty else {
            throw LaunchAgentPlistError.missingLabel(path: plistPath)
        }
        let keepAlive = try reader.keepAlive()
        let throttle = try reader.int("ThrottleInterval")
        return AgentDefinition(
            label: label,
            plistPath: plistPath,
            resolvedPlistPath: resolvedPlistPath,
            program: try reader.string("Program"),
            programArguments: try reader.strings("ProgramArguments") ?? [],
            schedule: try reader.schedule(throttle: throttle),
            queueDirectories: try reader.strings("QueueDirectories") ?? [],
            runAtLoad: try reader.bool("RunAtLoad") ?? false,
            keepAlive: keepAlive,
            throttleInterval: throttle,
            disabled: try reader.bool("Disabled") ?? false,
            standardOutPath: try reader.string("StandardOutPath"),
            standardErrorPath: try reader.string("StandardErrorPath"),
            workingDirectory: try reader.string("WorkingDirectory"),
            processType: try reader.string("ProcessType")
        )
    }

    /// Typed access that tells plist booleans and integers apart
    /// (`NSNumber` bridging would otherwise read `<true/>` as 1 and `<integer>1</integer>` as true).
    private struct Reader {
        let dict: [String: Any]
        let path: String

        func invalid(_ key: String) -> LaunchAgentPlistError {
            .invalidValue(path: path, key: key)
        }

        func string(_ key: String) throws -> String? {
            guard let value = dict[key] else { return nil }
            guard let string = value as? String else { throw invalid(key) }
            return string
        }

        func strings(_ key: String) throws -> [String]? {
            guard let value = dict[key] else { return nil }
            guard let array = value as? [Any] else { throw invalid(key) }
            return try array.map {
                guard let string = $0 as? String else { throw invalid(key) }
                return string
            }
        }

        func bool(_ key: String) throws -> Bool? {
            guard let value = dict[key] else { return nil }
            guard let bool = Self.asBool(value) else { throw invalid(key) }
            return bool
        }

        func int(_ key: String) throws -> Int? {
            guard let value = dict[key] else { return nil }
            guard let int = Self.asInt(value) else { throw invalid(key) }
            return int
        }

        static func asBool(_ value: Any) -> Bool? {
            guard let number = value as? NSNumber, CFGetTypeID(number) == CFBooleanGetTypeID() else { return nil }
            return number.boolValue
        }

        static func asInt(_ value: Any) -> Int? {
            guard let number = value as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID(),
                  !CFNumberIsFloatType(number) else { return nil }
            return number.intValue
        }

        func keepAlive() throws -> KeepAlivePolicy {
            guard let value = dict["KeepAlive"] else { return .none }
            if let bool = Self.asBool(value) {
                return bool ? .always : .none
            }
            guard let conditions = value as? [String: Any] else { throw invalid("KeepAlive") }
            var successfulExit: Bool?
            if let raw = conditions["SuccessfulExit"] {
                guard let bool = Self.asBool(raw) else { throw invalid("KeepAlive.SuccessfulExit") }
                successfulExit = bool
            }
            let others = conditions.keys.filter { $0 != "SuccessfulExit" }.sorted()
            return .conditional(successfulExit: successfulExit, otherKeys: others)
        }

        func schedule(throttle: Int?) throws -> Schedule {
            if let value = dict["StartCalendarInterval"] {
                return .calendar(try calendarEntries(value))
            }
            if let seconds = try int("StartInterval") {
                guard seconds > 0 else { throw invalid("StartInterval") }
                return .interval(seconds: seconds)
            }
            let watched = (try strings("WatchPaths") ?? []) + (try strings("QueueDirectories") ?? [])
            if !watched.isEmpty {
                return .watchPaths(watched, throttleSeconds: throttle)
            }
            return .none
        }

        func calendarEntries(_ value: Any) throws -> [CalendarEntry] {
            // launchd accepts either an array of dictionaries or one bare dictionary.
            if let single = value as? [String: Any] {
                return [try calendarEntry(single)]
            }
            guard let array = value as? [Any] else { throw invalid("StartCalendarInterval") }
            return try array.map {
                guard let entry = $0 as? [String: Any] else { throw invalid("StartCalendarInterval") }
                return try calendarEntry(entry)
            }
        }

        func calendarEntry(_ entry: [String: Any]) throws -> CalendarEntry {
            func field(_ key: String, _ range: ClosedRange<Int>) throws -> Int? {
                guard let raw = entry[key] else { return nil }
                guard let int = Self.asInt(raw), range.contains(int) else {
                    throw invalid("StartCalendarInterval.\(key)")
                }
                return int
            }
            return CalendarEntry(
                minute: try field("Minute", 0...59),
                hour: try field("Hour", 0...23),
                day: try field("Day", 1...31),
                weekday: try field("Weekday", 0...7),
                month: try field("Month", 1...12)
            )
        }
    }
}
