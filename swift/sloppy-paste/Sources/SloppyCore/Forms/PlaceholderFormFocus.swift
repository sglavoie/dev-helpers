import Foundation

/// One keyboard stop in the placeholder form. Tab and ⇧Tab walk these in
/// order, so the tab order follows the form layout rather than AppKit's
/// key-view loop, which skips popups and checkboxes unless Full Keyboard
/// Access is on.
public struct PlaceholderFocusStop: Sendable, Hashable {
    public enum Kind: Sendable, Hashable {
        /// The `{{#if key}}` checkbox.
        case guardToggle
        /// The "Include" checkbox of an optional wrapper field.
        case includeToggle
        /// Authored choices or history values.
        case dropdown
        /// The free-text field under a dropdown set to the custom entry.
        case customText
        /// A plain text field (no choices, no history).
        case text
    }

    public var key: String
    public var kind: Kind

    public init(key: String, kind: Kind) {
        self.key = key
        self.kind = kind
    }

    /// Whether the stop is a text field that takes real keyboard focus.
    public var isTextInput: Bool { kind == .customText || kind == .text }
}

extension PlaceholderFormModel {
    /// The visible stops in layout order: required fields, then optional ones.
    /// A disabled wrapper shows only its toggle, and a dropdown is followed by
    /// its custom text field while the custom entry is selected.
    public var focusStops: [PlaceholderFocusStop] {
        var stops: [PlaceholderFocusStop] = []
        for placeholder in orderedPlaceholders {
            let key = placeholder.key
            if placeholder.isGuardOnly {
                stops.append(PlaceholderFocusStop(key: key, kind: .guardToggle))
                continue
            }
            if isWrapperField(placeholder) {
                stops.append(PlaceholderFocusStop(key: key, kind: .includeToggle))
                if !isEnabled(placeholder) { continue }
            }
            switch control(for: placeholder) {
            case .dropdown:
                stops.append(PlaceholderFocusStop(key: key, kind: .dropdown))
                if showsCustomInput(placeholder) {
                    stops.append(PlaceholderFocusStop(key: key, kind: .customText))
                }
            case .textField, .guardToggle:
                stops.append(PlaceholderFocusStop(key: key, kind: .text))
            }
        }
        return stops
    }

    /// The stop `offset` places from `current`, wrapping around. A missing or
    /// vanished `current` starts from the first stop (or the last, backwards).
    public func focusStop(after current: PlaceholderFocusStop?, offset: Int) -> PlaceholderFocusStop? {
        let stops = focusStops
        guard !stops.isEmpty else { return nil }
        guard let current, let index = stops.firstIndex(of: current) else {
            return offset < 0 ? stops.last : stops.first
        }
        let count = stops.count
        return stops[((index + offset) % count + count) % count]
    }

    /// Where a failed submit sends focus: the first field with an error, on
    /// its custom text field when the custom entry is selected.
    public var firstErrorStop: PlaceholderFocusStop? {
        focusStops.first { stop in
            guard errors[stop.key] != nil else { return false }
            switch stop.kind {
            case .guardToggle, .includeToggle: return false
            case .dropdown: return !(useCustomInput[stop.key] ?? false)
            case .customText, .text: return true
            }
        }
    }

    /// The dropdown option `offset` places from the current selection,
    /// clamped to the list; nil when the field has no dropdown.
    public func adjacentOptionID(for placeholder: Placeholder, offset: Int) -> String? {
        guard control(for: placeholder) == .dropdown else { return nil }
        let options = dropdownOptions(for: placeholder)
        guard !options.isEmpty else { return nil }
        let selected = dropdownSelection(for: placeholder.key)
        let index = options.firstIndex { $0.id == selected } ?? 0
        return options[min(max(index + offset, 0), options.count - 1)].id
    }
}
