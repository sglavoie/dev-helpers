import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct ConfigValueEditorTests {
    static let text = """
        // "maxAgeSeconds": 1 in a comment
        {
          "labelPrefix": "com.sglavoie.",
          /* "agents": { "com.sglavoie.forgejo-sync": { "maxAgeSeconds": 5 } } */
          "agents": {
            // Uptime Kuma already pages the phone for this one.
            "com.sglavoie.forgejo-sync": {
              "displayName": "maxAgeSeconds: 2700",
              "maxAgeSeconds": 2700,
              "notify": false,
            },
            'com.sglavoie.other': { maxAgeSeconds: 60, evidencePaths: ["a", "b"], },
          },
        }
        """

    @Test func replacesOnlyTheNumber() throws {
        let output = try ConfigValueEditor.replaceAgentMaxAge(in: Self.text, label: "com.sglavoie.forgejo-sync", value: 3600)
        #expect(output == Self.text.replacingOccurrences(of: "\"maxAgeSeconds\": 2700,", with: "\"maxAgeSeconds\": 3600,"))
    }

    @Test func handlesJSON5KeysAndQuotes() throws {
        let output = try ConfigValueEditor.replaceAgentMaxAge(in: Self.text, label: "com.sglavoie.other", value: 120)
        #expect(output.contains("{ maxAgeSeconds: 120, evidencePaths"))
    }

    @Test func missingKeyThrows() {
        #expect(throws: ConfigEditError.keyNotFound("agents.com.sglavoie.nope.maxAgeSeconds")) {
            try ConfigValueEditor.replaceAgentMaxAge(in: Self.text, label: "com.sglavoie.nope", value: 1)
        }
    }

    @Test func worksOnTheShippedExample() throws {
        let output = try ConfigValueEditor.replaceAgentMaxAge(in: HeartbeatConfig.exampleText, label: "com.sglavoie.forgejo-sync", value: 3600)
        #expect(output.components(separatedBy: "\n").count == HeartbeatConfig.exampleText.components(separatedBy: "\n").count)
        #expect(output.contains("\"maxAgeSeconds\": 3600"))
    }
}
