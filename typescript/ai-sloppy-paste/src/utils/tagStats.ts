import { Snippet } from "../types";

export interface TagStatistics {
  tag: string;
  snippetCount: number;
  lastUsedAt?: number;
  totalUsageCount: number;
  neverUsed: boolean;
}

/**
 * Computes usage statistics for a list of tags based on snippet data
 */
export function computeTagStatistics(snippets: Snippet[], tags: string[]): TagStatistics[] {
  const stats: TagStatistics[] = [];

  for (const tag of tags) {
    // Find all snippets that have this tag
    const matchingSnippets = snippets.filter((snippet) => snippet.tags.includes(tag));

    // Compute statistics
    const snippetCount = matchingSnippets.length;
    const totalUsageCount = matchingSnippets.reduce((sum, snippet) => sum + (snippet.useCount || 0), 0);

    // Find the most recent lastUsedAt among matching snippets
    let lastUsedAt: number | undefined;
    for (const snippet of matchingSnippets) {
      if (snippet.lastUsedAt) {
        if (!lastUsedAt || snippet.lastUsedAt > lastUsedAt) {
          lastUsedAt = snippet.lastUsedAt;
        }
      }
    }

    // A tag is "never used" if no snippet with this tag has been used
    const neverUsed = !lastUsedAt;

    stats.push({
      tag,
      snippetCount,
      lastUsedAt,
      totalUsageCount,
      neverUsed,
    });
  }

  return stats;
}

/**
 * Formats a timestamp as a relative time string (e.g., "2h ago", "3d ago")
 */
export function formatRelativeTime(timestamp: number): string {
  const now = Date.now();
  const diff = now - timestamp;

  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const weeks = Math.floor(days / 7);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  if (seconds < 60) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 7) return `${days}d ago`;
  if (weeks < 4) return `${weeks}w ago`;
  if (months < 12) return `${months}mo ago`;
  return `${years}y ago`;
}

/**
 * Formats a number with commas for readability (e.g., 1234 -> "1,234")
 */
export function formatNumber(num: number): string {
  return num.toLocaleString();
}

/**
 * Sort options for tag statistics
 */
export enum TagSortOption {
  NameAsc = "name-asc",
  SnippetCountDesc = "snippet-count-desc",
  LastUsedDesc = "last-used-desc",
  UsageCountDesc = "usage-count-desc",
  NeverUsedFirst = "never-used-first",
}

export const TAG_SORT_LABELS: Record<TagSortOption, string> = {
  [TagSortOption.NameAsc]: "Name (A-Z)",
  [TagSortOption.SnippetCountDesc]: "Most Snippets",
  [TagSortOption.LastUsedDesc]: "Recently Used",
  [TagSortOption.UsageCountDesc]: "Most Used",
  [TagSortOption.NeverUsedFirst]: "Never Used First",
};

/**
 * Sorts tag statistics based on the selected sort option
 */
export function sortTagStatistics(stats: TagStatistics[], sortOption: TagSortOption): TagStatistics[] {
  const sorted = [...stats];

  switch (sortOption) {
    case TagSortOption.NameAsc:
      return sorted.sort((a, b) => a.tag.localeCompare(b.tag));

    case TagSortOption.SnippetCountDesc:
      return sorted.sort((a, b) => {
        if (b.snippetCount !== a.snippetCount) {
          return b.snippetCount - a.snippetCount;
        }
        // Tie-breaker: name
        return a.tag.localeCompare(b.tag);
      });

    case TagSortOption.LastUsedDesc:
      return sorted.sort((a, b) => {
        // Put never-used tags at the end
        if (a.neverUsed && !b.neverUsed) return 1;
        if (!a.neverUsed && b.neverUsed) return -1;
        if (a.neverUsed && b.neverUsed) return a.tag.localeCompare(b.tag);

        // Sort by most recent first
        const aTime = a.lastUsedAt || 0;
        const bTime = b.lastUsedAt || 0;
        if (bTime !== aTime) {
          return bTime - aTime;
        }
        // Tie-breaker: name
        return a.tag.localeCompare(b.tag);
      });

    case TagSortOption.UsageCountDesc:
      return sorted.sort((a, b) => {
        if (b.totalUsageCount !== a.totalUsageCount) {
          return b.totalUsageCount - a.totalUsageCount;
        }
        // Tie-breaker: name
        return a.tag.localeCompare(b.tag);
      });

    case TagSortOption.NeverUsedFirst:
      return sorted.sort((a, b) => {
        // Never used first
        if (a.neverUsed && !b.neverUsed) return -1;
        if (!a.neverUsed && b.neverUsed) return 1;

        // Within same category, sort by name
        return a.tag.localeCompare(b.tag);
      });

    default:
      return sorted;
  }
}
