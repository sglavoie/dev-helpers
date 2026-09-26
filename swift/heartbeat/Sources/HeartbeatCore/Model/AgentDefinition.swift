/// A LaunchAgent as declared by its plist.
public struct AgentDefinition: Equatable, Sendable {
    public var label: String
    /// Path of the plist as discovered (the symlink itself for stowed agents); what `launchctl bootstrap` takes.
    public var plistPath: String
    /// Fully resolved plist path (the stow target for symlinks, otherwise `plistPath`).
    public var resolvedPlistPath: String
    public var program: String?
    public var programArguments: [String]
    public var schedule: Schedule
    public var runAtLoad: Bool
    public var keepAlive: KeepAlivePolicy
    public var throttleInterval: Int?
    public var disabled: Bool
    public var standardOutPath: String?
    public var standardErrorPath: String?
    public var workingDirectory: String?
    public var processType: String?

    public init(
        label: String,
        plistPath: String,
        resolvedPlistPath: String? = nil,
        program: String? = nil,
        programArguments: [String] = [],
        schedule: Schedule = .none,
        runAtLoad: Bool = false,
        keepAlive: KeepAlivePolicy = .none,
        throttleInterval: Int? = nil,
        disabled: Bool = false,
        standardOutPath: String? = nil,
        standardErrorPath: String? = nil,
        workingDirectory: String? = nil,
        processType: String? = nil
    ) {
        self.label = label
        self.plistPath = plistPath
        self.resolvedPlistPath = resolvedPlistPath ?? plistPath
        self.program = program
        self.programArguments = programArguments
        self.schedule = schedule
        self.runAtLoad = runAtLoad
        self.keepAlive = keepAlive
        self.throttleInterval = throttleInterval
        self.disabled = disabled
        self.standardOutPath = standardOutPath
        self.standardErrorPath = standardErrorPath
        self.workingDirectory = workingDirectory
        self.processType = processType
    }

    /// The executable launchd runs: `Program`, else the first `ProgramArguments` element.
    public var executable: String? {
        program ?? programArguments.first
    }

    /// Distinct stdout/stderr log paths, in that order, skipping `/dev/null`.
    public var logPaths: [String] {
        var paths: [String] = []
        for path in [standardOutPath, standardErrorPath].compactMap({ $0 })
        where path != "/dev/null" && !paths.contains(path) {
            paths.append(path)
        }
        return paths
    }
}
