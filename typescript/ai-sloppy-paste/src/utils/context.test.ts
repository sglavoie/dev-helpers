import { describe, it, expect } from "vitest";
import {
  parseTitleContext,
  normalizeContext,
  contextAbbreviation,
  contextColor,
  contextBadgeIcon,
  getAllContexts,
  getSnippetContext,
} from "./context";
import { Snippet } from "../types";

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
    isPinned: false,
    ...overrides,
  };
}

describe("parseTitleContext", () => {
  it("should extract a lowercase prefix", () => {
    expect(parseTitleContext("asl: run")).toEqual({ context: "asl", displayTitle: "run" });
  });

  it("should extract a capitalized prefix", () => {
    expect(parseTitleContext("Refactor: debt triage")).toEqual({
      context: "Refactor",
      displayTitle: "debt triage",
    });
  });

  it("should preserve the prefix casing verbatim", () => {
    expect(parseTitleContext("Dead Code: exports").context).toBe("Dead Code");
  });

  it("should only strip the first colon", () => {
    expect(parseTitleContext("asl: run: with args")).toEqual({
      context: "asl",
      displayTitle: "run: with args",
    });
  });

  it("should allow underscores and hyphens in the prefix", () => {
    expect(parseTitleContext("dead-code: exports").context).toBe("dead-code");
    expect(parseTitleContext("dead_code: exports").context).toBe("dead_code");
  });

  it("should tolerate extra whitespace after the colon", () => {
    expect(parseTitleContext("asl:   run")).toEqual({ context: "asl", displayTitle: "run" });
  });

  describe("non-matches", () => {
    it("should not treat a title without a colon as a context", () => {
      expect(parseTitleContext("asl commit")).toEqual({ context: null, displayTitle: "asl commit" });
    });

    it("should not treat a URL as a context (no space after colon)", () => {
      expect(parseTitleContext("https://example.com/docs")).toEqual({
        context: null,
        displayTitle: "https://example.com/docs",
      });
    });

    it("should require whitespace after the colon", () => {
      expect(parseTitleContext("Note:something")).toEqual({
        context: null,
        displayTitle: "Note:something",
      });
    });

    it("should leave a colon-only title intact", () => {
      expect(parseTitleContext("asl:")).toEqual({ context: null, displayTitle: "asl:" });
      expect(parseTitleContext("asl: ")).toEqual({ context: null, displayTitle: "asl: " });
    });

    it("should reject a prefix longer than 24 characters", () => {
      const longPrefix = "a".repeat(25);
      expect(parseTitleContext(`${longPrefix}: value`)).toEqual({
        context: null,
        displayTitle: `${longPrefix}: value`,
      });
    });

    it("should accept a prefix of exactly 24 characters", () => {
      const prefix = "a".repeat(24);
      expect(parseTitleContext(`${prefix}: value`).context).toBe(prefix);
    });

    it("should reject a sentence-like prefix of more than 3 words", () => {
      expect(parseTitleContext("Remember to do this: now")).toEqual({
        context: null,
        displayTitle: "Remember to do this: now",
      });
    });

    it("should accept a 3-word prefix", () => {
      expect(parseTitleContext("Go To Def: usage").context).toBe("Go To Def");
    });

    it("should reject a prefix that does not start alphanumeric", () => {
      expect(parseTitleContext("-asl: run").context).toBeNull();
    });

    it("should handle an empty title", () => {
      expect(parseTitleContext("")).toEqual({ context: null, displayTitle: "" });
    });
  });
});

describe("normalizeContext", () => {
  it("should lowercase and trim", () => {
    expect(normalizeContext("  Refactor  ")).toBe("refactor");
    expect(normalizeContext("ASL")).toBe("asl");
  });
});

