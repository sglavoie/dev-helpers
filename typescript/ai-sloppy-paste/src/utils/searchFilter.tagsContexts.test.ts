import { describe, it, expect } from "vitest";
import { applySearchFilters } from "./searchFilter";
import { createSnippet, createQuery } from "./searchFilter.testHelpers";

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

    describe("context filters", () => {
      const contextSnippets = [
        createSnippet({ id: "1", title: "asl: run" }),
        createSnippet({ id: "2", title: "Refactor: debt triage" }),
        createSnippet({ id: "3", title: "refactor: dead code" }),
        createSnippet({ id: "4", title: "asl commit" }),
      ];

      it("should filter to a single context", () => {
        const query = createQuery({ contexts: ["asl"], hasOperators: true });
        expect(applySearchFilters(contextSnippets, query).map((s) => s.id)).toEqual(["1"]);
      });

      it("should match contexts case-insensitively", () => {
        const query = createQuery({ contexts: ["refactor"], hasOperators: true });
        expect(applySearchFilters(contextSnippets, query).map((s) => s.id)).toEqual(["2", "3"]);
      });

      it("should exclude snippets with no context", () => {
        const query = createQuery({ contexts: ["asl"], hasOperators: true });
        expect(applySearchFilters(contextSnippets, query).map((s) => s.id)).not.toContain("4");
      });

      // Unlike tag:, contexts are flat - no prefix/hierarchy matching
      it("should require exact equality, not a prefix match", () => {
        const snippets = [createSnippet({ id: "1", title: "asl-run: go" })];
        const query = createQuery({ contexts: ["asl"], hasOperators: true });
        expect(applySearchFilters(snippets, query)).toHaveLength(0);
      });

      it("should return nothing for two different required contexts", () => {
        const query = createQuery({ contexts: ["asl", "refactor"], hasOperators: true });
        expect(applySearchFilters(contextSnippets, query)).toHaveLength(0);
      });
    });

    describe("negative context filters", () => {
      const contextSnippets = [
        createSnippet({ id: "1", title: "asl: run" }),
        createSnippet({ id: "2", title: "Refactor: debt triage" }),
        createSnippet({ id: "3", title: "asl commit" }),
      ];

      it("should exclude a context", () => {
        const query = createQuery({ notContexts: ["asl"], hasOperators: true });
        expect(applySearchFilters(contextSnippets, query).map((s) => s.id)).toEqual(["2", "3"]);
      });

      it("should keep snippets with no context at all", () => {
        const query = createQuery({ notContexts: ["asl"], hasOperators: true });
        expect(applySearchFilters(contextSnippets, query).map((s) => s.id)).toContain("3");
      });

      it("should combine positive and negative context filters", () => {
        const query = createQuery({ contexts: ["asl"], notContexts: ["refactor"], hasOperators: true });
        expect(applySearchFilters(contextSnippets, query).map((s) => s.id)).toEqual(["1"]);
      });

      it("should combine context and tag filters", () => {
        const snippets = [
          createSnippet({ id: "1", title: "asl: run", tags: ["work"] }),
          createSnippet({ id: "2", title: "asl: commit", tags: ["personal"] }),
        ];
        const query = createQuery({ contexts: ["asl"], tags: ["work"], hasOperators: true });
        expect(applySearchFilters(snippets, query).map((s) => s.id)).toEqual(["1"]);
      });
    });
  });
});
