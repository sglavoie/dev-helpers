import Foundation

/// One device the Pi's lan-devices timer has seen, from `lan_devices.py list --json`.
public struct LanDevice: Equatable, Sendable {
    public enum Status: String, Sendable, CaseIterable {
        case pending, rejected, approved
    }

    public var mac: String
    public var status: Status
    public var ip: String?
    /// Given on approval.
    public var name: String?
    /// Given on rejection.
    public var note: String?
    public var firstSeen: Date?
    public var lastSeen: Date?

    public init(mac: String, status: Status, ip: String? = nil, name: String? = nil, note: String? = nil,
                firstSeen: Date? = nil, lastSeen: Date? = nil) {
        self.mac = mac
        self.status = status
        self.ip = ip
        self.name = name
        self.note = note
        self.firstSeen = firstSeen
        self.lastSeen = lastSeen
    }

    /// A locally administered address: the per-network random MAC of a phone, tablet or recent laptop.
    public var isPrivate: Bool {
        guard let first = UInt8(mac.prefix(2), radix: 16) else { return false }
        return first & 0x02 != 0
    }

    /// Pending and rejected first, then the most recently seen, then by address.
    static func menuOrder(_ a: LanDevice, _ b: LanDevice) -> Bool {
        let rank: [Status: Int] = [.rejected: 0, .pending: 1, .approved: 2]
        if a.status != b.status { return rank[a.status]! < rank[b.status]! }
        if a.lastSeen != b.lastSeen { return (a.lastSeen ?? .distantPast) > (b.lastSeen ?? .distantPast) }
        return (a.ip ?? "").compare(b.ip ?? "", options: .numeric) == .orderedAscending
    }
}

/// The outcome of asking the Pi for its device list.
public enum LanDevicesResult: Equatable, Sendable {
    case devices([LanDevice])
    case unreachable(String)
    case unreadable(String)
}

public struct LanDevicesCheck: Equatable, Sendable {
    public var host: String
    public var checkedAt: Date
    public var result: LanDevicesResult

    public init(host: String, checkedAt: Date, result: LanDevicesResult) {
        self.host = host
        self.checkedAt = checkedAt
        self.result = result
    }

    public var devices: [LanDevice] {
        if case .devices(let devices) = result { return devices }
        return []
    }

    /// The row's dot: red with a rejected device on the LAN in the last hour, amber with any pending, green when
    /// everything is approved, gray (`nil`) when the list couldn't be read. Never folded into the overall status:
    /// the Pi already sends the phone alert.
    public func severity(now: Date) -> Severity? {
        guard case .devices(let devices) = result else { return nil }
        if devices.contains(where: { $0.status == .rejected && $0.lastSeen.map { now.timeIntervalSince($0) < 3600 } == true }) {
            return .failing
        }
        return devices.contains { $0.status == .pending } ? .warning : .ok
    }
}

public enum LanDevicesParser {
    public struct ParseError: Error, Equatable, CustomStringConvertible {
        public var description: String
    }

    /// `{mac: {status, ip, name, note, first_seen, last_seen, ...}}`, times in Unix seconds. Entries that aren't
    /// objects or have an unknown status are skipped.
    public static func parse(_ data: Data) throws -> [LanDevice] {
        guard let object = try? JSONSerialization.jsonObject(with: data), let map = object as? [String: Any] else {
            throw ParseError(description: "not a JSON object")
        }
        func date(_ value: Any?) -> Date? { (value as? NSNumber).map { Date(timeIntervalSince1970: $0.doubleValue) } }
        func text(_ value: Any?) -> String? { (value as? String).flatMap { $0.isEmpty ? nil : $0 } }
        return map.compactMap { mac, value -> LanDevice? in
            guard let entry = value as? [String: Any], let status = (entry["status"] as? String).flatMap(LanDevice.Status.init)
            else { return nil }
            return LanDevice(mac: mac, status: status, ip: text(entry["ip"]), name: text(entry["name"]),
                             note: text(entry["note"]), firstSeen: date(entry["first_seen"]), lastSeen: date(entry["last_seen"]))
        }.sorted(by: LanDevice.menuOrder)
    }
}

/// A decision on one device, as `lan_devices.py approve|reject|forget` takes it.
public enum LanDeviceDecision: String, Sendable {
    case approve, reject, forget
}

public struct LanDevicesError: Error, Equatable, Sendable, CustomStringConvertible {
    public var description: String

    public init(description: String) {
        self.description = description
    }
}

/// Runs `lan_devices.py` on the Pi over ssh. ssh joins its arguments into one remote shell command, so every
/// argument after the script is single-quoted here.
public struct LanDevicesClient: Sendable {
    public static let script = "python3 pi-monitoring/lan_devices.py"
    public static let listCommand = "\(script) list --json"

    public var runner: any CommandRunning
    public var timeout: TimeInterval

    public init(runner: any CommandRunning = CommandRunner(), timeout: TimeInterval = 30) {
        self.runner = runner
        self.timeout = timeout
    }

