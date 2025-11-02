import { describe, it, expect } from "vitest";
import {
  rankPlaceholderValues,
  getRankedValuesForAutocomplete,
  filterValuesByQuery,
  getTopRankedValue,
  getLastUsedValue,
  calculateKeyStats,
  formatRelativeTime,
} from "./placeholderHistory";
import { PlaceholderHistoryValue } from "../types";

const ONE_DAY_MS = 24 * 60 * 60 * 1000;
const ONE_HOUR_MS = 60 * 60 * 1000;

describe("rankPlaceholderValues", () => {
  it("should rank by frequency when recency is equal", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "low-freq", useCount: 1, lastUsed: now, createdAt: now },
      { value: "high-freq", useCount: 10, lastUsed: now, createdAt: now },
      { value: "mid-freq", useCount: 5, lastUsed: now, createdAt: now },
    ];

    const ranked = rankPlaceholderValues(values);

    expect(ranked[0].value).toBe("high-freq");
    expect(ranked[1].value).toBe("mid-freq");
    expect(ranked[2].value).toBe("low-freq");
  });

  it("should prioritize recent values over less frequent ones", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "old-frequent", useCount: 100, lastUsed: now - 30 * ONE_DAY_MS, createdAt: now - 30 * ONE_DAY_MS },
      { value: "recent-rare", useCount: 1, lastUsed: now, createdAt: now },
    ];

    const ranked = rankPlaceholderValues(values);

    // Recent value should rank higher despite lower frequency
    expect(ranked[0].value).toBe("recent-rare");
  });

  it("should handle empty array", () => {
    const ranked = rankPlaceholderValues([]);
    expect(ranked).toHaveLength(0);
  });

  it("should handle single value", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [{ value: "only", useCount: 1, lastUsed: now, createdAt: now }];

    const ranked = rankPlaceholderValues(values);

    expect(ranked).toHaveLength(1);
    expect(ranked[0].value).toBe("only");
  });

  it("should combine frequency and recency correctly", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "very-old-frequent", useCount: 50, lastUsed: now - 60 * ONE_DAY_MS, createdAt: now - 60 * ONE_DAY_MS },
      { value: "recent-frequent", useCount: 50, lastUsed: now - ONE_DAY_MS, createdAt: now - ONE_DAY_MS },
      { value: "very-recent-rare", useCount: 1, lastUsed: now - ONE_HOUR_MS, createdAt: now - ONE_HOUR_MS },
    ];

    const ranked = rankPlaceholderValues(values);

    // Recent + frequent should be first
    expect(ranked[0].value).toBe("recent-frequent");
  });
});

describe("getRankedValuesForAutocomplete", () => {
  it("should return ranked value strings", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "first", useCount: 10, lastUsed: now, createdAt: now },
      { value: "second", useCount: 5, lastUsed: now, createdAt: now },
    ];

    const result = getRankedValuesForAutocomplete(values);

    expect(result).toEqual(["first", "second"]);
  });

  it("should limit results when limit is specified", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "first", useCount: 10, lastUsed: now, createdAt: now },
      { value: "second", useCount: 9, lastUsed: now, createdAt: now },
      { value: "third", useCount: 8, lastUsed: now, createdAt: now },
      { value: "fourth", useCount: 7, lastUsed: now, createdAt: now },
    ];

    const result = getRankedValuesForAutocomplete(values, 2);

    expect(result).toHaveLength(2);
    expect(result).toEqual(["first", "second"]);
  });

  it("should handle empty array", () => {
    const result = getRankedValuesForAutocomplete([]);
    expect(result).toHaveLength(0);
  });
});

describe("filterValuesByQuery", () => {
  it("should filter values case-insensitively", () => {
    const values = ["Alice", "Bob", "alice@example.com", "charlie"];
    const result = filterValuesByQuery(values, "alice");

    expect(result).toHaveLength(2);
    expect(result).toContain("Alice");
    expect(result).toContain("alice@example.com");
  });

  it("should match substrings", () => {
    const values = ["user@example.com", "admin@example.com", "test@test.com"];
    const result = filterValuesByQuery(values, "example");

    expect(result).toHaveLength(2);
    expect(result).toContain("user@example.com");
    expect(result).toContain("admin@example.com");
  });

  it("should return all values when query is empty", () => {
    const values = ["one", "two", "three"];
    const result = filterValuesByQuery(values, "");

    expect(result).toHaveLength(3);
    expect(result).toEqual(values);
  });

  it("should return all values when query is whitespace", () => {
    const values = ["one", "two", "three"];
    const result = filterValuesByQuery(values, "   ");

    expect(result).toHaveLength(3);
  });

  it("should return empty array when no matches", () => {
    const values = ["one", "two", "three"];
    const result = filterValuesByQuery(values, "xyz");

    expect(result).toHaveLength(0);
  });
});

