import Foundation

/// Failure boundaries for a snippet action: preparation, then one primary
/// clipboard operation, then best-effort use tracking. Log messages carry only
/// safe metadata, never snippet content or placeholder values.
public struct SnippetUse: Sendable {
    /// Records one use of a snippet with its submitted placeholder values.
    public var record: @Sendable (_ snippetID: String, _ placeholderValues: [PlaceholderValueToRecord]) async throws -> Void
    public var log: @Sendable (String) -> Void

    public init(
        record: @escaping @Sendable (String, [PlaceholderValueToRecord]) async throws -> Void,
        log: @escaping @Sendable (String) -> Void = { message in
            FileHandle.standardError.write(Data((message + "\n").utf8))
        }
    ) {
        self.record = record
        self.log = log
    }

    /// Runs an operation whose failure must not change an already-successful outcome.
    public func runBestEffort(_ failureLogMessage: String, _ operation: @Sendable () async throws -> Void) async {
        do {
            try await operation()
        } catch {
            log(failureLogMessage)
        }
    }

    public func recordUseBestEffort(snippetID: String, placeholderValues: [PlaceholderValueToRecord] = []) async {
        await runBestEffort("Unable to record snippet use: \(snippetID)") {
            try await record(snippetID, placeholderValues)
        }
    }

    /// Returns true only when preparation and the primary operation both
    /// succeeded; tracking runs only after that.
    public func run<Prepared: Sendable>(
        snippetID: String,
        placeholderValues: [PlaceholderValueToRecord] = [],
        prepare: @Sendable () async throws -> Prepared,
        primaryOperation: @Sendable (Prepared) async throws -> Void,
        onPreparationFailure: @Sendable (any Error) async throws -> Void,
        onPrimaryFailure: @Sendable (any Error) async throws -> Void
    ) async -> Bool {
        let prepared: Prepared
        do {
            prepared = try await prepare()
        } catch {
            await runBestEffort("Unable to show snippet preparation failure") { try await onPreparationFailure(error) }
            return false
        }

        do {
            try await primaryOperation(prepared)
        } catch {
            await runBestEffort("Unable to show primary clipboard failure") { try await onPrimaryFailure(error) }
            return false
        }

        await recordUseBestEffort(snippetID: snippetID, placeholderValues: placeholderValues)
        return true
    }

    /// Copies full stored content (secondary views) and records one use.
    public func copyContent(
        snippetID: String,
        content: String,
        copy: @Sendable (String) async throws -> Void,
        onPrimaryFailure: @Sendable (any Error) async throws -> Void
    ) async -> Bool {
        await run(
            snippetID: snippetID,
            prepare: { content },
            primaryOperation: copy,
            onPreparationFailure: { _ in },
            onPrimaryFailure: onPrimaryFailure
        )
    }
}
