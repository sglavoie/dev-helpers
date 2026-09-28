import AppKit
import SwiftUI

/// One "Edit Schedule…" window per agent: a plain NSWindow hosting `ScheduleEditorView`, like the log windows.
/// Choosing it again while the window is open brings that window back; closing it with unsaved changes asks first.
/// Each opening starts from the plist as it is then.
@MainActor
final class ScheduleEditorWindowController: NSObject, NSWindowDelegate {
    private var windows: [String: (window: NSWindow, model: ScheduleEditorModel)] = [:]
    private let save: (ScheduleEditorModel) async -> ScheduleEditorModel.SaveOutcome
    private let openConfig: () -> Void

    init(save: @escaping (ScheduleEditorModel) async -> ScheduleEditorModel.SaveOutcome, openConfig: @escaping () -> Void) {
        self.save = save
        self.openConfig = openConfig
    }

    func show(_ context: ScheduleEditorModel.Context) {
        let entry = windows[context.label] ?? makeWindow(context)
        windows[context.label] = entry
        NSApp.activate()
        entry.window.makeKeyAndOrderFront(nil)
    }

    func windowShouldClose(_ sender: NSWindow) -> Bool {
        guard let model = windows.values.first(where: { $0.window === sender })?.model,
              model.draft.hasChanges else { return true }
        let alert = NSAlert()
        alert.messageText = "Discard changes to \(model.context.name)'s schedule?"
        alert.informativeText = "The plist has not been changed."
        alert.addButton(withTitle: "Discard")
        alert.addButton(withTitle: "Keep Editing")
        return alert.runModal() == .alertFirstButtonReturn
    }

    func windowWillClose(_ notification: Notification) {
        guard let window = notification.object as? NSWindow else { return }
        if let label = windows.first(where: { $0.value.window === window })?.key {
            windows[label] = nil
        }
    }

    private func makeWindow(_ context: ScheduleEditorModel.Context) -> (window: NSWindow, model: ScheduleEditorModel) {
        let model = ScheduleEditorModel(context: context)
        let window = NSWindow(contentViewController: NSHostingController(rootView: ScheduleEditorView(model: model)))
        model.close = { [weak window] in window?.performClose(nil) }
        model.openConfig = openConfig
        model.save = { [weak self, weak window] model in
            guard let self else { return .notSaved("Heartbeat is shutting down") }
            let outcome = await save(model)
            // close() skips windowShouldClose, so a saved draft is not offered for discarding.
            if case .notSaved = outcome {} else { window?.close() }
            return outcome
        }
        window.title = "\(context.name) — Schedule"
        window.styleMask = [.titled, .closable, .resizable]
        window.setContentSize(NSSize(width: 640, height: 560))
        window.isReleasedWhenClosed = false
        window.collectionBehavior = [.moveToActiveSpace]
        window.delegate = self
        window.center()
        window.setFrameAutosaveName("HeartbeatSchedule-\(context.label)")
        return (window, model)
    }
}
