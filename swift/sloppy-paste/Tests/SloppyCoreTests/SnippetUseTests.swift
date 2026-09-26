import Foundation
import Synchronization
import Testing
@testable import SloppyCore

/// Thread-safe call log for the injected closures.
final class CallLog: Sendable {
    private let entries = Mutex<[String]>([])

    func append(_ entry: String) { entries.withLock { $0.append(entry) } }
    var all: [String] { entries.withLock { $0 } }
}

struct TestFailure: Error, Equatable {
    var message: String
}

@Suite struct SnippetUseTests {
    let calls = CallLog()
    let logs = CallLog()

    private func use(recordFails: Bool = false) -> SnippetUse {
        SnippetUse(
            record: { [calls] id, values in
                if recordFails { throw TestFailure(message: "contains snippet content and Ada") }
                calls.append("record \(id) \(values.map { "\($0.key)=\($0.value)/\($0.isSaved)" })")
            },
            log: { [logs] in logs.append($0) }
        )
    }

    @Test func recordFailureLogsOnlySafeMetadata() async {
        await use(recordFails: true).recordUseBestEffort(
            snippetID: "snippet-123", placeholderValues: [.init(key: "name", value: "Ada")])
        #expect(logs.all == ["Unable to record snippet use: snippet-123"])
        #expect(!logs.all.contains { $0.contains("Ada") })
    }

    @Test func preparationFailureStopsBeforeClipboardAndTracking() async {
        let ok = await use().run(
            snippetID: "snippet-123",
            prepare: { () throws -> String in throw TestFailure(message: "Could not prepare") },
            primaryOperation: { [calls] _ in calls.append("primary") },
            onPreparationFailure: { [calls] error in calls.append("prep-failed \((error as? TestFailure)?.message ?? "")") },
            onPrimaryFailure: { [calls] _ in calls.append("primary-failed") }
        )
        #expect(!ok)
        #expect(calls.all == ["prep-failed Could not prepare"])
    }

    @Test func clipboardFailureStopsBeforeTracking() async {
        let ok = await use().run(
            snippetID: "snippet-123",
            prepare: { "prepared content" },
            primaryOperation: { _ in throw TestFailure(message: "Clipboard unavailable") },
            onPreparationFailure: { [calls] _ in calls.append("prep-failed") },
            onPrimaryFailure: { [calls] error in calls.append("primary-failed \((error as? TestFailure)?.message ?? "")") }
        )
        #expect(!ok)
        #expect(calls.all == ["primary-failed Clipboard unavailable"])
    }

    @Test func tracksExactlyOnceAfterSuccess() async {
        let ok = await use().run(
            snippetID: "snippet-123",
            placeholderValues: [.init(key: "name", value: "Ada", isSaved: true)],
            prepare: { "prepared content" },
            primaryOperation: { [calls] content in calls.append("primary \(content)") },
            onPreparationFailure: { _ in },
            onPrimaryFailure: { _ in }
        )
        #expect(ok)
        #expect(calls.all == ["primary prepared content", "record snippet-123 [\"name=Ada/true\"]"])
    }

    @Test func failureHandlerErrorsAreSwallowedAndLogged() async {
        let ok = await use().run(
            snippetID: "snippet-123",
            prepare: { "x" },
            primaryOperation: { _ in throw TestFailure(message: "nope") },
            onPreparationFailure: { _ in },
            onPrimaryFailure: { _ in throw TestFailure(message: "toast failed") }
        )
        #expect(!ok)
        #expect(logs.all == ["Unable to show primary clipboard failure"])
    }

    @Test func trackingFailureKeepsTheSuccess() async {
        let ok = await use(recordFails: true).run(
            snippetID: "snippet-123",
            prepare: { "x" },
            primaryOperation: { _ in },
            onPreparationFailure: { _ in },
            onPrimaryFailure: { _ in }
        )
        #expect(ok)
        #expect(logs.all == ["Unable to record snippet use: snippet-123"])
    }

    @Test func copyContentCopiesAndRecordsOneUse() async {
        let ok = await use().copyContent(
            snippetID: "snippet-123",
            content: "stored snippet content",
            copy: { [calls] content in calls.append("copy \(content)") },
            onPrimaryFailure: { _ in }
        )
        #expect(ok)
        #expect(calls.all == ["copy stored snippet content", "record snippet-123 []"])
    }

    @Test func copyContentFailureRecordsNothing() async {
        let ok = await use().copyContent(
            snippetID: "snippet-123",
            content: "stored snippet content",
            copy: { _ in throw TestFailure(message: "Clipboard unavailable") },
            onPrimaryFailure: { [calls] error in calls.append("failed \((error as? TestFailure)?.message ?? "")") }
        )
        #expect(!ok)
        #expect(calls.all == ["failed Clipboard unavailable"])
    }
}
