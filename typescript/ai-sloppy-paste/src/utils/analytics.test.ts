import { describe, it, expect } from "vitest";
import {
  computeSnippetAnalytics,
  computeAnalyticsSummary,
  getTopSnippets,
  getUnusedTags,
  computeCleanupSuggestions,
  findSimilarSnippets,
} from "./analytics";
import { Snippet, TimeRange } from "../types";

function createSnippet(overrides: Partial<Snippet> = {}): Snippet {
  return {
    id: "test-id",
    title: "Test Snippet",
    content: "Test content",
    tags: [],
    createdAt: Date.now() - 60 * 24 * 60 * 60 * 1000, // 60 days ago
    updatedAt: Date.now(),
    lastUsedAt: Date.now() - 24 * 60 * 60 * 1000, // 1 day ago
    useCount: 5,
    isFavorite: false,
    isArchived: false,
    isPinned: false,
    ...overrides,
  };
}

describe("analytics", () => {
  describe("computeSnippetAnalytics", () => {
    it("should compute analytics for a recently used snippet", () => {
      const snippet = createSnippet({
        lastUsedAt: Date.now() - 2 * 24 * 60 * 60 * 1000, // 2 days ago
        useCount: 10,
      });

      const analytics = computeSnippetAnalytics(snippet);

      expect(analytics.snippet).toBe(snippet);
      expect(analytics.daysUnused).toBe(2);
      expect(analytics.isStale).toBe(false);
      expect(analytics.stalenessReason).toBeUndefined();
    });

    it("should mark snippet as stale when not used for 90+ days", () => {
      const snippet = createSnippet({
        lastUsedAt: Date.now() - 100 * 24 * 60 * 60 * 1000, // 100 days ago
        useCount: 5,
      });

      const analytics = computeSnippetAnalytics(snippet);

      expect(analytics.isStale).toBe(true);
      expect(analytics.daysUnused).toBe(100);
      expect(analytics.stalenessReason).toContain("Not used in 100 days");
    });

    it("should mark never-used snippet as stale when created 30+ days ago", () => {
      const snippet = createSnippet({
        createdAt: Date.now() - 45 * 24 * 60 * 60 * 1000, // 45 days ago
        lastUsedAt: undefined,
        useCount: 0,
      });

      const analytics = computeSnippetAnalytics(snippet);

      expect(analytics.isStale).toBe(true);
      expect(analytics.daysUnused).toBeUndefined();
      expect(analytics.stalenessReason).toContain("Never used");
    });

    it("should not mark never-used snippet as stale when created less than 30 days ago", () => {
      const snippet = createSnippet({
        createdAt: Date.now() - 15 * 24 * 60 * 60 * 1000, // 15 days ago
        lastUsedAt: undefined,
        useCount: 0,
      });

      const analytics = computeSnippetAnalytics(snippet);

      expect(analytics.isStale).toBe(false);
    });
  });

  describe("computeAnalyticsSummary", () => {
    it("should compute summary for empty data", () => {
      const summary = computeAnalyticsSummary([], []);

      expect(summary.totalSnippets).toBe(0);
      expect(summary.activeSnippets).toBe(0);
      expect(summary.staleSnippets).toBe(0);
      expect(summary.archivedSnippets).toBe(0);
      expect(summary.totalUsageCount).toBe(0);
      expect(summary.averageUsagePerSnippet).toBe(0);
      expect(summary.totalTags).toBe(0);
      expect(summary.unusedTags).toBe(0);
    });

    it("should compute summary correctly", () => {
      const snippets = [
        createSnippet({
          id: "1",
          lastUsedAt: Date.now() - 5 * 24 * 60 * 60 * 1000, // Active (5 days ago)
          useCount: 10,
          tags: ["work"],
        }),
        createSnippet({
          id: "2",
          lastUsedAt: Date.now() - 100 * 24 * 60 * 60 * 1000, // Stale (100 days ago)
          useCount: 5,
          tags: ["personal"],
        }),
        createSnippet({
          id: "3",
          isArchived: true,
          useCount: 3,
          tags: [],
        }),
      ];

      const tags = ["work", "personal", "unused-tag"];
      const summary = computeAnalyticsSummary(snippets, tags);

      expect(summary.totalSnippets).toBe(3);
      expect(summary.activeSnippets).toBe(1);
      expect(summary.staleSnippets).toBe(1);
      expect(summary.archivedSnippets).toBe(1);
      expect(summary.totalUsageCount).toBe(18);
      expect(summary.averageUsagePerSnippet).toBe(9); // (10 + 5) / 2 non-archived
      expect(summary.totalTags).toBe(3);
      expect(summary.unusedTags).toBe(1); // unused-tag
    });

    it("should count unused tags correctly", () => {
      const snippets = [createSnippet({ id: "1", tags: ["used-tag"] })];

      const tags = ["used-tag", "unused-1", "unused-2"];
      const summary = computeAnalyticsSummary(snippets, tags);

      expect(summary.unusedTags).toBe(2);
    });
  });

  describe("getTopSnippets", () => {
    const now = Date.now();
    const snippets = [
      createSnippet({
        id: "1",
        title: "High usage",
        useCount: 100,
        lastUsedAt: now - 2 * 24 * 60 * 60 * 1000, // 2 days ago
      }),
      createSnippet({
        id: "2",
        title: "Medium usage",
        useCount: 50,
        lastUsedAt: now - 10 * 24 * 60 * 60 * 1000, // 10 days ago
      }),
      createSnippet({
        id: "3",
        title: "Low usage",
        useCount: 10,
        lastUsedAt: now - 60 * 24 * 60 * 60 * 1000, // 60 days ago
      }),
      createSnippet({
        id: "4",
        title: "Archived",
        useCount: 200,
        isArchived: true,
      }),
      createSnippet({
        id: "5",
        title: "Never used",
        useCount: 0,
        lastUsedAt: undefined,
      }),
    ];

    it("should return top snippets for all time, sorted by usage", () => {
      const top = getTopSnippets(snippets, TimeRange.AllTime, 10);

      expect(top.map((s) => s.id)).toEqual(["1", "2", "3"]);
      expect(top).toHaveLength(3); // Excludes archived and never-used
    });

    it("should filter by time range - this week", () => {
      const top = getTopSnippets(snippets, TimeRange.ThisWeek, 10);

      expect(top.map((s) => s.id)).toEqual(["1"]); // Only snippet used 2 days ago
    });

    it("should filter by time range - this month", () => {
      const top = getTopSnippets(snippets, TimeRange.ThisMonth, 10);

      expect(top.map((s) => s.id)).toEqual(["1", "2"]); // Used within 30 days
    });

    it("should filter by time range - last 3 months", () => {
      const top = getTopSnippets(snippets, TimeRange.Last3Months, 10);

      expect(top.map((s) => s.id)).toEqual(["1", "2", "3"]); // All used within 90 days
    });

    it("should exclude archived snippets", () => {
      const top = getTopSnippets(snippets, TimeRange.AllTime, 10);

      expect(top.find((s) => s.id === "4")).toBeUndefined();
    });

    it("should respect limit parameter", () => {
      const top = getTopSnippets(snippets, TimeRange.AllTime, 2);

      expect(top).toHaveLength(2);
      expect(top.map((s) => s.id)).toEqual(["1", "2"]);
    });
  });

  describe("getUnusedTags", () => {
    it("should return tags with no snippets", () => {
      const snippets = [
        createSnippet({ id: "1", tags: ["used-1", "used-2"] }),
        createSnippet({ id: "2", tags: ["used-1"] }),
      ];

      const tags = ["used-1", "used-2", "unused-1", "unused-2"];
      const unused = getUnusedTags(snippets, tags);

      expect(unused).toEqual(["unused-1", "unused-2"]);
    });

    it("should not count archived snippet tags as used", () => {
      const snippets = [
        createSnippet({ id: "1", tags: ["active-tag"], isArchived: false }),
        createSnippet({ id: "2", tags: ["archived-tag"], isArchived: true }),
      ];

      const tags = ["active-tag", "archived-tag"];
      const unused = getUnusedTags(snippets, tags);

      expect(unused).toEqual(["archived-tag"]);
    });

    it("should return all tags when no snippets exist", () => {
      const unused = getUnusedTags([], ["tag-1", "tag-2"]);

      expect(unused).toEqual(["tag-1", "tag-2"]);
    });

    it("should return empty array when all tags are used", () => {
      const snippets = [createSnippet({ id: "1", tags: ["tag-1", "tag-2"] })];

      const unused = getUnusedTags(snippets, ["tag-1", "tag-2"]);

      expect(unused).toEqual([]);
    });
  });

  describe("computeCleanupSuggestions", () => {
    it("should suggest cleanup for never-used snippets older than 30 days", () => {
      const snippets = [
        createSnippet({
          id: "old-unused",
          title: "Old Unused",
          createdAt: Date.now() - 45 * 24 * 60 * 60 * 1000, // 45 days old
          lastUsedAt: undefined,
          useCount: 0,
        }),
      ];

      const suggestions = computeCleanupSuggestions(snippets, []);

      expect(suggestions).toHaveLength(1);
      expect(suggestions[0].type).toBe("never_used");
      expect(suggestions[0].snippet?.id).toBe("old-unused");
      expect(suggestions[0].reason).toContain("Never used");
    });

    it("should suggest cleanup for stale snippets (90+ days unused)", () => {
      const snippets = [
        createSnippet({
          id: "stale",
          title: "Stale Snippet",
          lastUsedAt: Date.now() - 120 * 24 * 60 * 60 * 1000, // 120 days ago
          useCount: 5,
        }),
      ];

      const suggestions = computeCleanupSuggestions(snippets, []);

      expect(suggestions).toHaveLength(1);
      expect(suggestions[0].type).toBe("stale");
      expect(suggestions[0].snippet?.id).toBe("stale");
      expect(suggestions[0].reason).toContain("Not used in 120 days");
    });

    it("should suggest cleanup for unused tags", () => {
      const snippets = [createSnippet({ id: "1", tags: ["used-tag"] })];

      const tags = ["used-tag", "orphan-tag"];
      const suggestions = computeCleanupSuggestions(snippets, tags);

      expect(suggestions).toHaveLength(1);
      expect(suggestions[0].type).toBe("unused_tag");
      expect(suggestions[0].tag).toBe("orphan-tag");
    });

    it("should not suggest cleanup for archived snippets", () => {
      const snippets = [
        createSnippet({
          id: "archived-stale",
          createdAt: Date.now() - 200 * 24 * 60 * 60 * 1000,
          lastUsedAt: Date.now() - 150 * 24 * 60 * 60 * 1000,
          useCount: 1,
          isArchived: true,
        }),
      ];

      const suggestions = computeCleanupSuggestions(snippets, []);

      expect(suggestions).toHaveLength(0);
    });

    it("should not suggest cleanup for recently created never-used snippets", () => {
      const snippets = [
        createSnippet({
          id: "new-unused",
          createdAt: Date.now() - 10 * 24 * 60 * 60 * 1000, // 10 days old
          lastUsedAt: undefined,
          useCount: 0,
        }),
      ];

      const suggestions = computeCleanupSuggestions(snippets, []);

      expect(suggestions).toHaveLength(0);
    });

    it("should return multiple suggestions of different types", () => {
      const snippets = [
        createSnippet({
          id: "never-used",
          createdAt: Date.now() - 50 * 24 * 60 * 60 * 1000,
          lastUsedAt: undefined,
          useCount: 0,
          tags: ["active-tag"],
        }),
        createSnippet({
          id: "stale",
          lastUsedAt: Date.now() - 100 * 24 * 60 * 60 * 1000,
          useCount: 3,
          tags: ["active-tag"],
        }),
      ];

      const tags = ["active-tag", "unused-tag"];
      const suggestions = computeCleanupSuggestions(snippets, tags);

      expect(suggestions).toHaveLength(3);
      expect(suggestions.filter((s) => s.type === "never_used")).toHaveLength(1);
      expect(suggestions.filter((s) => s.type === "stale")).toHaveLength(1);
      expect(suggestions.filter((s) => s.type === "unused_tag")).toHaveLength(1);
    });
  });

  describe("findSimilarSnippets", () => {
    it("should return score 1.0 for identical content", () => {
      const target = createSnippet({ id: "target", title: "Deploy script", content: "git push origin main force" });
      const duplicate = createSnippet({ id: "dup", title: "Deploy script", content: "git push origin main force" });

      const results = findSimilarSnippets(target, [target, duplicate]);

      expect(results).toHaveLength(1);
      expect(results[0].snippet.id).toBe("dup");
      expect(results[0].score).toBe(1.0);
    });

    it("should return empty array for completely different content", () => {
      const target = createSnippet({ id: "target", title: "Python loop", content: "for item list range" });
      const other = createSnippet({ id: "other", title: "SQL query", content: "select from where join" });

      const results = findSimilarSnippets(target, [target, other]);

      expect(results).toHaveLength(0);
    });

    it("should exclude snippets below threshold", () => {
      const target = createSnippet({ id: "target", title: "apple banana cherry", content: "fruit salad recipe" });
      const lowMatch = createSnippet({
        id: "low",
        title: "apple orange grape",
        content: "juice drink water lemon lime",
      });

      const results = findSimilarSnippets(target, [target, lowMatch], 0.9);

      expect(results).toHaveLength(0);
    });

    it("should exclude archived snippets", () => {
      const target = createSnippet({
        id: "target",
        title: "Deploy script",
        content: "git push origin main force deploy",
      });
      const archived = createSnippet({
        id: "archived",
        title: "Deploy script",
        content: "git push origin main force deploy",
        isArchived: true,
      });

      const results = findSimilarSnippets(target, [target, archived]);

      expect(results).toHaveLength(0);
    });

    it("should exclude the target snippet itself", () => {
      const target = createSnippet({ id: "target", title: "hello world", content: "hello world example" });

      const results = findSimilarSnippets(target, [target]);

      expect(results).toHaveLength(0);
    });

    it("should return empty array for empty content", () => {
      const target = createSnippet({ id: "target", title: "", content: "" });
      const other = createSnippet({ id: "other", title: "something", content: "relevant content here" });

      const results = findSimilarSnippets(target, [target, other]);

      expect(results).toHaveLength(0);
    });

    it("should return results sorted by score descending", () => {
      const target = createSnippet({
        id: "t",
        title: "git push deploy main branch origin remote",
        content: "git push",
      });
      const high = createSnippet({
        id: "high",
        title: "git push deploy main branch origin remote server",
        content: "git push deploy",
      });
      const low = createSnippet({ id: "low", title: "git push deploy", content: "simple" });

      const results = findSimilarSnippets(target, [target, high, low]);

      expect(results.length).toBeGreaterThan(0);
      for (let i = 1; i < results.length; i++) {
        expect(results[i - 1].score).toBeGreaterThanOrEqual(results[i].score);
      }
    });

    it("should return at most 5 results", () => {
      const sharedWords = "deploy git push origin main branch remote server production staging";
      const target = createSnippet({ id: "t", title: sharedWords, content: sharedWords });
      const others = Array.from({ length: 10 }, (_, i) =>
        createSnippet({ id: `s${i}`, title: sharedWords, content: `${sharedWords} unique${i}` }),
      );

      const results = findSimilarSnippets(target, [target, ...others]);

      expect(results.length).toBeLessThanOrEqual(5);
    });
  });
});
