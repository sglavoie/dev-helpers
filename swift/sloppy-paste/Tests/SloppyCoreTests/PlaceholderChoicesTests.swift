import Testing
@testable import SloppyCore

private func parse(_ text: String) -> PlaceholderSyntaxResult {
    PlaceholderSyntaxParser.parse(text)
}

private func extract(_ text: String) -> [Placeholder] {
    PlaceholderSyntaxParser.extractPlaceholders(text)
}

private func replace(_ text: String, _ values: [String: String]) -> String {
    PlaceholderRenderer.replacePlaceholders(text, values: values, placeholders: extract(text))
}

@Suite struct PlaceholderChoicesTests {
    @Test func extractsChoicesPreservingExplicitDefaults() {
        #expect(
            extract("{{tone[Formal|Casual|Technical]}}")[0]
                == Placeholder(key: "tone", choices: ["Formal", "Casual", "Technical"], isRequired: true))

        let withDefault = extract("{{tone[Formal|Casual]|Casual}}")[0]
        #expect(withDefault.choices == ["Formal", "Casual"])
        #expect(withDefault.defaultValue == "Casual")
        #expect(!withDefault.isRequired)

        #expect(extract("{{tone[Formal|Casual]|Custom}}")[0].defaultValue == "Custom")

        let emptyDefault = extract("{{tone[Formal|Casual]|}}")[0]
        #expect(emptyDefault.defaultValue == "")
        #expect(!emptyDefault.isRequired)
    }

    @Test func combinesNoSaveWrappersChoicesAndDefault() {
        #expect(
            extract("{{!$:amount[10|20]: USD|10}}")[0]
                == Placeholder(
                    key: "amount", choices: ["10", "20"], defaultValue: "10", isRequired: false, isSaved: false,
                    prefixWrapper: "$", suffixWrapper: " USD"))
    }

    @Test func trimsAndDecodesChoicesWithoutTreatingColonsAsWrappers() {
        #expect(extract(#"{{value[ A\|B | \[C\] | D\\E ]}}"#)[0].choices == ["A|B", "[C]", #"D\E"#])

        let withColons = extract("{{mode[fast:cheap|slow:careful]}}")[0]
        #expect(withColons.key == "mode")
        #expect(withColons.choices == ["fast:cheap", "slow:careful"])
    }

    @Test func declarationOwnsMetadataRegardlessOfOrder() {
        let before = extract("{{tone|legacy}} {{tone[Formal|Casual]|Casual}} {{tone}}")
        let after = extract("{{tone[Formal|Casual]|Casual}} {{tone|legacy}}")

        #expect(
            before == [
                Placeholder(key: "tone", choices: ["Formal", "Casual"], defaultValue: "Casual", isRequired: false)
            ])
        #expect(after == before)
    }

    @Test func occurrenceSpecificWrappers() {
        let text = "{{$:amount[10|20]: USD|10}} / {{~:amount[10|20]:kg|10}} / {{amount}}"
        #expect(parse(text).diagnostics.isEmpty)
        #expect(replace(text, ["amount": "25"]) == "$25 USD / ~25kg / 25")
        #expect(replace(text, ["amount": "   "]) == " /  / ")
    }

    @Test func reportsEveryConflictingDeclaration() {
        let diagnostics = parse("{{tone[A|B]}} {{tone[A|C]}} {{!tone[A|B]}}").diagnostics
        #expect(diagnostics.map(\.expression) == ["{{tone[A|B]}}", "{{tone[A|C]}}", "{{!tone[A|B]}}"])
        #expect(diagnostics.allSatisfy { $0.message.contains("Conflicting authored choices") })
    }

    @Test func wrapperTextMayDifferButRequiredStatusMayNot() {
        #expect(parse("{{$:amount[10|20]: USD}} {{~:amount[10|20]:kg}}").diagnostics.isEmpty)

        let diagnostics = parse("{{amount[10|20]}} {{$:amount[10|20]: USD}}").diagnostics
        #expect(diagnostics.count == 2)
        #expect(diagnostics.first?.message.contains("required/optional status") == true)
    }

    @Test func rawExpressionsAndExactSourceRanges() {
        let text = "Before {{tone[A|B]}} after"
        let occurrence = parse(text).occurrences[0]
        #expect(occurrence.raw == "{{tone[A|B]}}")
        #expect(String(text[occurrence.indices]) == occurrence.raw)
        #expect(occurrence.range == PlaceholderSourceRange(start: 7, end: 20))
        #expect(!occurrence.hasExplicitDefault)
    }

    @Test func rangesAreUTF16Offsets() {
        let text = "👩‍💻 日本 {{tone[A|B]}}"
        let occurrence = parse(text).occurrences[0]
        #expect(occurrence.range == PlaceholderSourceRange(start: 9, end: 22))
        #expect(String(text[occurrence.indices]) == "{{tone[A|B]}}")
    }

    @Test func bracketsInOrdinaryDefaultsAndWrappersStayLegacy() {
        let defaultText = "{{key|[none]}}"
        #expect(parse(defaultText).diagnostics.isEmpty)
        #expect(extract(defaultText)[0].key == "key")
        #expect(extract(defaultText)[0].defaultValue == "[none]")
        #expect(replace(defaultText, [:]) == "[none]")

        let wrapperText = "{{pre[fix]:key:suf[fix]}}"
        #expect(parse(wrapperText).diagnostics.isEmpty)
        let wrapper = extract(wrapperText)[0]
        #expect(wrapper.key == "key")
        #expect(wrapper.prefixWrapper == "pre[fix]")
        #expect(wrapper.suffixWrapper == "suf[fix]")
    }

    @Test func bracketPrefixWithChoicesIsDiagnosed() {
        let choiceText = "{{pre[fix]:key[a|b]:suffix}}"
        let parsedChoice = parse(choiceText)
        #expect(parsedChoice.diagnostics.count == 1)
        #expect(
            parsedChoice.diagnostics[0].message.contains(
                "brackets in a prefix wrapper cannot be combined with authored choices"))
        #expect(parsedChoice.diagnostics[0].message.contains("remove the brackets from the prefix wrapper"))
        _ = replace(choiceText, [:])

        let parsedLegacy = parse("{{pre[fix]:key:suffix}}")
        #expect(parsedLegacy.diagnostics.isEmpty)
        let occurrence = parsedLegacy.occurrences[0]
        #expect(occurrence.key == "key")
        #expect(occurrence.prefixWrapper == "pre[fix]")
        #expect(occurrence.suffixWrapper == "suffix")
        #expect(!occurrence.isChoiceDeclaration)
    }

    @Test func bracketsInChoiceSuffixAndDefaultAreDiagnosedPrecisely() {
        let suffix = parse("{{pre:key[a|b]:suf[fix]}}").diagnostics[0].message
        #expect(suffix.contains("brackets in a suffix wrapper"))
        #expect(suffix.contains("remove the brackets from the suffix wrapper"))
        #expect(!suffix.contains("only one choice list is allowed"))

        let defaultMessage = parse("{{key[a|b]|c[d]}}").diagnostics[0].message
        #expect(defaultMessage.contains("brackets in a default value"))
        #expect(defaultMessage.contains("remove the brackets from the default value"))
        #expect(!defaultMessage.contains("only one choice list is allowed"))

        #expect(parse("{{key[a|b][c|d]}}").diagnostics[0].message.contains("only one choice list is allowed"))
    }

    @Test func unmatchedClosingBracketInKey() {
        let message = parse("{{tone]}}").diagnostics[0].message
        #expect(message.contains("brackets are reserved in choice-capable key positions"))
        #expect(message.contains("rename the placeholder key"))
        #expect(!message.contains("escape a literal bracket as"))
    }

    @Test func unmatchedClosingBracketsInSuffixAndDefault() {
        let suffix = parse("{{pre:key[a|b]:suf]}}").diagnostics[0].message
        #expect(suffix.contains("brackets in a suffix wrapper"))
        #expect(suffix.contains("remove the brackets from the suffix wrapper"))
        #expect(!suffix.contains("rename the placeholder key"))

        let defaultMessage = parse("{{key[a|b]|c]}}").diagnostics[0].message
        #expect(defaultMessage.contains("brackets in a default value"))
        #expect(defaultMessage.contains("remove the brackets from the default value"))
        #expect(!defaultMessage.contains("rename the placeholder key"))

        let key = parse("{{ke]y[a|b]}}").diagnostics[0].message
        #expect(key.contains("brackets are reserved in choice-capable key positions"))
        #expect(key.contains("rename the placeholder key"))

        #expect(parse("{{key|c]}}").diagnostics.isEmpty)
        #expect(parse("{{pre:key:suf]|x}}").diagnostics.isEmpty)
    }

    @Test(arguments: [
        "{{tone[]}}",
        "{{tone[Only]}}",
        "{{tone[A||B]}}",
        "{{tone[A|A]}}",
        "{{tone[A|B}}",
        "{{tone]}}",
        "{{tone[A[B|C]}}",
        #"{{tone[A|B\}}"#,
        #"{{tone[A|B\q]}}"#,
        "{{prefix:tone[A|B]}}",
    ])
    func malformedChoicesKeepRuntimeFallbackSafe(text: String) {
        let parsed = parse(text)
        #expect(!parsed.diagnostics.isEmpty)
        #expect(parsed.diagnostics.first?.range.start == 0)
        _ = replace(text, [:])
    }
}

