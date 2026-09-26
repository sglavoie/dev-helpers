import Foundation

/// How a finished command ended.
public enum Termination: Equatable, Sendable {
    case exited(Int32)
    case signaled(Int32)
}

public struct CommandResult: Equatable, Sendable {
    public var argv: [String]
    public var termination: Termination
    /// At most `CommandRunner.outputLimit` bytes; the rest is read and discarded.
    public var stdout: Data
    public var stderr: Data
    public var stdoutTruncated: Bool
    public var stderrTruncated: Bool
    /// The timeout expired and the process group was killed.
    public var timedOut: Bool
    public var duration: TimeInterval

    public init(
        argv: [String],
        termination: Termination,
        stdout: Data = Data(),
        stderr: Data = Data(),
        stdoutTruncated: Bool = false,
        stderrTruncated: Bool = false,
        timedOut: Bool = false,
        duration: TimeInterval = 0
    ) {
        self.argv = argv
        self.termination = termination
        self.stdout = stdout
        self.stderr = stderr
        self.stdoutTruncated = stdoutTruncated
        self.stderrTruncated = stderrTruncated
        self.timedOut = timedOut
        self.duration = duration
    }

    /// The exit status, or `nil` if the process was killed by a signal.
    public var exitCode: Int32? {
        if case .exited(let code) = termination { return code }
        return nil
    }

    public var succeeded: Bool { !timedOut && termination == .exited(0) }
    public var stdoutText: String { String(decoding: stdout, as: UTF8.self) }
    public var stderrText: String { String(decoding: stderr, as: UTF8.self) }
}

public enum CommandError: Error, Equatable, Sendable, CustomStringConvertible {
    case emptyArguments
    /// The executable could not be found or spawned.
    case launchFailed(executable: String, message: String)

    public var description: String {
        switch self {
        case .emptyArguments: "no command given"
        case .launchFailed(let executable, let message): "could not start \(executable): \(message)"
        }
    }
}

/// Runs external commands; a protocol so launchctl and health checks can be tested with fakes.
public protocol CommandRunning: Sendable {
    func run(_ argv: [String], timeout: TimeInterval) throws -> CommandResult
}

/// Runs an argv (no shell) in its own process group with a timeout and a capped output buffer.
///
/// On timeout the whole process group is killed with SIGKILL, so a shell script's children
/// (e.g. a `sleep`) do not outlive it. A command that exits while a background child still holds
/// its output pipes gets `drainGrace` seconds before the group is killed too.
public struct CommandRunner: CommandRunning {
    public static let outputLimit = 64 * 1024
    /// PATH for GUI-launched commands, which do not inherit a login shell's PATH.
    public static let guiPath = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

    /// Environment for the child; `nil` inherits the current process environment.
    public var environment: [String: String]?
    public var outputLimit: Int
    public var drainGrace: TimeInterval

    public init(environment: [String: String]? = nil, outputLimit: Int = CommandRunner.outputLimit, drainGrace: TimeInterval = 1) {
        self.environment = environment
        self.outputLimit = outputLimit
        self.drainGrace = drainGrace
    }

    public func run(_ argv: [String], timeout: TimeInterval) throws -> CommandResult {
        guard let command = argv.first else { throw CommandError.emptyArguments }
        let env = environment ?? ProcessInfo.processInfo.environment
        let executable = try Self.resolve(command, path: env["PATH"] ?? Self.guiPath)
        let start = Date()

        var stdoutPipe: [Int32] = [-1, -1]
        var stderrPipe: [Int32] = [-1, -1]
        guard pipe(&stdoutPipe) == 0 else { throw CommandError.launchFailed(executable: command, message: Self.errnoText()) }
        guard pipe(&stderrPipe) == 0 else {
            close(stdoutPipe[0]); close(stdoutPipe[1])
            throw CommandError.launchFailed(executable: command, message: Self.errnoText())
        }

        let pid: pid_t
        do {
            pid = try Self.spawn(executable, argv: argv, environment: env, stdout: stdoutPipe[1], stderr: stderrPipe[1])
        } catch {
            for fd in stdoutPipe + stderrPipe { close(fd) }
            throw error
        }
        close(stdoutPipe[1])
        close(stderrPipe[1])

        var streams = [Stream(fd: stdoutPipe[0]), Stream(fd: stderrPipe[0])]
        for stream in streams { _ = fcntl(stream.fd, F_SETFL, fcntl(stream.fd, F_GETFL) | O_NONBLOCK) }

        let deadline = start.addingTimeInterval(timeout)
        var status: Int32?
        var timedOut = false
        var killDeadline: Date?  // once set, the group is killed when it passes
        var killed = false

        while status == nil || streams.contains(where: \.isOpen) {
            if status == nil {
                var raw: Int32 = 0
                if waitpid(pid, &raw, WNOHANG) == pid { status = raw }
            }
            let now = Date()
            if status == nil, now >= deadline, !killed {
                timedOut = true
                Self.killGroup(pid)
                killed = true
                killDeadline = now.addingTimeInterval(drainGrace)
            } else if status != nil, killDeadline == nil, streams.contains(where: \.isOpen) {
                killDeadline = now.addingTimeInterval(drainGrace)
            }
            if let limit = killDeadline, now >= limit {
                if !killed {
                    Self.killGroup(pid)
                    killed = true
                }
                if status != nil || now >= limit.addingTimeInterval(drainGrace) {
                    // Pipes held by a process outside the group; stop waiting for EOF.
                    for index in streams.indices { streams[index].close() }
                }
            }
            if status != nil, !streams.contains(where: \.isOpen) { break }

            var pollfds = streams.filter(\.isOpen).map { pollfd(fd: $0.fd, events: Int16(POLLIN), revents: 0) }
            _ = poll(&pollfds, nfds_t(pollfds.count), 20)
            for index in streams.indices where streams[index].isOpen {
                streams[index].drain(limit: outputLimit)
            }
        }
        if status == nil {
            var raw: Int32 = 0
            while waitpid(pid, &raw, 0) == -1, errno == EINTR {}
            status = raw
        }

        return CommandResult(
            argv: argv,
            termination: Self.termination(status ?? 0),
            stdout: streams[0].data,
            stderr: streams[1].data,
            stdoutTruncated: streams[0].truncated,
            stderrTruncated: streams[1].truncated,
            timedOut: timedOut,
            duration: Date().timeIntervalSince(start)
        )
    }

