import Foundation
import Testing
@testable import SloppyCore

@Suite struct PlaceholderHighlighterTests {
    /// Each span as `kind:text`, in order.
    private func tokens(_ text: String) -> [String] {
        let units = Array(text.utf16)
        return PlaceholderHighlighter.spans(text).map { span in
            let slice = String(decoding: units[span.range.start..<span.range.end], as: UTF16.self)
            return "\(span.kind):\(slice)"
        }
    }

    @Test func plainTextHasNoSpans() {
        #expect(tokens("git push --force") == [])
    }

    @Test func keyWithSaveMarkerAndDefault() {
        #expect(tokens("run {{!harness|claude}} now") == [
            "brace:{{", "punctuation:!", "key:harness", "punctuation:|", "literal:claude", "brace:}}",
        ])
    }

    @Test func wrappersAroundKey() {
        #expect(tokens("{{$:amount: USD|10}}") == [
            "brace:{{", "literal:$", "punctuation::", "key:amount", "punctuation::", "literal: USD",
            "punctuation:|", "literal:10", "brace:}}",
        ])
    }

    @Test func authoredChoicesWithEscapes() {
        #expect(tokens(#"{{tone[A\|B|C]|C}}"#) == [
            "brace:{{", "key:tone", "punctuation:[", #"literal:A\|B"#, "punctuation:|", "literal:C",
            "punctuation:]", "punctuation:|", "literal:C", "brace:}}",
        ])
    }

    @Test func conditionalBlock() {
        #expect(tokens(#"{{#if +k "Add"}}x{{#else}}y{{/if}}"#) == [
            "brace:{{", "control:#if", "punctuation:+", "key:k", #"literal:"Add""#, "brace:}}",
            "brace:{{", "control:#else", "brace:}}",
            "brace:{{", "control:/if", "brace:}}",
        ])
    }

    @Test func systemPlaceholder() {
        #expect(tokens("{{ DATE }} {{date}}") == [
            "brace:{{", "system:DATE", "brace:}}", "brace:{{", "key:date", "brace:}}",
        ])
    }

    @Test func offsetsAreUTF16() {
        let spans = PlaceholderHighlighter.spans("🎉 {{name}}")
        #expect(spans.first?.range == PlaceholderSourceRange(start: 3, end: 5))
    }
}
