import AppKit
import HeartbeatCore
import Observation
import SwiftUI

/// The last 200 lines of one of an agent's logs, re-read every 2 s while running.
@MainActor
@Observable
final class LogTailModel {
    static let refreshInterval: Duration = .seconds(2)

    private(set) var paths: [String]
    private(set) var standardErrorPath: String?
    var selectedPath: String {
        didSet {
            if selectedPath != oldValue {
                tail = nil
                error = nil
                fetchedAt = nil
                reload()
            }
        }
    }
    private(set) var tail: LogTail?
    private(set) var fetchedAt: Date?
    private(set) var isLoading = false
    var filter = ""
    /// Keep scrolled to the newest line. Lives here, not in `@State`: the Command Line Tools SDK lacks the
    /// SwiftUI macro plugin `@State` needs.
    var follow = true
    private(set) var error: String?
    private var loop: Task<Void, Never>?
    private var requestID = UUID()

    init(paths: [String], standardErrorPath: String? = nil) {
        self.paths = paths
        self.standardErrorPath = standardErrorPath
        selectedPath = paths.first ?? ""
    }

    func update(paths: [String], standardErrorPath: String? = nil) {
        self.paths = paths
        self.standardErrorPath = standardErrorPath
        if !paths.contains(selectedPath) { selectedPath = paths.first ?? "" }
    }

    func title(for path: String) -> String {
        path == standardErrorPath ? "Standard error" : "Standard output"
    }

    var displayedLines: [String] {
        let lines = tail?.lines ?? []
        return filter.isEmpty ? lines : lines.filter { $0.localizedCaseInsensitiveContains(filter) }
    }

    var displayedText: String { displayedLines.joined(separator: "\n") }

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
        let requestID = UUID()
        self.requestID = requestID
        let path = selectedPath
        guard !path.isEmpty else {
            tail = nil
            fetchedAt = nil
            isLoading = false
            error = "This agent has no StandardOutPath or StandardErrorPath."
            return
        }
        isLoading = true
        Task {
            let result = await Task.detached { Result { try LogTail.read(path: path) } }.value
            guard requestID == self.requestID else { return }
            isLoading = false
            switch result {
            case .success(let tail):
                self.tail = tail
                fetchedAt = Date()
                error = nil
            case .failure(let failure):
                error = (failure as? LogTailError)?.description ?? failure.localizedDescription
            }
        }
    }
}

struct LogTailView: View {
    @Bindable var model: LogTailModel
    let openLog: (String) -> Void

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            HStack {
                TextField("Filter displayed lines", text: $model.filter)
                    .textFieldStyle(.roundedBorder)
                    .accessibilityLabel("Filter displayed log lines")
                if !model.filter.isEmpty {
                    Button("Clear") { model.filter = "" }
                }
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
            if let error = model.error, model.tail != nil {
                Label("Refresh failed — showing results fetched at \(model.fetchedAt?.formatted(date: .abbreviated, time: .standard) ?? "an unknown time"). \(error)",
                      systemImage: "exclamationmark.triangle")
                    .font(.callout)
                    .foregroundStyle(.orange)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(8)
            }
            Divider()
            content
            Divider()
            footer
        }
        .frame(minWidth: 520, minHeight: 300)
    }

    private var header: some View {
        HStack {
            if model.paths.count > 1 {
                Picker("Log", selection: $model.selectedPath) {
                    ForEach(model.paths, id: \.self) { path in
                        Text(model.title(for: path)).tag(path)
                    }
                }
                .pickerStyle(.segmented)
                .labelsHidden()
                .fixedSize()
            }
            Text(model.selectedPath)
                .font(.callout)
                .foregroundStyle(.secondary)
                .lineLimit(1)
                .truncationMode(.middle)
                .textSelection(.enabled)
            Spacer()
            Toggle("Follow", isOn: $model.follow)
                .toggleStyle(.checkbox)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
    }

    @ViewBuilder private var content: some View {
        if let error = model.error, model.tail == nil {
            ContentUnavailableView("No log to show", systemImage: "doc.text.magnifyingglass", description: Text(error))
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if model.tail == nil {
            ProgressView("Reading log…")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            ScrollViewReader { proxy in
                ScrollView {
                    Text(model.displayedLines.isEmpty
                         ? (model.filter.isEmpty ? "(empty)" : "No matching lines in the displayed tail")
                         : model.displayedText)
                        .font(.system(size: 11, design: .monospaced))
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(8)
                    Color.clear.frame(height: 1).id("end")
                }
                // Short logs start at the top; long ones open scrolled to the newest line.
                .defaultScrollAnchor(.top, for: .alignment)
                .defaultScrollAnchor(.bottom, for: .initialOffset)
                .onChange(of: model.tail) {
                    if model.follow { proxy.scrollTo("end", anchor: .bottom) }
                }
                .onChange(of: model.follow) {
                    if model.follow { proxy.scrollTo("end", anchor: .bottom) }
                }
                .onChange(of: model.filter) {
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
                NSPasteboard.general.setString(model.displayedText, forType: .string)
            }
            .disabled(model.displayedLines.isEmpty)
            .help("Copy the displayed lines, including the current filter")
            Button("Reveal in Finder") {
                NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: model.selectedPath)])
            }
            .disabled(model.tail == nil)
            Button("Open") { openLog(model.selectedPath) }
                .disabled(model.tail == nil)
                .keyboardShortcut("o")
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
    }

    private var summary: String {
        guard let tail = model.tail else { return "" }
        var parts = [tail.truncated ? "Last \(tail.lines.count) lines" : "\(tail.lines.count) lines",
                     ByteCountFormatter.string(fromByteCount: Int64(tail.fileSize), countStyle: .file)]
        if !model.filter.isEmpty { parts.insert("\(model.displayedLines.count) matching", at: 0) }
        if let modified = tail.modified {
            parts.append("modified " + modified.formatted(date: .abbreviated, time: .standard))
        }
        return parts.joined(separator: " · ") + " — refreshes every 2 s"
    }
}
