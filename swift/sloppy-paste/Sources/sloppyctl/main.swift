import Foundation
import SloppyCore

let usage = """
    sloppyctl \(SloppyCore.version)

    Usage:
      sloppyctl import <export.json> [--merge | --replace] [--data <data.json>]
      sloppyctl stats [--data <data.json>] [--json]
      sloppyctl render (--text <content> | --file <path|-> | --id <snippet-id> [--data <data.json>]) [key=value ...]
      sloppyctl search [<query> ...] [--tag <tag>] [--favorites] [--archived] [--needs-attention]
                       [--sort updated-desc|most-used-desc|alphabetical|created-desc] [--data <data.json>] [--json]
      sloppyctl --version

    --data defaults to \(StorageFile.defaultURL.path).
    import merges by default; --replace writes data.json.bak first.
    render resolves system placeholders, conditionals and values; keys not
    given use their default, and guards use their `+` default.
    search takes the picker syntax (tag:, not:tag:, ctx:, not:ctx:, is:, not:,
    "exact phrase", fuzzy words) and prints pinned rows first, then the rest in
    sort order, as tab-separated id, title and tags.
    """

struct CommandError: Error, CustomStringConvertible {
    var description: String
}

/// Counts reported by `stats` and after `import`, keyed to match simple `jq` queries.
struct Stats: Codable {
    var snippets: Int
    var archived: Int
    var pinned: Int
    var favorites: Int
    var tagged: Int
    var untagged: Int
    var distinctTags: Int
    var totalUses: Int
    var stale: Int
    var contexts: Int
    var placeholderKeys: Int
    var placeholderValues: Int

    init(_ data: StorageData, now: Int64) {
        let snippets = data.snippets
        self.snippets = snippets.count
        archived = snippets.filter(\.isArchived).count
        pinned = snippets.filter(\.isPinned).count
        favorites = snippets.filter(\.isFavorite).count
        tagged = snippets.filter { !$0.tags.isEmpty }.count
        untagged = snippets.count - tagged
        distinctTags = Set(snippets.flatMap(\.tags)).count
        totalUses = snippets.reduce(0) { $0 + $1.useCount }
        stale = snippets.filter { Staleness.analyze($0, now: now).isStale }.count
        contexts = TitleContext.allContexts(snippets).count
        placeholderKeys = data.placeholderHistory.count
        placeholderValues = data.placeholderHistory.values.reduce(0) { $0 + $1.count }
    }

    var lines: [String] {
        [
            ("snippets", snippets), ("archived", archived), ("pinned", pinned), ("favorites", favorites),
            ("tagged", tagged), ("untagged", untagged), ("distinct tags", distinctTags),
            ("total uses", totalUses), ("stale", stale), ("contexts", contexts),
            ("placeholder keys", placeholderKeys), ("placeholder values", placeholderValues),
        ].map { "\($0.0.padding(toLength: 20, withPad: " ", startingAt: 0))\($0.1)" }
    }
}

struct Options {
    var positional: [String] = []
    var dataURL = StorageFile.defaultURL
    var mode = ImportMode.merge
    var json = false
    var text: String?
    var file: String?
    var snippetID: String?
    var filterOptions = SnippetFilterOptions()
    var sort = SortOption.updatedDesc

    init(_ arguments: ArraySlice<String>) throws {
        var iterator = arguments.makeIterator()
        while let argument = iterator.next() {
            switch argument {
            case "--data":
                guard let path = iterator.next() else { throw CommandError(description: "--data needs a path") }
                dataURL = URL(filePath: (path as NSString).expandingTildeInPath)
            case "--merge": mode = .merge
            case "--replace": mode = .replace
            case "--json": json = true
            case "--text", "--file", "--id":
                guard let value = iterator.next() else { throw CommandError(description: "\(argument) needs a value") }
                if argument == "--text" { text = value } else if argument == "--file" { file = value } else { snippetID = value }
            case "--tag":
                guard let tag = iterator.next() else { throw CommandError(description: "--tag needs a value") }
                filterOptions.selectedTag = tag
            case "--favorites": filterOptions.showOnlyFavorites = true
            case "--archived": filterOptions.showArchivedSnippets = true
            case "--needs-attention": filterOptions.showNeedsAttention = true
            case "--sort":
                guard let value = iterator.next(), let sort = SortOption(rawValue: value) else {
                    let choices = SortOption.allCases.map(\.rawValue).joined(separator: ", ")
                    throw CommandError(description: "--sort needs one of \(choices)")
                }
                self.sort = sort
            case let flag where flag.hasPrefix("--"):
                throw CommandError(description: "unknown option \(flag)")
            default: positional.append(argument)
            }
        }
    }
}

