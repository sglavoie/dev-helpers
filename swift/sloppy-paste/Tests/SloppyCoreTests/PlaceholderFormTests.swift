import Foundation
import Testing
@testable import SloppyCore

private typealias Choices = PlaceholderFormChoices

private func choicePlaceholder(
    defaultValue: String? = nil,
    isRequired: Bool = true,
    prefix: String? = nil,
    suffix: String? = nil
) -> Placeholder {
    Placeholder(key: "tone", choices: ["Formal", "Casual", "Technical"], defaultValue: defaultValue,
        isRequired: isRequired, prefixWrapper: prefix, suffixWrapper: suffix)
}

@Suite struct AuthoredChoiceFormStateTests {
    @Test func selectsFirstChoiceWithoutExplicitDefault() {
        let state = Choices.initialState(choicePlaceholder())
        #expect(state.formValue == "Formal")
        #expect(state.dropdownSelection == Choices.authoredChoiceOptionID(0))
        #expect(state.customValue == "")
        #expect(!state.useCustomInput)
    }

    @Test func selectsExactMatchingExplicitDefault() {
        let state = Choices.initialState(choicePlaceholder(defaultValue: "Casual", isRequired: false))
        #expect(state.formValue == "Casual")
        #expect(state.dropdownSelection == Choices.authoredChoiceOptionID(1))
        #expect(!state.useCustomInput)
    }

    @Test(arguments: ["Conversational", ""])
    func outOfListDefaultUsesCustom(defaultValue: String) {
        let state = Choices.initialState(choicePlaceholder(defaultValue: defaultValue, isRequired: false))
        #expect(state.formValue == defaultValue)
        #expect(state.dropdownSelection == Choices.customValueMarker)
        #expect(state.customValue == defaultValue)
        #expect(state.useCustomInput)
    }

    @Test func collisionSafeOptionIDsKeepAuthoredOrderAndValues() {
        let choices = [Choices.customValueMarker, Choices.authoredChoiceOptionID(0), "A|B"]
        let ids = choices.indices.map(Choices.authoredChoiceOptionID)
        #expect(ids == [Choices.authoredChoiceOptionID(0), Choices.authoredChoiceOptionID(1),
            Choices.authoredChoiceOptionID(2)])
        #expect(!ids.contains(Choices.customValueMarker))
        #expect(ids.map { Choices.authoredChoiceValue(choices, optionID: $0) } == choices)
        #expect(Choices.authoredChoiceValue(choices, optionID: "not-an-option-id") == nil)
        #expect(Choices.authoredChoiceValue(choices, optionID: "__AUTHORED_CHOICE__9") == nil)
        #expect(Choices.authoredChoiceValue(choices, optionID: "__AUTHORED_CHOICE__-1") == nil)
    }

    @Test func customTextSurvivesModeSwitches() throws {
        let choices = ["Formal", "Casual"]
        let custom = try #require(Choices.resolveSelection(choices, optionID: Choices.customValueMarker,
            customValue: "Conversational"))
        #expect(custom.formValue == "Conversational")
        #expect(custom.customValue == "Conversational")
        #expect(custom.useCustomInput)

        let authored = try #require(Choices.resolveSelection(choices, optionID: Choices.authoredChoiceOptionID(1),
            customValue: custom.customValue))
        #expect(authored.formValue == "Casual")
        #expect(authored.customValue == "Conversational")
        #expect(!authored.useCustomInput)

        let back = try #require(Choices.resolveSelection(choices, optionID: Choices.customValueMarker,
            customValue: authored.customValue))
        #expect(back.formValue == "Conversational")
        #expect(back.customValue == "Conversational")
        #expect(back.useCustomInput)
    }

    @Test func wrapperWithoutDefaultStartsDisabledWithFirstChoice() {
        let state = Choices.initialState(choicePlaceholder(isRequired: false, prefix: "(", suffix: ")"))
        #expect(state.formValue == "Formal")
        #expect(state.dropdownSelection == Choices.authoredChoiceOptionID(0))
        #expect(state.enabledOptional == false)
    }

    @Test func wrapperWithNonEmptyDefaultStartsEnabled() {
        let state = Choices.initialState(
            choicePlaceholder(defaultValue: "Casual", isRequired: false, prefix: "(", suffix: ")"))
        #expect(state.formValue == "Casual")
        #expect(state.dropdownSelection == Choices.authoredChoiceOptionID(1))
        #expect(state.enabledOptional == true)
    }

    @Test func blankCustomOnRequiredFieldIsAnError() {
        let placeholder = choicePlaceholder()
        let custom = Choices.resolveSelection(placeholder.choices!, optionID: Choices.customValueMarker, customValue: "")
        #expect(Choices.requiredErrors([placeholder], finalValues: ["tone": custom?.formValue ?? ""])
            == ["tone": "This field is required"])
    }
}

