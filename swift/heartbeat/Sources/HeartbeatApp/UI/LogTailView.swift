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
    var selectedPath: String {
        didSet { if selectedPath != oldValue { reload() } }
    }
    private(set) var tail: LogTail?
    /// Keep scrolled to the newest line. Lives here, not in `@State`: the Command Line Tools SDK lacks the
    /// SwiftUI macro plugin `@State` needs.
    var follow = true
    private(set) var error: String?
    private var loop: Task<Void, Never>?

    init(paths: [String]) {
        self.paths = paths
        selectedPath = paths.first ?? ""
    }

    func update(paths: [String]) {
        self.paths = paths
        if !paths.contains(selectedPath) { selectedPath = paths.first ?? "" }
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
        let path = selectedPath
        guard !path.isEmpty else {
            tail = nil
            error = "This agent has no StandardOutPath or StandardErrorPath."
            return
        }
        Task {
            let result = await Task.detached { Result { try LogTail.read(path: path) } }.value
            guard path == selectedPath else { return }
            switch result {
            case .success(let tail):
                self.tail = tail
                error = nil
            case .failure(let failure):
                tail = nil
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
                        Text(URL(fileURLWithPath: path).lastPathComponent).tag(path)
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
        if let error = model.error {
            ContentUnavailableView("No log to show", systemImage: "doc.text.magnifyingglass", description: Text(error))
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            ScrollViewReader { proxy in
                ScrollView {
                    Text(model.tail?.text.isEmpty == false ? model.tail!.text : "(empty)")
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
                NSPasteboard.general.setString(model.tail?.text ?? "", forType: .string)
            }
            .disabled(model.tail == nil)
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
        if let modified = tail.modified {
            parts.append("modified " + modified.formatted(date: .abbreviated, time: .standard))
        }
        return parts.joined(separator: " · ") + " — refreshes every 2 s"
    }
}