/// Diagnostics generated by running the TS `parsePlaceholderSyntax` under
/// Node, so the Swift messages and UTF-16 ranges must match verbatim.
@Suite struct PlaceholderDiagnosticGoldenTests {
    struct Expected: Sendable, Equatable, CustomStringConvertible {
        var start: Int
        var end: Int
        var message: String

        init(_ start: Int, _ end: Int, _ message: String) {
            self.start = start
            self.end = end
            self.message = message
        }

        var description: String { "[\(start), \(end)) \(message)" }
    }

    static let cases: [(String, [Expected])] = [
        (#"{{tone[]}}"#, [
            .init(0, 10, #"Invalid authored choices in {{tone[]}}: choice values cannot be empty; remove the empty entry or enter a value"#),
        ]),
        (#"{{tone[Only]}}"#, [
            .init(0, 14, #"Invalid authored choices in {{tone[Only]}}: add at least two unique choices separated by `|`"#),
        ]),
        (#"{{tone[A||B]}}"#, [
            .init(0, 14, #"Invalid authored choices in {{tone[A||B]}}: choice values cannot be empty; remove the empty entry or enter a value"#),
        ]),
        (#"{{tone[A|A]}}"#, [
            .init(0, 13, #"Invalid authored choices in {{tone[A|A]}}: choice "A" is duplicated; keep each choice unique"#),
        ]),
        (#"{{tone[A|B}}"#, [
            .init(0, 12, #"Invalid authored choices in {{tone[A|B}}: the choice list is missing a closing bracket `]`"#),
        ]),
        (#"{{tone]}}"#, [
            .init(0, 9, #"Invalid authored choices in {{tone]}}: found an unmatched closing bracket; brackets are reserved in choice-capable key positions, so rename the placeholder key to remove `]`"#),
        ]),
        (#"{{tone[A[B|C]}}"#, [
            .init(0, 15, #"Invalid authored choices in {{tone[A[B|C]}}: nested choice brackets are not allowed; escape a literal bracket as \["#),
        ]),
        (#"{{tone[A|B\}}"#, [
            .init(0, 13, #"Invalid authored choices in {{tone[A|B\}}: the choice list ends with a dangling escape; remove it or escape a supported character"#),
        ]),
        (#"{{tone[A|B\q]}}"#, [
            .init(0, 15, #"Invalid authored choices in {{tone[A|B\q]}}: unsupported escape \q; only \|, \[, \], and \\ are supported"#),
        ]),
        (#"{{prefix:tone[A|B]}}"#, [
            .init(0, 20, #"Invalid authored choices in {{prefix:tone[A|B]}}: wrapper syntax must contain exactly two colons: `prefix:key[one|two]:suffix`"#),
        ]),
        (#"{{pre[fix]:key[a|b]:suffix}}"#, [
            .init(0, 28, #"Invalid authored choices in {{pre[fix]:key[a|b]:suffix}}: brackets in a prefix wrapper cannot be combined with authored choices; remove the brackets from the prefix wrapper or remove the choice list"#),
        ]),
        (#"{{pre:key[a|b]:suf[fix]}}"#, [
            .init(0, 25, #"Invalid authored choices in {{pre:key[a|b]:suf[fix]}}: brackets in a suffix wrapper cannot be combined with authored choices; remove the brackets from the suffix wrapper or remove the choice list"#),
        ]),
        (#"{{key[a|b]|c[d]}}"#, [
            .init(0, 17, #"Invalid authored choices in {{key[a|b]|c[d]}}: brackets in a default value cannot be combined with authored choices; remove the brackets from the default value or remove the choice list"#),
        ]),
        (#"{{key[a|b][c|d]}}"#, [
            .init(0, 17, #"Invalid authored choices in {{key[a|b][c|d]}}: only one choice list is allowed per placeholder expression"#),
        ]),
        (#"{{pre:key[a|b]:suf]}}"#, [
            .init(0, 21, #"Invalid authored choices in {{pre:key[a|b]:suf]}}: brackets in a suffix wrapper cannot be combined with authored choices; remove the brackets from the suffix wrapper or remove the choice list"#),
        ]),
        (#"{{key[a|b]|c]}}"#, [
            .init(0, 15, #"Invalid authored choices in {{key[a|b]|c]}}: brackets in a default value cannot be combined with authored choices; remove the brackets from the default value or remove the choice list"#),
        ]),
        (#"{{ke]y[a|b]}}"#, [
            .init(0, 13, #"Invalid authored choices in {{ke]y[a|b]}}: found an unmatched closing bracket; brackets are reserved in choice-capable key positions, so rename the placeholder key to remove `]`"#),
        ]),
        (#"{{[a|b]}}"#, [
            .init(0, 9, #"Invalid authored choices in {{[a|b]}}: enter a placeholder key before the choice list"#),
        ]),
        (#"{{key[a|b]x}}"#, [
            .init(0, 13, #"Invalid authored choices in {{key[a|b]x}}: place the closing choice bracket immediately after the final choice"#),
        ]),
        (#"{{tone[A|B]}} {{tone[A|C]}} {{!tone[A|B]}}"#, [
            .init(0, 13, #"Conflicting authored choices for "tone" in {{tone[A|B]}}: all declarations must use the same choices, save policy."#),
            .init(14, 27, #"Conflicting authored choices for "tone" in {{tone[A|C]}}: all declarations must use the same choices, save policy."#),
            .init(28, 42, #"Conflicting authored choices for "tone" in {{!tone[A|B]}}: all declarations must use the same choices, save policy."#),
        ]),
        (#"{{amount[10|20]}} {{$:amount[10|20]: USD}}"#, [
            .init(0, 17, #"Conflicting authored choices for "amount" in {{amount[10|20]}}: all declarations must use the same required/optional status."#),
            .init(18, 42, #"Conflicting authored choices for "amount" in {{$:amount[10|20]: USD}}: all declarations must use the same required/optional status."#),
        ]),
        (#"{{q["x"|"x"]}}"#, [
            .init(0, 14, #"Invalid authored choices in {{q["x"|"x"]}}: choice "\"x\"" is duplicated; keep each choice unique"#),
        ]),
        (#"{{a[x|y]|1}} {{a[x|y]|2}}"#, [
            .init(0, 12, #"Conflicting authored choices for "a" in {{a[x|y]|1}}: all declarations must use the same default."#),
            .init(13, 25, #"Conflicting authored choices for "a" in {{a[x|y]|2}}: all declarations must use the same default."#),
        ]),
        (#"{{:[a|b]:x}}"#, [
            .init(0, 12, #"Invalid authored choices in {{:[a|b]:x}}: enter a placeholder key before the choice list"#),
        ]),
        (#"{{k|v[a|b]}}"#, []),
        (#"👩‍💻 {{t[A|B]}} 日本 {{t[A|C]}}"#, [
            .init(6, 16, #"Conflicting authored choices for "t" in {{t[A|B]}}: all declarations must use the same choices."#),
            .init(20, 30, #"Conflicting authored choices for "t" in {{t[A|C]}}: all declarations must use the same choices."#),
        ]),
    ]

    @Test(arguments: cases)
    func matchesTypeScript(text: String, expected: [Expected]) {
        let actual = PlaceholderSyntaxParser.parse(text).diagnostics.map {
            Expected($0.range.start, $0.range.end, $0.message)
        }
        #expect(actual == expected)
    }
}
