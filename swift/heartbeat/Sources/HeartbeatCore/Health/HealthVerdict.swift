import Foundation

/// How bad an agent's health is. Ordered so that `max` is "worst reason wins".
public enum Severity: String, Comparable, Codable, Sendable, CaseIterable {
    /// `hidden` in config: not shown and not counted.
    case hidden
    case ok
    /// Unloaded on purpose (plist `Disabled` or paused by Heartbeat); gray, doesn't color the icon.
    case paused
    /// Amber: something can't be verified or looks off.
    case warning
    /// Red: failed or overdue.
    case failing

    var rank: Int {
        switch self {
        case .hidden: 0
        case .ok: 1
        case .paused: 2
        case .warning: 3
        case .failing: 4
        }
    }

    public static func < (lhs: Severity, rhs: Severity) -> Bool {
        lhs.rank < rhs.rank
    }
}

/// Why a rule fired; stable identifiers for JSON output and notification de-duplication.
public enum HealthReasonCode: String, Codable, Sendable {
    case hidden
    case paused
    case notLoaded
    case unreadableState
    case exitCode
    case signal
    case restarted
    case notRunning
    case overdue
    case cannotVerify
    case healthCheckFailed
    case healthCheckWarning
    case healthCheckStale
    case receiptStatus
    case receiptNotReported
    case receiptUnavailable
}

/// One finding from one rule.
public struct HealthReason: Equatable, Codable, Sendable {
    /// The numbered rule from the plan (1-8).
    public var rule: Int
    public var code: HealthReasonCode
    public var severity: Severity
    public var message: String

    public init(rule: Int, code: HealthReasonCode, severity: Severity, message: String) {
        self.rule = rule
        self.code = code
        self.severity = severity
        self.message = message
    }
}

/// The numbers behind the overdue rule, kept so `heartbeatctl explain` can show its working.
public struct OverdueTrace: Equatable, Codable, Sendable {
    public enum Kind: String, Codable, Sendable {
        /// `StartInterval` or `maxAgeSeconds`: evidence age vs `allowedAge`, once awake since `t0` that long.
        case maxAge
        /// `StartCalendarInterval`: evidence vs `latestSlot`, due by `deadline`.
        case calendar
    }

    public var kind: Kind
    public var evidence: Date?
    public var allowedAge: TimeInterval?
    /// Interval rule: the later of the last wake/boot and when the service was seen loaded.
    public var t0: Date?
    /// Calendar rule: S, the latest slot at or before now.
    public var latestSlot: Date?
    /// Calendar rule: E + grace, after which a missing run is overdue.
    public var deadline: Date?
    /// Why the check didn't apply this time (slot before boot, agent running, ...), if it didn't.
    public var skipped: String?
    public var nextExpected: Date?

    public init(
        kind: Kind, evidence: Date? = nil, allowedAge: TimeInterval? = nil, t0: Date? = nil,
        latestSlot: Date? = nil, deadline: Date? = nil, skipped: String? = nil, nextExpected: Date? = nil
    ) {
        self.kind = kind
        self.evidence = evidence
        self.allowedAge = allowedAge
        self.t0 = t0
        self.latestSlot = latestSlot
        self.deadline = deadline
        self.skipped = skipped
        self.nextExpected = nextExpected
    }
}

/// The evaluated health of one agent at one moment.
public struct HealthVerdict: Equatable, Codable, Sendable {
    public var label: String
    public var severity: Severity
    /// Every finding, worst first. Empty when the agent is healthy.
    public var reasons: [HealthReason]
    /// True when the KeepAlive rule saw no PID this poll; the caller feeds it back as the not-running streak.
    public var keepAliveMissing: Bool
    public var overdue: OverdueTrace?

    public init(label: String, severity: Severity, reasons: [HealthReason] = [], keepAliveMissing: Bool = false,
                overdue: OverdueTrace? = nil) {
        self.label = label
        self.severity = severity
        self.reasons = reasons
        self.keepAliveMissing = keepAliveMissing
        self.overdue = overdue
    }

    /// The worst reason's message, for one-line rows.
    public var summary: String? {
        reasons.first?.message
    }
}
