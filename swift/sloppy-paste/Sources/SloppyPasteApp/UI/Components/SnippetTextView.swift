import AppKit
import SwiftUI

/// Moves keyboard focus to an AppKit-backed editor field. The snippet editor
/// uses these instead of `@FocusState`, which fights direct first-responder
/// changes (the content `NSTextView`) inside the non-activating panel.
@MainActor
final class FocusHandle {
    fileprivate weak var view: NSView?

    /// Makes the field first responder, and takes focus back on the next
    /// turn if something dropped it to the window meanwhile.
    func focus() {
        guard let view, let window = view.window else { return }
        if !isFocused {
            window.makeFirstResponder(view)
        }
        DispatchQueue.main.async { [weak self] in
            MainActor.assumeIsolated {
                guard let self, let view = self.view, let window = view.window, !self.isFocused else { return }
                let responder = window.firstResponder
                if responder === window || responder == nil || (responder as? NSView)?.window !== window {
                    window.makeFirstResponder(view)
                }
            }
        }
    }

    /// True while the field (or its field editor) is first responder.
    var isFocused: Bool {
        guard let view, let responder = view.window?.firstResponder else { return false }
        if responder === view { return true }
        if let editor = responder as? NSText, let field = view as? NSTextField {
            return field.currentEditor() === editor
        }
        return false
    }

    /// Replaces the text view's selection with `text` (undoable, and reported
    /// back through the binding), then selects `selection`, a range relative
    /// to `text`.
    func insert(_ text: String, selecting selection: NSRange? = nil) {
        guard let textView = view as? NSTextView else { return }
        let start = textView.selectedRange().location
        textView.insertText(text, replacementRange: textView.selectedRange())
        if let selection {
            textView.setSelectedRange(NSRange(location: start + selection.location, length: selection.length))
        }
        focus()
    }
}

/// A plain-text, monospaced `NSTextView` for snippet content. Smart quotes,
/// dashes and other substitutions are off, so `{{#if k "Label"}}` survives.
/// Tab is left to the KeyRouter (next field); ⌥Tab inserts a tab character.
struct SnippetTextView: NSViewRepresentable {
    @Binding var text: String
    var handle: FocusHandle
    var onFocus: @MainActor () -> Void = {}

    func makeCoordinator() -> Coordinator {
        Coordinator(text: $text)
    }

    func makeNSView(context: Context) -> NSScrollView {
        let scrollView = NSScrollView()
        scrollView.hasVerticalScroller = true
        scrollView.borderType = .noBorder
        scrollView.drawsBackground = false

        let textView = FocusReportingTextView()
        textView.minSize = .zero
        textView.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: .greatestFiniteMagnitude)
        textView.isVerticallyResizable = true
        textView.isHorizontallyResizable = false
        textView.autoresizingMask = .width
        textView.textContainer?.widthTracksTextView = true
        scrollView.documentView = textView

        textView.isRichText = false
        textView.importsGraphics = false
        textView.allowsUndo = true
        textView.isAutomaticQuoteSubstitutionEnabled = false
        textView.isAutomaticDashSubstitutionEnabled = false
        textView.isAutomaticTextReplacementEnabled = false
        textView.isAutomaticSpellingCorrectionEnabled = false
        textView.isAutomaticLinkDetectionEnabled = false
        textView.isAutomaticDataDetectionEnabled = false
        textView.isAutomaticTextCompletionEnabled = false
        textView.isContinuousSpellCheckingEnabled = false
        textView.isGrammarCheckingEnabled = false
        textView.smartInsertDeleteEnabled = false
        textView.font = .monospacedSystemFont(ofSize: NSFont.systemFontSize, weight: .regular)
        textView.textContainerInset = NSSize(width: 4, height: 6)
        textView.drawsBackground = false
        textView.string = text
        textView.delegate = context.coordinator
        return scrollView
    }

    func updateNSView(_ scrollView: NSScrollView, context: Context) {
        context.coordinator.text = $text
        guard let textView = scrollView.documentView as? FocusReportingTextView else { return }
        textView.onBecomeFirstResponder = onFocus
        handle.view = textView
        if textView.string != text {
            let selection = textView.selectedRange()
            textView.string = text
            let length = (text as NSString).length
            textView.setSelectedRange(NSRange(location: min(selection.location, length), length: 0))
        }
    }

    final class Coordinator: NSObject, NSTextViewDelegate {
        var text: Binding<String>

        init(text: Binding<String>) {
            self.text = text
        }

        func textDidChange(_ notification: Notification) {
            guard let textView = notification.object as? NSTextView else { return }
            MainActor.assumeIsolated {
                text.wrappedValue = textView.string
            }
        }
    }
}

private final class FocusReportingTextView: NSTextView {
    var onBecomeFirstResponder: (@MainActor () -> Void)?

    override func becomeFirstResponder() -> Bool {
        let accepted = super.becomeFirstResponder()
        if accepted { onBecomeFirstResponder?() }
        return accepted
    }
}

/// An `NSTextField` for the editor's title, description and tags, focused
/// through a `FocusHandle` rather than `@FocusState`.
struct EditorTextField: NSViewRepresentable {
    var placeholder: String
    @Binding var text: String
    var handle: FocusHandle
    /// Wraps onto several lines (Return still ends editing; ⌥Return adds a line).
    var multiline = false
    var onFocus: @MainActor () -> Void = {}

    func makeCoordinator() -> Coordinator {
        Coordinator(text: $text)
    }

    func makeNSView(context: Context) -> NSTextField {
        let field = FocusReportingTextField()
        field.placeholderString = placeholder
        field.stringValue = text
        field.delegate = context.coordinator
        field.bezelStyle = .roundedBezel
        field.isBezeled = true
        field.usesSingleLineMode = !multiline
        field.cell?.wraps = multiline
        field.cell?.isScrollable = !multiline
        field.lineBreakMode = multiline ? .byWordWrapping : .byClipping
        field.maximumNumberOfLines = multiline ? 3 : 1
        field.setContentHuggingPriority(.defaultLow, for: .horizontal)
        field.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        return field
    }

    func updateNSView(_ field: NSTextField, context: Context) {
        context.coordinator.text = $text
        (field as? FocusReportingTextField)?.onBecomeFirstResponder = onFocus
        handle.view = field
        if field.stringValue != text {
            field.stringValue = text
        }
    }

    final class Coordinator: NSObject, NSTextFieldDelegate {
        var text: Binding<String>

        init(text: Binding<String>) {
            self.text = text
        }

        func controlTextDidChange(_ notification: Notification) {
            guard let field = notification.object as? NSTextField else { return }
            MainActor.assumeIsolated {
                text.wrappedValue = field.stringValue
            }
        }
    }
}

private final class FocusReportingTextField: NSTextField {
    var onBecomeFirstResponder: (@MainActor () -> Void)?

    override func becomeFirstResponder() -> Bool {
        let accepted = super.becomeFirstResponder()
        if accepted { onBecomeFirstResponder?() }
        return accepted
    }
}
