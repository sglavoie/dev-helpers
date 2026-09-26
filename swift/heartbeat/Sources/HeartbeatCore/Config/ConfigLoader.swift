import Foundation

/// The outcome of one config load.
public struct ConfigLoadResult: Equatable, Sendable {
    public enum Source: Equatable, Sendable {
        /// No config file: built-in defaults.
        case defaults
        /// Parsed from the file just now.
        case file
        /// The file failed to parse; this is the last config that did (or the defaults).
        case lastGood
    }

    /// Always usable, `~` already expanded.
    public var config: HeartbeatConfig
    public var source: Source
    /// Unknown keys and clamped values; the config still applies.
    public var warnings: [String]
    /// Why the file couldn't be used (invalid JSON, wrong types); the overall icon goes amber.
    public var error: String?

    public init(config: HeartbeatConfig, source: Source, warnings: [String] = [], error: String? = nil) {
        self.config = config
        self.source = source
        self.warnings = warnings
        self.error = error
    }
}

public enum ConfigError: Error, Equatable, Sendable, CustomStringConvertible {
    case invalidJSON(String)
    case notAnObject
    case invalidValue(String)

    public var description: String {
        switch self {
        case .invalidJSON(let message): "invalid JSON: \(message)"
        case .notAnObject: "the top level must be a JSON object"
        case .invalidValue(let message): message
        }
    }
}

/// Loads `config.json`, keeping the last good config when a later edit breaks it.
public struct ConfigLoader: Sendable {
    public var url: URL
    public var home: String
    /// The newest config that loaded (or the defaults when the file was missing).
    public private(set) var lastGood: HeartbeatConfig?

    public init(url: URL = ConfigLoader.defaultURL, home: String = NSHomeDirectory()) {
        self.url = url
        self.home = home
    }

    /// `~/.config/heartbeat/config.json`
    public static var defaultURL: URL {
        URL(fileURLWithPath: NSHomeDirectory())
            .appending(path: ".config/heartbeat/config.json", directoryHint: .notDirectory)
    }

    public mutating func load() -> ConfigLoadResult {
        let data: Data
        do {
            data = try Data(contentsOf: url)
        } catch CocoaError.fileReadNoSuchFile {
            let config = HeartbeatConfig.defaults.expandingTilde(home: home)
            lastGood = config
            return ConfigLoadResult(config: config, source: .defaults)
        } catch {
            return fallback("cannot read \(url.path): \(error.localizedDescription)")
        }

        do {
            let (config, warnings) = try Self.parse(data)
            let expanded = config.expandingTilde(home: home)
            lastGood = expanded
            return ConfigLoadResult(config: expanded, source: .file, warnings: warnings)
        } catch {
            return fallback("\(url.lastPathComponent): \(error)")
        }
    }

    func fallback(_ message: String) -> ConfigLoadResult {
        ConfigLoadResult(config: lastGood ?? HeartbeatConfig.defaults.expandingTilde(home: home), source: .lastGood,
                         error: message)
    }

    /// Decodes and validates a config file. Throws for invalid JSON or wrongly typed values; unknown keys and
    /// out-of-range numbers only produce warnings. JSON5 is accepted so the file can carry `//` comments.
    public static func parse(_ data: Data) throws -> (HeartbeatConfig, [String]) {
        let object: Any
        do {
            object = try JSONSerialization.jsonObject(with: data, options: .json5Allowed)
        } catch {
            throw ConfigError.invalidJSON(jsonErrorText(error))
        }
        guard let root = object as? [String: Any] else { throw ConfigError.notAnObject }

        var config: HeartbeatConfig
        do {
            let decoder = JSONDecoder()
            decoder.allowsJSON5 = true
            config = try decoder.decode(HeartbeatConfig.self, from: data)
        } catch let error as DecodingError {
            throw ConfigError.invalidValue(decodingErrorText(error))
        }

        var warnings = unknownKeys(in: root)
        validate(&config, warnings: &warnings)
        return (config, warnings)
    }