/// The TS submission tests, driven through the form model, `SnippetPreparation`
/// and `SnippetUse` against an in-memory repository.
@Suite struct PlaceholderFormSubmissionTests {
    static let now: Int64 = 1_700_000_000_000

    static let placeholders: [Placeholder] = [
        Placeholder(key: "required", isRequired: true),
        Placeholder(key: "optional", defaultValue: "fallback", isRequired: false),
        Placeholder(key: "wrapped", isRequired: false, prefixWrapper: "[", suffixWrapper: "]"),
        Placeholder(key: "guard", isRequired: false, isSaved: false, isGuardOnly: true),
        Placeholder(key: "private", isRequired: false, isSaved: false),
        Placeholder(key: "blank", isRequired: false),
    ]

    static let submittedValues = [
        "required": "Ada", "optional": "fallback", "wrapped": "", "guard": "true", "private": "secret", "blank": "  ",
    ]

    /// Stands in for the app's store: one repository, a write counter and a clipboard.
    actor Store {
        var repo = SnippetRepository()
        var writes = 0
        var clipboard: [String] = []
        var failCopy = false

        func add(_ content: String) -> Snippet {
            repo.addSnippet(title: "Test", content: content, now: 1, id: "snippet-1")
        }
        func setFailCopy() { failCopy = true }
        func copy(_ content: String) throws {
            if failCopy { throw TestFailure(message: "Clipboard unavailable") }
            clipboard.append(content)
        }
        func record(_ id: String, _ values: [PlaceholderValueToRecord]) throws {
            try repo.recordUse(id: id, placeholderValues: values, now: PlaceholderFormSubmissionTests.now)
            writes += 1
        }
        func history(_ key: String) -> [PlaceholderHistoryValue] { repo.data.placeholderHistory(forKey: key) }
        func useCount() -> Int { repo.snippets[0].useCount }
    }

    private func submit(_ store: Store, _ snippet: Snippet, _ placeholders: [Placeholder],
        _ finalValues: [String: String]) async -> Bool
    {
        let prepared = SnippetPreparation.submission(content: snippet.content, placeholders: placeholders,
            finalValues: finalValues)
        return await SnippetUse(record: { try await store.record($0, $1) }, log: { _ in }).run(
            snippetID: snippet.id,
            placeholderValues: prepared.placeholderValues,
            prepare: { prepared.content },
            primaryOperation: { try await store.copy($0) },
            onPreparationFailure: { _ in },
            onPrimaryFailure: { _ in }
        )
    }

    @Test func tracksNormalizedValuesAndParsedMetadata() {
        #expect(SnippetPreparation.trackedValues(Self.placeholders, finalValues: Self.submittedValues) == [
            .init(key: "required", value: "Ada", isSaved: true),
            .init(key: "optional", value: "fallback", isSaved: true),
            .init(key: "wrapped", value: "", isSaved: true),
            .init(key: "guard", value: "true", isSaved: false),
            .init(key: "private", value: "secret", isSaved: false),
            .init(key: "blank", value: "  ", isSaved: true),
        ])
    }

    @Test func copiesOnceAndRecordsEligibleValuesWithOneWrite() async {
        let store = Store()
        let snippet = await store.add("Hello {{required}}")
        #expect(await submit(store, snippet, Self.placeholders, Self.submittedValues))

        #expect(await store.clipboard == ["Hello Ada"])
        #expect(await store.writes == 1)
        #expect(await store.useCount() == 1)
        #expect(await store.history("required").map(\.value) == ["Ada"])
        #expect(await store.history("optional").map(\.value) == ["fallback"])
        for key in ["wrapped", "guard", "private", "blank"] {
            #expect(await store.history(key).isEmpty)
        }
    }

    @Test func recordsAuthoredAndCustomChoicesHonouringNoSave() async {
        let store = Store()
        let snippet = await store.add("{{tone[Formal|Casual]}} / {{detail[Short|Long]|Other}} / {{!private[Yes|No]|No}}")
        let placeholders = PlaceholderSyntaxParser.extractPlaceholders(snippet.content)
        #expect(placeholders == [
            Placeholder(key: "tone", choices: ["Formal", "Casual"], isRequired: true),
            Placeholder(key: "detail", choices: ["Short", "Long"], defaultValue: "Other", isRequired: false),
            Placeholder(key: "private", choices: ["Yes", "No"], defaultValue: "No", isRequired: false, isSaved: false),
        ])

        #expect(await submit(store, snippet, placeholders,
            ["tone": "Formal", "detail": "Conversational", "private": "Yes"]))
        #expect(await store.clipboard == ["Formal / Conversational / Yes"])
        #expect(await store.history("tone").map(\.value) == ["Formal"])
        #expect(await store.history("detail").map(\.value) == ["Conversational"])
        #expect(await store.history("private").isEmpty)
    }

    @Test func copyFailureTracksNothing() async {
        let store = Store()
        let snippet = await store.add("Hello {{required}}")
        await store.setFailCopy()
        #expect(!(await submit(store, snippet, Self.placeholders, Self.submittedValues)))
        #expect(await store.writes == 0)
        #expect(await store.useCount() == 0)
        #expect(await store.history("required").isEmpty)
    }
}

