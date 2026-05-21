import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { processSystemPlaceholders, getSystemPlaceholderNames } from "./placeholders";

describe("processSystemPlaceholders", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2024-03-15T14:30:45.000Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should replace {{DATE}} with ISO date format", () => {
    const text = "Today is {{DATE}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("Today is 2024-03-15");
  });

  it("should replace {{TIME}} with 24-hour time format", () => {
    const text = "The time is {{TIME}}";
    const result = processSystemPlaceholders(text);
    expect(result).toMatch(/^\The time is \d{2}:\d{2}$/);
  });

  it("should replace {{DATETIME}} with date and time", () => {
    const text = "Timestamp: {{DATETIME}}";
    const result = processSystemPlaceholders(text);
    expect(result).toMatch(/^Timestamp: 2024-03-15 \d{2}:\d{2}$/);
  });

  it("should replace {{TODAY}} with human-readable date", () => {
    const text = "Today is {{TODAY}}";
    const result = processSystemPlaceholders(text);
    expect(result).toContain("Friday");
    expect(result).toContain("March");
    expect(result).toContain("15");
    expect(result).toContain("2024");
  });

  it("should replace {{NOW}} with ISO timestamp", () => {
    const text = "Now: {{NOW}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("Now: 2024-03-15T14:30:45.000Z");
  });

  it("should replace {{YEAR}} with current year", () => {
    const text = "Copyright {{YEAR}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("Copyright 2024");
  });

  it("should replace {{MONTH}} with month name", () => {
    const text = "Month: {{MONTH}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("Month: March");
  });

  it("should replace {{DAY}} with day name", () => {
    const text = "Day: {{DAY}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("Day: Friday");
  });

  it("should replace multiple system placeholders", () => {
    const text = "Date: {{DATE}}, Year: {{YEAR}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("Date: 2024-03-15, Year: 2024");
  });

  it("should replace multiple occurrences of the same placeholder", () => {
    const text = "{{DATE}} to {{DATE}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("2024-03-15 to 2024-03-15");
  });

  it("should NOT replace lowercase system placeholder names", () => {
    const text = "{{date}} {{time}} {{now}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("{{date}} {{time}} {{now}}");
  });

  it("should NOT replace user placeholders", () => {
    const text = "Hello {{name}}, today is {{DATE}}";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("Hello {{name}}, today is 2024-03-15");
  });

  it("should handle text with no placeholders", () => {
    const text = "No placeholders here";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("No placeholders here");
  });

  it("should handle empty string", () => {
    const text = "";
    const result = processSystemPlaceholders(text);
    expect(result).toBe("");
  });

  it("should resolve system placeholders with whitespace around the name", () => {
    // Regression: extractPlaceholders treats `{{ DATE }}` as the key `DATE`,
    // so processSystemPlaceholders must also tolerate that whitespace or the
    // placeholder would survive into the prompt for user input.
    expect(processSystemPlaceholders("Date: {{ DATE }}")).toBe("Date: 2024-03-15");
    expect(processSystemPlaceholders("Year: {{  YEAR  }}")).toBe("Year: 2024");
  });

  it("should use the local-timezone calendar date for {{DATE}}", () => {
    // Regression: {{DATE}} previously used toISOString() (UTC), so running the
    // snippet at 23:00 local in a UTC-negative timezone would emit tomorrow's
    // date. The system time below corresponds to 2024-03-15 in every local
    // timezone (08:00 UTC is well inside the day everywhere on Earth).
    vi.setSystemTime(new Date("2024-03-15T08:00:00.000Z"));
    const result = processSystemPlaceholders("{{DATE}}");
    expect(result).toMatch(/^2024-03-(14|15|16)$/); // Local date, not blindly UTC.
    // The local date can only be 2024-03-15 (every TZ rolls over at midnight
    // local, and 08:00 UTC is at least 19:00 on the previous day at UTC-13 and
    // at most 22:00 on the next day at UTC+14 — neither boundary is crossed
    // since 08:00 UTC is within the same calendar day in every IANA timezone).
    const now = new Date("2024-03-15T08:00:00.000Z");
    const expectedYear = now.getFullYear();
    const expectedMonth = String(now.getMonth() + 1).padStart(2, "0");
    const expectedDay = String(now.getDate()).padStart(2, "0");
    expect(result).toBe(`${expectedYear}-${expectedMonth}-${expectedDay}`);
  });
});

describe("getSystemPlaceholderNames", () => {
  it("should return all system placeholder names", () => {
    const names = getSystemPlaceholderNames();
    expect(names).toContain("DATE");
    expect(names).toContain("TIME");
    expect(names).toContain("DATETIME");
    expect(names).toContain("TODAY");
    expect(names).toContain("NOW");
    expect(names).toContain("YEAR");
    expect(names).toContain("MONTH");
    expect(names).toContain("DAY");
  });

  it("should return at least 8 system placeholders", () => {
    const names = getSystemPlaceholderNames();
    expect(names.length).toBeGreaterThanOrEqual(8);
  });
});
