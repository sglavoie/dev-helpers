import AppKit
import SwiftUI

/// The Pi journal row's "View Journal…" window: a plain NSWindow hosting `PiJournalView`, like the agents' log
/// windows. It asks the Pi only while it is open.
@MainActor
final class PiJournalWindowController: NSObject, NSWindowDelegate {
    private var entry: (window: NSWindow, model: PiJournalModel)?

    func show(host: String) {
        let entry = self.entry ?? makeWindow(host: host)
        self.entry = entry
        entry.model.update(host: host)
        entry.model.start()
        NSApp.activate()
        entry.window.makeKeyAndOrderFront(nil)
    }

    func windowWillClose(_ notification: Notification) {
        entry?.model.stop()
    }

    private func makeWindow(host: String) -> (window: NSWindow, model: PiJournalModel) {
        let model = PiJournalModel(host: host)
        let window = NSWindow(contentViewController: NSHostingController(rootView: PiJournalView(model: model)))
        window.title = "Pi — Journal Errors (last hour)"
        window.styleMask = [.titled, .closable, .miniaturizable, .resizable]
        window.setContentSize(NSSize(width: 900, height: 520))
        window.isReleasedWhenClosed = false
        window.collectionBehavior = [.moveToActiveSpace]
        window.delegate = self
        window.center()
        window.setFrameAutosaveName("HeartbeatPiJournal")
        return (window, model)
    }
}
