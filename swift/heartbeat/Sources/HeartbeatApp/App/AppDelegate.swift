import AppKit
import HeartbeatCore

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem?

    func applicationDidFinishLaunching(_ notification: Notification) {
        statusItem = makeStatusItem()
    }

    private func makeStatusItem() -> NSStatusItem {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let button = item.button {
            let image = NSImage(systemSymbolName: "heart", accessibilityDescription: "Heartbeat")
            image?.isTemplate = true
            button.image = image
            button.toolTip = "Heartbeat"
        }
        item.menu = makeMenu()
        return item
    }

    private func makeMenu() -> NSMenu {
        let menu = NSMenu()
        let version = NSMenuItem(title: "Heartbeat \(HeartbeatCore.version)", action: nil, keyEquivalent: "")
        version.isEnabled = false
        menu.addItem(version)
        menu.addItem(.separator())
        menu.addItem(
            NSMenuItem(title: "Quit Heartbeat", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"))
        return menu
    }
}
