import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct ReceiptReaderTests {
    static let config = ReceiptConfig(path: "/unused")

    /// Captured from ~/Library/Application Support/pi-backup-fetch/last-run.json.
    static let piBackupFetch = #"""
    {
      "finished": "2026-09-26T18:58:28.660686+00:00",
      "msg": "Newest Mac set 20260926T184625Z; pruned 1",
      "reported": true,
      "status": "up"
    }
    """#

    static func parse(_ json: String, _ config: ReceiptConfig = config) -> ReceiptStatus {
        ReceiptReader.parse(Data(json.utf8), config: config)
    }

    @Test func realReceiptIsOkWithFinishedTime() throws {
        let status = Self.parse(Self.piBackupFetch)
        guard case .ok(let finished) = status else {
            Issue.record("expected ok, got \(status)")
            return
        }
        let date = try #require(finished)
        #expect(abs(date.timeIntervalSince1970 - TimeFixtures.utc("2026-09-26T18:58:28Z").timeIntervalSince1970 - 0.660686) < 0.001)
        #expect(status.finished == finished)
    }

    @Test func reportedFalse() {
        let status = Self.parse(#"{"finished": "2026-09-26T18:58:28Z", "reported": false, "status": "up"}"#)
        #expect(status == .notReported(finished: TimeFixtures.utc("2026-09-26T18:58:28Z")))
    }

    @Test func statusDownWinsOverReported() {
        #expect(Self.parse(#"{"reported": false, "status": "down"}"#) == .badStatus("down", finished: nil))
        #expect(Self.parse(#"{"reported": true, "status": "down"}"#) == .badStatus("down", finished: nil))
    }

    @Test func customKeysAndOkValues() {
        let config = ReceiptConfig(path: "/unused", reportedKey: "pushed", statusKey: "result", okValues: ["ok", "skipped"])
        #expect(Self.parse(#"{"pushed": true, "result": "skipped"}"#, config) == .ok(finished: nil))
        #expect(Self.parse(#"{"pushed": true, "result": "up"}"#, config) == .badStatus("up", finished: nil))
    }

    @Test(arguments: [
        ("[]", "not a JSON object"),
        ("garbage", "not a JSON object"),
        (#"{"reported": true}"#, "no \"status\" key"),
        (#"{"reported": true, "status": 1}"#, "\"status\" is not a string"),
        (#"{"status": "up"}"#, "no \"reported\" key"),
        (#"{"status": "up", "reported": 1}"#, "\"reported\" is not a boolean"),
        (#"{"status": "up", "reported": "true"}"#, "\"reported\" is not a boolean"),
    ])
    func unreadable(_ json: String, _ reason: String) {
        #expect(Self.parse(json) == .unreadable(reason: reason))
    }

    @Test func unparseableFinishedIsNil() {
        #expect(Self.parse(#"{"finished": "yesterday", "reported": true, "status": "up"}"#) == .ok(finished: nil))
    }

    @Test func readsFromDiskAndReportsMissing() throws {
        let directory = FileManager.default.temporaryDirectory.appending(path: "HeartbeatCoreTests-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: directory) }
        let path = directory.appending(path: "last-run.json").path
        #expect(ReceiptReader.read(ReceiptConfig(path: path)) == .missing(path: path))

        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try Data(#"{"reported": false, "status": "up"}"#.utf8).write(to: URL(fileURLWithPath: path))
        #expect(ReceiptReader.read(ReceiptConfig(path: path)) == .notReported(finished: nil))

        #expect(ReceiptReader.read(ReceiptConfig(path: directory.path)).finished == nil)
        if case .unreadable = ReceiptReader.read(ReceiptConfig(path: directory.path)) {} else {
            Issue.record("a directory should be unreadable")
        }
    }
}
