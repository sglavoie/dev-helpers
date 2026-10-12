import { describe, it, expect } from "vitest";
import { applySearchFilters, matchesFuzzySearch } from "./searchFilter";
import { createSnippet, createQuery } from "./searchFilter.testHelpers";

describe("searchFilter", () => {
  describe("matchesFuzzySearch", () => {
    it("should match snippet with search text", () => {
      const snippet = createSnippet({ title: "API Documentation" });
      expect(matchesFuzzySearch(snippet, "api")).toBe(true);
    });

    it("should match all words", () => {
      const snippet = createSnippet({ title: "API", content: "rest endpoint" });
      expect(matchesFuzzySearch(snippet, "api rest")).toBe(true);
    });

    it("should return true for empty search text", () => {
      const snippet = createSnippet({ title: "Test" });
      expect(matchesFuzzySearch(snippet, "")).toBe(true);
      expect(matchesFuzzySearch(snippet, "   ")).toBe(true);
    });

    it("should return false when word not found", () => {
      const snippet = createSnippet({ title: "API Documentation" });
      expect(matchesFuzzySearch(snippet, "nonexistent")).toBe(false);
    });

    it("should be case-insensitive", () => {
      const snippet = createSnippet({ title: "api documentation" });
      expect(matchesFuzzySearch(snippet, "API")).toBe(true);
    });

    it("should match against tags case-insensitively", () => {
      const snippet = createSnippet({ title: "x", content: "y", tags: ["Work"] });
      expect(matchesFuzzySearch(snippet, "work")).toBe(true);
    });
  });

  describe("fuzzy text tag matching (case-insensitive)", () => {
    it("should match a mixed-case tag when fuzzy text is lowercase", () => {
      const snippets = [createSnippet({ id: "1", title: "x", content: "y", tags: ["Work"] })];
      const query = createQuery({ fuzzyText: "work", hasOperators: true });
      const result = applySearchFilters(snippets, query);
      expect(result).toHaveLength(1);
      expect(result[0].id).toBe("1");
    });
  });
});
