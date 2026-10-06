import AppKit
import Carbon.HIToolbox
import SloppyCore
import SwiftUI

/// The Settings window's form. Picker preferences share their @AppStorage
/// keys with the views that read them, so changes apply the next time the
/// picker renders.
struct SettingsView: View {
    @Environment(SnippetStore.self) private var store
    @Environment(AccessibilityPermission.self) private var accessibility
    @Environment(HotKeySettings.self) private var hotKeys
    @Environment(LaunchAtLogin.self) private var launchAtLogin
    let importExport: ImportExportController

    @AppStorage("picker.sortOption") private var sortRaw = SortOption.updatedDesc.rawValue
    @AppStorage("picker.showingDetail") private var showingDetail = false
    @AppStorage("picker.showRecentSection") private var showRecentSection = true
    @AppStorage("placeholders.maxDisplayedHistoryValues")
    private var maxDisplayedHistoryValues = StorageConstants.defaultMaxDisplayedHistoryValues
    @AppStorage(Paster.delayDefaultsKey) private var pasteDelay = Paster.defaultDelayMilliseconds
    @AppStorage(PickerPanelController.widthDefaultsKey)
    private var pickerWidth = Double(PanelSize.defaultSize.width)
    @AppStorage(PickerPanelController.heightDefaultsKey)
    private var pickerHeight = Double(PanelSize.defaultSize.height)
    @AppStorage(DetailPaneWidth.defaultsKey) private var detailWidth = Double(DetailPaneWidth.defaultWidth)

    var body: some View {
        Form {
            Section("General") {
                LabeledContent("Open Picker") {
                    HotKeyRecorder()
                }
                if let error = hotKeys.registrationError {
                    Text(error).font(.caption).foregroundStyle(.red)
                }
                Toggle("Launch at login", isOn: Binding(
                    get: { launchAtLogin.isEnabled },
                    set: { launchAtLogin.setEnabled($0) }))
                if launchAtLogin.needsApproval {
                    HStack {
                        Text("Allow Sloppy Paste in Login Items to finish.")
                            .font(.caption).foregroundStyle(.secondary)
                        Button("Open Login Items…") { launchAtLogin.openSystemSettings() }
                            .controlSize(.small)
                    }
                }
                if let error = launchAtLogin.lastError {
                    Text(error).font(.caption).foregroundStyle(.red)
                }
            }

            Section("Picker") {
                Picker("Default sort", selection: $sortRaw) {
                    ForEach(SortOption.allCases, id: \.rawValue) { Text($0.label).tag($0.rawValue) }
                }
                Toggle("Show detail pane", isOn: $showingDetail)
                LabeledContent("Detail pane width") {
                    SizeField(
                        value: Binding(
                            get: { Double(DetailPaneWidth.sanitized(detailWidth)) },
                            set: { detailWidth = Double(DetailPaneWidth.sanitized($0)) }),
                        range: DetailPaneWidth.minimum...DetailPaneWidth.maximum)
                }
                Toggle("Show Recently Used section", isOn: $showRecentSection)
                LabeledContent("Width") {
                    SizeField(value: sizeBinding(\.width, $pickerWidth),
                              range: PanelSize.minimum.width...PanelSize.maximum.width)
                }
                LabeledContent("Height") {
                    SizeField(value: sizeBinding(\.height, $pickerHeight),
                              range: PanelSize.minimum.height...PanelSize.maximum.height)
                }
                HStack {
                    Text("Points at 100% zoom. Dragging the picker's borders also sets them.")
                        .font(.caption).foregroundStyle(.secondary)
                    Spacer()
                    if CGSize(width: pickerWidth, height: pickerHeight) != PanelSize.defaultSize {
                        Button("Reset Size") {
                            pickerWidth = Double(PanelSize.defaultSize.width)
                            pickerHeight = Double(PanelSize.defaultSize.height)
                        }
                        .controlSize(.small)
                    }
                }
                LabeledContent("Placeholder history values shown") {
                    Stepper("\(maxDisplayedHistoryValues)", value: $maxDisplayedHistoryValues, in: 5...100, step: 5)
                }
            }

            Section("Pasting") {
                LabeledContent("Delay before ⌘V") {
                    Stepper("\(pasteDelay) ms", value: $pasteDelay, in: 0...1000, step: 10)
                }
                Text("Raise it if some apps receive the paste before they have focus again.")
                    .font(.caption).foregroundStyle(.secondary)
                LabeledContent("Accessibility") {
                    if accessibility.isTrusted {
                        Label("Granted", systemImage: "checkmark.circle.fill").foregroundStyle(.green)
                    } else {
                        HStack {
                            Label("Not granted (copy only)", systemImage: "exclamationmark.triangle.fill")
                                .foregroundStyle(.orange)
                            Button("Open System Settings…") { accessibility.openSystemSettings() }
                                .controlSize(.small)
                        }
                    }
                }
            }

            Section("Data") {
                LabeledContent("Data file") {
                    HStack {
                        Text(store.file.url.path(percentEncoded: false))
                            .lineLimit(1)
                            .truncationMode(.middle)
                            .textSelection(.enabled)
                        Button("Reveal") { revealDataFile() }
                            .controlSize(.small)
                    }
                }
                LabeledContent("Storage", value: importExport.storageDescription())
                HStack {
                    Button("Import…") { importExport.importData() }
                    Button("Export…") { importExport.exportData() }
                }
            }
        }
        .formStyle(.grouped)
        .frame(width: 520)
        .fixedSize(horizontal: false, vertical: true)
        .onAppear {
            accessibility.refresh()
            launchAtLogin.refresh()
            store.reloadIfChanged()
        }
    }

