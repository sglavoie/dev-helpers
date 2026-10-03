import AppKit
import HeartbeatCore
import Observation
import SwiftUI

/// One agent's schedule editor: the draft, the maxAge and Run Now options, and the save state.
@MainActor
@Observable
final class ScheduleEditorModel {
    /// What the editor knows about the agent when it opens.
    struct Context {
        enum MaxAge: Equatable {
            /// No override: the derived default follows the schedule by itself.
            case none
            /// `agents.<label>.maxAgeSeconds` in config.json, which the editor can update.
            case inFile(Int)
            /// An override the editor can't write (built-in defaults, or a config that failed to load).
            case notEditable(Int, reason: String)
        }

        var label: String
        var name: String
        var agent: AgentDefinition
        var isLoaded: Bool
        var isRunning: Bool
        var maxAge: MaxAge

        var isStowManaged: Bool { ScheduleApplier.isStowManaged(agent) }
    }

    enum SaveOutcome {
        case saved
        /// Nothing was written; the window stays open with the reason.
        case notSaved(String)
        /// The plist was written but a later step failed; the window closes and an alert explains.
        case savedWithProblem(String)
    }

    let context: Context
    var draft: ScheduleDraft
    var updateMaxAge = true
    var runNow = false
    private(set) var isSaving = false
    private(set) var saveError: String?

    @ObservationIgnored var save: (ScheduleEditorModel) async -> SaveOutcome = { _ in .notSaved("not wired") }
    @ObservationIgnored var close: () -> Void = {}
    @ObservationIgnored var openConfig: () -> Void = {}

    init(context: Context) {
        self.context = context
        draft = ScheduleDraft(agent: context.agent)
    }

    /// The new `maxAgeSeconds` to offer, when config.json has one the new schedule makes too tight or too loose.
    var suggestedMaxAge: (current: Int, suggested: Int)? {
        guard let current = maxAgeOverride, let schedule = draft.schedule,
              let suggested = MaxAgeSuggestion.suggested(for: schedule, now: Date(), calendar: .current),
              MaxAgeSuggestion.needsUpdate(current: current, suggested: suggested) else { return nil }
        return (current, suggested)
    }

    private var maxAgeOverride: Int? {
        switch context.maxAge {
        case .none: nil
        case .inFile(let value), .notEditable(let value, _): value
        }
    }

    var change: ScheduleApplier.Change? {
        guard let schedule = draft.schedule else { return nil }
        var maxAge: Int?
        if case .inFile = context.maxAge, updateMaxAge { maxAge = suggestedMaxAge?.suggested }
        return ScheduleApplier.Change(schedule: schedule, throttle: draft.throttleUpdate, maxAgeSeconds: maxAge)
    }

    var canSave: Bool { !isSaving && draft.isValid && draft.hasChanges }

    func submit() {
        guard canSave else { return }
        isSaving = true
        saveError = nil
        Task {
            let outcome = await save(self)
            isSaving = false
            if case .notSaved(let reason) = outcome { saveError = reason }
        }
    }
}

