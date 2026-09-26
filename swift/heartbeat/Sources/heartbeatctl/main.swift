import Foundation
import HeartbeatCore

let usage = """
    heartbeatctl \(HeartbeatCore.version)

    Usage:
      heartbeatctl --version
    """

struct CommandError: Error, CustomStringConvertible {
    var description: String
}

/// Exit status for command-line usage errors (sysexits EX_USAGE).
let usageExitCode: Int32 = 64

func run(_ arguments: [String]) throws {
    guard let command = arguments.dropFirst().first else {
        print(usage)
        return
    }
    switch command {
    case "--version", "version":
        print("heartbeatctl \(HeartbeatCore.version)")
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
