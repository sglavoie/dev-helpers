import Foundation

/// Runs an editor without a timeout. Call off the main thread: the editor may stay open indefinitely.
/// stderr is continuously drained with a bounded capture; descendants are never killed on editor exit.
public enum LogOpener {
    public static func run(_ argv: [String]) throws -> CommandResult {
        guard let command = argv.first else { throw CommandError.emptyArguments }
        var environment = ProcessInfo.processInfo.environment
        environment["PATH"] = CommandRunner.guiPath
        let executable = try CommandRunner.resolve(command, path: CommandRunner.guiPath)
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = Array(argv.dropFirst())
        process.environment = environment
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        let stderr = Pipe()
        process.standardError = stderr

        // Own a duplicate so Stream can close it independently of FileHandle's lifetime.
        let fd = dup(stderr.fileHandleForReading.fileDescriptor)
        guard fd >= 0 else {
            throw CommandError.launchFailed(executable: command, message: CommandRunner.errnoText())
        }
        var stream = CommandRunner.Stream(fd: fd)
        defer { stream.close() }
        guard fcntl(fd, F_SETFL, fcntl(fd, F_GETFL) | O_NONBLOCK) != -1 else {
            throw CommandError.launchFailed(executable: command, message: CommandRunner.errnoText())
        }
        let started = Date()
        do {
            try process.run()
        } catch {
            throw CommandError.launchFailed(executable: command, message: error.localizedDescription)
        }
        try? stderr.fileHandleForWriting.close()
        try? stderr.fileHandleForReading.close()

        while process.isRunning {
            if stream.isOpen {
                var descriptor = pollfd(fd: fd, events: Int16(POLLIN), revents: 0)
                _ = poll(&descriptor, 1, 100)
                stream.drain(limit: CommandRunner.outputLimit)
            } else {
                // An editor can close stderr before it exits.
                _ = poll(nil, 0, 100)
            }
        }
        process.waitUntilExit()
        // Capture buffered output, but don't wait for EOF from an editor's background children.
        if stream.isOpen { stream.drain(limit: CommandRunner.outputLimit) }
        let result = CommandResult(
            argv: argv,
            termination: process.terminationReason == .exit
                ? .exited(process.terminationStatus) : .signaled(process.terminationStatus),
            stderr: stream.data,
            stderrTruncated: stream.truncated,
            duration: Date().timeIntervalSince(started)
        )
        if stream.isOpen {
            // A background editor may still own stderr. Keep draining it until EOF so a later
            // write doesn't receive SIGPIPE, without delaying the parent command's result.
            discardUntilEOF(fd: stream.fd)
            stream.isOpen = false // ownership passed to the dispatch source
        }
        return result
    }

    private static func discardUntilEOF(fd: Int32) {
        let source = DispatchSource.makeReadSource(fileDescriptor: fd, queue: .global(qos: .utility))
        source.setCancelHandler { Darwin.close(fd) }
        source.setEventHandler {
            var buffer = [UInt8](repeating: 0, count: 16 * 1024)
            while true {
                let count = read(fd, &buffer, buffer.count)
                if count > 0 { continue }
                if count < 0, errno == EINTR { continue }
                if count < 0, errno == EAGAIN { return }
                source.cancel()
                source.setEventHandler(handler: nil)
                return
            }
        }
        source.resume()
    }
}