    /// One dimension of the stored picker size, clamped like the picker clamps it.
    private func sizeBinding(_ dimension: KeyPath<CGSize, CGFloat>, _ storage: Binding<Double>) -> Binding<Double> {
        Binding(
            get: { Double(PanelSize.sanitized(width: pickerWidth, height: pickerHeight)[keyPath: dimension]) },
            set: { newValue in
                let size = dimension == \CGSize.width
                    ? PanelSize.sanitized(width: newValue, height: pickerHeight)
                    : PanelSize.sanitized(width: pickerWidth, height: newValue)
                storage.wrappedValue = Double(size[keyPath: dimension])
            })
    }

    private func revealDataFile() {
        let url = store.file.url
        if FileManager.default.fileExists(atPath: url.path) {
            NSWorkspace.shared.activateFileViewerSelecting([url])
        } else {
            NSWorkspace.shared.open(url.deletingLastPathComponent().deletingLastPathComponent())
        }
    }
}

/// Click, then press the new shortcut; ⎋ cancels. The current hotkey is
/// unregistered while recording so pressing it again is recorded too.
struct HotKeyRecorder: View {
    @Environment(HotKeySettings.self) private var hotKeys
    @ViewState private var isRecording = false
    @ViewState private var hint: String?
    @ViewState private var monitor: Any?

    var body: some View {
        HStack(spacing: 8) {
            if let hint {
                Text(hint).font(.caption).foregroundStyle(.secondary)
            }
            Button {
                isRecording ? stopRecording() : startRecording()
            } label: {
                Text(isRecording ? "Type shortcut…" : hotKeys.hotKey.displayString)
                    .frame(minWidth: 110)
            }
            .help(isRecording ? "Press the new shortcut, or ⎋ to cancel" : "Click to record a new shortcut")
            if hotKeys.hotKey != .default, !isRecording {
                Button("Reset") { hotKeys.update(.default) }
                    .controlSize(.small)
            }
        }
        .onDisappear { stopRecording() }
    }

    private func startRecording() {
        guard monitor == nil else { return }
        hotKeys.suspend()
        isRecording = true
        hint = nil
        monitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { event in
            // NSEvent is not Sendable, so only a Bool crosses the isolation hop.
            nonisolated(unsafe) let event = event
            let consumed = MainActor.assumeIsolated { record(event) }
            return consumed ? nil : event
        }
    }

    /// Returns true to swallow the key while recording.
    private func record(_ event: NSEvent) -> Bool {
        guard isRecording else { return false }
        if Int(event.keyCode) == kVK_Escape,
            event.modifierFlags.intersection([.command, .option, .control, .shift]).isEmpty
        {
            stopRecording()
            return true
        }
        guard let hotKey = HotKey(event: event) else {
            hint = "Include ⌘, ⌥ or ⌃"
            return true
        }
        stopRecording(saving: hotKey)
        return true
    }

    private func stopRecording(saving hotKey: HotKey? = nil) {
        if let monitor {
            NSEvent.removeMonitor(monitor)
        }
        let wasRecording = isRecording
        monitor = nil
        isRecording = false
        if let hotKey {
            hint = hotKeys.update(hotKey) ? nil : "Not available"
        } else {
            hint = nil
            if wasRecording { hotKeys.resume() }
        }
    }
}

/// A point value typed in or stepped by 20.
private struct SizeField: View {
    @Binding var value: Double
    let range: ClosedRange<CGFloat>

    var body: some View {
        HStack(spacing: 6) {
            TextField("", value: $value, format: .number.grouping(.never))
                .labelsHidden()
                .multilineTextAlignment(.trailing)
                .frame(width: 64)
            Text("pt").foregroundStyle(.secondary)
            Stepper("", value: $value, in: Double(range.lowerBound)...Double(range.upperBound), step: 20)
                .labelsHidden()
        }
    }
}
