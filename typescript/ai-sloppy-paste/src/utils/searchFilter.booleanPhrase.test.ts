import { describe, it, expect } from "vitest";
import { applySearchFilters } from "./searchFilter";
import { createSnippet, createQuery } from "./searchFilter.testHelpers";

describe("searchFilter", () => {
  describe("applySearchFilters", () => {
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
  });
});
