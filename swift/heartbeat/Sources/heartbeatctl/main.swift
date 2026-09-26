import Foundation
import HeartbeatCore

let usage = """
    heartbeatctl \(HeartbeatCore.version)

    Usage:
      heartbeatctl list
      heartbeatctl --version
    """

struct CommandError: Error, CustomStringConvertible {
    var description: String
}

/// Exit status for command-line usage errors (sysexits EX_USAGE).
let usageExitCode: Int32 = 64

/// Short runtime columns for one agent: state, pid, last exit, runs.
func runtimeColumns(_ status: ServiceStatus) -> [String] {
    switch status {
    case .loaded(let runtime):
        let lastExit = switch runtime.lastExit {
        case .exited(let code): "\(code)"
        case .signaled(let signal, let name): "sig \(signal) (\(name))"
        case .neverExited, nil: "-"
        }
        return [runtime.state.description, runtime.pid.map(String.init) ?? "-", lastExit, runtime.runs.map(String.init) ?? "-"]
    case .notLoaded:
        return ["not loaded", "-", "-", "-"]
    case .unknown:
        return ["unknown", "-", "-", "-"]
    }
}

/// Prints each discovered agent with its launchd runtime and schedule, then any problems.
func list() throws {
    let result = try AgentDiscovery().discover()
    let statuses = LaunchctlClient().print(result.agents.map(\.label))
    let header = ["LABEL", "STATE", "PID", "LAST EXIT", "RUNS", "SCHEDULE"]
    var rows = [header]
    for (agent, status) in zip(result.agents, statuses) {
        let flags = agent.disabled ? "  [Disabled]" : ""
        rows.append([agent.label] + runtimeColumns(status) + [ScheduleDescription.describe(agent) + flags])
    }
    let widths = header.indices.map { column in rows.map { $0[column].count }.max() ?? 0 }
    for row in rows {
        let padded = row.enumerated().map { column, cell in
            column == row.count - 1 ? cell : cell.padding(toLength: widths[column], withPad: " ", startingAt: 0)
        }
        print(padded.joined(separator: "  "))
    }
    for (agent, status) in zip(result.agents, statuses) {
        if case .unknown(let reason) = status { print("! \(agent.label): \(reason)") }
    }
    for problem in result.problems {
        print("! \(problem)")
    }
}

func run(_ arguments: [String]) throws {
    guard let command = arguments.dropFirst().first else {
        print(usage)
        return
    }
    switch command {
    case "--version", "version":
        print("heartbeatctl \(HeartbeatCore.version)")
    case "list":
        try list()
    case "-h", "--help", "help":
        print(usage)
    default:
        throw CommandError(description: "unknown command \(command)\n\n\(usage)")
    }
}

do {
    try run(CommandLine.arguments)
} catch {
    FileHandle.standardError.write(Data("error: \(error)\n".utf8))
    exit(usageExitCode)
}
