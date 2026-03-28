import { useMemo, useState } from "react";
import { Snippet, SortOption } from "../types";
import { computeSnippetAnalytics } from "../utils/analytics";
import { parseSearchQuery } from "../utils/queryParser";
import { applySearchFilters } from "../utils/searchFilter";
import { expandTagsWithParents, isChildOf } from "../utils/tags";

export function useSnippetFiltering(snippets: Snippet[], sortOption: SortOption, showRecentSection: boolean) {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedTag, setSelectedTag] = useState<string>("All");
  const [showOnlyFavorites, setShowOnlyFavorites] = useState(false);
  const [showArchivedSnippets, setShowArchivedSnippets] = useState(false);
  const [showNeedsAttention, setShowNeedsAttention] = useState(false);

  const parsedQuery = useMemo(() => parseSearchQuery(searchQuery), [searchQuery]);

  const filtered = useMemo(() => {
    const baseSnippets = showArchivedSnippets
      ? snippets.filter((s) => s.isArchived)
      : snippets.filter((s) => !s.isArchived);

    let result: Snippet[];

    if (parsedQuery.hasOperators) {
      result = applySearchFilters(baseSnippets, parsedQuery);
    } else {
      const tagFiltered =
        selectedTag === "All"
          ? baseSnippets
          : baseSnippets.filter((s) => s.tags.some((tag) => tag === selectedTag || isChildOf(tag, selectedTag)));

      result = showOnlyFavorites ? tagFiltered.filter((s) => s.isFavorite) : tagFiltered;
    }

    if (showNeedsAttention) {
      return result.filter((s) => computeSnippetAnalytics(s).isStale);
    }

    return result;
  }, [snippets, parsedQuery, showArchivedSnippets, selectedTag, showOnlyFavorites, showNeedsAttention]);

  const visibleTags = useMemo(() => {
    const tagSet = new Set<string>();
    filtered.forEach((snippet) => {
      snippet.tags.forEach((tag) => tagSet.add(tag));
    });
    return expandTagsWithParents(Array.from(tagSet));
  }, [filtered]);

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
          return b.updatedAt - a.updatedAt;
        case SortOption.MostUsedDesc:
          return (b.useCount || 0) - (a.useCount || 0);
        case SortOption.MostUsedAsc:
          return (a.useCount || 0) - (b.useCount || 0);
        case SortOption.Alphabetical:
          return a.title.localeCompare(b.title);
        case SortOption.LastUsed:
          if (!a.lastUsedAt && !b.lastUsedAt) return b.updatedAt - a.updatedAt;
          if (!a.lastUsedAt) return 1;
          if (!b.lastUsedAt) return -1;
          return b.lastUsedAt - a.lastUsedAt;
        case SortOption.CreatedDesc:
          return b.createdAt - a.createdAt;
        default:
          return b.updatedAt - a.updatedAt;
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
    pinnedSnippets,
    recentSnippets,
    sortedSnippets,
  };
}
