import { describe, it, expect } from "vitest";
import { parseSearchQuery, isEmptyQuery, type ParsedQuery } from "./queryParser";

describe("queryParser", () => {
  describe("parseSearchQuery", () => {
    describe("empty and whitespace queries", () => {
      it("should return empty result for empty string", () => {
        const result = parseSearchQuery("");
        expect(result).toEqual({
          tags: [],
          notTags: [],
          is: [],
          not: [],
          exactPhrases: [],
          fuzzyText: "",
          hasOperators: false,
        });
      });

      it("should return empty result for whitespace only", () => {
        const result = parseSearchQuery("   ");
        expect(result).toEqual({
          tags: [],
          notTags: [],
          is: [],
          not: [],
          exactPhrases: [],
          fuzzyText: "",
          hasOperators: false,
        });
      });
    });

    describe("tag operators", () => {
      it("should parse single tag operator", () => {
        const result = parseSearchQuery("tag:work");
        expect(result.tags).toEqual(["work"]);
        expect(result.hasOperators).toBe(true);
      });

      it("should parse hierarchical tag", () => {
        const result = parseSearchQuery("tag:work/projects");
        expect(result.tags).toEqual(["work/projects"]);
      });

      it("should parse multiple tag operators", () => {
        const result = parseSearchQuery("tag:work tag:client");
        expect(result.tags).toEqual(["work", "client"]);
      });

      it("should normalize tag to lowercase", () => {
        const result = parseSearchQuery("tag:Work");
        expect(result.tags).toEqual(["work"]);
      });

      it("should normalize hierarchical tag to lowercase", () => {
        const result = parseSearchQuery("tag:WORK/PROJECTS");
        expect(result.tags).toEqual(["work/projects"]);
      });

      it("should parse tag with fuzzy text", () => {
        const result = parseSearchQuery("tag:work api");
        expect(result.tags).toEqual(["work"]);
        expect(result.fuzzyText).toBe("api");
      });
    });

    describe("negative tag operators", () => {
      it("should parse negative tag operator", () => {
        const result = parseSearchQuery("not:tag:personal");
        expect(result.notTags).toEqual(["personal"]);
        expect(result.hasOperators).toBe(true);
      });

      it("should parse negative hierarchical tag", () => {
        const result = parseSearchQuery("not:tag:work/client");
        expect(result.notTags).toEqual(["work/client"]);
      });

      it("should parse multiple negative tags", () => {
        const result = parseSearchQuery("not:tag:work not:tag:personal");
        expect(result.notTags).toEqual(["work", "personal"]);
      });

      it("should normalize negative tag to lowercase", () => {
        const result = parseSearchQuery("not:tag:PERSONAL");
        expect(result.notTags).toEqual(["personal"]);
      });

      it("should parse mix of positive and negative tags", () => {
        const result = parseSearchQuery("tag:work not:tag:client");
        expect(result.tags).toEqual(["work"]);
        expect(result.notTags).toEqual(["client"]);
      });
    });

    describe("is: boolean operators", () => {
      it("should parse is:favorite", () => {
        const result = parseSearchQuery("is:favorite");
        expect(result.is).toEqual(["favorite"]);
        expect(result.hasOperators).toBe(true);
      });

      it("should parse is:archived", () => {
        const result = parseSearchQuery("is:archived");
        expect(result.is).toEqual(["archived"]);
      });

      it("should parse is:untagged", () => {
        const result = parseSearchQuery("is:untagged");
        expect(result.is).toEqual(["untagged"]);
      });

      it("should parse multiple is: operators", () => {
        const result = parseSearchQuery("is:favorite is:archived");
        expect(result.is).toEqual(["favorite", "archived"]);
      });

      it("should ignore invalid is: values", () => {
        const result = parseSearchQuery("is:invalid");
        expect(result.is).toEqual([]);
        expect(result.fuzzyText).toBe("is:invalid");
        expect(result.hasOperators).toBe(false);
      });
    });

    describe("not: boolean operators", () => {
      it("should parse not:favorite", () => {
        const result = parseSearchQuery("not:favorite");
        expect(result.not).toEqual(["favorite"]);
        expect(result.hasOperators).toBe(true);
      });

      it("should parse not:archived", () => {
        const result = parseSearchQuery("not:archived");
        expect(result.not).toEqual(["archived"]);
      });

      it("should parse not:untagged", () => {
        const result = parseSearchQuery("not:untagged");
        expect(result.not).toEqual(["untagged"]);
      });

      it("should parse multiple not: operators", () => {
        const result = parseSearchQuery("not:favorite not:archived");
        expect(result.not).toEqual(["favorite", "archived"]);
      });

      it("should ignore invalid not: values (treat as fuzzy)", () => {
        const result = parseSearchQuery("not:invalid");
        expect(result.not).toEqual([]);
        expect(result.fuzzyText).toBe("not:invalid");
        expect(result.hasOperators).toBe(false);
      });

      it("should distinguish not:tag: from not: boolean", () => {
        const result = parseSearchQuery("not:archived not:tag:work");
        expect(result.not).toEqual(["archived"]);
        expect(result.notTags).toEqual(["work"]);
      });
    });

    describe("exact phrase operators", () => {
      it("should parse single exact phrase", () => {
        const result = parseSearchQuery('"meeting notes"');
        expect(result.exactPhrases).toEqual(["meeting notes"]);
        expect(result.hasOperators).toBe(true);
      });

      it("should parse multiple exact phrases", () => {
        const result = parseSearchQuery('"phrase one" "phrase two"');
        expect(result.exactPhrases).toEqual(["phrase one", "phrase two"]);
      });

      it("should parse exact phrase with fuzzy text", () => {
        const result = parseSearchQuery('"api docs" rest');
        expect(result.exactPhrases).toEqual(["api docs"]);
        expect(result.fuzzyText).toBe("rest");
      });

      it("should ignore empty quotes", () => {
        const result = parseSearchQuery('""');
        expect(result.exactPhrases).toEqual([]);
      });

      it("should trim whitespace in phrases", () => {
        const result = parseSearchQuery('"  meeting notes  "');
        expect(result.exactPhrases).toEqual(["meeting notes"]);
      });

      it("should handle unclosed quotes as fuzzy text", () => {
        const result = parseSearchQuery('"unclosed');
        expect(result.exactPhrases).toEqual([]);
        expect(result.fuzzyText).toBe('"unclosed');
        expect(result.hasOperators).toBe(false);
      });
    });

    describe("fuzzy text", () => {
      it("should parse simple fuzzy text", () => {
        const result = parseSearchQuery("simple search");
        expect(result.fuzzyText).toBe("simple search");
        expect(result.hasOperators).toBe(false);
      });

      it("should parse multiple words as fuzzy text", () => {
        const result = parseSearchQuery("multiple words search query");
        expect(result.fuzzyText).toBe("multiple words search query");
      });

      it("should preserve fuzzy text after operators", () => {
        const result = parseSearchQuery("tag:work api rest");
        expect(result.fuzzyText).toBe("api rest");
      });

      it("should handle unknown operators as fuzzy text", () => {
        const result = parseSearchQuery("unknown:operator search");
        expect(result.fuzzyText).toBe("unknown:operator search");
        expect(result.hasOperators).toBe(false);
      });
    });

    describe("combined operators", () => {
      it("should parse tag + is: + fuzzy", () => {
        const result = parseSearchQuery("tag:work is:favorite api");
        expect(result.tags).toEqual(["work"]);
        expect(result.is).toEqual(["favorite"]);
        expect(result.fuzzyText).toBe("api");
        expect(result.hasOperators).toBe(true);
      });

      it("should parse tag + not: + exact + fuzzy", () => {
        const result = parseSearchQuery('tag:work not:archived "meeting" api');
        expect(result.tags).toEqual(["work"]);
        expect(result.not).toEqual(["archived"]);
        expect(result.exactPhrases).toEqual(["meeting"]);
        expect(result.fuzzyText).toBe("api");
      });

      it("should parse kitchen sink query", () => {
        const result = parseSearchQuery(
          'tag:work tag:client not:tag:personal is:favorite not:archived "api docs" rest endpoint',
        );
        expect(result.tags).toEqual(["work", "client"]);
        expect(result.notTags).toEqual(["personal"]);
        expect(result.is).toEqual(["favorite"]);
        expect(result.not).toEqual(["archived"]);
        expect(result.exactPhrases).toEqual(["api docs"]);
        expect(result.fuzzyText).toBe("rest endpoint");
        expect(result.hasOperators).toBe(true);
      });
    });

    describe("edge cases", () => {
      it("should handle tag: without value", () => {
        const result = parseSearchQuery("tag:");
        expect(result.tags).toEqual([]);
        expect(result.fuzzyText).toBe("tag:");
        expect(result.hasOperators).toBe(false);
      });

      it("should handle is: without value", () => {
        const result = parseSearchQuery("is:");
        expect(result.is).toEqual([]);
        expect(result.fuzzyText).toBe("is:");
      });

      it("should handle not:tag: without value", () => {
        const result = parseSearchQuery("not:tag:");
        expect(result.notTags).toEqual([]);
        expect(result.fuzzyText).toBe("not:tag:");
      });

      it("should handle multiple spaces between operators", () => {
        const result = parseSearchQuery("tag:work    is:favorite");
        expect(result.tags).toEqual(["work"]);
        expect(result.is).toEqual(["favorite"]);
      });

      it("should handle leading and trailing whitespace", () => {
        const result = parseSearchQuery("  tag:work  ");
        expect(result.tags).toEqual(["work"]);
      });

      it("should handle mixed valid and invalid operators", () => {
        const result = parseSearchQuery("tag:work invalid:operator is:favorite");
        expect(result.tags).toEqual(["work"]);
        expect(result.is).toEqual(["favorite"]);
        expect(result.fuzzyText).toBe("invalid:operator");
      });
    });

    describe("hasOperators flag", () => {
      it("should be true when tag operator present", () => {
        const result = parseSearchQuery("tag:work");
        expect(result.hasOperators).toBe(true);
      });

      it("should be true when is: operator present", () => {
        const result = parseSearchQuery("is:favorite");
        expect(result.hasOperators).toBe(true);
      });

      it("should be true when not: operator present", () => {
        const result = parseSearchQuery("not:archived");
        expect(result.hasOperators).toBe(true);
      });

      it("should be true when exact phrase present", () => {
        const result = parseSearchQuery('"exact phrase"');
        expect(result.hasOperators).toBe(true);
      });

      it("should be false for pure fuzzy search", () => {
        const result = parseSearchQuery("simple search");
        expect(result.hasOperators).toBe(false);
      });

      it("should be false for empty query", () => {
        const result = parseSearchQuery("");
        expect(result.hasOperators).toBe(false);
      });

      it("should be false when only invalid operators present", () => {
        const result = parseSearchQuery("is:invalid unknown:operator");
        expect(result.hasOperators).toBe(false);
      });
    });
  });

  describe("isEmptyQuery", () => {
    it("should return true for empty parsed query", () => {
      const query = parseSearchQuery("");
      expect(isEmptyQuery(query)).toBe(true);
    });

    it("should return false when operators present", () => {
      const query = parseSearchQuery("tag:work");
      expect(isEmptyQuery(query)).toBe(false);
    });

    it("should return false when fuzzy text present", () => {
      const query = parseSearchQuery("search text");
      expect(isEmptyQuery(query)).toBe(false);
    });

    it("should return false when both operators and fuzzy text present", () => {
      const query = parseSearchQuery("tag:work api");
      expect(isEmptyQuery(query)).toBe(false);
    });
  });
});