func nowMs() -> Int64 { Date().epochMilliseconds }

func load(_ file: StorageFile) throws -> StorageData {
    let result = try file.load(now: nowMs())
    if let quarantined = result.quarantinedURL {
        FileHandle.standardError.write(Data("warning: data file was corrupt; moved to \(quarantined.path)\n".utf8))
    }
    return result.data
}

func printStats(_ stats: Stats, json: Bool) throws {
    if json {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        print(String(decoding: try encoder.encode(stats), as: UTF8.self))
    } else {
        stats.lines.forEach { print($0) }
    }
}

/// Parses `key=value` arguments and fills keys the caller left out from the
/// snippet's defaults, the way the placeholder form pre-fills its fields.
func renderValues(_ assignments: [String], placeholders: [Placeholder]) throws -> [String: String] {
    var values: [String: String] = [:]
    for assignment in assignments {
        guard let equals = assignment.firstIndex(of: "=") else {
            throw CommandError(description: "expected key=value, got \(assignment)")
        }
        values[String(assignment[..<equals])] = String(assignment[assignment.index(after: equals)...])
    }
    for placeholder in placeholders where values[placeholder.key] == nil {
        if placeholder.isGuardOnly {
            values[placeholder.key] = placeholder.defaultOn ? "true" : ""
        } else if let defaultValue = placeholder.defaultValue {
            values[placeholder.key] = defaultValue
        } else if placeholder.isRequired {
            FileHandle.standardError.write(Data("warning: no value for required placeholder \(placeholder.key)\n".utf8))
        }
    }
    return values
}

func renderContent(_ options: Options, file: StorageFile) throws -> String {
    switch (options.text, options.file, options.snippetID) {
    case (let text?, nil, nil): return text
    case (nil, "-", nil): return String(decoding: FileHandle.standardInput.readDataToEndOfFile(), as: UTF8.self)
    case (nil, let path?, nil): return try String(contentsOfFile: (path as NSString).expandingTildeInPath, encoding: .utf8)
    case (nil, nil, let id?):
        guard let snippet = try load(file).snippets.first(where: { $0.id == id }) else {
            throw CommandError(description: "no snippet with id \(id)")
        }
        return snippet.content
    default: throw CommandError(description: "render needs exactly one of --text, --file or --id")
    }
}

func run(_ arguments: [String]) throws {
    guard let command = arguments.dropFirst().first else {
        print(usage)
        return
    }
    let options = try Options(arguments.dropFirst(2))
    let file = StorageFile(url: options.dataURL)

    switch command {
    case "import":
        guard options.positional.count == 1 else { throw CommandError(description: "import needs one export file") }
        let exportURL = URL(filePath: (options.positional[0] as NSString).expandingTildeInPath)
        let payload = try ImportExport.decodeImport(Data(contentsOf: exportURL))
        let current = try load(file)
        let next = try file.importPayload(payload, into: current, mode: options.mode)
        let added = next.snippets.count - (options.mode == .merge ? current.snippets.count : 0)
        if !options.json {
            print("Imported \(payload.snippets.count) snippets (\(options.mode.rawValue), \(added) added) into \(file.url.path)")
        }
        try printStats(Stats(next, now: nowMs()), json: options.json)
    case "stats":
        guard options.positional.isEmpty else { throw CommandError(description: "stats takes no arguments") }
        try printStats(Stats(try load(file), now: nowMs()), json: options.json)
    case "render":
        let content = try renderContent(options, file: file)
        let now = nowMs()
        let placeholders = PlaceholderSyntaxParser.extractPlaceholders(SystemPlaceholders.process(content, now: now))
        let values = try renderValues(options.positional, placeholders: placeholders)
        print(PlaceholderRenderer.render(content, values: values, now: now), terminator: "")
    case "search":
        let list = SnippetListPipeline.build(
            try load(file).snippets, query: options.positional.joined(separator: " "),
            options: options.filterOptions, sort: options.sort, showRecentSection: false, now: nowMs())
        if options.json {
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
            print(String(decoding: try encoder.encode(list.rows), as: UTF8.self))
        } else {
            for snippet in list.rows {
                print([snippet.id, snippet.title, snippet.tags.joined(separator: ",")].joined(separator: "\t"))
            }
        }
    case "--version", "version":
        print("sloppyctl \(SloppyCore.version)")
    case "-h", "--help", "help":
        print(usage)
    default:
        throw CommandError(description: "unknown command \(command)\n\n\(usage)")
    }
}

do {
    try run(CommandLine.arguments)
} catch {
    FileHandle.standardError.write(Data("error: \(error)\n".utf8))
    exit(1)
}
