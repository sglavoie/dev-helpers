import Foundation
import Testing
@testable import SloppyCore

@Suite struct SnippetPreviewTests {
    /// 2024-03-15T14:30:45.000Z, a Friday.
    static let now: Int64 = 1_710_513_045_000
    static let utc = TimeZone(identifier: "UTC")!

    private func preview(_ content: String) -> String {
        SnippetPreview.build(content, now: Self.now, timeZone: Self.utc)
    }

    @Test func emptyWithoutPlaceholderSyntax() {
        #expect(preview("Plain content") == "")
    }

    @Test func defaultsNoSaveLabelsAndWrappers() {
        #expect(preview("{{name}} {{role|Editor}} {{!$:amount: USD|10}}")
            == "⟨name⟩ ⟨role = \"Editor\"⟩ ⟨!$amount USD = \"10\"⟩")
    }

    @Test func decodedAuthoredChoicesWithExplicitDefault() {
        #expect(preview(#"{{tone[Formal|A\|B|\[Custom\]|Path\\Name]|Formal}} then {{tone}}"#)
            == #"⟨tone — choices: "Formal", "A|B", "[Custom]", "Path\\Name"; default: "Formal"⟩ then ⟨tone⟩"#)
    }

    @Test func choiceWithoutDefaultVersusExplicitEmptyDefault() {
        #expect(preview("{{tone[Formal|Casual]}} / {{detail[Short|Long]|}}")
            == #"⟨tone — choices: "Formal", "Casual"⟩ / ⟨detail — choices: "Short", "Long"; default: ""⟩"#)
    }

    @Test func conditionalBlocksBeforeInnerPlaceholders() {
        #expect(preview("{{#if name}}Hi {{name}}{{#else}}Hello{{/if}}") == "⟨if name: Hi ⟨name⟩ | else: Hello⟩")
    }

    @Test func conditionalLabelsAndPlusPrefixAreHidden() {
        #expect(preview(#"{{#if +k "Add a note"}}Note{{/if}}"#) == "⟨if k: Note⟩")
    }

    @Test func systemPlaceholdersBeforeUserPlaceholders() {
        let result = preview("On {{DATE}}, greet {{name}}")
        #expect(result == "On ⟨DATE → 2024-03-15⟩, greet ⟨name⟩")
        #expect(!result.contains("{{DATE}}"))
    }

    @Test func capsAt500() {
        let result = preview("{{name}}" + String(repeating: "x", count: 600))
        #expect(result.utf16.count == 500)
        #expect(result.hasPrefix("⟨name⟩"))
    }

    @Test func capNeverSplitsAnEmoji() {
        let result = preview("{{a}}" + String(repeating: "x", count: 496) + "👍🏽tail")
        // "⟨a⟩" is 3 units, 496 x's reach 499, and the 4-unit emoji no longer fits.
        #expect(result.utf16.count == 499)
        #expect(result.hasSuffix("x"))
    }
}
