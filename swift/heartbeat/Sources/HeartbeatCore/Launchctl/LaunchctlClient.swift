import Foundation

public enum LaunchctlError: Error, Equatable, Sendable, CustomStringConvertible {
    /// launchctl ran but failed; `message` is its stderr (or stdout when stderr is empty).
    case commandFailed(argv: [String], status: Termination, message: String)
    case timedOut(argv: [String])

    public var description: String {
        switch self {
        case .commandFailed(let argv, let status, let message):
            let code = switch status {
            case .exited(let code): "exit \(code)"
            case .signaled(let signal): "signal \(signal)"
            }
            let detail = message.isEmpty ? "" : ": \(message)"
            return "\(argv.joined(separator: " ")) failed (\(code))\(detail)"
        case .timedOut(let argv):
            return "\(argv.joined(separator: " ")) timed out"
        }
    }
}

/// Talks to launchd about services in the user's GUI domain (`gui/$UID`).
public struct LaunchctlClient: Sendable {
    public static let launchctlPath = "/bin/launchctl"
    /// `launchctl print` exit status for a service that is not loaded.
    public static let notLoadedExitCode: Int32 = 113

    public var runner: any CommandRunning
    public var uid: uid_t
    public var timeout: TimeInterval

    public init(runner: any CommandRunning = CommandRunner(), uid: uid_t = getuid(), timeout: TimeInterval = 5) {
        self.runner = runner
        self.uid = uid
        self.timeout = timeout
    }

    public var domain: String { "gui/\(uid)" }

    public func serviceTarget(_ label: String) -> String { "\(domain)/\(label)" }

    /// `launchctl print gui/$UID/<label>`. Never throws: failures become `.unknown`.
    public func print(_ label: String) -> ServiceStatus {
        let argv = [Self.launchctlPath, "print", serviceTarget(label)]
        let result: CommandResult
        do {
            result = try runner.run(argv, timeout: timeout)
        } catch {
            return .unknown(reason: "\(error)")
        }
        if result.timedOut { return .unknown(reason: "launchctl print timed out") }
        switch result.termination {
        case .exited(0):
            do {
                return .loaded(try LaunchctlPrintParser.parse(result.stdoutText))
            } catch {
                return .unknown(reason: "\(error)")
            }
        case .exited(Self.notLoadedExitCode):
            return .notLoaded
        default:
            return .unknown(reason: LaunchctlError.commandFailed(argv: argv, status: result.termination, message: Self.message(result)).description)
        }
    }

    /// Prints several labels with at most `maxConcurrent` launchctl processes; results follow `labels`' order.
    public func print(_ labels: [String], maxConcurrent: Int = 4) -> [ServiceStatus] {
        let results = ResultBox(count: labels.count)
        let gate = DispatchSemaphore(value: max(1, maxConcurrent))
        DispatchQueue.concurrentPerform(iterations: labels.count) { index in
            gate.wait()
            defer { gate.signal() }
            results.set(index, print(labels[index]))
        }
        return results.values
    }

    /// `launchctl kickstart [-k] gui/$UID/<label>`: run now, or kill and restart when `restart` is true.
    public func kickstart(_ label: String, restart: Bool = false) throws {
        try perform(["kickstart"] + (restart ? ["-k"] : []) + [serviceTarget(label)])
    }

    /// `launchctl bootstrap gui/$UID <plist>`: load the agent from its plist.
    public func bootstrap(plistPath: String) throws {
        try perform(["bootstrap", domain, plistPath])
    }

    /// `launchctl bootout gui/$UID/<label>`: unload the agent.
    public func bootout(_ label: String) throws {
        try perform(["bootout", serviceTarget(label)])
    }

    /// Bootout then bootstrap, so launchd picks up an edited plist. launchd often refuses a bootstrap right after a
    /// bootout (EIO while the old job is torn down), so it is retried with a growing delay.
    public func reload(_ label: String, plistPath: String, attempts: Int = 5, delay: TimeInterval = 0.2) throws {
        try bootout(label)
        var attempt = 1
        while true {
            do {
                return try bootstrap(plistPath: plistPath)
            } catch is LaunchctlError where attempt < attempts {
                Thread.sleep(forTimeInterval: delay * Double(attempt))
                attempt += 1
            }
        }
    }

    func perform(_ arguments: [String]) throws {
        let argv = [Self.launchctlPath] + arguments
        let result = try runner.run(argv, timeout: timeout)
        if result.timedOut { throw LaunchctlError.timedOut(argv: argv) }
        guard result.termination == .exited(0) else {
            throw LaunchctlError.commandFailed(argv: argv, status: result.termination, message: Self.message(result))
        }
    }

    static func message(_ result: CommandResult) -> String {
        let stderr = result.stderrText.trimmingCharacters(in: .whitespacesAndNewlines)
        return stderr.isEmpty ? result.stdoutText.trimmingCharacters(in: .whitespacesAndNewlines) : stderr
    }

    /// Collects results written from concurrent iterations.
    final class ResultBox: @unchecked Sendable {
        private let lock = NSLock()
        private var storage: [ServiceStatus]

        init(count: Int) {
            storage = Array(repeating: .unknown(reason: "not checked"), count: count)
        }

        func set(_ index: Int, _ value: ServiceStatus) {
            lock.withLock { storage[index] = value }
        }

        var values: [ServiceStatus] {
            lock.withLock { storage }
        }
    }
}
