import AppKit
import SloppyCore
import UniformTypeIdentifiers

/// Import and export through the system open and save panels. The panels are
/// modal, so the picker hides and the app activates first; afterwards the app
/// that was frontmost comes back unless a Sloppy Paste window (Settings) is
/// still open. Results show in the HUD, since the picker is hidden by then.
@MainActor
final class ImportExportController {
    private let store: SnippetStore
    private let picker: PickerPanelController
    private let hud = HUD()

    init(store: SnippetStore, picker: PickerPanelController) {
        self.store = store
        self.picker = picker
    }

    /// Saves every snippet, tag and placeholder history value as an export
    /// file the Raycast extension (and this app) can import.
    func exportData() {
        runModal {
            let panel = NSSavePanel()
            panel.title = "Export All Snippets"
            panel.prompt = "Export"
            panel.allowedContentTypes = [.json]
            panel.canCreateDirectories = true
            panel.directoryURL = URL.downloadsDirectory
            panel.nameFieldStringValue = ImportExport.exportFileName(now: store.now())
            guard panel.runModal() == .OK, let url = panel.url else { return }

            store.reloadIfChanged()
            do {
                let export = ImportExport.makeExport(store.data, now: store.now())
                try ImportExport.encodeExport(export).write(to: url, options: .atomic)
                hud.show("Export successful", message: "Saved to \(url.lastPathComponent)", symbol: "square.and.arrow.up")
            } catch {
                hud.show("Export failed", message: error.localizedDescription, symbol: "xmark.octagon")
            }
        }
    }

    /// Picks an export file, then asks whether to merge it into the current
    /// snippets or replace them (after writing a `.bak` backup).
    func importData() {
        runModal {
            let panel = NSOpenPanel()
            panel.title = "Import Snippets"
            panel.prompt = "Choose"
            panel.message = "Choose a Sloppy Paste or Raycast export file"
            panel.allowedContentTypes = [.json]
            panel.allowsMultipleSelection = false
            panel.canChooseDirectories = false
            panel.directoryURL = URL.downloadsDirectory
            guard panel.runModal() == .OK, let url = panel.url else { return }

            let payload: ImportPayload
            do {
                payload = try ImportExport.decodeImport(Data(contentsOf: url))
            } catch let error as DecodingError {
                failImport("Invalid file format: \(Self.describe(error))")
                return
            } catch {
                failImport(error.localizedDescription)
                return
            }

            store.reloadIfChanged()
            guard let mode = askImportMode(payload, fileName: url.lastPathComponent) else { return }
            let summary = ImportSummary(payload, current: store.data, mode: mode)
            do {
                try store.importPayload(payload, mode: mode)
                hud.show("Import successful", message: summary.resultMessage, symbol: "square.and.arrow.down")
            } catch {
                failImport(error.localizedDescription)
            }
        }
    }

    /// Size and location of the data file, for ⇧⌘S and the Settings window.
    func storageDescription() -> String {
        "Using \(Formatting.size(store.file.size())) for \(ImportSummary.snippets(store.snippets.count))"
    }

    private func askImportMode(_ payload: ImportPayload, fileName: String) -> ImportMode? {
        let merge = ImportSummary(payload, current: store.data, mode: .merge)
        let alert = NSAlert()
        alert.messageText = "Import \(ImportSummary.snippets(merge.fileCount)) from \"\(fileName)\"?"
        alert.informativeText = """
            Merge adds the \(merge.importedCount) snippets you don't have yet and keeps your \
            \(merge.currentCount). Replace swaps all \(merge.currentCount) current snippets for the \
            file's, after backing up the data file to \(store.file.backupURL.lastPathComponent).
            """
        alert.addButton(withTitle: "Merge")
        alert.addButton(withTitle: "Cancel")
        let replace = alert.addButton(withTitle: "Replace…")
        replace.hasDestructiveAction = true
        switch alert.runModal() {
        case .alertFirstButtonReturn: return .merge
        case .alertThirdButtonReturn: return confirmReplace(merge) ? .replace : nil
        default: return nil
        }
    }

    private func confirmReplace(_ summary: ImportSummary) -> Bool {
        guard summary.currentCount > 0 else { return true }
        let alert = NSAlert()
        alert.alertStyle = .critical
        alert.messageText = "Replace all \(ImportSummary.snippets(summary.currentCount))?"
        alert.informativeText = "Your current data is backed up to \(store.file.backupURL.path) first."
        let replace = alert.addButton(withTitle: "Replace")
        replace.hasDestructiveAction = true
        alert.addButton(withTitle: "Cancel")
        return alert.runModal() == .alertFirstButtonReturn
    }

    private func failImport(_ message: String) {
        hud.show("Import failed", message: message, symbol: "xmark.octagon")
    }

    private static func describe(_ error: DecodingError) -> String {
        switch error {
        case .keyNotFound(let key, _): "missing \"\(key.stringValue)\""
        case .typeMismatch(_, let context), .valueNotFound(_, let context), .dataCorrupted(let context):
            context.debugDescription
        @unknown default: error.localizedDescription
        }
    }

    /// Hides the picker and activates the app for a modal panel, then gives
    /// focus back to the app that had it.
    private func runModal(_ body: () -> Void) {
        // The picker and the status menu leave the target app frontmost.
        let frontmost = NSWorkspace.shared.frontmostApplication
        let returnApp = frontmost?.processIdentifier == ProcessInfo.processInfo.processIdentifier ? nil : frontmost
        picker.hide()
        NSApp.activate()
        body()
        let hasOwnWindow = NSApp.windows.contains { $0.isVisible && $0.canBecomeMain && !($0 is NSPanel) }
        if !hasOwnWindow, let returnApp, !returnApp.isTerminated {
            returnApp.activate()
        }
    }
}
