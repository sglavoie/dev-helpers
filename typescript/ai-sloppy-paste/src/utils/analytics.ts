import { Snippet, TimeRange, SnippetAnalytics, CleanupSuggestion, AnalyticsSummary } from "../types";

const MS_PER_DAY = 24 * 60 * 60 * 1000;
const STALE_THRESHOLD_DAYS = 90;
const NEVER_USED_AGE_THRESHOLD_DAYS = 30;
const ACTIVE_THRESHOLD_DAYS = 30;

/**
 * Computes analytics for a single snippet
 * Pinned snippets are never marked as stale since they're intentionally kept visible
 */
export function computeSnippetAnalytics(snippet: Snippet): SnippetAnalytics {
  const now = Date.now();
  const daysUnused = snippet.lastUsedAt ? Math.floor((now - snippet.lastUsedAt) / MS_PER_DAY) : undefined;
  const daysSinceCreation = Math.floor((now - snippet.createdAt) / MS_PER_DAY);

  let isStale = false;
  let stalenessReason: string | undefined;

  // Pinned snippets are intentionally kept visible and should not be flagged as stale
  if (!snippet.isPinned) {
    if (snippet.useCount === 0 && daysSinceCreation >= NEVER_USED_AGE_THRESHOLD_DAYS) {
      isStale = true;
      stalenessReason = `Never used (created ${daysSinceCreation} days ago)`;
    } else if (daysUnused !== undefined && daysUnused >= STALE_THRESHOLD_DAYS) {
      isStale = true;
      stalenessReason = `Not used in ${daysUnused} days`;
    }
  }

  return {
    snippet,
    daysUnused,
    isStale,
    stalenessReason,
  };
}

/**
 * Computes aggregate analytics summary for all snippets
 */
export function computeAnalyticsSummary(snippets: Snippet[], tags: string[]): AnalyticsSummary {
  const now = Date.now();
  const activeThreshold = now - ACTIVE_THRESHOLD_DAYS * MS_PER_DAY;

  let activeSnippets = 0;
  let staleSnippets = 0;
  let archivedSnippets = 0;
  let totalUsageCount = 0;

  const usedTags = new Set<string>();

  for (const snippet of snippets) {
    totalUsageCount += snippet.useCount;

    if (snippet.isArchived) {
      archivedSnippets++;
      continue;
    }

    snippet.tags.forEach((tag) => usedTags.add(tag));

    const analytics = computeSnippetAnalytics(snippet);
    if (analytics.isStale) {
      staleSnippets++;
    }

    if (snippet.lastUsedAt && snippet.lastUsedAt >= activeThreshold) {
      activeSnippets++;
    }
  }

  const nonArchivedCount = snippets.length - archivedSnippets;
  const unusedTagCount = tags.filter((tag) => !usedTags.has(tag)).length;

  return {
    totalSnippets: snippets.length,
    activeSnippets,
    staleSnippets,
    archivedSnippets,
    totalUsageCount,
    averageUsagePerSnippet: nonArchivedCount > 0 ? Math.round(totalUsageCount / nonArchivedCount) : 0,
    totalTags: tags.length,
    unusedTags: unusedTagCount,
  };
}

/**
 * Returns the start timestamp for a given time range
 */
function getTimeRangeStart(timeRange: TimeRange): number {
  const now = Date.now();

  switch (timeRange) {
    case TimeRange.ThisWeek:
      return now - 7 * MS_PER_DAY;
    case TimeRange.ThisMonth:
      return now - 30 * MS_PER_DAY;
    case TimeRange.Last3Months:
      return now - 90 * MS_PER_DAY;
    case TimeRange.AllTime:
      return 0;
  }
}

/**
 * Returns top snippets by usage, filtered by time range
 */
export function getTopSnippets(snippets: Snippet[], timeRange: TimeRange, limit: number = 10): Snippet[] {
  const rangeStart = getTimeRangeStart(timeRange);

  const filtered = snippets.filter((snippet) => {
    if (snippet.isArchived) return false;
    if (timeRange === TimeRange.AllTime) return snippet.useCount > 0;
    return snippet.lastUsedAt !== undefined && snippet.lastUsedAt >= rangeStart;
  });

  return filtered.sort((a, b) => b.useCount - a.useCount).slice(0, limit);
}

/**
 * Returns tags that have no snippets associated with them
 */
export function getUnusedTags(snippets: Snippet[], tags: string[]): string[] {
  const usedTags = new Set<string>();
  for (const snippet of snippets) {
    if (!snippet.isArchived) {
      snippet.tags.forEach((tag) => usedTags.add(tag));
    }
  }
  return tags.filter((tag) => !usedTags.has(tag));
}

/**
 * Generates cleanup suggestions for stale snippets and unused tags
 * Pinned and archived snippets are excluded from suggestions
 */
export function computeCleanupSuggestions(snippets: Snippet[], tags: string[]): CleanupSuggestion[] {
  const suggestions: CleanupSuggestion[] = [];

  for (const snippet of snippets) {
    if (snippet.isArchived || snippet.isPinned) continue;

    const analytics = computeSnippetAnalytics(snippet);

    if (snippet.useCount === 0) {
      const daysSinceCreation = Math.floor((Date.now() - snippet.createdAt) / MS_PER_DAY);
      if (daysSinceCreation >= NEVER_USED_AGE_THRESHOLD_DAYS) {
        suggestions.push({
          id: `never-used-${snippet.id}`,
          type: "never_used",
          snippet,
          reason: `Never used (created ${daysSinceCreation} days ago)`,
          suggestedAction: "Archive or delete this snippet",
        });
      }
    } else if (analytics.isStale && analytics.daysUnused !== undefined) {
      suggestions.push({
        id: `stale-${snippet.id}`,
        type: "stale",
        snippet,
        reason: `Not used in ${analytics.daysUnused} days`,
        suggestedAction: "Review and archive if no longer needed",
      });
    }
  }

  const unusedTags = getUnusedTags(snippets, tags);
  for (const tag of unusedTags) {
    suggestions.push({
      id: `unused-tag-${tag}`,
      type: "unused_tag",
      tag,
      reason: "No snippets use this tag",
      suggestedAction: "Delete this tag",
    });
  }

  return suggestions;
}
