import Foundation

/// Writes a new schedule to an agent's plist (through the stow symlink, to the file it points at) and,
/// optionally, a new `maxAgeSeconds` to `config.json`. Both edits are computed and checked before either file
/// is touched, and each file is replaced atomically with its permissions kept.
public struct ScheduleApplier: Sendable {
    public struct Change: Sendable {
        public var schedule: Schedule
        public var throttle: ThrottleUpdate
        /// New `agents.<label>.maxAgeSeconds`, or `nil` to leave config.json alone.
        public var maxAgeSeconds: Int?

        public init(schedule: Schedule, throttle: ThrottleUpdate = .keep, maxAgeSeconds: Int? = nil) {
            self.schedule = schedule
            self.throttle = throttle
            self.maxAgeSeconds = maxAgeSeconds
        }
    }

    public var configURL: URL

    public init(configURL: URL = ConfigLoader.defaultURL) {
        self.configURL = configURL
    }

    /// A stowed agent's plist is a symlink into the dotfiles repo; anything else was written by an installer
    /// that may write it again.
    public static func isStowManaged(_ agent: AgentDefinition) -> Bool {
        agent.plistPath != agent.resolvedPlistPath
    }

    public func apply(_ change: Change, to agent: AgentDefinition) throws {
        let plistPath = agent.resolvedPlistPath
        let plist = try PlistScheduleWriter.apply(change.schedule, throttle: change.throttle,
                                                  to: try Data(contentsOf: URL(fileURLWithPath: plistPath)))
        var config: (path: String, data: Data)?
        if let maxAge = change.maxAgeSeconds {
            let path = configURL.resolvingSymlinksInPath().path
            let text = try String(contentsOfFile: path, encoding: .utf8)
            let edited = try ConfigValueEditor.replaceAgentMaxAge(in: text, label: agent.label, value: maxAge)
            config = (path, Data(edited.utf8))
        }
        try Self.replace(plistPath, with: plist)
        if let config { try Self.replace(config.path, with: config.data) }
    }

    /// Writes a sibling temp file with the original's permissions, then renames it over `path`.
    static func replace(_ path: String, with data: Data) throws {
        let url = URL(fileURLWithPath: path)
        let permissions = try FileManager.default.attributesOfItem(atPath: path)[.posixPermissions] as? NSNumber
        let temp = url.deletingLastPathComponent()
            .appending(path: ".\(url.lastPathComponent).heartbeat-\(UUID().uuidString)", directoryHint: .notDirectory)
        try data.write(to: temp)
        do {
            if let permissions {
                try FileManager.default.setAttributes([.posixPermissions: permissions], ofItemAtPath: temp.path)
            }
            guard rename(temp.path, path) == 0 else {
                throw CocoaError(.fileWriteUnknown, userInfo: [NSFilePathErrorKey: path,
                                 NSLocalizedDescriptionKey: String(cString: strerror(errno))])
            }
        } catch {
            try? FileManager.default.removeItem(at: temp)
            throw error
        }
    }
}
