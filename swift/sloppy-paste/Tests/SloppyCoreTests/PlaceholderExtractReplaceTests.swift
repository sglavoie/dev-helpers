import Testing
@testable import SloppyCore

private func extract(_ text: String) -> [Placeholder] {
    PlaceholderSyntaxParser.extractPlaceholders(text)
}

private func replace(_ text: String, _ values: [String: String]) -> String {
    PlaceholderRenderer.replacePlaceholders(text, values: values, placeholders: extract(text))
}

@Suite struct ExtractPlaceholdersTests {
    @Test func basic() {
        #expect(extract("Hello {{name}}, welcome!") == [Placeholder(key: "name", isRequired: true)])
    }

    @Test func defaultValues() {
        #expect(
            extract("Hello {{name|John}}, your ID is {{id|123}}") == [
                Placeholder(key: "name", defaultValue: "John", isRequired: false),
                Placeholder(key: "id", defaultValue: "123", isRequired: false),
            ])
    }

    @Test func duplicates() {
        let result = extract("{{name}} and {{name}} again")
        #expect(result.map(\.key) == ["name"])
    }

    @Test func mixedRequiredAndOptional() {
        #expect(extract("{{required}} and {{optional|default}}").map(\.isRequired) == [true, false])
    }

    @Test func noPlaceholders() {
        #expect(extract("No placeholders here").isEmpty)
    }

    @Test func trimsKeysAndDefaults() {
        let result = extract("{{ name | John Doe }}")
        #expect(result[0].key == "name")
        #expect(result[0].defaultValue == "John Doe")
    }

    @Test func emptyDefault() {
        #expect(extract("Hello {{name|}}!") == [Placeholder(key: "name", defaultValue: "", isRequired: false)])
    }

    @Test func noDefaultVersusEmptyDefault() {
        #expect(
            extract("{{noDefault}} vs {{emptyDefault|}}") == [
                Placeholder(key: "noDefault", isRequired: true),
                Placeholder(key: "emptyDefault", defaultValue: "", isRequired: false),
            ])
    }

    @Test func noSaveFlag() {
        #expect(extract("Date: {{!date}}") == [Placeholder(key: "date", isRequired: true, isSaved: false)])
    }

    @Test func noSaveFlagWithDefault() {
        #expect(
            extract("{{!timestamp|123}}") == [
                Placeholder(key: "timestamp", defaultValue: "123", isRequired: false, isSaved: false)
            ])
    }

    @Test func prefixWrapper() {
        #expect(extract("Order {{#:id:}}") == [Placeholder(key: "id", isRequired: false, prefixWrapper: "#")])
    }

    @Test func suffixWrapper() {
        #expect(extract("Price {{:amount:%}}") == [Placeholder(key: "amount", isRequired: false, suffixWrapper: "%")])
    }

    @Test func prefixAndSuffixWrappers() {
        #expect(
            extract("Price {{$:amount: USD}}") == [
                Placeholder(key: "amount", isRequired: false, prefixWrapper: "$", suffixWrapper: " USD")
            ])
    }

    /// Wrapper syntax shows prefix+value+suffix or nothing, so it implies optionality.
    @Test(arguments: ["Order {{#:id:}}", "Price {{:amount:%}}", "{{with :context:}}"])
    func wrapperImpliesOptional(text: String) {
        let result = extract(text)
        #expect(result.count == 1)
        #expect(result.first?.isRequired == false)
    }

    /// Empty wrappers are equivalent to `{{key}}`; `!` only affects isSaved.
    @Test func noSaveWithEmptyWrappers() {
        #expect(extract("Event on {{!:date:}}") == [Placeholder(key: "date", isRequired: true, isSaved: false)])
    }

    @Test func noSaveWithWrappersAndDefault() {
        #expect(
            extract("{{!$:price: USD|0.00}}") == [
                Placeholder(
                    key: "price", defaultValue: "0.00", isRequired: false, isSaved: false, prefixWrapper: "$",
                    suffixWrapper: " USD")
            ])
    }

    @Test func explicitEmptyWrappers() {
        #expect(extract("{{:key:}}") == [Placeholder(key: "key", isRequired: true)])
    }

    @Test func invalidColonCountIsLiteralKey() {
        let result = extract("{{key:value}}")
        #expect(result[0].key == "key:value")
        #expect(result[0].prefixWrapper == nil)
        #expect(result[0].suffixWrapper == nil)
    }

    @Test func guardOnlyKeysFollowValueKeys() {
        let result = extract(#"{{#if +cc "Copy me"}}{{name}}{{/if}} {{#if bcc}}x{{/if}} {{#if name}}y{{/if}}"#)
        #expect(
            result == [
                Placeholder(key: "name", isRequired: true),
                Placeholder(key: "cc", isRequired: false, isSaved: false, isGuardOnly: true, label: "Copy me", defaultOn: true),
                Placeholder(key: "bcc", isRequired: false, isSaved: false, isGuardOnly: true),
            ])
    }
}