    /// Resolves a bare command name against `path`; names containing "/" are used as given.
    static func resolve(_ command: String, path: String) throws -> String {
        let fileManager = FileManager.default
        if command.contains("/") {
            let expanded = (command as NSString).expandingTildeInPath
            guard fileManager.isExecutableFile(atPath: expanded) else {
                throw CommandError.launchFailed(executable: command, message: "not an executable file")
            }
            return expanded
        }
        for directory in path.split(separator: ":") {
            let candidate = "\(directory)/\(command)"
            if fileManager.isExecutableFile(atPath: candidate) { return candidate }
        }
        throw CommandError.launchFailed(executable: command, message: "not found in PATH")
    }

    static func spawn(_ executable: String, argv: [String], environment: [String: String], stdout: Int32, stderr: Int32) throws -> pid_t {
        var actions: posix_spawn_file_actions_t?
        posix_spawn_file_actions_init(&actions)
        defer { posix_spawn_file_actions_destroy(&actions) }
        posix_spawn_file_actions_addopen(&actions, STDIN_FILENO, "/dev/null", O_RDONLY, 0)
        posix_spawn_file_actions_adddup2(&actions, stdout, STDOUT_FILENO)
        posix_spawn_file_actions_adddup2(&actions, stderr, STDERR_FILENO)

        var attributes: posix_spawnattr_t?
        posix_spawnattr_init(&attributes)
        defer { posix_spawnattr_destroy(&attributes) }
        // Own process group (pgid = pid) so a timeout can kill the whole tree; close every
        // other inherited descriptor; reset signal handling the parent may have changed.
        posix_spawnattr_setpgroup(&attributes, 0)
        var noSignals = sigset_t()
        sigemptyset(&noSignals)
        posix_spawnattr_setsigmask(&attributes, &noSignals)
        var allSignals = sigset_t()
        sigfillset(&allSignals)
        posix_spawnattr_setsigdefault(&attributes, &allSignals)
        posix_spawnattr_setflags(&attributes, Int16(POSIX_SPAWN_SETPGROUP | POSIX_SPAWN_SETSIGMASK | POSIX_SPAWN_SETSIGDEF | POSIX_SPAWN_CLOEXEC_DEFAULT))

        let cArgs = argv.map { strdup($0) } + [nil]
        let cEnv = environment.map { strdup("\($0.key)=\($0.value)") } + [nil]
        defer {
            cArgs.forEach { free($0) }
            cEnv.forEach { free($0) }
        }
        var pid: pid_t = 0
        let result = posix_spawn(&pid, executable, &actions, &attributes, cArgs, cEnv)
        guard result == 0 else {
            throw CommandError.launchFailed(executable: executable, message: String(cString: strerror(result)))
        }
        return pid
    }

    static func killGroup(_ pid: pid_t) {
        kill(-pid, SIGKILL)
    }

    static func termination(_ status: Int32) -> Termination {
        let signal = status & 0x7f
        return signal == 0 ? .exited((status >> 8) & 0xff) : .signaled(signal)
    }

    static func errnoText() -> String {
        String(cString: strerror(errno))
    }

    /// One non-blocking output pipe being drained into a capped buffer.
    struct Stream {
        var fd: Int32
        var isOpen = true
        var data = Data()
        var truncated = false

        init(fd: Int32) {
            self.fd = fd
        }

        mutating func drain(limit: Int) {
            var buffer = [UInt8](repeating: 0, count: 16 * 1024)
            while true {
                let count = read(fd, &buffer, buffer.count)
                if count > 0 {
                    let room = limit - data.count
                    if room > 0 { data.append(contentsOf: buffer[0..<min(room, count)]) }
                    if count > room { truncated = true }
                } else if count == 0 {
                    close()
                    return
                } else {
                    if errno == EINTR { continue }
                    if errno != EAGAIN { close() }
                    return
                }
            }
        }

        mutating func close() {
            guard isOpen else { return }
            Darwin.close(fd)
            isOpen = false
        }
    }
}
