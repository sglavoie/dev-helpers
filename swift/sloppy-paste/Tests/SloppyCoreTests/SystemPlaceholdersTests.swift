import Foundation
import Testing
@testable import SloppyCore

@Suite struct SystemPlaceholdersTests {
    /// 2024-03-15T14:30:45.000Z, a Friday.
    static let now: Int64 = 1_710_513_045_000
    static let utc = TimeZone(identifier: "UTC")!

    private func process(_ text: String, now: Int64 = now, timeZone: TimeZone = utc) -> String {
        SystemPlaceholders.process(text, now: now, timeZone: timeZone)
    }

    static let cases: [(String, String)] = [
        ("Today is {{DATE}}", "Today is 2024-03-15"),
        ("The time is {{TIME}}", "The time is 14:30"),
        ("Timestamp: {{DATETIME}}", "Timestamp: 2024-03-15 14:30"),
        ("Today is {{TODAY}}", "Today is Friday, March 15, 2024"),
        ("Now: {{NOW}}", "Now: 2024-03-15T14:30:45.000Z"),
        ("Copyright {{YEAR}}", "Copyright 2024"),
        ("Month: {{MONTH}}", "Month: March"),
        ("Day: {{DAY}}", "Day: Friday"),
        ("Date: {{DATE}}, Year: {{YEAR}}", "Date: 2024-03-15, Year: 2024"),
        ("{{DATE}} to {{DATE}}", "2024-03-15 to 2024-03-15"),
        ("{{date}} {{time}} {{now}}", "{{date}} {{time}} {{now}}"),
        ("Hello {{name}}, today is {{DATE}}", "Hello {{name}}, today is 2024-03-15"),
        ("No placeholders here", "No placeholders here"),
        ("", ""),
        ("Date: {{ DATE }}", "Date: 2024-03-15"),
        ("Year: {{  YEAR  }}", "Year: 2024"),
        ("{{DATES}} {{#if DATE}}x{{/if}} {{DATE|x}}", "{{DATES}} {{#if DATE}}x{{/if}} {{DATE|x}}"),
        ("📅 {{DATE}} 日付 {{DAY}}", "📅 2024-03-15 日付 Friday"),
        ("{{{{DATE}}}}", "{{2024-03-15}}"),
    ]

    @Test(arguments: cases)
    func replaces(text: String, expected: String) {
        #expect(process(text) == expected)
    }

    /// `{{DATE}}` is the local calendar date, not the UTC one.
    @Test func dateUsesLocalTimeZone() {
        let lateEvening: Int64 = 1_710_471_600_000  // 2024-03-15T03:00:00Z
        let losAngeles = TimeZone(identifier: "America/Los_Angeles")!
        #expect(process("{{DATE}} {{TIME}} {{DAY}}", now: lateEvening, timeZone: losAngeles) == "2024-03-14 20:00 Thursday")
        #expect(process("{{NOW}}", now: lateEvening, timeZone: losAngeles) == "2024-03-15T03:00:00.000Z")
        let tokyo = TimeZone(identifier: "Asia/Tokyo")!
        #expect(process("{{DATETIME}}", now: lateEvening, timeZone: tokyo) == "2024-03-15 12:00")
    }

    @Test func midnightIsZeroHour() {
        #expect(process("{{TIME}}", now: 1_710_460_800_000) == "00:00")  // 2024-03-15T00:00:00Z
    }

    @Test func names() {
        #expect(SystemPlaceholders.names == ["DATE", "TIME", "DATETIME", "TODAY", "NOW", "YEAR", "MONTH", "DAY"])
        #expect(SystemPlaceholders.names.allSatisfy { SystemPlaceholders.value(for: $0, now: Self.now) != nil })
        #expect(SystemPlaceholders.value(for: "date", now: Self.now) == nil)
    }
}
