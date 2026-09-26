import AppKit
import SwiftUI

/// One "View Log" window per agent: a plain NSWindow hosting `LogTailView`, as the app has no SwiftUI scenes.
/// The tail refreshes every 2 s only while its window is open.
@MainActor
final class LogWindowController: NSObject, NSWindowDelegate {
    private var windows: [String: (window: NSWindow, model: LogTailModel)] = [:]
    private let openLog: (String) -> Void

    init(openLog: @escaping (String) -> Void) {
        self.openLog = openLog
    }

    func show(label: String, name: String, paths: [String]) {
        let entry: (window: NSWindow, model: LogTailModel)
        if let existing = windows[label] {
            entry = existing
        } else {
            entry = makeWindow(label: label, name: name, paths: paths)
            windows[label] = entry
        }
        entry.model.update(paths: paths)
        entry.model.start()
        NSApp.activate()
        entry.window.makeKeyAndOrderFront(nil)
    }

    func windowWillClose(_ notification: Notification) {
        guard let window = notification.object as? NSWindow else { return }
        windows.values.first { $0.window === window }?.model.stop()
    }

    private func makeWindow(label: String, name: String, paths: [String]) -> (window: NSWindow, model: LogTailModel) {
        let model = LogTailModel(paths: paths)
        let view = LogTailView(model: model, openLog: openLog)
        let window = NSWindow(contentViewController: NSHostingController(rootView: view))
        window.title = "\(name) — Log"
        window.styleMask = [.titled, .closable, .miniaturizable, .resizable]
        window.setContentSize(NSSize(width: 820, height: 520))
        window.isReleasedWhenClosed = false
        window.collectionBehavior = [.moveToActiveSpace]
        window.delegate = self
        window.center()
        window.setFrameAutosaveName("HeartbeatLog-\(label)")
        return (window, model)
    }
}
