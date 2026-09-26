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

/// Prints each discovered agent with its schedule, then any discovery problems.
func list() throws {
    let result = try AgentDiscovery().discover()
    let width = result.agents.map(\.label.count).max() ?? 0
    for agent in result.agents {
        let padded = agent.label.padding(toLength: width, withPad: " ", startingAt: 0)
        let flags = agent.disabled ? "  [Disabled]" : ""
        print("\(padded)  \(ScheduleDescription.describe(agent))\(flags)")
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
