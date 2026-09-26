import Foundation
import Testing
@testable import SloppyCore

private func process(_ text: String, _ values: [String: String]) -> String {
    ConditionalBlocks.process(text, values: values)
}

private func extract(_ text: String) -> [Placeholder] {
    PlaceholderSyntaxParser.extractPlaceholders(text)
}

private func bodies(_ text: String, _ key: String) -> [ConditionalBlockBody] {
    ConditionalBlocks.extractBodies(text, key: key)
}

@Suite struct ProcessConditionalBlocksTests {
    static let cases: [(String, [String: String], String)] = [
        // if-only, if-else, falsy values
        ("{{#if name}}Hello{{/if}}", ["name": "Alice"], "Hello"),
        ("{{#if name}}Hello{{/if}}", ["name": ""], ""),
        ("{{#if cc}}CC: {{cc}}{{#else}}(No CC){{/if}}", ["cc": "boss@co.com"], "CC: {{cc}}"),
        ("{{#if cc}}CC: {{cc}}{{#else}}(No CC){{/if}}", ["cc": ""], "(No CC)"),
        ("{{#if missing}}shown{{/if}}", [:], ""),
        ("{{#if key}}shown{{/if}}", ["key": "   "], ""),
        // newline cleanup
        ("Before\n{{#if key}}\nblock content\n{{/if}}\nAfter", ["key": ""], "Before\n\nAfter"),
        ("Before\n{{#if key}}\nblock content\n{{/if}}\nAfter", ["key": "yes"], "Before\nblock content\nAfter"),
        ("Hello {{#if name}}{{name}}{{/if}} world", ["name": "Alice"], "Hello {{name}} world"),
        ("Hello {{#if name}}{{name}}{{/if}} world", ["name": ""], "Hello  world"),
        // independent siblings
        ("{{#if a}}A{{/if}} and {{#if b}}B{{/if}}", ["a": "yes", "b": ""], "A and "),
        ("{{#if a}}A{{/if}} and {{#if b}}B{{/if}}", ["a": "", "b": "yes"], " and B"),
        ("{{#if a}}A{{/if}} and {{#if b}}B{{/if}}", ["a": "yes", "b": "yes"], "A and B"),
        // guard-only, labels and `+`
        ("{{#if show}}visible{{/if}}", ["show": "true"], "visible"),
        ("{{#if show}}visible{{/if}}", ["show": ""], ""),
        (#"{{#if SIG "Include signature"}}Best regards{{/if}}"#, ["SIG": "true"], "Best regards"),
        (#"{{#if SIG "Include signature"}}Best regards{{/if}}"#, ["SIG": ""], ""),
        (#"{{#if FORMAL "Use formal tone"}}Dear Sir{{#else}}Hey{{/if}}"#, ["FORMAL": "true"], "Dear Sir"),
        (#"{{#if FORMAL "Use formal tone"}}Dear Sir{{#else}}Hey{{/if}}"#, ["FORMAL": ""], "Hey"),
        ("Before\n{{#if SIG \"Include signature\"}}\nBest regards\n{{/if}}\nAfter", ["SIG": ""], "Before\n\nAfter"),
        ("Before\n{{#if SIG \"Include signature\"}}\nBest regards\n{{/if}}\nAfter", ["SIG": "true"], "Before\nBest regards\nAfter"),
        ("{{#if +show}}visible{{/if}}", ["show": "true"], "visible"),
        ("{{#if +show}}visible{{/if}}", ["show": ""], ""),
        (#"{{#if +SIG "Include signature"}}Best regards{{/if}}"#, ["SIG": "true"], "Best regards"),
        (#"{{#if +SIG "Include signature"}}Best regards{{/if}}"#, ["SIG": ""], ""),
        // {{/else}} closing tag
        ("{{#if x}}yes{{#else}}no{{/else}}{{/if}}", ["x": "true"], "yes"),
        ("{{#if x}}yes{{#else}}no{{/else}}{{/if}}", ["x": ""], "no"),
        ("{{#if x}}yes{{#else}}no{{/if}}", ["x": "true"], "yes"),
        ("{{#if x}}yes{{#else}}no{{/if}}", ["x": ""], "no"),
        ("{{#if +loop}}/loop {{/if}}Commit {{#if loop}}each round{{#else}}once{{/else}}{{/if}} done", ["loop": "true"], "/loop Commit each round done"),
        ("{{#if +loop}}/loop {{/if}}Commit {{#if loop}}each round{{#else}}once{{/else}}{{/if}} done", ["loop": ""], "Commit once done"),
        ("{{#if a}}{{#if b}}both{{#else}}a only{{/else}}{{/if}}{{#else}}none{{/else}}{{/if}}", ["a": "1", "b": "1"], "both"),
        ("{{#if a}}{{#if b}}both{{#else}}a only{{/else}}{{/if}}{{#else}}none{{/else}}{{/if}}", ["a": "1", "b": ""], "a only"),
        ("{{#if a}}{{#if b}}both{{#else}}a only{{/else}}{{/if}}{{#else}}none{{/else}}{{/if}}", ["a": "", "b": "1"], "none"),
        // nesting
        ("{{#if a}}outer {{#if b}}inner{{/if}}{{/if}}", ["a": "yes", "b": "yes"], "outer inner"),
        ("{{#if a}}outer {{#if b}}inner{{/if}}{{/if}}", ["a": "yes", "b": ""], "outer "),
        ("{{#if a}}outer {{#if b}}inner{{/if}}{{/if}}", ["a": "", "b": "yes"], ""),
        ("{{#if a}}{{#if b}}both{{#else}}a only{{/if}}{{#else}}neither{{/if}}", ["a": "yes", "b": "yes"], "both"),
        ("{{#if a}}{{#if b}}both{{#else}}a only{{/if}}{{#else}}neither{{/if}}", ["a": "yes", "b": ""], "a only"),
        ("{{#if a}}{{#if b}}both{{#else}}a only{{/if}}{{#else}}neither{{/if}}", ["a": "", "b": "yes"], "neither"),
        ("{{#if a}}{{#if b}}both{{#else}}a only{{/if}}{{#else}}neither{{/if}}", ["a": "", "b": ""], "neither"),
        ("{{#if a}}{{#if b}}{{#if c}}all three{{/if}}{{/if}}{{/if}}", ["a": "1", "b": "1", "c": "1"], "all three"),
        ("{{#if a}}{{#if b}}{{#if c}}all three{{/if}}{{/if}}{{/if}}", ["a": "1", "b": "1", "c": ""], ""),
        ("{{#if a}}{{#if b}}{{#if c}}all three{{/if}}{{/if}}{{/if}}", ["a": "1", "b": "", "c": "1"], ""),
        ("{{#if a}}{{#if b}}{{#if c}}all three{{/if}}{{/if}}{{/if}}", ["a": "", "b": "1", "c": "1"], ""),
        ("{{#if formal}}Dear {{#if name}}{{name}}{{#else}}Sir{{/if}}{{#else}}Hey{{/if}}", ["formal": "yes", "name": "Alice"], "Dear {{name}}"),
        ("{{#if formal}}Dear {{#if name}}{{name}}{{#else}}Sir{{/if}}{{#else}}Hey{{/if}}", ["formal": "yes", "name": ""], "Dear Sir"),
        ("{{#if formal}}Dear {{#if name}}{{name}}{{#else}}Sir{{/if}}{{#else}}Hey{{/if}}", ["formal": "", "name": "Alice"], "Hey"),
        ("{{#if a}}A{{/if}} {{#if b}}{{#if c}}BC{{/if}}{{/if}}", ["a": "1", "b": "1", "c": "1"], "A BC"),
        ("{{#if a}}A{{/if}} {{#if b}}{{#if c}}BC{{/if}}{{/if}}", ["a": "1", "b": "1", "c": ""], "A "),
        ("{{#if a}}A{{/if}} {{#if b}}{{#if c}}BC{{/if}}{{/if}}", ["a": "", "b": "1", "c": "1"], " BC"),
        ("Before\n{{#if a}}\n{{#if b}}\nInner\n{{/if}}\n{{/if}}\nAfter", ["a": "1", "b": "1"], "Before\nInner\nAfter"),
        ("Before\n{{#if a}}\n{{#if b}}\nInner\n{{/if}}\n{{/if}}\nAfter", ["a": "1", "b": ""], "Before\n\nAfter"),
        ("Before\n{{#if a}}\n{{#if b}}\nInner\n{{/if}}\n{{/if}}\nAfter", ["a": "", "b": "1"], "Before\n\nAfter"),
        // emoji and CJK stay intact around tags
        ("{{#if 名前}}こんにちは{{#else}}やあ{{/if}}", ["名前": "太郎"], "こんにちは"),
        ("{{#if 名前}}こんにちは{{#else}}やあ{{/if}}", ["名前": ""], "やあ"),
        (#"{{#if +ok "✅ Include"}}👨‍👩‍👧 family{{/if}}"#, ["ok": "true"], "👨‍👩‍👧 family"),
        // A combining mark after `{{/if}}` must not hide the tag.
        ("{{#if k}}e\u{301}{{/if}}\u{301}x", ["k": "1"], "e\u{301}\u{301}x"),
    ]

    @Test(arguments: cases)
    func resolves(text: String, values: [String: String], expected: String) {
        #expect(process(text, values) == expected)
    }

    @Test func unbalancedTagsTerminate() {
        #expect(process("{{#if a}}{{#if b}}unclosed{{/if}}", ["a": "1", "b": "1"]) == "{{#if a}}unclosed")
    }

    @Test func nestingDeeperThanTenLevelsStopsResolving() {
        let depth = 11
        let text = String(repeating: "{{#if a}}", count: depth) + "x" + String(repeating: "{{/if}}", count: depth)
        #expect(process(text, ["a": "1"]) == "{{#if a}}x{{/if}}")
    }

    @Test func fullTemplateWithGuardPlaceholderAndRepeatedConditional() {
        let text = "{{#if +loop}}/loop {{!duration|5}}m {{/if}}Commit {{#if loop}}each round{{#else}}once{{/else}}{{/if}} using /gitsummary"
        let placeholders = extract(text)
        let keys = placeholders.map(\.key)
        #expect(keys.contains("duration"))
        #expect(keys.contains("loop"))
        #expect(!keys.contains("/else"))

        let loop = placeholders.first { $0.key == "loop" }
        #expect(loop?.isGuardOnly == true)
        #expect(loop?.defaultOn == true)

        let on = PlaceholderRenderer.replacePlaceholders(
            process(text, ["loop": "true"]), values: ["duration": "5", "loop": "true"], placeholders: placeholders)
        #expect(on == "/loop 5m Commit each round using /gitsummary")
        let off = PlaceholderRenderer.replacePlaceholders(
            process(text, ["loop": ""]), values: ["duration": "5", "loop": ""], placeholders: placeholders)
        #expect(off == "Commit once using /gitsummary")
    }
}

@Suite struct ConditionalGuardExtractionTests {
    @Test func guardOnlyKeyExtracted() {
        #expect(
            extract("{{#if include_sig}}\nBest regards\n{{/if}}") == [
                Placeholder(key: "include_sig", isRequired: false, isSaved: false, isGuardOnly: true)
            ])
    }

    @Test func guardKeyAlsoUsedAsValueIsNotGuardOnly() {
        #expect(extract("{{#if cc}}\nCC: {{cc}}\n{{/if}}") == [Placeholder(key: "cc", isRequired: true)])
        #expect(extract("{{#if cc \"Show CC field\"}}\nCC: {{cc}}\n{{/if}}") == [Placeholder(key: "cc", isRequired: true)])
        #expect(extract("{{#if +cc}}\nCC: {{cc}}\n{{/if}}") == [Placeholder(key: "cc", isRequired: true)])
    }

    @Test func controlTokensAreNotKeys() {
        #expect(extract("{{#if foo}}\ncontent\n{{/if}}").map(\.key) == ["foo"])
        #expect(extract("{{#if x}}\nA\n{{#else}}\nB\n{{/else}}\n{{/if}}").map(\.key) == ["x"])
    }

    @Test func labels() {
        let labeled = extract("{{#if SIG \"Include signature\"}}\nBest regards\n{{/if}}")
        #expect(labeled == [Placeholder(key: "SIG", isRequired: false, isSaved: false, isGuardOnly: true, label: "Include signature")])
        #expect(extract("{{#if toggle}}\nyes\n{{/if}}")[0].label == nil)
        let special = extract("{{#if TERMS \"Accept terms & conditions\"}}\nI agree\n{{/if}}")
        #expect(special[0].key == "TERMS")
        #expect(special[0].label == "Accept terms & conditions")
    }

    @Test func plusPrefixSetsDefaultOn() {
        #expect(
            extract("{{#if +include_sig}}\nBest regards\n{{/if}}") == [
                Placeholder(key: "include_sig", isRequired: false, isSaved: false, isGuardOnly: true, defaultOn: true)
            ])
        #expect(extract("{{#if toggle}}\nyes\n{{/if}}")[0].defaultOn == false)
        #expect(
            extract("{{#if +SIG \"Include signature\"}}\nBest regards\n{{/if}}") == [
                Placeholder(
                    key: "SIG", isRequired: false, isSaved: false, isGuardOnly: true, label: "Include signature",
                    defaultOn: true)
            ])
    }
}

@Suite struct ExtractConditionalBlockBodiesTests {
    static let cases: [(String, String, [ConditionalBlockBody])] = [
        ("{{#if k}}A{{/if}}", "k", [.init(ifBody: "A")]),
        ("{{#if k}}A{{#else}}B{{/if}}", "k", [.init(ifBody: "A", elseBody: "B")]),
        ("{{#if k}}A{{#else}}B{{/else}}{{/if}}", "k", [.init(ifBody: "A", elseBody: "B")]),
        ("{{#if +k}}A{{/if}}", "k", [.init(ifBody: "A")]),
        (#"{{#if k "Label"}}A{{/if}}"#, "k", [.init(ifBody: "A")]),
        ("{{#if outer}}x{{#if inner}}y{{/if}}z{{/if}}", "outer", [.init(ifBody: "x{{#if inner}}y{{/if}}z")]),
        ("{{#if outer}}x{{#if inner}}y{{/if}}z{{/if}}", "inner", [.init(ifBody: "y")]),
        ("{{#if other}}A{{/if}}{{#if k}}B{{/if}}", "k", [.init(ifBody: "B")]),
        ("{{#if k}}A{{/if}} {{#if k}}B{{/if}}", "k", [.init(ifBody: "A"), .init(ifBody: "B")]),
        ("{{#if k}}A", "k", []),
        ("plain text {{name}} no blocks", "k", []),
        ("before {{#if k}}middle{{/if}} after", "k", [.init(ifBody: "middle")]),
        ("{{#if k}}A{{/if}}-{{#if k}}B{{/if}}-{{#if k}}C{{/if}}", "k", [.init(ifBody: "A"), .init(ifBody: "B"), .init(ifBody: "C")]),
        ("{{#if k}}A{{#if nested}}B", "k", []),
        ("{{#if outer}}{{#if inner}}{{#else}}fallback{{/if}}{{/if}}", "outer", [.init(ifBody: "{{#if inner}}{{#else}}fallback{{/if}}")]),
        ("{{#if 鍵}}🙂{{#else}}😢{{/else}}{{/if}}", "鍵", [.init(ifBody: "🙂", elseBody: "😢")]),
    ]

    @Test(arguments: cases)
    func extracts(text: String, key: String, expected: [ConditionalBlockBody]) {
        #expect(bodies(text, key) == expected)
    }
}

/// Outputs of the TS `replacePlaceholders(processConditionalBlocks(t, v), v, extractPlaceholders(t))`
/// under Node 24, covering wrappers with empty values, emoji, CJK and CRLF.
@Suite struct RenderPipelineGoldenTests {
    static let cases: [(String, [String: String], String)] = [
        ("{{#if 名前}}こんにちは {{名前}}{{#else}}やあ{{/if}}", ["名前": "太郎"], "こんにちは 太郎"),
        ("{{#if 名前}}こんにちは {{名前}}{{#else}}やあ{{/if}}", ["名前": ""], "やあ"),
        ("👋\n{{#if e}}\n🎉 {{e}}\n{{/if}}\n👍", ["e": "🚀"], "👋\n🎉 🚀\n👍"),
        ("👋\n{{#if e}}\n🎉 {{e}}\n{{/if}}\n👍", ["e": ""], "👋\n\n👍"),
        (#"{{#if +ok "✅ Include"}}👨‍👩‍👧 family{{/if}}"#, ["ok": "true"], "👨‍👩‍👧 family"),
        ("a\r\n{{#if k}}\r\nX\r\n{{/if}}\r\nb", ["k": "1"], "a\r\n\r\nX\r\r\nb"),
        ("a\r\n{{#if k}}\r\nX\r\n{{/if}}\r\nb", ["k": ""], "a\r\n\r\nb"),
        ("Hi{{ :name:}}!{{#if name}} 🙂{{/if}}", ["name": ""], "Hi!"),
        ("Hi{{ :name:}}!{{#if name}} 🙂{{/if}}", ["name": "  "], "Hi!"),
        ("Hi{{ :name:}}!{{#if name}} 🙂{{/if}}", ["name": "李"], "Hi李! 🙂"),
        ("{{#if k}}e\u{301}{{/if}}\u{301}x", ["k": "1"], "e\u{301}\u{301}x"),
    ]

    @Test(arguments: cases)
    func renders(text: String, values: [String: String], expected: String) {
        #expect(PlaceholderRenderer.render(text, values: values, now: 0) == expected)
    }

    @Test func systemPlaceholdersResolveBeforeExtraction() {
        let utc = TimeZone(identifier: "UTC")!
        let rendered = PlaceholderRenderer.render(
            "{{#if who}}Hi {{who}}, {{/if}}it is {{ DAY }} {{$:tip: 💸|}}", values: ["who": "Zoë", "tip": ""],
            now: 1_710_513_045_000, timeZone: utc)
        #expect(rendered == "Hi Zoë, it is Friday ")
    }
}
