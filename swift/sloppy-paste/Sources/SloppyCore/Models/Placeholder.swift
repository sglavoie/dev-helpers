/// A placeholder parsed from snippet content.
public struct Placeholder: Sendable, Hashable {
    public var key: String
    /// Stable authored choices, in declaration order.
    public var choices: [String]?
    public var defaultValue: String?
    public var isRequired: Bool
    /// Whether values are saved to history (default true).
    public var isSaved: Bool
    public var prefixWrapper: String?
    public var suffixWrapper: String?
    /// Key used only in `{{#if key}}`; renders as a checkbox.
    public var isGuardOnly: Bool
    /// Optional human-readable label for guard-only checkboxes.
    public var label: String?
    /// Whether a guard-only checkbox defaults to checked.
    public var defaultOn: Bool

    public init(
        key: String,
        choices: [String]? = nil,
        defaultValue: String? = nil,
        isRequired: Bool,
        isSaved: Bool = true,
        prefixWrapper: String? = nil,
        suffixWrapper: String? = nil,
        isGuardOnly: Bool = false,
        label: String? = nil,
        defaultOn: Bool = false
    ) {
        self.key = key
        self.choices = choices
        self.defaultValue = defaultValue
        self.isRequired = isRequired
        self.isSaved = isSaved
        self.prefixWrapper = prefixWrapper
        self.suffixWrapper = suffixWrapper
        self.isGuardOnly = isGuardOnly
        self.label = label
        self.defaultOn = defaultOn
    }
}
