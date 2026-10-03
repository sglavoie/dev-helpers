import AppKit
import HeartbeatCore
import Observation
import SwiftUI

/// The Pi's journal errors for the last hour, fetched over ssh when the window opens, every 30 s while it stays
/// open, and on Refresh.
@MainActor
@Observable
final class PiJournalModel {
    static let refreshInterval: Duration = .seconds(30)

    private(set) var host: String
    private(set) var log: PiJournalLog?
    private(set) var error: String?
    private(set) var fetchedAt: Date?
    private(set) var isLoading = false
    /// Keep scrolled to the newest line. Lives here, not in `@State`: the Command Line Tools SDK lacks the
    /// SwiftUI macro plugin `@State` needs.
    var follow = true
    private let client: PiStatusClient
    private var loop: Task<Void, Never>?

    init(host: String, client: PiStatusClient = PiStatusClient()) {
        self.host = host
        self.client = client
    }

    func update(host: String) {
        guard host != self.host else { return }
        self.host = host
        log = nil
        error = nil
        fetchedAt = nil
        reload()
    }

    func start() {
        guard loop == nil else { return reload() }
        loop = Task { [weak self] in
            while !Task.isCancelled {
                self?.reload()
                try? await Task.sleep(for: Self.refreshInterval)
            }
        }
    }

    func stop() {
        loop?.cancel()
        loop = nil
    }

    func reload() {
        guard !isLoading else { return }
        isLoading = true
        let host = host, client = client
        Task {
            let result = await Task.detached(priority: .utility) { client.journal(host: host) }.value
            isLoading = false
            guard host == self.host else { return reload() }
            switch result {
            case .success(let log):
                self.log = log
                fetchedAt = Date()
                error = nil
            case .failure(let failure):
                error = failure.description
            }
        }
    }
}

struct PiJournalView: View {
    @Bindable var model: PiJournalModel

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            if let error = model.error, model.log != nil {
                Label("Refresh failed — showing results fetched at \(model.fetchedAt?.formatted(date: .abbreviated, time: .standard) ?? "an unknown time"). \(error)",
                      systemImage: "exclamationmark.triangle")
                    .font(.callout)
                    .foregroundStyle(.orange)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(8)
                Divider()
            }
            content
            Divider()
            footer
        }
        .frame(minWidth: 520, minHeight: 300)
    }

    private var header: some View {
        HStack {
            Text("ssh \(model.host) \(PiStatusClient.journalCommand)")
                .font(.callout)
                .foregroundStyle(.secondary)
                .lineLimit(1)
                .truncationMode(.middle)
                .textSelection(.enabled)
            Spacer()
            if model.isLoading { ProgressView().controlSize(.small) }
            Toggle("Follow", isOn: $model.follow)
                .toggleStyle(.checkbox)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
    }

    @ViewBuilder private var content: some View {
        if let error = model.error, model.log == nil {
            ContentUnavailableView("Cannot read the Pi's journal", systemImage: "exclamationmark.triangle",
                                   description: Text(error))
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if let log = model.log, log.lines.isEmpty {
            ContentUnavailableView(model.error == nil ? "No errors in the last hour" : "No errors in the saved results",
                                   systemImage: "checkmark.circle")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if model.log == nil {
            ProgressView("Asking the Pi…")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            ScrollViewReader { proxy in
                ScrollView {
                    Text(model.log?.lines.joined(separator: "\n") ?? "")
                        .font(.system(size: 11, design: .monospaced))
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(8)
                    Color.clear.frame(height: 1).id("end")
                }
                .defaultScrollAnchor(.top, for: .alignment)
                .defaultScrollAnchor(.bottom, for: .initialOffset)
                .onChange(of: model.log) {
                    if model.follow { proxy.scrollTo("end", anchor: .bottom) }
                }
                .onChange(of: model.follow) {
                    if model.follow { proxy.scrollTo("end", anchor: .bottom) }
                }
            }
        }
    }

    private var footer: some View {
        HStack {
            Text(summary)
                .font(.callout)
                .foregroundStyle(.secondary)
            Spacer()
            Button("Copy") {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(model.log?.lines.joined(separator: "\n") ?? "", forType: .string)
            }
            .disabled(model.log?.lines.isEmpty ?? true)
            Button("Refresh") { model.reload() }
                .disabled(model.isLoading)
                .keyboardShortcut("r")
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
    }

    private var summary: String {
        var parts: [String] = []
        if let log = model.log {
            let count = "\(log.lines.count) \(log.lines.count == 1 ? "line" : "lines")"
            parts.append(log.truncated ? "Newest \(count) (older ones cut)" : count)
        }
        if let fetchedAt = model.fetchedAt {
            parts.append("fetched " + fetchedAt.formatted(date: .omitted, time: .standard))
        }
        return parts.isEmpty ? "" : parts.joined(separator: " · ") + " — refreshes every 30 s"
    }
}
