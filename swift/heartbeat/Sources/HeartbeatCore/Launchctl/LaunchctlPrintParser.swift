import Foundation

public enum LaunchctlPrintError: Error, Equatable, Sendable, CustomStringConvertible {
    /// The first line is not `<domain>/<label> = {`.
    case missingHeader
    /// No top-level `state = …` line.
    case missingState
    /// A top-level field the parser relies on has an unexpected value.
    case invalidValue(key: String, value: String)

    public var description: String {
        switch self {
        case .missingHeader: "launchctl print output has no service header"
        case .missingState: "launchctl print output has no state"
        case .invalidValue(let key, let value): "launchctl print: unexpected \(key) = \(value)"
        }
    }
}

/// Parses `launchctl print gui/$UID/<label>` output.
///
/// Only top-level fields (lines indented by exactly one tab) are read: nested blocks such as
/// `resource coalition` repeat keys like `state = active`.
public enum LaunchctlPrintParser {
    public static func parse(_ output: String) throws -> ServiceRuntime {
        let lines = output.split(separator: "\n", omittingEmptySubsequences: true)
        guard let header = lines.first, !header.hasPrefix("\t"), header.contains("/"), header.hasSuffix(" = {") else {
            throw LaunchctlPrintError.missingHeader
        }

        var fields: [String: String] = [:]
        for line in lines.dropFirst() {
            guard line.hasPrefix("\t"), !line.dropFirst().hasPrefix("\t") else { continue }
            guard let separator = line.range(of: " = ") else { continue }
            let key = String(line[line.index(after: line.startIndex)..<separator.lowerBound])
            let value = String(line[separator.upperBound...])
            if fields[key] == nil { fields[key] = value }
        }

        guard let stateText = fields["state"] else { throw LaunchctlPrintError.missingState }
        var runtime = ServiceRuntime(state: ServiceState(stateText), path: fields["path"])
        if let text = fields["pid"] {
            guard let pid = Int32(text) else { throw LaunchctlPrintError.invalidValue(key: "pid", value: text) }
            runtime.pid = pid
        }
        if let text = fields["runs"] {
            guard let runs = Int(text) else { throw LaunchctlPrintError.invalidValue(key: "runs", value: text) }
            runtime.runs = runs
        }
        if let text = fields["last terminating signal"] {
            runtime.lastExit = try signal(text)
        } else if let text = fields["last exit code"] {
            runtime.lastExit = try exitCode(text)
        }
        return runtime
    }

    /// `(never exited)`, `1`, or `78: EX_CONFIG` (some macOS versions append a description).
    static func exitCode(_ text: String) throws -> LastExit {
        if text.hasPrefix("(") { return .neverExited }
        let number = text.split(separator: ":", maxSplits: 1).first.map { $0.trimmingCharacters(in: .whitespaces) } ?? ""
        guard let code = Int32(number) else { throw LaunchctlPrintError.invalidValue(key: "last exit code", value: text) }
        return .exited(code: code)
    }

    /// `Terminated: 15`, or a bare signal number.
    static func signal(_ text: String) throws -> LastExit {
        let parts = text.split(separator: ":").map { $0.trimmingCharacters(in: .whitespaces) }
        guard let last = parts.last, let number = Int32(last) else {
            throw LaunchctlPrintError.invalidValue(key: "last terminating signal", value: text)
        }
        let name = parts.count > 1 ? parts.dropLast().joined(separator: ": ") : "signal \(number)"
        return .signaled(signal: number, name: name)
    }
}