    static func argv(host: String, _ command: String) -> [String] {
        [PiStatusClient.sshPath, "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "--", host, command]
    }

    /// POSIX single quotes; a quote inside becomes `'\''`.
    public static func quote(_ word: String) -> String {
        "'" + word.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }

    public static func decisionCommand(_ decision: LanDeviceDecision, mac: String, text: String) -> String {
        let words = [decision.rawValue, mac] + (decision == .forget || text.isEmpty ? [] : [text])
        return ([script] + words.map(quote)).joined(separator: " ")
    }

    /// Blocks for up to `timeout`; call it off the main thread. Never throws.
    public func check(host: String, now: () -> Date = Date.init) -> LanDevicesCheck {
        LanDevicesCheck(host: host, checkedAt: now(), result: list(host: host))
    }

    public func list(host: String) -> LanDevicesResult {
        switch run(host: host, Self.listCommand) {
        case .failure(.unreachable(let detail)): return .unreachable(detail)
        case .failure(.failed(let detail)): return .unreadable(detail)
        case .success(let result):
            do { return .devices(try LanDevicesParser.parse(result.stdout)) } catch {
                return .unreadable("lan_devices.py output: \(error)")
            }
        }
    }

    /// Approves, rejects or forgets one device; on success, the script's own confirmation line.
    public func decide(host: String, mac: String, _ decision: LanDeviceDecision, text: String = "")
        -> Result<String, LanDevicesError> {
        switch run(host: host, Self.decisionCommand(decision, mac: mac, text: text)) {
        case .failure(.unreachable(let detail)), .failure(.failed(let detail)):
            return .failure(LanDevicesError(description: detail))
        case .success(let result):
            return .success(PiStatusClient.firstLine(result.stdoutText))
        }
    }

    private enum RunFailure: Error {
        case unreachable(String)
        case failed(String)
    }

    private func run(host: String, _ command: String) -> Result<CommandResult, RunFailure> {
        let result: CommandResult
        do {
            result = try runner.run(Self.argv(host: host, command), timeout: timeout)
        } catch {
            return .failure(.unreachable("\(error)"))
        }
        if result.timedOut { return .failure(.unreachable("ssh timed out after \(Int(timeout)) s")) }
        // The script's own errors (an unknown device, say) end with sys.exit(message), so the last stderr line is
        // the message rather than a traceback's first line.
        let stderr = result.stderrText.split(whereSeparator: \.isNewline).last
            .map { $0.trimmingCharacters(in: .whitespaces) } ?? ""
        switch result.termination {
        case .exited(0): return .success(result)
        case .exited(PiStatusClient.sshFailureExitCode):
            return .failure(.unreachable(stderr.isEmpty ? "ssh exited with status 255" : stderr))
        case .exited(let code):
            return .failure(.failed(stderr.isEmpty ? "lan_devices.py exited with status \(code)" : stderr))
        case .signaled(let signal):
            return .failure(.unreachable("ssh killed by signal \(signal)"))
        }
    }
}

extension StatusFormatter {
    /// "LAN devices — 1 pending · 4 approved", "LAN devices — all 5 approved", "LAN devices — unknown".
    public func lanDevicesRow(_ check: LanDevicesCheck?) -> String {
        guard let check else { return "LAN devices — checking…" }
        guard case .devices(let devices) = check.result else { return "LAN devices — unknown" }
        if devices.isEmpty { return "LAN devices — none seen yet" }
        let counts = LanDevice.Status.allCases.map { status in (status, devices.filter { $0.status == status }.count) }
        if counts.allSatisfy({ $0.0 == .approved || $0.1 == 0 }) { return "LAN devices — all \(devices.count) approved" }
        return "LAN devices — " + counts.filter { $0.1 > 0 }.map { "\($0.1) \($0.0.rawValue)" }.joined(separator: " · ")
    }

    /// "192.168.1.64  Ana's iPhone · 3 min ago", the name (or note, or MAC) after the address.
    public func lanDeviceTitle(_ device: LanDevice, now: Date) -> String {
        let label = device.name ?? device.note ?? device.mac
        return "\(device.ip ?? "?")  \(label)" + (device.lastSeen.map { " · \(ago($0, now: now))" } ?? "")
    }

    /// The device's submenu facts.
    public func lanDeviceInfo(_ device: LanDevice, now: Date) -> [String] {
        var lines = ["\(device.status.rawValue.capitalized)" + (device.name.map { ": \($0)" } ?? device.note.map { ": \($0)" } ?? "")]
        lines.append("MAC: \(device.mac)" + (device.isPrivate ? " (private, randomized)" : ""))
        if let ip = device.ip { lines.append("IP: \(ip)") }
        if let lastSeen = device.lastSeen { lines.append("Last seen: \(ago(lastSeen, now: now))") }
        if let firstSeen = device.firstSeen { lines.append("First seen: \(stamp(firstSeen))") }
        return lines
    }

    /// Why the list couldn't be read, with the host and check age.
    public func lanDevicesMenuInfo(_ check: LanDevicesCheck?, now: Date) -> [String] {
        guard let check else { return ["Not checked yet"] }
        var lines: [String] = []
        switch check.result {
        case .devices: break
        case .unreachable(let detail), .unreadable(let detail): lines.append(detail)
        }
        lines.append("Host: \(check.host) · checked \(ago(check.checkedAt, now: now))")
        return lines
    }
}
