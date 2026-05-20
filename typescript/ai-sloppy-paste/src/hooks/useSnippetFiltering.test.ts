import { describe, it, expect } from "vitest";
import { filterSnippets, FilterOptions } from "./useSnippetFiltering";
import { parseSearchQuery } from "../utils/queryParser";
import type { Snippet } from "../types";

const DAY_MS = 24 * 60 * 60 * 1000;

function createSnippet(overrides: Partial<Snippet>): Snippet {
  const now = Date.now();
  return {
    id: "test-id",
    title: "Test Snippet",
    content: "Test content",
    tags: [],
    createdAt: now,
    updatedAt: now,
    useCount: 0,
    isFavorite: false,
    isArchived: false,
    isPinned: false,
    ...overrides,
  };
}

const DEFAULT_OPTIONS: FilterOptions = {
  selectedTag: "All",
  showOnlyFavorites: false,
  showArchivedSnippets: false,
  showNeedsAttention: false,
};

describe("filterSnippets", () => {
  describe("without search operators", () => {
    it("applies tag filter", () => {
      const snippets = [
        createSnippet({ id: "1", tags: ["work"] }),
        createSnippet({ id: "2", tags: ["personal"] }),
        createSnippet({ id: "3", tags: ["work/projects"] }),
      ];
      const parsed = parseSearchQuery("");
      const result = filterSnippets(snippets, parsed, { ...DEFAULT_OPTIONS, selectedTag: "work" });
      expect(result.map((s) => s.id).sort()).toEqual(["1", "3"]);
    });

    it("applies favorites filter", () => {
      const snippets = [createSnippet({ id: "1", isFavorite: true }), createSnippet({ id: "2", isFavorite: false })];
      const parsed = parseSearchQuery("");
      const result = filterSnippets(snippets, parsed, { ...DEFAULT_OPTIONS, showOnlyFavorites: true });
      expect(result.map((s) => s.id)).toEqual(["1"]);
    });

    it("returns all non-archived snippets when no filters are active", () => {
      const snippets = [
        createSnippet({ id: "1" }),
        createSnippet({ id: "2", isArchived: true }),
        createSnippet({ id: "3" }),
      ];
      const parsed = parseSearchQuery("");
      const result = filterSnippets(snippets, parsed, DEFAULT_OPTIONS);
      expect(result.map((s) => s.id).sort()).toEqual(["1", "3"]);
    });
  });

  describe("with search operators AND tag filter (regression for the bug fix)", () => {
    it("applies BOTH the tag dropdown filter and the search operators", () => {
      const snippets = [
        createSnippet({ id: "1", tags: ["work"], title: "API notes" }),
        createSnippet({ id: "2", tags: ["personal"], title: "API recipes" }),
        createSnippet({ id: "3", tags: ["work"], title: "Random" }),
      ];
      // Query "api" produces fuzzy text -> hasOperators true.
      const parsed = parseSearchQuery("api");
      expect(parsed.hasOperators).toBe(true);

      const result = filterSnippets(snippets, parsed, { ...DEFAULT_OPTIONS, selectedTag: "work" });

      // BEFORE the fix the tag dropdown was ignored once operators were present,
      // so the result would have included snippet 2 (personal/API recipes).
      expect(result.map((s) => s.id)).toEqual(["1"]);
    });

    it("combines tag dropdown with explicit tag: operator", () => {
      const snippets = [
        createSnippet({ id: "1", tags: ["work", "client"] }),
        createSnippet({ id: "2", tags: ["work"] }),
        createSnippet({ id: "3", tags: ["personal", "client"] }),
      ];
      const parsed = parseSearchQuery("tag:client");
      const result = filterSnippets(snippets, parsed, { ...DEFAULT_OPTIONS, selectedTag: "work" });
      expect(result.map((s) => s.id)).toEqual(["1"]);
    });
  });

  describe("with search operators AND favorites toggle", () => {
    it("applies BOTH the favorites toggle and the search operators", () => {
      const snippets = [
        createSnippet({ id: "1", isFavorite: true, content: "api docs" }),
        createSnippet({ id: "2", isFavorite: false, content: "api docs" }),
        createSnippet({ id: "3", isFavorite: true, content: "other content" }),
      ];
      const parsed = parseSearchQuery("api");
      expect(parsed.hasOperators).toBe(true);

      const result = filterSnippets(snippets, parsed, { ...DEFAULT_OPTIONS, showOnlyFavorites: true });
      // Only snippet 1 is both favorited AND matches "api".
      expect(result.map((s) => s.id)).toEqual(["1"]);
    });
  });

  describe("archive view", () => {
    it("returns only archived snippets when showArchivedSnippets is true", () => {
      const snippets = [
        createSnippet({ id: "1", isArchived: true }),
        createSnippet({ id: "2", isArchived: false }),
        createSnippet({ id: "3", isArchived: true }),
      ];
      const parsed = parseSearchQuery("");
      const result = filterSnippets(snippets, parsed, { ...DEFAULT_OPTIONS, showArchivedSnippets: true });
      expect(result.map((s) => s.id).sort()).toEqual(["1", "3"]);
    });

    it("excludes archived snippets in the default view", () => {
      const snippets = [createSnippet({ id: "1", isArchived: true }), createSnippet({ id: "2", isArchived: false })];
      const parsed = parseSearchQuery("");
      const result = filterSnippets(snippets, parsed, DEFAULT_OPTIONS);
      expect(result.map((s) => s.id)).toEqual(["2"]);
    });
  });

  describe("showNeedsAttention", () => {
    it("further restricts results to stale snippets", () => {
      const now = Date.now();
      const longAgo = now - 200 * DAY_MS;
      const snippets = [
        // Stale: used 200 days ago.
        createSnippet({ id: "stale", createdAt: longAgo, updatedAt: longAgo, lastUsedAt: longAgo, useCount: 3 }),
        // Fresh: used today.
        createSnippet({ id: "fresh", lastUsedAt: now, useCount: 5 }),
        // Never used, but recent — not stale.
        createSnippet({ id: "new" }),
      ];
      const parsed = parseSearchQuery("");
      const result = filterSnippets(snippets, parsed, { ...DEFAULT_OPTIONS, showNeedsAttention: true });
      expect(result.map((s) => s.id)).toEqual(["stale"]);
    });

    it("combines with other filters", () => {
      const now = Date.now();
      const longAgo = now - 200 * DAY_MS;
      const snippets = [
        createSnippet({
          id: "stale-work",
          tags: ["work"],
          createdAt: longAgo,
          updatedAt: longAgo,
          lastUsedAt: longAgo,
          useCount: 1,
        }),
        createSnippet({
          id: "stale-personal",
          tags: ["personal"],
          createdAt: longAgo,
          updatedAt: longAgo,
          lastUsedAt: longAgo,
          useCount: 1,
        }),
        createSnippet({ id: "fresh-work", tags: ["work"], lastUsedAt: now, useCount: 1 }),
      ];
      const parsed = parseSearchQuery("");
      const result = filterSnippets(snippets, parsed, {
        ...DEFAULT_OPTIONS,
        selectedTag: "work",
        showNeedsAttention: true,
      });
      expect(result.map((s) => s.id)).toEqual(["stale-work"]);
    });
  });
});
