import AppKit

struct PasteboardWriteError: Error, CustomStringConvertible {
    var description: String { "Could not write to the clipboard" }
}

/// The general pasteboard, plain text only.
@MainActor
enum Pasteboard {
    static func copy(_ text: String) throws {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        guard pasteboard.setString(text, forType: .string) else {
            throw PasteboardWriteError()
        }
    }

    static func string() -> String? {
        NSPasteboard.general.string(forType: .string)
    }
}
