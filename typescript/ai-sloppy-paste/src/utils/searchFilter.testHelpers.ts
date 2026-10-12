import type { Snippet } from "../types";
import type { ParsedQuery } from "./queryParser";

// Helper to create test snippets
export function createSnippet(overrides: Partial<Snippet>): Snippet {
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

// Helper to create parsed queries
export function createQuery(overrides: Partial<ParsedQuery>): ParsedQuery {
  return {
    tags: [],
    notTags: [],
    contexts: [],
    notContexts: [],
    is: [],
    not: [],
    exactPhrases: [],
    fuzzyText: "",
    hasOperators: false,
    hasStructuredOperators: false,
    ...overrides,
  };
}
