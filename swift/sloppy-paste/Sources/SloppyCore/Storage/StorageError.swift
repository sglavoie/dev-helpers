import Foundation

public enum StorageError: Error, Equatable, LocalizedError {
    case snippetNotFound
    case cannotMergeTagWithItself
    case emptyPlaceholderValue
    case placeholderKeyNotFound
    case placeholderValueNotFound
    case duplicatePlaceholderValue

    public var errorDescription: String? {
        switch self {
        case .snippetNotFound: "Snippet not found"
        case .cannotMergeTagWithItself: "Cannot merge a tag with itself"
        case .emptyPlaceholderValue: "New value cannot be empty"
        case .placeholderKeyNotFound: "Placeholder key not found"
        case .placeholderValueNotFound: "Value not found"
        case .duplicatePlaceholderValue: "A value with this name already exists"
        }
    }
}
