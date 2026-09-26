public enum SortOption: String, Codable, Sendable, CaseIterable {
    case updatedDesc = "updated-desc"
    case mostUsedDesc = "most-used-desc"
    case alphabetical = "alphabetical"
    case createdDesc = "created-desc"

    public var label: String {
        switch self {
        case .updatedDesc: "Recently Active"
        case .mostUsedDesc: "Most Used"
        case .alphabetical: "Alphabetical (A-Z)"
        case .createdDesc: "Recently Created"
        }
    }
}