struct ScheduleEditorView: View {
    @Bindable var model: ScheduleEditorModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Form {
                Section {
                    Picker("Trigger", selection: $model.draft.kind) {
                        ForEach(ScheduleDraft.Kind.allCases, id: \.self) { Text($0.title).tag($0) }
                    }
                    .pickerStyle(.segmented)
                    fields
                }
                preview
                notes
            }
            .formStyle(.grouped)
            .disabled(model.isSaving)
            Divider()
            footer
        }
        .frame(minWidth: 520, minHeight: 420)
    }

    // MARK: Fields

    @ViewBuilder private var fields: some View {
        switch model.draft.kind {
        case .interval:
            HStack {
                TextField("Every", value: $model.draft.intervalValue, format: .number)
                    .frame(maxWidth: 160)
                Picker("", selection: $model.draft.intervalUnit) {
                    ForEach(ScheduleDraft.IntervalUnit.allCases, id: \.self) { Text($0.title).tag($0) }
                }
                .labelsHidden()
                .fixedSize()
            }
        case .calendar:
            ForEach(model.draft.calendarEntries.indices, id: \.self) { index in
                calendarRow(index)
            }
            Button("Add Time") { model.draft.calendarEntries.append(CalendarEntry(minute: 0, hour: 9)) }
        case .watchPaths:
            ForEach(model.draft.watchPaths.indices, id: \.self) { index in
                HStack {
                    TextField("Path", text: pathBinding(index))
                        .labelsHidden()
                    Button { model.draft.watchPaths.remove(at: index) } label: { Image(systemName: "minus.circle") }
                        .buttonStyle(.borderless)
                }
            }
            HStack {
                Button("Add Path…", action: choosePaths)
                Button("Add Empty Row") { model.draft.watchPaths.append("") }
            }
            TextField("Throttle (seconds, empty for launchd's 10 s)", text: throttleText)
            ForEach(model.context.agent.queueDirectories, id: \.self) { path in
                LabeledContent("QueueDirectories", value: path)
                    .foregroundStyle(.secondary)
            }
        case .none:
            Text("No timed or path trigger. RunAtLoad, KeepAlive or Run Now still start the agent.")
                .foregroundStyle(.secondary)
        }
    }

    private func calendarRow(_ index: Int) -> some View {
        HStack(spacing: 6) {
            fieldPicker("Month", \.month, index, options: (1...12).map { ($0, ScheduleDescription.monthNames[$0 - 1]) })
            fieldPicker("Day", \.day, index, options: (1...31).map { ($0, "\($0)") })
            fieldPicker("Weekday", \.weekday, index, options: weekdayOptions(index))
            fieldPicker("Hour", \.hour, index, options: (0...23).map { ($0, pad($0)) })
            fieldPicker("Minute", \.minute, index, options: (0...59).map { ($0, pad($0)) })
            Button { model.draft.calendarEntries.remove(at: index) } label: { Image(systemName: "minus.circle") }
                .buttonStyle(.borderless)
                .disabled(model.draft.calendarEntries.count == 1)
        }
    }

    /// Sunday is 0; a plist's 7 (also Sunday) stays selectable as written.
    private func weekdayOptions(_ index: Int) -> [(Int, String)] {
        var options = (0...6).map { ($0, ScheduleDescription.weekdayNames[$0]) }
        if model.draft.calendarEntries.indices.contains(index), model.draft.calendarEntries[index].weekday == 7 { options.append((7, "Sun (7)")) }
        return options
    }

    private func fieldPicker(_ title: String, _ key: WritableKeyPath<CalendarEntry, Int?>, _ index: Int,
                             options: [(Int, String)]) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(title).font(.caption).foregroundStyle(.secondary)
            Picker(title, selection: entryBinding(index, key)) {
                Text("Any").tag(Int?.none)
                ForEach(options, id: \.0) { Text($0.1).tag(Int?.some($0.0)) }
            }
            .labelsHidden()
            .fixedSize()
        }
    }

    /// Index bindings that tolerate a row removed while its view is still being torn down.
    private func entryBinding(_ index: Int, _ key: WritableKeyPath<CalendarEntry, Int?>) -> Binding<Int?> {
        Binding(
            get: { model.draft.calendarEntries.indices.contains(index) ? model.draft.calendarEntries[index][keyPath: key] : nil },
            set: { if model.draft.calendarEntries.indices.contains(index) { model.draft.calendarEntries[index][keyPath: key] = $0 } })
    }

    private func pathBinding(_ index: Int) -> Binding<String> {
        Binding(
            get: { model.draft.watchPaths.indices.contains(index) ? model.draft.watchPaths[index] : "" },
            set: { if model.draft.watchPaths.indices.contains(index) { model.draft.watchPaths[index] = $0 } })
    }

    private var throttleText: Binding<String> {
        Binding(
            get: { model.draft.throttleSeconds.map(String.init) ?? "" },
            set: { text in
                let trimmed = text.trimmingCharacters(in: .whitespaces)
                model.draft.throttleSeconds = trimmed.isEmpty ? nil : Int(trimmed) ?? model.draft.throttleSeconds
            })
    }

    private func choosePaths() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = true
        panel.prompt = "Watch"
        guard panel.runModal() == .OK else { return }
        model.draft.watchPaths.removeAll { $0.trimmingCharacters(in: .whitespaces).isEmpty }
        model.draft.watchPaths += panel.urls.map(\.path).filter { !model.draft.watchPaths.contains($0) }
    }

    private func pad(_ value: Int) -> String { value < 10 ? "0\(value)" : "\(value)" }

    // MARK: Preview and notes

    @ViewBuilder private var preview: some View {
        Section("Preview") {
            if let preview = model.draft.preview(now: Date(), calendar: .current) {
                Text(preview.description).font(.headline)
                if !preview.upcoming.isEmpty {
                    Text("Next: " + preview.upcoming.map { $0.formatted(.dateTime.weekday().month().day().hour().minute()) }
                        .joined(separator: " · "))
                        .foregroundStyle(.secondary)
                }
                if let note = preview.note { Text(note).foregroundStyle(.secondary) }
            }
            ForEach(model.draft.errors, id: \.self) { error in
                Label(error, systemImage: "exclamationmark.circle").foregroundStyle(.red)
            }
        }
    }

    @ViewBuilder private var notes: some View {
        let context = model.context
        Section {
            if let maxAge = model.suggestedMaxAge {
                switch context.maxAge {
                case .inFile:
                    Toggle("Update maxAgeSeconds in config.json: \(duration(maxAge.current)) → \(duration(maxAge.suggested))",
                           isOn: $model.updateMaxAge)
                        .toggleStyle(.checkbox)
                case .notEditable(_, let reason):
                    HStack {
                        Label("maxAgeSeconds \(duration(maxAge.current)) no longer fits; about \(duration(maxAge.suggested)) would. \(reason)",
                              systemImage: "exclamationmark.triangle")
                            .foregroundStyle(.orange)
                        Button("Open Config…", action: model.openConfig)
                    }
                case .none:
                    EmptyView()
                }
            }
            if context.isLoaded, !context.agent.runAtLoad {
                Toggle("Run now after saving", isOn: $model.runNow).toggleStyle(.checkbox)
            }
            if !context.isStowManaged {
                Label("Not stow-managed: an installer wrote this plist, and reinstalling may revert the change.",
                      systemImage: "exclamationmark.triangle")
                    .foregroundStyle(.orange)
            }
            if context.isLoaded, context.isRunning || context.agent.keepAlive != .none {
                Label("Saving reloads the agent, which stops its running process.", systemImage: "arrow.clockwise")
                    .foregroundStyle(.secondary)
            } else if context.isLoaded, context.agent.runAtLoad {
                Label("Saving reloads the agent, and RunAtLoad runs it right away.", systemImage: "arrow.clockwise")
                    .foregroundStyle(.secondary)
            } else if !context.isLoaded {
                Label("Not loaded: the new schedule applies the next time you choose Load.", systemImage: "pause.circle")
                    .foregroundStyle(.secondary)
            }
            LabeledContent("Writes to", value: context.agent.resolvedPlistPath)
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
        }
    }

    private func duration(_ seconds: Int) -> String { ScheduleDescription.duration(seconds) }

    // MARK: Footer

    private var footer: some View {
        HStack {
            if let error = model.saveError {
                Label(error, systemImage: "xmark.octagon").foregroundStyle(.red).lineLimit(3)
            }
            Spacer()
            if model.isSaving { ProgressView().controlSize(.small) }
            Button("Cancel", action: model.close)
                .keyboardShortcut(.cancelAction)
                .disabled(model.isSaving)
            Button(model.context.isLoaded ? "Save & Reload" : "Save", action: model.submit)
                .keyboardShortcut(.defaultAction)
                .disabled(!model.canSave)
        }
        .padding(12)
    }
}