@Suite struct ReplacePlaceholdersTests {
    static let cases: [(String, [String: String], String)] = [
        ("Hello {{name}}!", ["name": "Alice"], "Hello Alice!"),
        ("Hello {{name|Guest}}!", [:], "Hello Guest!"),
        ("Hello {{name|Guest}}!", ["name": "Alice"], "Hello Alice!"),
        ("{{name}} and {{name}} again", ["name": "Alice"], "Alice and Alice again"),
        ("{{first}} {{last}}", ["first": "John", "last": "Doe"], "John Doe"),
        ("{{my-variable}} test", ["my-variable": "value"], "value test"),
        ("Hello {{name}}!", [:], "Hello !"),
        ("Hello {{name|}}!", [:], "Hello !"),
        ("Hello {{name|}}!", ["name": "Alice"], "Hello Alice!"),
        ("Order {{#:id:}}", ["id": "12345"], "Order #12345"),
        ("Price {{:amount:%}}", ["amount": "25"], "Price 25%"),
        ("Price {{$:amount: USD}}", ["amount": "25.50"], "Price $25.50 USD"),
        ("Message{{with :context:}}", ["context": ""], "Message"),
        ("Message{{with :context:}}", ["context": "   "], "Message"),
        ("Price {{$:amount: USD|0.00}}", [:], "Price $0.00 USD"),
        ("Message{{with :context:|}}", [:], "Message"),
        ("Hello {{ name }}!", ["name": "Alice"], "Hello Alice!"),
        ("Hello {{ name | World }}!", [:], "Hello World!"),
        ("Price {{$:amount: USD}} and qty {{qty}}", ["amount": "25", "qty": "3"], "Price $25 USD and qty 3"),
        ("Order {{#:id:}} (ref {{id}})", ["id": "42"], "Order #42 (ref 42)"),
        ("[{{$:price: USD}}] {{price}}", ["price": ""], "[] "),
        ("👩‍💻 {{name}} 日本語 {{#:id:}}", ["name": "Zoë", "id": "7"], "👩‍💻 Zoë 日本語 #7"),
    ]

    @Test(arguments: cases)
    func replaces(text: String, values: [String: String], expected: String) {
        #expect(replace(text, values) == expected)
    }

    /// Wrappers are emitted verbatim, including their own whitespace.
    @Test func wrapperWithWhitespaceAroundKey() {
        let result = replace("Price {{ $ : amount : USD }} done", ["amount": "25"])
        #expect(result.contains("25"))
        #expect(!result.contains("{{"))
        #expect(result.contains("done"))
    }

    @Test func unknownKeysStayVerbatim() {
        let text = "{{a}} {{b}}"
        let result = PlaceholderRenderer.replacePlaceholders(
            text, values: ["a": "1", "b": "2"], placeholders: [Placeholder(key: "a", isRequired: true)])
        #expect(result == "1 {{b}}")
    }
}
