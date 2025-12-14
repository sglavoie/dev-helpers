import { describe, it, expect } from "vitest";
import { computeTagStatistics, formatRelativeTime, formatNumber, sortTagStatistics, TagSortOption } from "./tagStats";
import { Snippet } from "../types";

describe("tagStats", () => {
  describe("computeTagStatistics", () => {
    it("should compute statistics for tags with snippets", () => {
      const snippets: Snippet[] = [
        {
          id: "1",
          title: "Snippet 1",
          content: "Content 1",
          tags: ["work", "dev"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: 5000,
          useCount: 10,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
        {
          id: "2",
          title: "Snippet 2",
          content: "Content 2",
          tags: ["work", "api"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: 8000,
          useCount: 25,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
        {
          id: "3",
          title: "Snippet 3",
          content: "Content 3",
          tags: ["personal"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: 3000,
          useCount: 5,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
      ];

      const tags = ["work", "dev", "api", "personal"];
      const stats = computeTagStatistics(snippets, tags);

      expect(stats).toHaveLength(4);

      // Work tag should have 2 snippets, 35 total uses, most recent lastUsedAt of 8000
      const workStats = stats.find((s) => s.tag === "work");
      expect(workStats).toEqual({
        tag: "work",
        snippetCount: 2,
        lastUsedAt: 8000,
        totalUsageCount: 35,
        neverUsed: false,
      });

      // Dev tag should have 1 snippet
      const devStats = stats.find((s) => s.tag === "dev");
      expect(devStats).toEqual({
        tag: "dev",
        snippetCount: 1,
        lastUsedAt: 5000,
        totalUsageCount: 10,
        neverUsed: false,
      });

      // Personal tag should have 1 snippet
      const personalStats = stats.find((s) => s.tag === "personal");
      expect(personalStats).toEqual({
        tag: "personal",
        snippetCount: 1,
        lastUsedAt: 3000,
        totalUsageCount: 5,
        neverUsed: false,
      });
    });

    it("should handle tags with no snippets", () => {
      const snippets: Snippet[] = [
        {
          id: "1",
          title: "Snippet 1",
          content: "Content 1",
          tags: ["work"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: 5000,
          useCount: 10,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
      ];

      const tags = ["work", "personal", "dev"];
      const stats = computeTagStatistics(snippets, tags);

      // Personal and dev tags should have 0 snippets
      const personalStats = stats.find((s) => s.tag === "personal");
      expect(personalStats).toEqual({
        tag: "personal",
        snippetCount: 0,
        lastUsedAt: undefined,
        totalUsageCount: 0,
        neverUsed: true,
      });

      const devStats = stats.find((s) => s.tag === "dev");
      expect(devStats).toEqual({
        tag: "dev",
        snippetCount: 0,
        lastUsedAt: undefined,
        totalUsageCount: 0,
        neverUsed: true,
      });
    });

    it("should mark tags as never used when snippets exist but have never been used", () => {
      const snippets: Snippet[] = [
        {
          id: "1",
          title: "Snippet 1",
          content: "Content 1",
          tags: ["work"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: undefined, // Never used
          useCount: 0,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
        {
          id: "2",
          title: "Snippet 2",
          content: "Content 2",
          tags: ["work"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: undefined, // Never used
          useCount: 0,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
      ];

      const tags = ["work"];
      const stats = computeTagStatistics(snippets, tags);

      const workStats = stats.find((s) => s.tag === "work");
      expect(workStats).toEqual({
        tag: "work",
        snippetCount: 2,
        lastUsedAt: undefined,
        totalUsageCount: 0,
        neverUsed: true,
      });
    });

    it("should handle empty snippets and tags", () => {
      const stats = computeTagStatistics([], []);
      expect(stats).toEqual([]);
    });

    it("should correctly sum usage counts", () => {
      const snippets: Snippet[] = [
        {
          id: "1",
          title: "Snippet 1",
          content: "Content 1",
          tags: ["work"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: 5000,
          useCount: 100,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
        {
          id: "2",
          title: "Snippet 2",
          content: "Content 2",
          tags: ["work"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: 6000,
          useCount: 50,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
        {
          id: "3",
          title: "Snippet 3",
          content: "Content 3",
          tags: ["work"],
          createdAt: 1000,
          updatedAt: 2000,
          lastUsedAt: 7000,
          useCount: 25,
          isFavorite: false,
          isArchived: false,
          isPinned: false,
        },
      ];

      const tags = ["work"];
      const stats = computeTagStatistics(snippets, tags);

      const workStats = stats.find((s) => s.tag === "work");
      expect(workStats?.totalUsageCount).toBe(175);
    });
  });

  describe("formatRelativeTime", () => {
    it("should format times correctly", () => {
      const now = Date.now();

      expect(formatRelativeTime(now - 30 * 1000)).toBe("just now"); // 30 seconds ago
      expect(formatRelativeTime(now - 5 * 60 * 1000)).toBe("5m ago"); // 5 minutes ago
      expect(formatRelativeTime(now - 3 * 60 * 60 * 1000)).toBe("3h ago"); // 3 hours ago
      expect(formatRelativeTime(now - 2 * 24 * 60 * 60 * 1000)).toBe("2d ago"); // 2 days ago
      expect(formatRelativeTime(now - 2 * 7 * 24 * 60 * 60 * 1000)).toBe("2w ago"); // 2 weeks ago
      expect(formatRelativeTime(now - 2 * 30 * 24 * 60 * 60 * 1000)).toBe("2mo ago"); // 2 months ago
      expect(formatRelativeTime(now - 2 * 365 * 24 * 60 * 60 * 1000)).toBe("2y ago"); // 2 years ago
    });
  });

  describe("formatNumber", () => {
    it("should format numbers with commas", () => {
      expect(formatNumber(0)).toBe("0");
      expect(formatNumber(100)).toBe("100");
      expect(formatNumber(1000)).toBe("1,000");
      expect(formatNumber(1234567)).toBe("1,234,567");
    });
  });

  describe("sortTagStatistics", () => {
    const stats = [
      {
        tag: "zebra",
        snippetCount: 5,
        lastUsedAt: 1000,
        totalUsageCount: 50,
        neverUsed: false,
      },
      {
        tag: "alpha",
        snippetCount: 10,
        lastUsedAt: 5000,
        totalUsageCount: 100,
        neverUsed: false,
      },
      {
        tag: "beta",
        snippetCount: 3,
        lastUsedAt: undefined,
        totalUsageCount: 0,
        neverUsed: true,
      },
      {
        tag: "gamma",
        snippetCount: 8,
        lastUsedAt: 8000,
        totalUsageCount: 200,
        neverUsed: false,
      },
    ];

    it("should sort by name ascending", () => {
      const sorted = sortTagStatistics(stats, TagSortOption.NameAsc);
      expect(sorted.map((s) => s.tag)).toEqual(["alpha", "beta", "gamma", "zebra"]);
    });

    it("should sort by snippet count descending", () => {
      const sorted = sortTagStatistics(stats, TagSortOption.SnippetCountDesc);
      expect(sorted.map((s) => s.tag)).toEqual(["alpha", "gamma", "zebra", "beta"]);
    });

    it("should sort by last used descending (never used at end)", () => {
      const sorted = sortTagStatistics(stats, TagSortOption.LastUsedDesc);
      expect(sorted.map((s) => s.tag)).toEqual(["gamma", "alpha", "zebra", "beta"]);
    });

    it("should sort by usage count descending", () => {
      const sorted = sortTagStatistics(stats, TagSortOption.UsageCountDesc);
      expect(sorted.map((s) => s.tag)).toEqual(["gamma", "alpha", "zebra", "beta"]);
    });

    it("should sort never used first", () => {
      const sorted = sortTagStatistics(stats, TagSortOption.NeverUsedFirst);
      expect(sorted.map((s) => s.tag)).toEqual(["beta", "alpha", "gamma", "zebra"]);
    });

    it("should use name as tie-breaker for snippet count", () => {
      const tieStats = [
        { tag: "zebra", snippetCount: 5, lastUsedAt: 1000, totalUsageCount: 10, neverUsed: false },
        { tag: "alpha", snippetCount: 5, lastUsedAt: 2000, totalUsageCount: 20, neverUsed: false },
      ];

      const sorted = sortTagStatistics(tieStats, TagSortOption.SnippetCountDesc);
      expect(sorted.map((s) => s.tag)).toEqual(["alpha", "zebra"]);
    });
  });
});
