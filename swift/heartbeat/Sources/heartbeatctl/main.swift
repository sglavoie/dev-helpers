import Foundation
import HeartbeatCore

let usage = """
    heartbeatctl \(HeartbeatCore.version)

    Usage:
      heartbeatctl status [--json] [--all] [--health]
      heartbeatctl list [--json]
      heartbeatctl explain <label>
      heartbeatctl check-config
      heartbeatctl --version

    status lists failing, warning and paused agents (--all adds ok and hidden
    ones). --health runs the configured health commands now instead of using
    the app's last results from state.json. explain accepts a full label or the
    part after the label prefix.

    Exit status: 0 ok, 1 warning, 2 failing, 3 unknown, 64 usage error.
    check-config exits 0 when the config is clean, 1 on warnings, 2 on errors.
    """

struct CommandError: Error, CustomStringConvertible {
    var description: String
}

/// Exit status for command-line usage errors (sysexits EX_USAGE).
let usageExitCode: Int32 = 64
/// Exit status when heartbeatctl itself fails, matching `OverallStatus.unknown`.
let unknownExitCode = OverallStatus.unknown.exitCode

struct Options {
    var positional: [String] = []
    var json = false
    var all = false
    var health = false

    init(_ arguments: ArraySlice<String>, allowed: Set<String>, positional maxPositional: Int = 0) throws {
        for argument in arguments {
            if argument.hasPrefix("--") {
                guard allowed.contains(argument) else { throw CommandError(description: "unknown option \(argument)") }
                switch argument {
                case "--json": json = true
                case "--all": all = true
                case "--health": health = true
                default: break
                }
            } else {
                positional.append(argument)
            }
        }
        if positional.count > maxPositional {
            throw CommandError(description: "unexpected argument \(positional[maxPositional])")
        }
    }
}

let formatter = StatusFormatter()

func loadConfig() -> ConfigLoadResult {
    var loader = ConfigLoader()
    return loader.load()
}

/// The CLI never writes state.json: it judges with the app's saved history as it is.
func snapshot(health: Bool = false) -> Snapshot {
    let state = StateStore().read() ?? .empty
    return SnapshotBuilder().buildSync(config: loadConfig(), state: state, ledger: .readOnly, runHealthChecks: health)
}

func status(_ options: Options) -> Int32 {
    let snapshot = snapshot(health: options.health)
    print(options.json ? formatter.statusJSON(snapshot, all: options.all) : formatter.statusText(snapshot, all: options.all))
    return snapshot.overall.exitCode
}

func list(_ options: Options) -> Int32 {
    let snapshot = snapshot()
    print(options.json ? formatter.listJSON(snapshot) : formatter.listText(snapshot))
    return snapshot.fatalError == nil ? 0 : unknownExitCode
}

func explain(_ options: Options) throws -> Int32 {
    guard options.positional.count == 1, let query = options.positional.first else {
        throw CommandError(description: "explain needs one label\n\n\(usage)")
    }
    let snapshot = snapshot(health: options.health)
    let label = snapshot.agent(query) != nil ? query : snapshot.config.labelPrefix + query
    guard let agent = snapshot.agent(label) else {
        let known = snapshot.agents.map(\.label).joined(separator: "\n  ")
        FileHandle.standardError.write(Data("error: no agent \(query)\(known.isEmpty ? "" : "; known:\n  \(known)")\n".utf8))
        return unknownExitCode
    }
    print(formatter.explain(agent, snapshot: snapshot))
    return Snapshot.exitCode(agent.severity)
}

func checkConfig() -> Int32 {
    let result = loadConfig()
    let discovered = try? AgentDiscovery(labelPrefix: result.config.labelPrefix).discover().agents.map(\.label)
    print(formatter.configReport(result, path: ConfigLoader.defaultURL.path, discovered: discovered))
    if result.error != nil { return 2 }
    return result.warnings.isEmpty && formatter.unknownLabels(result.config, discovered: discovered).isEmpty ? 0 : 1
}

func run(_ arguments: [String]) throws -> Int32 {
    guard let command = arguments.dropFirst().first else {
        print(usage)
        return 0
    }
    let rest = arguments.dropFirst(2)
    switch command {
    case "--version", "version":
        print("heartbeatctl \(HeartbeatCore.version)")
    case "status":
        return status(try Options(rest, allowed: ["--json", "--all", "--health"]))
    case "list":
        return list(try Options(rest, allowed: ["--json"]))
    case "explain":
        return try explain(try Options(rest, allowed: ["--health"], positional: 1))
    case "check-config":
        _ = try Options(rest, allowed: [])
        return checkConfig()
    case "-h", "--help", "help":
        print(usage)
    default:
        throw CommandError(description: "unknown command \(command)\n\n\(usage)")
    }
    return 0
}

do {
    exit(try run(CommandLine.arguments))
} catch let error as CommandError {
    FileHandle.standardError.write(Data("error: \(error)\n".utf8))
    exit(usageExitCode)
} catch {
    FileHandle.standardError.write(Data("error: \(error)\n".utf8))
    exit(unknownExitCode)
}
