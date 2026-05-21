import { Snippet } from "../types";

export function createSnippet(overrides: Partial<Snippet> = {}): Snippet {
  return {
    id: "test-id",
    title: "Test Snippet",
    content: "Test content",
    tags: [],
    createdAt: Date.now() - 60 * 24 * 60 * 60 * 1000,
    updatedAt: Date.now(),
    lastUsedAt: Date.now() - 24 * 60 * 60 * 1000,
    useCount: 5,
    isFavorite: false,
    isArchived: false,
    isPinned: false,
    ...overrides,
  };
}
