import AppKit
import SwiftUI

/// The Settings window: a plain NSWindow hosting `SettingsView`, since the
/// SwiftUI Settings scene is unreliable in accessory apps. Showing it hides
/// the picker and activates the app so the window can take focus.
@MainActor
final class SettingsWindowController: NSObject, NSWindowDelegate {
    private var window: NSWindow?
    private let makeView: @MainActor () -> AnyView
    private let picker: PickerPanelController
    /// The app that was frontmost before Settings opened, reactivated on close.
    private var returnApp: NSRunningApplication?

    init(picker: PickerPanelController, makeView: @escaping @MainActor () -> AnyView) {
        self.picker = picker
        self.makeView = makeView
    }

    func show() {
        let frontmost = NSWorkspace.shared.frontmostApplication
        if frontmost?.processIdentifier != ProcessInfo.processInfo.processIdentifier {
            returnApp = frontmost
        }
        picker.hide()
        let window = self.window ?? makeWindow()
        self.window = window
        NSApp.activate()
        if !window.isVisible {
            window.center()
        }
        window.makeKeyAndOrderFront(nil)
    }

    func windowWillClose(_ notification: Notification) {
        if let returnApp, !returnApp.isTerminated {
            returnApp.activate()
        }
        returnApp = nil
    }

    private func makeWindow() -> NSWindow {
        let hosting = NSHostingController(rootView: makeView())
        hosting.sizingOptions = [.preferredContentSize]
        let window = NSWindow(contentViewController: hosting)
        window.title = "Sloppy Paste Settings"
        window.styleMask = [.titled, .closable, .miniaturizable]
        window.isReleasedWhenClosed = false
        window.collectionBehavior = [.moveToActiveSpace]
        window.delegate = self
        window.setFrameAutosaveName("SloppyPasteSettings")
        return window
    }
}
