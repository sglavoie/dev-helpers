import { useMemo, useState } from "react";
import { Snippet, SortOption } from "../types";
import { computeSnippetAnalytics } from "../utils/analytics";
import { ParsedQuery, parseSearchQuery } from "../utils/queryParser";
import { applySearchFilters } from "../utils/searchFilter";
import { expandTagsWithParents, isChildOf } from "../utils/tags";

export interface FilterOptions {
  selectedTag: string;
  showOnlyFavorites: boolean;
  showArchivedSnippets: boolean;
  showNeedsAttention: boolean;
}

/**
 * Pure filtering pipeline shared by the hook and unit tests.
 *
 * Tag-dropdown and favorites filters are applied BEFORE search operators
 * so dropdowns/toggles don't silently stop working once the user types
 * a query that contains operators (e.g. `tag:foo`).
 */
export function filterSnippets(snippets: Snippet[], parsedQuery: ParsedQuery, options: FilterOptions): Snippet[] {
  const { selectedTag, showOnlyFavorites, showArchivedSnippets, showNeedsAttention } = options;

  const baseSnippets = showArchivedSnippets
    ? snippets.filter((s) => s.isArchived)
    : snippets.filter((s) => !s.isArchived);

  const tagFiltered =
    selectedTag === "All"
      ? baseSnippets
      : baseSnippets.filter((s) => s.tags.some((tag) => tag === selectedTag || isChildOf(tag, selectedTag)));
  const favoriteFiltered = showOnlyFavorites ? tagFiltered.filter((s) => s.isFavorite) : tagFiltered;

  const result = parsedQuery.hasOperators ? applySearchFilters(favoriteFiltered, parsedQuery) : favoriteFiltered;

  if (showNeedsAttention) {
    return result.filter((s) => computeSnippetAnalytics(s).isStale);
  }

  return result;
}

export function useSnippetFiltering(snippets: Snippet[], sortOption: SortOption, showRecentSection: boolean) {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedTag, setSelectedTag] = useState<string>("All");
  const [showOnlyFavorites, setShowOnlyFavorites] = useState(false);
  const [showArchivedSnippets, setShowArchivedSnippets] = useState(false);
  const [showNeedsAttention, setShowNeedsAttention] = useState(false);

  const parsedQuery = useMemo(() => parseSearchQuery(searchQuery), [searchQuery]);

  const filtered = useMemo(
    () =>
      filterSnippets(snippets, parsedQuery, {
        selectedTag,
        showOnlyFavorites,
        showArchivedSnippets,
        showNeedsAttention,
      }),
    [snippets, parsedQuery, showArchivedSnippets, selectedTag, showOnlyFavorites, showNeedsAttention],
  );

  const visibleTags = useMemo(() => {
    const tagSet = new Set<string>();
    filtered.forEach((snippet) => {
      snippet.tags.forEach((tag) => tagSet.add(tag));
    });
    return expandTagsWithParents(Array.from(tagSet));
  }, [filtered]);

  const allTags = useMemo(() => {
    const tagSet = new Set<string>();
    snippets.forEach((snippet) => {
      snippet.tags.forEach((tag) => tagSet.add(tag));
    });
    return expandTagsWithParents(Array.from(tagSet));
  }, [snippets]);

  const pinnedSnippets = useMemo(
    () => [...filtered].filter((s) => s.isPinned).sort((a, b) => a.title.localeCompare(b.title)),
    [filtered],
  );

  const pinnedIds = useMemo(() => new Set(pinnedSnippets.map((s) => s.id)), [pinnedSnippets]);

  const recentSnippets = useMemo(
    () =>
      showRecentSection
        ? [...filtered]
            .filter((s) => s.lastUsedAt && !pinnedIds.has(s.id))
            .sort((a, b) => (b.lastUsedAt || 0) - (a.lastUsedAt || 0))
            .slice(0, 5)
        : [],
    [filtered, pinnedIds, showRecentSection],
  );

  const sortedSnippets = useMemo(() => {
    const recentIds = new Set(recentSnippets.map((s) => s.id));
    const remaining = filtered.filter((s) => !pinnedIds.has(s.id) && (!showRecentSection || !recentIds.has(s.id)));

    return [...remaining].sort((a, b) => {
      switch (sortOption) {
        case SortOption.UpdatedDesc:
          return Math.max(b.updatedAt, b.lastUsedAt ?? 0) - Math.max(a.updatedAt, a.lastUsedAt ?? 0);
        case SortOption.MostUsedDesc:
          return (b.useCount || 0) - (a.useCount || 0);
        case SortOption.Alphabetical:
          return a.title.localeCompare(b.title);
        case SortOption.CreatedDesc:
          return b.createdAt - a.createdAt;
        default:
          return Math.max(b.updatedAt, b.lastUsedAt ?? 0) - Math.max(a.updatedAt, a.lastUsedAt ?? 0);
      }
    });
  }, [filtered, pinnedIds, recentSnippets, showRecentSection, sortOption]);

  return {
    searchQuery,
    setSearchQuery,
    selectedTag,
    setSelectedTag,
    showOnlyFavorites,
    setShowOnlyFavorites,
    showArchivedSnippets,
    setShowArchivedSnippets,
    showNeedsAttention,
    setShowNeedsAttention,
    filtered,
    visibleTags,
    allTags,
    pinnedSnippets,
    recentSnippets,
    sortedSnippets,
    hasStructuredOperators: parsedQuery.hasStructuredOperators,
  };
}