@Suite struct PlaceholderFormModelTests {
    static let now: Int64 = 1_700_000_000_000
    static let day: Int64 = 86_400_000

    private func model(_ content: String, history: PlaceholderHistory = [:], max: Int = 20) -> PlaceholderFormModel {
        PlaceholderFormModel(snippetContent: content,
            placeholders: PlaceholderSyntaxParser.extractPlaceholders(content), history: history,
            maxDisplayValues: max, now: Self.now)
    }

    private func value(_ v: String, uses: Int = 1, daysAgo: Int64) -> PlaceholderHistoryValue {
        PlaceholderHistoryValue(value: v, useCount: uses, lastUsed: Self.now - daysAgo * Self.day, createdAt: 0)
    }

    private func placeholder(_ m: PlaceholderFormModel, _ key: String) -> Placeholder {
        m.placeholders.first { $0.key == key }!
    }

    @Test func prefillsLastUsedValueAndRanksSuggestions() {
        let m = model("Hi {{name}}", history: ["name": [
            value("Bob", uses: 500, daysAgo: 10), value("Ada", uses: 1, daysAgo: 0),
        ]])
        #expect(m.value(for: "name") == "Ada")
        #expect(m.prefilledKeys == ["name"])
        #expect(m.historySuggestions["name"] == ["Bob", "Ada"])
        #expect(m.dropdownSelection(for: "name") == "Ada")
        #expect(!m.showsCustomInput(placeholder(m, "name")))
        #expect(m.control(for: placeholder(m, "name")) == .dropdown)
        #expect(m.title(for: placeholder(m, "name")) == "name * (↻ last used)")
        #expect(m.summary(mode: .paste) == "All 1 field pre-filled from history — submit to paste.")
    }

    @Test func suggestionsHonourTheDisplayLimit() {
        let m = model("{{k}}", history: ["k": (0..<30).map { value("v\($0)", daysAgo: Int64($0)) }], max: 5)
        #expect(m.historySuggestions["k"]?.count == 5)
        #expect(m.dropdownOptions(for: placeholder(m, "k")).count == 6)
        #expect(m.dropdownOptions(for: placeholder(m, "k")).last?.title == "Enter new value...")
    }

    @Test func withoutHistoryUsesDefaultInTextField() {
        let m = model("{{k|fallback}} {{req}}")
        #expect(m.value(for: "k") == "fallback")
        #expect(m.control(for: placeholder(m, "k")) == .textField)
        #expect(m.showsCustomInput(placeholder(m, "k")))
        #expect(m.prefilledKeys.isEmpty)
        #expect(m.summary(mode: .copy) == nil)
        #expect(m.orderedPlaceholders.map(\.key) == ["req", "k"])
        #expect(m.hasBothSections)
        #expect(m.navigationTitle(snippetTitle: "T") == "Fill Placeholders (0/1): T")
    }

    @Test func partialPrefillSummary() {
        let m = model("{{a}} {{b}}", history: ["a": [value("x", daysAgo: 1)]])
        #expect(m.summary(mode: .copy) == "1 of 2 fields pre-filled from history — review and submit.")
        #expect(m.filledRequiredCount == 1)
    }

    @Test func wrapperEnablementFollowsHistoryOrDefault() {
        let m = model("{{(:a:)}} {{(:b:)|x}} {{(:c:)}}", history: ["c": [value("y", daysAgo: 1)]])
        #expect(!m.isEnabled(placeholder(m, "a")))
        #expect(m.isEnabled(placeholder(m, "b")))
        #expect(m.isEnabled(placeholder(m, "c")))
        #expect(m.previewContent == " (x) (y)")
    }