    static func validate(_ config: inout HeartbeatConfig, warnings: inout [String]) {
        if config.version != HeartbeatConfig.currentVersion {
            warnings.append("version \(config.version) is not \(HeartbeatConfig.currentVersion); reading it as version 1")
        }
        if config.pollSeconds < HeartbeatConfig.minimumPollSeconds {
            warnings.append("pollSeconds \(config.pollSeconds) is below the minimum; using \(HeartbeatConfig.minimumPollSeconds)")
            config.pollSeconds = HeartbeatConfig.minimumPollSeconds
        }
        if config.piStatusSeconds < config.pollSeconds {
            warnings.append("piStatusSeconds \(config.piStatusSeconds) is below pollSeconds; using \(config.pollSeconds)")
            config.piStatusSeconds = config.pollSeconds
        }
        for label in config.agents.keys.sorted() {
            guard var agent = config.agents[label] else { continue }
            if var health = agent.health {
                if health.command.isEmpty {
                    warnings.append("agents.\(label).health.command is empty; health check ignored")
                    agent.health = nil
                } else {
                    if health.intervalSeconds < HeartbeatConfig.minimumPollSeconds {
                        warnings.append("agents.\(label).health.intervalSeconds \(health.intervalSeconds) is below the minimum; using \(HeartbeatConfig.minimumPollSeconds)")
                        health.intervalSeconds = HeartbeatConfig.minimumPollSeconds
                    }
                    if health.timeoutSeconds < 1 {
                        warnings.append("agents.\(label).health.timeoutSeconds \(health.timeoutSeconds) is below 1; using \(HealthCommandConfig.defaultTimeoutSeconds)")
                        health.timeoutSeconds = HealthCommandConfig.defaultTimeoutSeconds
                    }
                    agent.health = health
                }
            }
            if let maxAge = agent.maxAgeSeconds, maxAge < 1 {
                warnings.append("agents.\(label).maxAgeSeconds \(maxAge) is below 1; ignored")
                agent.maxAgeSeconds = nil
            }
            if let grace = agent.graceSeconds, grace < 0 {
                warnings.append("agents.\(label).graceSeconds \(grace) is negative; using the default")
                agent.graceSeconds = nil
            }
            config.agents[label] = agent
        }
    }

    /// Keys the decoder would silently drop, as dotted paths.
    static func unknownKeys(in root: [String: Any]) -> [String] {
        var warnings: [String] = []
        func check(_ object: [String: Any], known: [String], path: String) {
            for key in object.keys.sorted() where !known.contains(key) {
                warnings.append("unknown key \(path.isEmpty ? key : "\(path).\(key)")")
            }
        }
        check(root, known: HeartbeatConfig.CodingKeys.allCases.map(\.rawValue), path: "")
        guard let agents = root["agents"] as? [String: Any] else { return warnings }
        for label in agents.keys.sorted() {
            guard let agent = agents[label] as? [String: Any] else { continue }
            let path = "agents.\(label)"
            check(agent, known: AgentConfig.CodingKeys.allCases.map(\.rawValue), path: path)
            if let health = agent["health"] as? [String: Any] {
                check(health, known: HealthCommandConfig.CodingKeys.allCases.map(\.rawValue), path: "\(path).health")
            }
            if let receipt = agent["receipt"] as? [String: Any] {
                check(receipt, known: ReceiptConfig.CodingKeys.allCases.map(\.rawValue), path: "\(path).receipt")
            }
        }
        return warnings
    }

    /// `~` or `~/...` → the home directory; anything else unchanged.
    public static func expandTilde(_ path: String, home: String) -> String {
        if path == "~" { return home }
        if path.hasPrefix("~/") { return home + path.dropFirst() }
        return path
    }

    static func jsonErrorText(_ error: Error) -> String {
        let nsError = error as NSError
        return (nsError.userInfo[NSDebugDescriptionErrorKey] as? String) ?? nsError.localizedDescription
    }

    static func decodingErrorText(_ error: DecodingError) -> String {
        func path(_ context: DecodingError.Context) -> String {
            let keys = context.codingPath.map { $0.intValue.map(String.init) ?? $0.stringValue }
            return keys.isEmpty ? "config" : keys.joined(separator: ".")
        }
        switch error {
        case .typeMismatch(_, let context), .valueNotFound(_, let context), .dataCorrupted(let context):
            // Foundation reports some mismatches (1.5 for an Int) as corrupt data with the reason underneath.
            let reason = context.underlyingError.map(jsonErrorText) ?? context.debugDescription
            return "\(path(context)): \(reason)"
        case .keyNotFound(let key, let context):
            let parent = path(context)
            return "\(parent == "config" ? key.stringValue : "\(parent).\(key.stringValue)") is required"
        @unknown default:
            return "\(error)"
        }
    }
}
