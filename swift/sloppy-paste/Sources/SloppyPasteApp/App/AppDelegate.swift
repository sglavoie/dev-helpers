import AppKit
import SwiftUI

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem?
    private let store = SnippetStore()
    private let accessibility = AccessibilityPermission()
    private let launchAtLogin = LaunchAtLogin()
    private lazy var picker = PickerPanelController(store: store, accessibility: accessibility)
    private lazy var importExport = ImportExportController(store: store, picker: picker)
    private lazy var hotKeys = HotKeySettings(center: CarbonHotKeyCenter()) { [weak self] in
        self?.picker.toggle()
    }
    private lazy var settings = SettingsWindowController(picker: picker) { [unowned self] in
        AnyView(
            SettingsView(importExport: importExport)
                .environment(store)
                .environment(accessibility)
                .environment(hotKeys)
                .environment(launchAtLogin))
    }
    private var openPickerItem: NSMenuItem?
    private var accessibilityItem: NSMenuItem?
    private var isShowingQuarantineAlert = false

    func applicationDidFinishLaunching(_ notification: Notification) {
        store.onQuarantine = { [weak self] url in
            // Loads run inside actions and the file watcher; alert afterwards.
            DispatchQueue.main.async { self?.showQuarantineAlert(url) }
        }
        store.load()
        store.startWatching()
        NSApp.mainMenu = makeMainMenu()
        statusItem = makeStatusItem()
        picker.commands = PickerCommands(
            importData: { [weak self] in self?.importExport.importData() },
            exportData: { [weak self] in self?.importExport.exportData() },
            openSettings: { [weak self] in self?.settings.show() },
            storageDescription: { [weak self] in self?.importExport.storageDescription() ?? "" })
        hotKeys.registerStored()
        accessibility.requestIfNeeded()
    }

    func applicationWillTerminate(_ notification: Notification) {
        hotKeys.unregister()
    }

    @objc private func openPicker(_ sender: Any?) {
        picker.show()
    }

    @objc private func newSnippet(_ sender: Any?) {
        picker.show(.editor(.new))
    }

    @objc private func newSnippetFromClipboard(_ sender: Any?) {
        picker.show(.editor(.fromClipboard))
    }

    @objc private func importData(_ sender: Any?) {
        importExport.importData()
    }

    @objc private func exportData(_ sender: Any?) {
        importExport.exportData()
    }

    @objc private func openSettings(_ sender: Any?) {
        settings.show()
    }

    @objc private func openAccessibilitySettings(_ sender: Any?) {
        accessibility.openSystemSettings()
    }

    private func showQuarantineAlert(_ url: URL) {
        guard !isShowingQuarantineAlert else { return }
        isShowingQuarantineAlert = true
        defer { isShowingQuarantineAlert = false }
        picker.hide()
        let previousApp = NSWorkspace.shared.frontmostApplication
        NSApp.activate()
        let alert = NSAlert()
        alert.alertStyle = .warning
        alert.messageText = "The snippet data file could not be read"
        alert.informativeText = "It was moved aside to \(url.path) and Sloppy Paste continues with empty data. "
            + "Fix the file and move it back to \(store.file.url.path) to restore your snippets."
        alert.addButton(withTitle: "OK")
        alert.addButton(withTitle: "Show in Finder")
        let response = alert.runModal()
        store.clearQuarantineNotice()
        if response == .alertSecondButtonReturn {
            NSWorkspace.shared.activateFileViewerSelecting([url])
        } else if previousApp?.processIdentifier != ProcessInfo.processInfo.processIdentifier {
            previousApp?.activate()
        }
    }

    private func makeStatusItem() -> NSStatusItem {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        if let button = item.button {
            let image = NSImage(
                systemSymbolName: "doc.on.clipboard",
                accessibilityDescription: "Sloppy Paste"
            )
            image?.isTemplate = true
            button.image = image
            button.toolTip = "Sloppy Paste"
        }

        let menu = NSMenu()
        let open = NSMenuItem(title: "Open Picker", action: #selector(openPicker(_:)), keyEquivalent: "")
        open.target = self
        menu.addItem(open)
        openPickerItem = open
        updateOpenPickerItem()
        addItems(to: menu, [
            ("New Snippet", #selector(newSnippet(_:))),
            ("New Snippet from Clipboard", #selector(newSnippetFromClipboard(_:))),
        ])
        menu.addItem(.separator())
        addItems(to: menu, [
            ("Import…", #selector(importData(_:))),
            ("Export…", #selector(exportData(_:))),
        ])
        menu.addItem(.separator())
        let settingsItem = NSMenuItem(title: "Settings…", action: #selector(openSettings(_:)), keyEquivalent: ",")
        settingsItem.target = self
        menu.addItem(settingsItem)
        let accessibilityItem = NSMenuItem(
            title: "", action: #selector(openAccessibilitySettings(_:)), keyEquivalent: "")
        accessibilityItem.target = self
        menu.addItem(accessibilityItem)
        self.accessibilityItem = accessibilityItem
        updateAccessibilityItem()
        menu.addItem(.separator())
        menu.addItem(
            NSMenuItem(
                title: "Quit Sloppy Paste",
                action: #selector(NSApplication.terminate(_:)),
                keyEquivalent: "q"
            )
        )
        menu.delegate = self
        item.menu = menu
        return item
    }

    private func addItems(to menu: NSMenu, _ items: [(String, Selector)]) {
        for (title, action) in items {
            let item = NSMenuItem(title: title, action: action, keyEquivalent: "")
            item.target = self
            menu.addItem(item)
        }
    }

    /// Shows the current global hotkey next to Open Picker.
    private func updateOpenPickerItem() {
        guard let item = openPickerItem else { return }
        if let equivalent = hotKeys.hotKey.menuKeyEquivalent {
            item.keyEquivalent = equivalent.key
            item.keyEquivalentModifierMask = equivalent.modifiers
        } else {
            item.keyEquivalent = ""
            item.keyEquivalentModifierMask = []
        }
    }

    /// Never shown (accessory app), but its key equivalents give the Settings
    /// window and text fields ⌘W, cut, copy, paste, undo and select all.
    private func makeMainMenu() -> NSMenu {
        let main = NSMenu()
        let appItem = NSMenuItem()
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "Close Window", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        appItem.submenu = appMenu
        main.addItem(appItem)

        let editItem = NSMenuItem()
        let edit = NSMenu(title: "Edit")
        edit.addItem(withTitle: "Undo", action: Selector(("undo:")), keyEquivalent: "z")
        edit.addItem(withTitle: "Redo", action: Selector(("redo:")), keyEquivalent: "Z")
        edit.addItem(.separator())
        edit.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        edit.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        edit.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        edit.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        editItem.submenu = edit
        main.addItem(editItem)
        return main
    }

    private func updateAccessibilityItem() {
        accessibility.refresh()
        accessibilityItem?.title = accessibility.isTrusted
            ? "Accessibility: Granted"
            : "Accessibility: Not Granted (copy only)…"
    }
}

extension AppDelegate: NSMenuDelegate {
    func menuWillOpen(_ menu: NSMenu) {
        updateOpenPickerItem()
        updateAccessibilityItem()
    }
}
