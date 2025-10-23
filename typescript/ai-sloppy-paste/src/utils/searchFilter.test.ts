import { describe, it, expect } from "vitest";
import { applySearchFilters, matchesFuzzySearch } from "./searchFilter";
import type { Snippet } from "../types";
import type { ParsedQuery } from "./queryParser";

// Helper to create test snippets
function createSnippet(overrides: Partial<Snippet>): Snippet {
  return {
    id: "test-id",
    title: "Test Snippet",
    content: "Test content",
    tags: [],
    createdAt: Date.now(),
    updatedAt: Date.now(),
    useCount: 0,
    isFavorite: false,
    isArchived: false,
    ...overrides,
  };
}

// Helper to create parsed queries
function createQuery(overrides: Partial<ParsedQuery>): ParsedQuery {
  return {
    tags: [],
    notTags: [],
    is: [],
    not: [],
    exactPhrases: [],
    fuzzyText: "",
    hasOperators: false,
    ...overrides,
  };
}

describe("searchFilter", () => {
  describe("applySearchFilters", () => {
    describe("tag filters", () => {
      it("should match snippet with required tag", () => {
        const snippets = [createSnippet({ id: "1", tags: ["work"] }), createSnippet({ id: "2", tags: ["personal"] })];
        const query = createQuery({ tags: ["work"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should match snippet with hierarchical tag", () => {
        const snippets = [
          createSnippet({ id: "1", tags: ["work/projects"] }),
          createSnippet({ id: "2", tags: ["personal"] }),
        ];
        const query = createQuery({ tags: ["work"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should require ALL tags (AND logic)", () => {
        const snippets = [
          createSnippet({ id: "1", tags: ["work", "client"] }),
          createSnippet({ id: "2", tags: ["work"] }),
          createSnippet({ id: "3", tags: ["client"] }),
        ];
        const query = createQuery({ tags: ["work", "client"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should return empty array when tag not found", () => {
        const snippets = [createSnippet({ tags: ["work"] })];
        const query = createQuery({ tags: ["nonexistent"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(0);
      });
    });

    describe("negative tag filters", () => {
      it("should exclude snippet with excluded tag", () => {
        const snippets = [createSnippet({ id: "1", tags: ["work"] }), createSnippet({ id: "2", tags: ["personal"] })];
        const query = createQuery({ notTags: ["personal"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should exclude snippet with hierarchical excluded tag", () => {
        const snippets = [
          createSnippet({ id: "1", tags: ["work/client"] }),
          createSnippet({ id: "2", tags: ["personal"] }),
        ];
        const query = createQuery({ notTags: ["work"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("2");
      });

      it("should handle multiple exclusions (must exclude all)", () => {
        const snippets = [
          createSnippet({ id: "1", tags: ["work"] }),
          createSnippet({ id: "2", tags: ["personal"] }),
          createSnippet({ id: "3", tags: ["hobby"] }),
        ];
        const query = createQuery({ notTags: ["work", "personal"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("3");
      });

      it("should combine positive and negative tag filters", () => {
        const snippets = [
          createSnippet({ id: "1", tags: ["work", "client"] }),
          createSnippet({ id: "2", tags: ["work", "internal"] }),
          createSnippet({ id: "3", tags: ["personal"] }),
        ];
        const query = createQuery({
          tags: ["work"],
          notTags: ["client"],
          hasOperators: true,
        });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("2");
      });
    });

    describe("is: boolean filters", () => {
      it("should filter by is:favorite", () => {
        const snippets = [createSnippet({ id: "1", isFavorite: true }), createSnippet({ id: "2", isFavorite: false })];
        const query = createQuery({ is: ["favorite"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should filter by is:archived", () => {
        const snippets = [createSnippet({ id: "1", isArchived: true }), createSnippet({ id: "2", isArchived: false })];
        const query = createQuery({ is: ["archived"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should filter by is:untagged", () => {
        const snippets = [createSnippet({ id: "1", tags: [] }), createSnippet({ id: "2", tags: ["work"] })];
        const query = createQuery({ is: ["untagged"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should handle multiple is: conditions (AND logic)", () => {
        const snippets = [
          createSnippet({ id: "1", isFavorite: true, isArchived: true }),
          createSnippet({ id: "2", isFavorite: true, isArchived: false }),
          createSnippet({ id: "3", isFavorite: false, isArchived: true }),
        ];
        const query = createQuery({ is: ["favorite", "archived"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });
    });

    describe("not: boolean filters", () => {
      it("should filter by not:favorite", () => {
        const snippets = [createSnippet({ id: "1", isFavorite: true }), createSnippet({ id: "2", isFavorite: false })];
        const query = createQuery({ not: ["favorite"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("2");
      });

      it("should filter by not:archived", () => {
        const snippets = [createSnippet({ id: "1", isArchived: true }), createSnippet({ id: "2", isArchived: false })];
        const query = createQuery({ not: ["archived"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("2");
      });

      it("should filter by not:untagged", () => {
        const snippets = [createSnippet({ id: "1", tags: [] }), createSnippet({ id: "2", tags: ["work"] })];
        const query = createQuery({ not: ["untagged"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("2");
      });
    });

    describe("exact phrase filters", () => {
      it("should match exact phrase in title", () => {
        const snippets = [
          createSnippet({ id: "1", title: "Meeting notes from yesterday" }),
          createSnippet({ id: "2", title: "Random notes" }),
        ];
        const query = createQuery({ exactPhrases: ["meeting notes"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should match exact phrase in content", () => {
        const snippets = [
          createSnippet({ id: "1", content: "This has api documentation" }),
          createSnippet({ id: "2", content: "This has something else" }),
        ];
        const query = createQuery({ exactPhrases: ["api documentation"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should be case-insensitive", () => {
        const snippets = [
          createSnippet({ id: "1", title: "API Documentation" }),
          createSnippet({ id: "2", title: "Something else" }),
        ];
        const query = createQuery({ exactPhrases: ["api documentation"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should require ALL exact phrases (AND logic)", () => {
        const snippets = [
          createSnippet({ id: "1", content: "meeting notes and api docs" }),
          createSnippet({ id: "2", content: "meeting notes only" }),
          createSnippet({ id: "3", content: "api docs only" }),
        ];
        const query = createQuery({
          exactPhrases: ["meeting notes", "api docs"],
          hasOperators: true,
        });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should return empty when phrase not found", () => {
        const snippets = [createSnippet({ title: "Test", content: "Content" })];
        const query = createQuery({ exactPhrases: ["nonexistent phrase"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(0);
      });
    });

    describe("fuzzy text filters", () => {
      it("should match single word in title", () => {
        const snippets = [
          createSnippet({ id: "1", title: "API Documentation" }),
          createSnippet({ id: "2", title: "Meeting Notes" }),
        ];
        const query = createQuery({ fuzzyText: "api" });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should match word in content", () => {
        const snippets = [
          createSnippet({ id: "1", content: "Contains python code" }),
          createSnippet({ id: "2", content: "Contains javascript code" }),
        ];
        const query = createQuery({ fuzzyText: "python" });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should match word in tags", () => {
        const snippets = [
          createSnippet({ id: "1", tags: ["work/client"] }),
          createSnippet({ id: "2", tags: ["personal"] }),
        ];
        const query = createQuery({ fuzzyText: "client" });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should require ALL words to match (AND logic)", () => {
        const snippets = [
          createSnippet({ id: "1", title: "API", content: "rest endpoint" }),
          createSnippet({ id: "2", title: "API", content: "graphql" }),
          createSnippet({ id: "3", title: "Rest", content: "endpoint" }),
        ];
        const query = createQuery({ fuzzyText: "api rest" });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should be case-insensitive", () => {
        const snippets = [createSnippet({ id: "1", title: "API Documentation" })];
        const query = createQuery({ fuzzyText: "API" });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
      });

      it("should match words in different fields", () => {
        const snippets = [
          createSnippet({ id: "1", title: "Meeting", tags: ["work"] }),
          createSnippet({ id: "2", title: "Notes", tags: ["personal"] }),
        ];
        const query = createQuery({ fuzzyText: "meeting work" });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });
    });

    describe("combined filters", () => {
      it("should combine tag + boolean + fuzzy filters", () => {
        const snippets = [
          createSnippet({ id: "1", tags: ["work"], isFavorite: true, content: "api docs" }),
          createSnippet({ id: "2", tags: ["work"], isFavorite: false, content: "api docs" }),
          createSnippet({ id: "3", tags: ["work"], isFavorite: true, content: "other" }),
          createSnippet({ id: "4", tags: ["personal"], isFavorite: true, content: "api docs" }),
        ];
        const query = createQuery({
          tags: ["work"],
          is: ["favorite"],
          fuzzyText: "api",
          hasOperators: true,
        });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should combine all operator types", () => {
        const snippets = [
          createSnippet({
            id: "1",
            tags: ["work", "client"],
            isFavorite: true,
            isArchived: false,
            title: "API meeting notes",
            content: "rest endpoint",
          }),
          createSnippet({
            id: "2",
            tags: ["work", "internal"],
            isFavorite: true,
            isArchived: false,
            title: "API meeting notes",
            content: "rest endpoint",
          }),
        ];
        const query = createQuery({
          tags: ["work"],
          notTags: ["internal"],
          is: ["favorite"],
          not: ["archived"],
          exactPhrases: ["meeting notes"],
          fuzzyText: "api rest",
          hasOperators: true,
        });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe("1");
      });

      it("should return all snippets for empty query", () => {
        const snippets = [createSnippet({ id: "1" }), createSnippet({ id: "2" }), createSnippet({ id: "3" })];
        const query = createQuery({});
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(3);
      });
    });

    describe("edge cases", () => {
      it("should handle empty snippets array", () => {
        const snippets: Snippet[] = [];
        const query = createQuery({ tags: ["work"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(0);
      });

      it("should return empty for contradictory filters", () => {
        const snippets = [createSnippet({ id: "1", isArchived: true }), createSnippet({ id: "2", isArchived: false })];
        const query = createQuery({
          is: ["archived"],
          not: ["archived"],
          hasOperators: true,
        });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(0);
      });

      it("should handle snippets with no tags", () => {
        const snippets = [createSnippet({ id: "1", tags: [] })];
        const query = createQuery({ tags: ["work"], hasOperators: true });
        const result = applySearchFilters(snippets, query);
        expect(result).toHaveLength(0);
      });
    });
  });

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
  });
});