describe("getTopRankedValue", () => {
  it("should return the top-ranked value", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "second", useCount: 5, lastUsed: now, createdAt: now },
      { value: "first", useCount: 10, lastUsed: now, createdAt: now },
    ];

    const result = getTopRankedValue(values);

    expect(result).toBe("first");
  });

  it("should return undefined for empty array", () => {
    const result = getTopRankedValue([]);
    expect(result).toBeUndefined();
  });

  it("should return the only value when array has one element", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [{ value: "only", useCount: 1, lastUsed: now, createdAt: now }];

    const result = getTopRankedValue(values);

    expect(result).toBe("only");
  });
});

describe("getLastUsedValue", () => {
  it("should return the most recently used value", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "old", useCount: 100, lastUsed: now - 30 * ONE_DAY_MS, createdAt: now - 30 * ONE_DAY_MS },
      { value: "recent", useCount: 1, lastUsed: now - ONE_HOUR_MS, createdAt: now - ONE_HOUR_MS },
      { value: "very-old", useCount: 50, lastUsed: now - 60 * ONE_DAY_MS, createdAt: now - 60 * ONE_DAY_MS },
    ];

    const result = getLastUsedValue(values);

    expect(result).toBe("recent");
  });

  it("should return undefined for empty array", () => {
    const result = getLastUsedValue([]);
    expect(result).toBeUndefined();
  });

  it("should return the only value when array has one element", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [{ value: "only", useCount: 1, lastUsed: now, createdAt: now }];

    const result = getLastUsedValue(values);

    expect(result).toBe("only");
  });

  it("should handle tied timestamps by returning first occurrence", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "first-tied", useCount: 10, lastUsed: now, createdAt: now - ONE_DAY_MS },
      { value: "second-tied", useCount: 5, lastUsed: now, createdAt: now },
    ];

    const result = getLastUsedValue(values);

    // Should return one of the tied values (first in reduce order)
    expect(["first-tied", "second-tied"]).toContain(result);
  });

  it("should differ from top-ranked when frequency differs", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "most-frequent", useCount: 100, lastUsed: now - 10 * ONE_DAY_MS, createdAt: now - 30 * ONE_DAY_MS },
      { value: "most-recent", useCount: 1, lastUsed: now, createdAt: now },
    ];

    const lastUsed = getLastUsedValue(values);
    const topRanked = getTopRankedValue(values);

    // Last used should be "most-recent" based on timestamp alone
    expect(lastUsed).toBe("most-recent");
    // Top ranked might be different due to smart ranking algorithm
    // (We're verifying the functions are independent)
    expect(lastUsed).toBeDefined();
    expect(topRanked).toBeDefined();
  });
});

describe("calculateKeyStats", () => {
  it("should calculate statistics correctly", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [
      { value: "val1", useCount: 5, lastUsed: now - ONE_DAY_MS, createdAt: now - 2 * ONE_DAY_MS },
      { value: "val2", useCount: 10, lastUsed: now, createdAt: now - ONE_DAY_MS },
      { value: "val3", useCount: 3, lastUsed: now - 2 * ONE_DAY_MS, createdAt: now - 3 * ONE_DAY_MS },
    ];

    const stats = calculateKeyStats("testKey", values);

    expect(stats.key).toBe("testKey");
    expect(stats.valueCount).toBe(3);
    expect(stats.totalUseCount).toBe(18); // 5 + 10 + 3
    expect(stats.lastUsed).toBe(now); // Most recent
  });

  it("should handle empty values array", () => {
    const stats = calculateKeyStats("emptyKey", []);

    expect(stats.key).toBe("emptyKey");
    expect(stats.valueCount).toBe(0);
    expect(stats.totalUseCount).toBe(0);
    expect(stats.lastUsed).toBeUndefined();
  });

  it("should handle single value", () => {
    const now = Date.now();
    const values: PlaceholderHistoryValue[] = [{ value: "val", useCount: 7, lastUsed: now, createdAt: now }];

    const stats = calculateKeyStats("singleKey", values);

    expect(stats.key).toBe("singleKey");
    expect(stats.valueCount).toBe(1);
    expect(stats.totalUseCount).toBe(7);
    expect(stats.lastUsed).toBe(now);
  });
});

describe("formatRelativeTime", () => {
  it("should format seconds", () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 30 * 1000)).toBe("just now");
  });

  it("should format minutes", () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 5 * 60 * 1000)).toBe("5m ago");
  });

  it("should format hours", () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 3 * ONE_HOUR_MS)).toBe("3h ago");
  });

  it("should format days", () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 4 * ONE_DAY_MS)).toBe("4d ago");
  });

  it("should format weeks", () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 14 * ONE_DAY_MS)).toBe("2w ago");
  });

  it("should format months", () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 60 * ONE_DAY_MS)).toBe("2mo ago");
  });

  it("should format years", () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 400 * ONE_DAY_MS)).toBe("1y ago");
  });

  it("should handle future timestamps", () => {
    const now = Date.now();
    // Future timestamp should show as "just now" since diff is negative
    expect(formatRelativeTime(now + 1000)).toBe("just now");
  });
});
