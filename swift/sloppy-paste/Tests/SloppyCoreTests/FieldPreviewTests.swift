import Foundation
import Testing
@testable import SloppyCore

@Suite struct FieldPreviewTests {
    private func placeholder(
        _ key: String,
        isRequired: Bool = false,
        defaultValue: String? = nil,
        prefix: String? = nil,
        suffix: String? = nil,
        guardOnly: Bool = false
    ) -> Placeholder {
        Placeholder(key: key, defaultValue: defaultValue, isRequired: isRequired, prefixWrapper: prefix,
            suffixWrapper: suffix, isGuardOnly: guardOnly)
    }

    private func preview(_ p: Placeholder, _ content: String, _ value: String = "", truncate: Bool = true) -> String? {
        FieldPreview.build(p, snippetContent: content, currentValue: value, truncate: truncate)
    }

    @Test func requiredFieldsHaveNoPreview() {
        #expect(preview(placeholder("name", isRequired: true), "Hello {{name}}") == nil)
    }

    @Test func guardWithSingleBody() {
        #expect(preview(placeholder("k", guardOnly: true), "{{#if k}}Hello world{{/if}}") == "Toggles: \"Hello world\"")
    }

    @Test func guardWithElseBody() {
        #expect(preview(placeholder("k", guardOnly: true), "{{#if k}}A{{#else}}B{{/if}}") == "Toggles: \"A\" • Else: \"B\"")
    }

    @Test func guardBodiesTruncateAt80() {
        let content = "{{#if k}}\(String(repeating: "x", count: 100)){{/if}}"
        #expect(preview(placeholder("k", guardOnly: true), content)
            == "Toggles: \"\(String(repeating: "x", count: 80))…\"")
    }

    @Test func fullBodyWithoutTruncation() {
        let content = "{{#if k}}\(String(repeating: "x", count: 100)){{/if}}"
        #expect(preview(placeholder("k", guardOnly: true), content, truncate: false)
            == "Toggles: \"\(String(repeating: "x", count: 100))\"")
    }

    @Test func fullElseBodyWithoutTruncation() {
        let a = String(repeating: "a", count: 100)
        let b = String(repeating: "b", count: 100)
        #expect(preview(placeholder("k", guardOnly: true), "{{#if k}}\(a){{#else}}\(b){{/if}}", truncate: false)
            == "Toggles: \"\(a)\" • Else: \"\(b)\"")
    }

    @Test func collapsesWhitespaceRuns() {
        #expect(preview(placeholder("k", guardOnly: true), "{{#if k}}line1\n\n  line2\tline3{{/if}}")
            == "Toggles: \"line1 line2 line3\"")
    }

    @Test func keepsNestedTemplateTextVerbatim() {
        #expect(preview(placeholder("k", guardOnly: true), "{{#if k}}Hello {{name}}{{/if}}")
            == "Toggles: \"Hello {{name}}\"")
    }

    @Test func guardWithoutBlocksHasNoPreview() {
        #expect(preview(placeholder("k", guardOnly: true), "no conditionals here") == nil)
    }

    @Test func wrapperWithValue() {
        #expect(preview(placeholder("link", prefix: "(", suffix: ")"), "{{(:link:)}}", "foo") == "Wraps as: (foo)")
    }

    @Test func wrapperWithEmptyValue() {
        #expect(preview(placeholder("link", prefix: "(", suffix: ")"), "{{(:link:)}}") == "Wraps as: (<value>)")
    }

    @Test func wrapperWithOnlySuffix() {
        #expect(preview(placeholder("label", suffix: ":"), "{{label:}}") == "Wraps as: <value>:")
    }

    @Test func optionalWithDefault() {
        #expect(preview(placeholder("name", defaultValue: "Alice"), "Hello {{name|Alice}}")
            == "Empty → uses default: \"Alice\"")
    }

    @Test func optionalWithEmptyDefault() {
        #expect(preview(placeholder("k", defaultValue: ""), "Hello {{k|}}") == "Empty → renders as empty string.")
    }

    @Test func optionalWithoutDefault() {
        #expect(preview(placeholder("k"), "Hello {{k}}") == "Optional — leave blank to omit.")
    }

    @Test func infoTextSummarisesRules() {
        #expect(FieldPreview.infoText(placeholder("k", guardOnly: true))
            == "Conditional — checked = block shown, unchecked = block omitted")
        #expect(FieldPreview.infoText(placeholder("k", isRequired: true)) == "Required field")
        #expect(FieldPreview.infoText(placeholder("k", prefix: "(", suffix: ")"))
            == "Optional wrapper • Output when included: \"(value)\" • Uncheck to omit it")
        #expect(FieldPreview.infoText(placeholder("k")) == "Optional (default: \"none\")")
        let choice = Placeholder(key: "tone", choices: ["A|B", "C"], isRequired: true, isSaved: false)
        #expect(FieldPreview.infoText(choice)
            == "Required field • Configured choices: \"A|B\", \"C\" • Choose Enter custom value… to type another value • Won't be saved to history")
    }
}
