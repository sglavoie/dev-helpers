import Foundation
import Testing
@testable import SloppyCore

private func model(_ content: String, history: PlaceholderHistory = [:]) -> PlaceholderFormModel {
    PlaceholderFormModel(snippetContent: content, placeholders: PlaceholderSyntaxParser.extractPlaceholders(content),
        history: history, now: 1_000)
}

private func stop(_ key: String, _ kind: PlaceholderFocusStop.Kind) -> PlaceholderFocusStop {
    PlaceholderFocusStop(key: key, kind: kind)
}

@Suite struct PlaceholderFormFocusTests {
    @Test func requiredStopsComeBeforeOptionalOnes() {
        let form = model("{{note|}} {{name}} {{#if sig}}-- me{{/if}} {{tone[Formal|Casual]}}")
        #expect(form.focusStops == [
            stop("name", .text),
            stop("tone", .dropdown),
            stop("note", .text),
            stop("sig", .guardToggle),
        ])
    }

    @Test func disabledWrapperShowsOnlyItsToggle() {
        var form = model("Hi{{, :who:}}")
        #expect(form.focusStops == [stop("who", .includeToggle)])
        form.setEnabled("who", true)
        #expect(form.focusStops == [stop("who", .includeToggle), stop("who", .text)])
    }

    @Test func customEntryAddsTextStopAfterDropdown() throws {
        let history: PlaceholderHistory = ["name": [PlaceholderHistoryValue(value: "Ada", useCount: 1, lastUsed: 900,
            createdAt: 1)]]
        var form = model("{{name}}", history: history)
        #expect(form.focusStops == [stop("name", .dropdown)])
        let placeholder = try #require(form.placeholders.first)
        form.selectOption(PlaceholderFormChoices.customValueMarker, for: placeholder)
        #expect(form.focusStops == [stop("name", .dropdown), stop("name", .customText)])
    }

    @Test func tabWrapsBothWays() {
        let form = model("{{a}} {{b}} {{c}}")
        #expect(form.focusStop(after: nil, offset: 1) == stop("a", .text))
        #expect(form.focusStop(after: nil, offset: -1) == stop("c", .text))
        #expect(form.focusStop(after: stop("c", .text), offset: 1) == stop("a", .text))
        #expect(form.focusStop(after: stop("a", .text), offset: -1) == stop("c", .text))
        #expect(form.focusStop(after: stop("gone", .dropdown), offset: 1) == stop("a", .text))
    }

    @Test func firstErrorStopPrefersCustomTextField() throws {
        let history: PlaceholderHistory = ["b": [PlaceholderHistoryValue(value: "x", useCount: 1, lastUsed: 900,
            createdAt: 1)]]
        var form = model("{{a|z}} {{b}} {{c}}", history: history)
        let b = try #require(form.placeholders.first { $0.key == "b" })
        form.selectOption(PlaceholderFormChoices.customValueMarker, for: b)
        let values = form.submit()
        #expect(values == nil)
        #expect(form.firstErrorStop == stop("b", .customText))
    }

    @Test func adjacentOptionClampsToTheList() throws {
        let form = model("{{tone[Formal|Casual]}}")
        let tone = try #require(form.placeholders.first)
        let options = form.dropdownOptions(for: tone).map(\.id)
        #expect(form.adjacentOptionID(for: tone, offset: -1) == options[0])
        #expect(form.adjacentOptionID(for: tone, offset: 1) == options[1])
        #expect(form.adjacentOptionID(for: tone, offset: 10) == options.last)
        let plain = model("{{x}}")
        #expect(plain.adjacentOptionID(for: try #require(plain.placeholders.first), offset: 1) == nil)
    }
}
