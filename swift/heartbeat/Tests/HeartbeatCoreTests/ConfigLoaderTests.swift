import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct ConfigLoaderTests {
    let directory: URL
    var loader: ConfigLoader

    init() {
        directory = FileManager.default.temporaryDirectory
            .appending(path: "HeartbeatCoreTests-\(UUID().uuidString)", directoryHint: .isDirectory)
        loader = ConfigLoader(url: directory.appending(path: "config.json"), home: "/Users/test")
    }

    func write(_ json: String) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try Data(json.utf8).write(to: loader.url)
    }

    static let planExample = #"""
    { "version": 1, "labelPrefix": "com.sglavoie.", "pollSeconds": 60, "notifications": true,
      "piHost": "pi.tailb5cfdf.ts.net", "piStatusSeconds": 300,
      "openLogCommand": ["open", "-a", "Ghostty", "--args", "-e", "nvim", "{path}"],
      "agents": {
        "com.sglavoie.forgejo-sync": { "displayName": "Forgejo sync", "maxAgeSeconds": 2700, "notify": false },
        "com.sglavoie.pi-backup-fetch": { "notify": false,
          "receipt": { "path": "~/Library/Application Support/pi-backup-fetch/last-run.json",
                       "reportedKey": "reported", "statusKey": "status", "okValues": ["up"] } },
        "com.sglavoie.brainnotes-vault-guard": { "health": {
          "command": ["/Users/sglavoie/.local/bin/check-brainnotes-vault-health.sh"],
          "warningExitCodes": [2] } } } }
    """#

    @Test func defaultLocation() {
        #expect(ConfigLoader.defaultURL.path.hasSuffix("/.config/heartbeat/config.json"))
    }

    @Test mutating func missingFileGivesSeededDefaults() {
        let result = loader.load()
        #expect(result.source == .defaults)
        #expect(result.error == nil)
        #expect(result.warnings.isEmpty)
        let config = result.config
        #expect(config.pollSeconds == 60)
        #expect(config.labelPrefix == "com.sglavoie.")
        #expect(config.piHost == "pi.tailb5cfdf.ts.net")
        #expect(config.piStatusSeconds == 300)
        #expect(config.notifications)
        #expect(!config.notifies("com.sglavoie.forgejo-sync"))
        #expect(!config.notifies("com.sglavoie.pi-backup-fetch"))
        #expect(config.notifies("com.sglavoie.sync-legacy"))
        #expect(config.agent("com.sglavoie.pi-backup-fetch").receipt?.path
                == "/Users/test/Library/Application Support/pi-backup-fetch/last-run.json")
        let health = config.agent("com.sglavoie.brainnotes-vault-guard").health
        #expect(health?.command == ["/Users/test/.local/bin/check-brainnotes-vault-health.sh"])
        #expect(health?.warningExitCodes == [2])
        #expect(health?.intervalSeconds == 300)
        #expect(health?.timeoutSeconds == 30)
        #expect(health?.staleAfter == 900)
    }

    @Test mutating func planExampleParsesWithoutWarnings() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try write(Self.planExample)
        let result = loader.load()
        #expect(result.source == .file)
        #expect(result.error == nil)
        #expect(result.warnings.isEmpty)
        #expect(result.config.openLogCommand?.last == "{path}")
        let forgejo = result.config.agent("com.sglavoie.forgejo-sync")
        #expect(forgejo.displayName == "Forgejo sync")
        #expect(forgejo.maxAgeSeconds == 2700)
        #expect(forgejo.notify == false)
        #expect(result.config.agent("com.sglavoie.pi-backup-fetch").receipt?.okValues == ["up"])
    }

    @Test mutating func emptyObjectUsesDefaultsIncludingSeeds() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try write("{}")
        let result = loader.load()
        #expect(result.source == .file)
        #expect(result.config == HeartbeatConfig.defaults.expandingTilde(home: "/Users/test"))
    }

    @Test mutating func agentsKeyReplacesSeedsWholesale() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try write(#"{"agents": {"com.sglavoie.sync-legacy": {"hidden": true, "ignoreExitCodes": [3], "graceSeconds": 60}}}"#)
        let config = loader.load().config
        #expect(config.agents.keys.sorted() == ["com.sglavoie.sync-legacy"])
        #expect(config.notifies("com.sglavoie.forgejo-sync"))
        let options = config.agent("com.sglavoie.sync-legacy").healthOptions()
        #expect(options.hidden)
        #expect(options.ignoreExitCodes == [3])
        #expect(options.graceSeconds == 60)
    }

    @Test mutating func tildeIsExpandedInPathsOnly() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try write(#"""
        {"agents": {"a": {"evidencePaths": ["~/x.log", "/abs/y", "~"], "displayName": "~/not a path",
          "health": {"command": ["~/bin/check", "~/arg"]}, "receipt": {"path": "~/r.json"}}}}
        """#)
        let agent = loader.load().config.agent("a")
        #expect(agent.evidencePaths == ["/Users/test/x.log", "/abs/y", "/Users/test"])
        #expect(agent.displayName == "~/not a path")
        #expect(agent.health?.command == ["/Users/test/bin/check", "~/arg"])
        #expect(agent.receipt?.path == "/Users/test/r.json")
        #expect(agent.receipt?.reportedKey == "reported")
        #expect(agent.receipt?.statusKey == "status")
    }

    @Test func unknownKeysAreWarningsWithPaths() throws {
        let json = #"""
        {"polSeconds": 30, "agents": {"a": {"notfy": false, "health": {"command": ["x"], "timeout": 5},
          "receipt": {"path": "p", "okValue": "up"}}}}
        """#
        let (config, warnings) = try ConfigLoader.parse(Data(json.utf8))
        #expect(warnings == ["unknown key polSeconds", "unknown key agents.a.notfy", "unknown key agents.a.health.timeout",
                             "unknown key agents.a.receipt.okValue"])
        #expect(config.pollSeconds == 60)
    }

    @Test func outOfRangeValuesAreClampedWithWarnings() throws {
        let json = #"""
        {"version": 2, "pollSeconds": 5, "piStatusSeconds": 10, "agents": {
          "a": {"maxAgeSeconds": 0, "graceSeconds": -1, "health": {"command": ["x"], "intervalSeconds": 1, "timeoutSeconds": 0}},
          "b": {"health": {"command": []}}}}
        """#
        let (config, warnings) = try ConfigLoader.parse(Data(json.utf8))
        #expect(config.pollSeconds == 15)
        #expect(config.piStatusSeconds == 15)
        #expect(config.agent("a").maxAgeSeconds == nil)
        #expect(config.agent("a").graceSeconds == nil)
        #expect(config.agent("a").health?.intervalSeconds == 15)
        #expect(config.agent("a").health?.timeoutSeconds == 30)
        #expect(config.agent("b").health == nil)
        #expect(warnings.count == 8)
        #expect(warnings.first == "version 2 is not 1; reading it as version 1")
        #expect(warnings.contains("agents.b.health.command is empty; health check ignored"))
    }

    @Test(arguments: [
        ("{", "invalid JSON"),
        ("[1, 2]", "the top level must be a JSON object"),
        (#"{"pollSeconds": "fast"}"#, "pollSeconds:"),
        (#"{"agents": {"a": {"health": {"timeoutSeconds": 5}}}}"#, "agents.a.health.command is required"),
        (#"{"agents": {"a": {"receipt": {}}}}"#, "agents.a.receipt.path is required"),
        (#"{"agents": {"a": {"ignoreExitCodes": [1.5]}}}"#, "Number 1.5 is not representable"),
    ])
    func invalidFilesThrow(_ json: String, _ message: String) {
        #expect {
            try ConfigLoader.parse(Data(json.utf8))
        } throws: { error in
            "\(error)".contains(message)
        }
    }

    @Test mutating func invalidEditKeepsLastGoodConfig() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try write(#"{"pollSeconds": 120}"#)
        #expect(loader.load().config.pollSeconds == 120)

        try write(#"{"pollSeconds": 120,"#)
        let broken = loader.load()
        #expect(broken.source == .lastGood)
        #expect(broken.config.pollSeconds == 120)
        #expect(broken.error?.hasPrefix("config.json: invalid JSON") == true)

        try write(#"{"pollSeconds": 90}"#)
        let fixed = loader.load()
        #expect(fixed.source == .file)
        #expect(fixed.error == nil)
        #expect(fixed.config.pollSeconds == 90)
    }

    @Test mutating func invalidFileOnFirstLoadFallsBackToDefaults() throws {
        defer { try? FileManager.default.removeItem(at: directory) }
        try write("not json")
        let result = loader.load()
        #expect(result.source == .lastGood)
        #expect(result.error != nil)
        #expect(result.config == HeartbeatConfig.defaults.expandingTilde(home: "/Users/test"))
    }

    @Test func tildeExpansion() {
        #expect(ConfigLoader.expandTilde("~", home: "/h") == "/h")
        #expect(ConfigLoader.expandTilde("~/a/b", home: "/h") == "/h/a/b")
        #expect(ConfigLoader.expandTilde("~other/a", home: "/h") == "~other/a")
        #expect(ConfigLoader.expandTilde("/a/~/b", home: "/h") == "/a/~/b")
    }

    @Test func globalSwitchOverridesAgentNotify() {
        var config = HeartbeatConfig(agents: ["a": AgentConfig(notify: true)])
        #expect(config.notifies("a"))
        config.notifications = false
        #expect(!config.notifies("a"))
        #expect(!config.notifies("b"))
    }
}