    @Test func guardsDefaultToDefaultOnAndMapToTrueOrEmpty() throws {
        var m = model(#"{{#if a}}A{{/if}}{{#if +b "Bee"}}B{{/if}}"#)
        let b = placeholder(m, "b")
        #expect(m.control(for: b) == .guardToggle)
        #expect(m.guardLabel(for: b) == "Bee")
        #expect(m.guardLabel(for: placeholder(m, "a")) == "Include a?")
        #expect(m.previewValues["a"] == "")
        #expect(m.previewValues["b"] == "true")
        #expect(m.previewContent == "B")

        m.setEnabled("a", true)
        m.setEnabled("b", false)
        let submitted = m.submit()
        let values = try #require(submitted)
        #expect(values["a"] == "true")
        #expect(values["b"] == "")
        #expect(m.fieldPreview(for: placeholder(m, "a")) == "Toggles: \"A\"")
    }

    @Test func customEntryKeepsTypedTextAcrossHistoryPicks() {
        var m = model("{{k}}", history: ["k": [value("old", daysAgo: 1)]])
        let k = placeholder(m, "k")
        m.selectOption(PlaceholderFormChoices.customValueMarker, for: k)
        #expect(m.value(for: "k") == "")
        #expect(m.showsCustomInput(k))
        m.setCustomValue("k", "new")
        m.selectOption("old", for: k)
        #expect(m.value(for: "k") == "old")
        #expect(!m.showsCustomInput(k))
        m.selectOption(PlaceholderFormChoices.customValueMarker, for: k)
        #expect(m.value(for: "k") == "new")
    }

    @Test func authoredChoiceDropdownIgnoresUnknownOptions() {
        var m = model("{{tone[Formal|Casual]}}")
        let tone = placeholder(m, "tone")
        #expect(m.dropdownOptions(for: tone).map(\.title) == ["Formal", "Casual", "Enter custom value…"])
        m.selectOption(PlaceholderFormChoices.authoredChoiceOptionID(1), for: tone)
        #expect(m.value(for: "tone") == "Casual")
        m.selectOption("Formal", for: tone)
        #expect(m.value(for: "tone") == "Casual")
        #expect(m.dropdownSelection(for: "tone") == PlaceholderFormChoices.authoredChoiceOptionID(1))
    }

    @Test func requiredErrorsBlockSubmitAndClearOnEdit() throws {
        var m = model("Hello {{name}}{{(:x:)}}")
        let rejected = m.submit()
        #expect(rejected == nil)
        #expect(m.errors == ["name": "This field is required"])
        m.setCustomValue("name", "Ada")
        #expect(m.errors.isEmpty)
        let submission = m.prepareSubmission()
        let prepared = try #require(submission)
        #expect(prepared.content == "Hello Ada")
        #expect(prepared.placeholderValues == [
            .init(key: "name", value: "Ada"), .init(key: "x", value: ""),
        ])
    }

    @Test func disabledWrapperIsBlankedEvenWithAValue() throws {
        var m = model("A{{(:x:)}}B")
        m.setCustomValue("x", "val")
        #expect(m.previewContent == "AB")
        m.setEnabled("x", true)
        #expect(m.previewContent == "A(val)B")
        m.setEnabled("x", false)
        let submitted = m.submit()
        #expect(try #require(submitted)["x"] == "")
    }

    @Test func useDefaultsResetsOptionalFieldsOnly() {
        var m = model("{{req}} {{opt|dflt}} {{(:w:)}} {{tone[A|B]|B}} {{#if g}}G{{/if}}",
            history: ["req": [value("r", daysAgo: 1)], "opt": [value("dflt", daysAgo: 2), value("h", daysAgo: 1)]])
        m.setCustomValue("req", "typed")
        m.selectOption("h", for: placeholder(m, "opt"))
        m.setEnabled("w", true)
        m.selectOption(PlaceholderFormChoices.authoredChoiceOptionID(0), for: placeholder(m, "tone"))
        m.setEnabled("g", true)

        m.useDefaults()
        #expect(m.value(for: "req") == "typed")
        #expect(m.value(for: "opt") == "dflt")
        #expect(m.dropdownSelection(for: "opt") == "dflt")
        #expect(!m.showsCustomInput(placeholder(m, "opt")))
        #expect(!m.isEnabled(placeholder(m, "w")))
        #expect(m.value(for: "tone") == "B")
        #expect(m.isEnabled(placeholder(m, "g")))
    }

    @Test func useDefaultsFallsBackToCustomWhenDefaultIsNotInHistory() {
        var m = model("{{opt|dflt}}", history: ["opt": [value("h", daysAgo: 1)]])
        m.useDefaults()
        #expect(m.value(for: "opt") == "dflt")
        #expect(m.customValue(for: "opt") == "dflt")
        #expect(m.dropdownSelection(for: "opt") == PlaceholderFormChoices.customValueMarker)
        #expect(m.showsCustomInput(placeholder(m, "opt")))
    }

    @Test func previewRendersEmojiAndCJK() {
        var m = model("👋 {{名前}}さん{{#if +g}} 🎉{{/if}}")
        m.setCustomValue("名前", "太郎")
        #expect(m.previewContent == "👋 太郎さん 🎉")
    }
}
