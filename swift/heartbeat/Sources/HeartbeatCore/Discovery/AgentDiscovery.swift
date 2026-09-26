import Foundation

public enum DiscoveryProblem: Equatable, Sendable, CustomStringConvertible {
    /// A plist symlink whose target is missing (e.g. a stow package was moved).
    case brokenSymlink(path: String, destination: String)
    /// A matching file that could not be parsed.
    case invalidPlist(path: String, message: String)

    public var path: String {
        switch self {
        case .brokenSymlink(let path, _), .invalidPlist(let path, _): path
        }
    }

    public var description: String {
        switch self {
        case .brokenSymlink(let path, let destination): "\(path): broken symlink to \(destination)"
        case .invalidPlist(_, let message): message
        }
    }
}

public struct DiscoveryResult: Equatable, Sendable {
    /// Agents sorted by label.
    public var agents: [AgentDefinition]
    /// Problems sorted by path.
    public var problems: [DiscoveryProblem]

    public init(agents: [AgentDefinition] = [], problems: [DiscoveryProblem] = []) {
        self.agents = agents
        self.problems = problems
    }
}

/// Finds `<labelPrefix>*.plist` in a LaunchAgents directory, following stow symlinks.
public struct AgentDiscovery: Sendable {
    public static let defaultLabelPrefix = "com.sglavoie."
    public static var defaultDirectory: URL {
        FileManager.default.homeDirectoryForCurrentUser.appending(path: "Library/LaunchAgents", directoryHint: .isDirectory)
    }

    public var directory: URL
    public var labelPrefix: String

    public init(directory: URL = AgentDiscovery.defaultDirectory, labelPrefix: String = AgentDiscovery.defaultLabelPrefix) {
        self.directory = directory
        self.labelPrefix = labelPrefix
    }

    public func discover() throws -> DiscoveryResult {
        let fileManager = FileManager.default
        let names = try fileManager.contentsOfDirectory(atPath: directory.path)
            .filter { $0.hasPrefix(labelPrefix) && $0.hasSuffix(".plist") }
            .sorted()

        var result = DiscoveryResult()
        for name in names {
            let path = directory.appending(path: name).path
            let resolved: String
            switch Self.resolveSymlinks(path) {
            case .success(let target):
                resolved = target
            case .failure(let broken):
                result.problems.append(.brokenSymlink(path: path, destination: broken.destination))
                continue
            }
            var isDirectory: ObjCBool = false
            guard fileManager.fileExists(atPath: resolved, isDirectory: &isDirectory), !isDirectory.boolValue else {
                continue
            }
            do {
                let agent = try LaunchAgentPlistParser.parse(contentsOf: URL(fileURLWithPath: resolved), plistPath: path)
                result.agents.append(agent)
            } catch {
                result.problems.append(.invalidPlist(path: path, message: "\(error)"))
            }
        }
        result.agents.sort { $0.label < $1.label }
        result.problems.sort { $0.path < $1.path }
        return result
    }

    struct BrokenLink: Error, Equatable {
        var destination: String
    }

    /// Follows a symlink chain (relative targets resolve against the link's directory).
    /// Fails with the last unresolvable destination when the chain ends at a missing file or loops.
    static func resolveSymlinks(_ path: String) -> Result<String, BrokenLink> {
        let fileManager = FileManager.default
        var current = path
        for _ in 0..<32 {
            guard let destination = try? fileManager.destinationOfSymbolicLink(atPath: current) else {
                return .success(current)
            }
            // Lexical resolution only: `standardizingPath` would also resolve /var -> /private/var.
            let base = URL(fileURLWithPath: current).deletingLastPathComponent()
            current = URL(fileURLWithPath: destination, relativeTo: base).standardized.path
            if (try? fileManager.attributesOfItem(atPath: current)) == nil {
                return .failure(BrokenLink(destination: destination))
            }
        }
        return .failure(BrokenLink(destination: current))
    }
}
