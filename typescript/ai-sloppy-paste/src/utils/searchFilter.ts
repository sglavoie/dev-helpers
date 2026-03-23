import type { Snippet } from "../types";
import type { ParsedQuery } from "./queryParser";
import { isChildOf } from "./tags";

/**
 * Applies search filters from a parsed query to an array of snippets.
 *
 * All filters use AND logic - a snippet must pass ALL conditions to be included.
 *
 * Filter order:
 * 1. Tag filters (hierarchical matching via isChildOf)
 * 2. Negative tag filters
 * 3. Boolean "is:" filters (favorite, archived, untagged)
 * 4. Boolean "not:" filters
 * 5. Exact phrase filters (case-insensitive substring in title or content)
 * 6. Fuzzy text filters (all words must match somewhere in title/content/tags)
 *
 * @param snippets - Array of snippets to filter
 * @param query - Parsed query object with extracted operators
 * @returns Filtered array of snippets matching all conditions
 */
export function applySearchFilters(snippets: Snippet[], query: ParsedQuery): Snippet[] {
  return snippets.filter((snippet) => {
    // 1. Tag filters - snippet must have ALL required tags
    // Supports hierarchical matching: tag:work matches work/projects
    for (const requiredTag of query.tags) {
      const hasTag = snippet.tags.some(
        (snippetTag) => snippetTag === requiredTag || isChildOf(snippetTag, requiredTag),
      );
      if (!hasTag) return false;
    }

    // 2. Negative tag filters - snippet must NOT have any excluded tags
    for (const excludedTag of query.notTags) {
      const hasTag = snippet.tags.some(
        (snippetTag) => snippetTag === excludedTag || isChildOf(snippetTag, excludedTag),
      );
      if (hasTag) return false;
    }

    // 3. Boolean "is:" filters - check snippet properties
    for (const condition of query.is) {
      if (condition === "favorite" && !snippet.isFavorite) return false;
      if (condition === "archived" && !snippet.isArchived) return false;
      if (condition === "untagged" && snippet.tags.length > 0) return false;
    }

    // 4. Boolean "not:" filters - check negated properties
    for (const condition of query.not) {
      if (condition === "favorite" && snippet.isFavorite) return false;
      if (condition === "archived" && snippet.isArchived) return false;
      if (condition === "untagged" && snippet.tags.length === 0) return false;
    }

    // 5. Exact phrase filters - ALL phrases must appear in title or content
    // Case-insensitive substring matching
    for (const phrase of query.exactPhrases) {
      const lowerPhrase = phrase.toLowerCase();
      const inTitle = snippet.title.toLowerCase().includes(lowerPhrase);
      const inContent = snippet.content.toLowerCase().includes(lowerPhrase);
      if (!inTitle && !inContent) return false;
    }

    // 6. Fuzzy text - ALL words must appear somewhere in title/content/tags
    // Case-insensitive partial matching
    if (query.fuzzyText) {
      const words = query.fuzzyText
        .toLowerCase()
        .split(/\s+/)
        .filter((w) => w);
      for (const word of words) {
        const inTitle = snippet.title.toLowerCase().includes(word);
        const inContent = snippet.content.toLowerCase().includes(word);
        const inTags = snippet.tags.some((tag) => tag.includes(word));
        if (!inTitle && !inContent && !inTags) return false;
      }
    }

    // All filters passed
    return true;
  });
}

/**
 * Helper to match a snippet against fuzzy search text (no operators).
 * This is a simplified version of applySearchFilters for backward compatibility.
 *
 * @param snippet - Snippet to check
 * @param searchText - Fuzzy search text (all words must match)
 * @returns true if snippet matches search text
 */
export function matchesFuzzySearch(snippet: Snippet, searchText: string): boolean {
  if (!searchText.trim()) return true;

  const words = searchText
    .toLowerCase()
    .split(/\s+/)
    .filter((w) => w);
  for (const word of words) {
    const inTitle = snippet.title.toLowerCase().includes(word);
    const inContent = snippet.content.toLowerCase().includes(word);
    const inTags = snippet.tags.some((tag) => tag.includes(word));
    if (!inTitle && !inContent && !inTags) return false;
  }

  return true;
}
