import Testing
@testable import SloppyCore

@Suite struct ValidationTests {
    @Test func validTitles() {
        #expect(Validation.validateTitle("Valid Title") == .valid)
        #expect(Validation.validateTitle("  Trimmed  ") == .valid)
    }

    @Test(arguments: ["", "   "])
    func emptyTitle(title: String) {
        #expect(Validation.validateTitle(title) == .invalid("Title is required"))
    }

    @Test func longTitle() {
        let result = Validation.validateTitle(String(repeating: "a", count: ValidationLimits.titleMaxLength + 1))
        #expect(!result.isValid)
        #expect(result.error?.contains("200 characters or less") == true)
    }

    @Test func titleLengthCountsCharacters() {
        #expect(Validation.validateTitle(String(repeating: "👩‍💻", count: ValidationLimits.titleMaxLength)) == .valid)
    }

    @Test func validContent() {
        #expect(Validation.validateContent("Valid content") == .valid)
        #expect(Validation.validateContent("  Trimmed content  ") == .valid)
    }

    @Test(arguments: ["", "   "])
    func emptyContent(content: String) {
        #expect(Validation.validateContent(content) == .invalid("Content is required"))
    }

    @Test func largeContent() {
        let result = Validation.validateContent(String(repeating: "a", count: ValidationLimits.contentMaxLength + 1))
        #expect(!result.isValid)
        #expect(result.error == "Content is too large (100.0KB). Maximum size is 100KB.")
    }

    @Test func reportsSyntaxDiagnostic() {
        let result = Validation.validateContent("{{first[A||B]}}") { _ in "Malformed {{first[A||B]}}" }
        #expect(result == .invalid("Malformed {{first[A||B]}}"))
    }

    @Test func acceptsValidAuthoredChoices() {
        #expect(Validation.validateContent("Use {{tone[Formal|Casual]|Casual}} here") == .valid)
    }

    @Test func rejectsFirstMalformedOrConflictingChoice() {
        let malformed = Validation.validateContent("{{first[A||B]}} {{second[X|X]}}")
        #expect(!malformed.isValid)
        #expect(malformed.error?.contains("{{first[A||B]}}") == true)

        let conflicting = Validation.validateContent("{{tone[A|B]}} {{tone[A|C]}}")
        #expect(!conflicting.isValid)
        #expect(conflicting.error?.contains("Conflicting authored choices") == true)
        #expect(conflicting.error?.contains("{{tone[A|B]}}") == true)
    }

    @Test func realParserSizeCheckedBeforeSyntax() {
        let large = String(repeating: "x", count: ValidationLimits.contentMaxLength + 1) + " {{tone[A||B]}}"
        #expect(Validation.validateContent(large).error?.contains("too large") == true)
    }

    @Test func sizeCheckedBeforeSyntax() {
        let large = String(repeating: "x", count: ValidationLimits.contentMaxLength + 1) + " {{tone[A||B]}}"
        let result = Validation.validateContent(large) { _ in "syntax" }
        #expect(result.error?.contains("too large") == true)
    }

    @Test(arguments: [
        ("valid-tag", ValidationResult.valid),
        ("Tag_123", ValidationResult(isValid: true, normalizedValue: "tag_123")),
        ("  trimmed  ", .valid),
        ("work/projects", .valid),
        ("work/projects/client-a", .valid),
        ("dev/backend/api", .valid),
        ("valid_tag", .valid),
        ("invalid tag", ValidationResult(isValid: true, normalizedValue: "invalid-tag")),
        ("work project", ValidationResult(isValid: true, normalizedValue: "work-project")),
        ("a/b/c/d/e", .valid),
        ("", .invalid("Tag name is required")),
    ])
    func validateTag(tag: String, expected: ValidationResult) {
        #expect(Validation.validateTag(tag) == expected)
    }

    @Test(arguments: [
        "invalid@tag", "invalid#tag", "invalid.tag", "café",
        "/invalid", "invalid/", "/invalid/tag", "invalid/tag/",
        "invalid//tag", "work///projects",
    ])
    func rejectsTag(tag: String) {
        #expect(!Validation.validateTag(tag).isValid)
    }

    @Test func longTag() {
        let result = Validation.validateTag(String(repeating: "a", count: ValidationLimits.tagMaxLength + 1))
        #expect(result.error?.contains("50 characters or less") == true)
    }

    @Test func deepTag() {
        #expect(Validation.validateTag("a/b/c/d/e/f").error?.contains("hierarchy too deep") == true)
    }

    @Test func characterInfo() {
        let short = Validation.characterInfo("hello", maxLength: 100)
        #expect(short.count == 5)
        #expect(short.remaining == 95)
        #expect(short.info == "5 / 100")
        #expect(Validation.characterInfo(String(repeating: "a", count: 60), maxLength: 100).info.contains("remaining"))
        #expect(Validation.characterInfo(String(repeating: "a", count: 95), maxLength: 100).info.contains("⚠️"))

        let over = Validation.characterInfo(String(repeating: "a", count: 110), maxLength: 100).info
        #expect(over.contains("❌"))
        #expect(over.contains("Exceeds limit"))

        let atLimit = Validation.characterInfo(String(repeating: "a", count: 100), maxLength: 100)
        #expect(atLimit.remaining == 0)
        #expect(atLimit.info.contains("⚠️"))
        #expect(!atLimit.info.contains("Exceeds limit"))
    }
}