describe("contextAbbreviation", () => {
  describe("contexts that fit whole (<= 5 characters)", () => {
    it("should show a short context whole", () => {
      expect(contextAbbreviation("asl")).toBe("ASL");
      expect(contextAbbreviation("PLAN")).toBe("PLAN");
    });

    it("should uppercase regardless of the original casing", () => {
      expect(contextAbbreviation("plan")).toBe("PLAN");
      expect(contextAbbreviation("Plan")).toBe("PLAN");
    });

    it("should show a 5-character context whole", () => {
      expect(contextAbbreviation("build")).toBe("BUILD");
    });

    it("should show a short multi-word context whole", () => {
      expect(contextAbbreviation("Go To")).toBe("GO TO");
    });

    it("should trim surrounding whitespace", () => {
      expect(contextAbbreviation("  asl  ")).toBe("ASL");
    });
  });

  describe("contexts too long to fit (> 5 characters)", () => {
    it("should truncate a single word to 4 characters", () => {
      expect(contextAbbreviation("workflow")).toBe("WORK");
      expect(contextAbbreviation("Refactor")).toBe("REFA");
      expect(contextAbbreviation("planning")).toBe("PLAN");
    });

    it("should use initials for multi-word contexts", () => {
      expect(contextAbbreviation("Dead Code")).toBe("DC");
      expect(contextAbbreviation("go to def")).toBe("GTD");
    });

    it("should treat hyphens and underscores as word separators", () => {
      expect(contextAbbreviation("dead-code")).toBe("DC");
      expect(contextAbbreviation("dead_code")).toBe("DC");
    });
  });

  it("should always be uppercase", () => {
    for (const context of ["asl", "plan", "Go To", "workflow", "dead-code", "R&D"]) {
      const label = contextAbbreviation(context);
      expect(label).toBe(label.toUpperCase());
    }
  });

  it("should never exceed 5 characters", () => {
    for (const context of ["a b c d e f", "workflow", "Dead Code", "build", "a-b-c-d-e-f-g"]) {
      expect(contextAbbreviation(context).length).toBeLessThanOrEqual(5);
    }
  });

  it("should return an empty string for an empty context", () => {
    expect(contextAbbreviation("   ")).toBe("");
  });
});

describe("contextColor", () => {
  it("should be deterministic", () => {
    expect(contextColor("asl")).toEqual(contextColor("asl"));
  });

  it("should ignore casing and surrounding whitespace", () => {
    expect(contextColor("Refactor")).toEqual(contextColor("  refactor  "));
  });

  it("should return hex literals for both themes", () => {
    const color = contextColor("asl");
    expect(color.light).toMatch(/^#[0-9A-Fa-f]{6}$/);
    expect(color.dark).toMatch(/^#[0-9A-Fa-f]{6}$/);
  });

  it("should distinguish common contexts", () => {
    expect(contextColor("asl")).not.toEqual(contextColor("Refactor"));
  });
});

describe("contextBadgeIcon", () => {
  it("should build per-theme data-URI SVG sources", () => {
    const icon = contextBadgeIcon("asl") as { source: { light: string; dark: string } };
    expect(icon.source.light).toMatch(/^data:image\/svg\+xml;base64,/);
    expect(icon.source.dark).toMatch(/^data:image\/svg\+xml;base64,/);
  });

  it("should embed the abbreviation and colour in the markup", () => {
    const icon = contextBadgeIcon("Refactor") as { source: { light: string } };
    const svg = Buffer.from(icon.source.light.split(",")[1], "base64").toString("utf8");
    expect(svg).toContain(">REFA<");
    expect(svg).toContain(contextColor("Refactor").light);
    expect(svg).toContain('viewBox="0 0 40 40"');
  });

  it("should XML-escape the abbreviation", () => {
    const icon = contextBadgeIcon("R&D") as { source: { light: string } };
    const svg = Buffer.from(icon.source.light.split(",")[1], "base64").toString("utf8");
    expect(svg).toContain(">R&amp;D<");
  });

  it("should return the same cached object for the same context", () => {
    expect(contextBadgeIcon("asl")).toBe(contextBadgeIcon("asl"));
  });
});

describe("getAllContexts", () => {
  it("should return distinct normalized contexts, sorted", () => {
    const snippets = [
      createSnippet({ title: "Refactor: debt triage" }),
      createSnippet({ title: "asl: run" }),
      createSnippet({ title: "refactor: dead code" }),
      createSnippet({ title: "asl commit" }),
    ];
    expect(getAllContexts(snippets)).toEqual(["asl", "refactor"]);
  });

  it("should return an empty array when no snippet has a context", () => {
    expect(getAllContexts([createSnippet({ title: "plain title" })])).toEqual([]);
  });
});

describe("getSnippetContext", () => {
  it("should return the normalized context", () => {
    expect(getSnippetContext(createSnippet({ title: "Refactor: debt" }))).toBe("refactor");
  });

  it("should return null when there is no context", () => {
    expect(getSnippetContext(createSnippet({ title: "plain" }))).toBeNull();
  });
});
