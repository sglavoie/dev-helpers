import { describe, it, expect } from "vitest";
import type { Snippet } from "../types";
import { applySearchFilters } from "./searchFilter";
import { createSnippet, createQuery } from "./searchFilter.testHelpers";

describe("searchFilter", () => {
  describe("applySearchFilters", () => {
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
});
