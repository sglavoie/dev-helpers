import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct LanDevicesTests {
    static let host = "pi"
    static let now = Date(timeIntervalSince1970: 1_791_500_000)
    static let json = """
    {
     "7e:cd:bf:a0:7b:e6": {"status": "pending", "ip": "192.168.1.66", "first_seen": 1791490000, "last_seen": 1791499900, "alerted": 1791499900},
     "a4:fc:14:0c:ce:54": {"status": "approved", "ip": "192.168.1.68", "name": "Mac Studio", "first_seen": 1791490000, "last_seen": 1791499990},
     "e6:17:c5:93:6c:6d": {"status": "rejected", "ip": "192.168.1.64", "note": "not ours", "first_seen": 1791490000, "last_seen": 1791491000},
     "d8:31:39:d3:b2:ad": {"status": "approved", "ip": "192.168.1.254", "name": "Telmex router", "first_seen": 1791490000, "last_seen": 1791499990},
     "00:11:22:33:44:55": {"status": "maybe"},
     "bad": 3
    }
    """

    @Test func parsesKnownEntriesUnapprovedFirstThenRecentThenByAddress() throws {
        let devices = try LanDevicesParser.parse(Data(Self.json.utf8))
        #expect(devices.map(\.ip) == ["192.168.1.64", "192.168.1.66", "192.168.1.68", "192.168.1.254"])
        #expect(devices[0].note == "not ours")
        #expect(devices[2].name == "Mac Studio")
        #expect(devices[1].lastSeen == Date(timeIntervalSince1970: 1_791_499_900))
        #expect(throws: LanDevicesParser.ParseError.self) { try LanDevicesParser.parse(Data("[]".utf8)) }
    }

    @Test func privateAddressesHaveTheLocalBitSet() {
        #expect(LanDevice(mac: "e6:17:c5:93:6c:6d", status: .pending).isPrivate)
        #expect(!LanDevice(mac: "a4:fc:14:0c:ce:54", status: .pending).isPrivate)
    }

    @Test func severityIsRedOnlyForARejectedDeviceSeenThisHour() throws {
        let devices = try LanDevicesParser.parse(Data(Self.json.utf8))
        let check = { (devices: [LanDevice]) in LanDevicesCheck(host: Self.host, checkedAt: Self.now, result: .devices(devices)) }
        #expect(check(devices).severity(now: Self.now) == .warning)
        var back = devices
        back[0].lastSeen = Self.now.addingTimeInterval(-60)
        #expect(check(back).severity(now: Self.now) == .failing)
        #expect(check(devices.filter { $0.status == .approved }).severity(now: Self.now) == .ok)
        #expect(LanDevicesCheck(host: Self.host, checkedAt: Self.now, result: .unreachable("x")).severity(now: Self.now) == nil)
    }

    @Test func decisionsQuoteEveryWordForTheRemoteShell() {
        #expect(LanDevicesClient.decisionCommand(.approve, mac: "7e:cd:bf:a0:7b:e6", text: "Ana's iPhone")
            == #"python3 pi-monitoring/lan_devices.py 'approve' '7e:cd:bf:a0:7b:e6' 'Ana'\''s iPhone'"#)
        #expect(LanDevicesClient.decisionCommand(.reject, mac: "m", text: "")
            == "python3 pi-monitoring/lan_devices.py 'reject' 'm'")
        #expect(LanDevicesClient.decisionCommand(.forget, mac: "m", text: "ignored")
            == "python3 pi-monitoring/lan_devices.py 'forget' 'm'")
        #expect(LanDevicesClient.quote("$(rm -rf ~)") == "'$(rm -rf ~)'")
    }

    @Test func listRunsOverBatchModeSsh() {
        let runner = FakeRunner(fallback: result(0, stdout: Self.json))
        let listed = LanDevicesClient(runner: runner).list(host: Self.host)
        #expect(runner.recorded == [[
            "/usr/bin/ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "--", Self.host,
            "python3 pi-monitoring/lan_devices.py list --json",
        ]])
        guard case .devices(let devices) = listed else {
            Issue.record("expected devices, got \(listed)")
            return
        }
        #expect(devices.count == 4)
    }

    @Test func failuresAreUnreachableOrUnreadable() {
        func list(_ response: CommandResult?) -> LanDevicesResult {
            LanDevicesClient(runner: FakeRunner(fallback: response)).list(host: Self.host)
        }
        #expect(list(result(255, stderr: "ssh: connect to host pi port 22: Operation timed out\n"))
            == .unreachable("ssh: connect to host pi port 22: Operation timed out"))
        #expect(list(result(2, stderr: "python3: can't open file 'pi-monitoring/lan_devices.py'\n"))
            == .unreadable("python3: can't open file 'pi-monitoring/lan_devices.py'"))
        #expect(list(result(0, stdout: "nope")) == .unreadable("lan_devices.py output: not a JSON object"))
        #expect(list(nil) == .unreachable("could not start /usr/bin/ssh: no fake response"))
    }

    @Test func decideReturnsTheScriptsLineOrItsExitMessage() {
        let ok = FakeRunner(fallback: result(0, stdout: "Approved 192.168.1.66  7e:cd:bf:a0:7b:e6 (My iPhone)\n"))
        #expect(LanDevicesClient(runner: ok).decide(host: Self.host, mac: "7e:cd:bf:a0:7b:e6", .approve, text: "My iPhone")
            == .success("Approved 192.168.1.66  7e:cd:bf:a0:7b:e6 (My iPhone)"))
        let missing = FakeRunner(fallback: result(1, stderr: "m has not been seen on the LAN; pis devices lists the devices seen\n"))
        #expect(LanDevicesClient(runner: missing).decide(host: Self.host, mac: "m", .forget)
            == .failure(LanDevicesError(description: "m has not been seen on the LAN; pis devices lists the devices seen")))
    }

    @Test func rowAndDeviceText() throws {
        let formatter = StatusFormatter()
        let devices = try LanDevicesParser.parse(Data(Self.json.utf8))
        let check = LanDevicesCheck(host: Self.host, checkedAt: Self.now, result: .devices(devices))
        #expect(formatter.lanDevicesRow(nil) == "LAN devices — checking…")
        #expect(formatter.lanDevicesRow(check) == "LAN devices — 1 pending · 1 rejected · 2 approved")
        let approved = LanDevicesCheck(host: Self.host, checkedAt: Self.now,
                                       result: .devices(devices.filter { $0.status == .approved }))
        #expect(formatter.lanDevicesRow(approved) == "LAN devices — all 2 approved")
        #expect(formatter.lanDevicesRow(LanDevicesCheck(host: Self.host, checkedAt: Self.now, result: .unreachable("x")))
            == "LAN devices — unknown")
        #expect(formatter.lanDeviceTitle(devices[2], now: Self.now).hasPrefix("192.168.1.68  Mac Studio · "))
        #expect(formatter.lanDeviceTitle(devices[1], now: Self.now).hasPrefix("192.168.1.66  7e:cd:bf:a0:7b:e6 · "))
        let info = formatter.lanDeviceInfo(devices[1], now: Self.now)
        #expect(info[0] == "Pending")
        #expect(info[1] == "MAC: 7e:cd:bf:a0:7b:e6 (private, randomized)")
    }
}
