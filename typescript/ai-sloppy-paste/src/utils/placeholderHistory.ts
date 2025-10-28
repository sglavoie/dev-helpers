import { PlaceholderHistoryValue } from "../types";

/**
 * Smart ranking algorithm that combines recency and frequency
 *
 * The score is calculated using:
 * - Frequency weight: How many times the value has been used
 * - Recency weight: How recently the value was used
 *
 * Formula: score = (frequency_factor * useCount) + (recency_factor * days_ago_inverse)
 */
export function rankPlaceholderValues(values: PlaceholderHistoryValue[]): PlaceholderHistoryValue[] {
  if (values.length === 0) return [];

  const now = Date.now();
  const ONE_DAY_MS = 24 * 60 * 60 * 1000;

  // Weights for the ranking algorithm
  const FREQUENCY_WEIGHT = 1.0;
  const RECENCY_WEIGHT = 2.0;

  // Calculate scores for each value
  const scored = values.map((value) => {
    // Calculate frequency score (normalized by max use count)
    const frequencyScore = value.useCount;

    // Calculate recency score (inverse of days ago)
    const daysAgo = Math.max(1, (now - value.lastUsed) / ONE_DAY_MS);
    const recencyScore = 1 / daysAgo;

    // Combined score
    const score = FREQUENCY_WEIGHT * frequencyScore + RECENCY_WEIGHT * recencyScore * 100;

    return { value, score };
  });

  // Sort by score (descending) and return sorted values
  return scored.sort((a, b) => b.score - a.score).map((item) => item.value);
}

/**
 * Get ranked placeholder values for a specific key (for autocomplete)
 */
export function getRankedValuesForAutocomplete(values: PlaceholderHistoryValue[], limit?: number): string[] {
  const ranked = rankPlaceholderValues(values);
  const result = ranked.map((v) => v.value);
  return limit ? result.slice(0, limit) : result;
}

/**
 * Filter values by search query (case-insensitive substring match)
 */
export function filterValuesByQuery(values: string[], query: string): string[] {
  if (!query.trim()) return values;

  const lowerQuery = query.toLowerCase();
  return values.filter((value) => value.toLowerCase().includes(lowerQuery));
}

/**
 * Get the top-ranked value (for pre-filling form fields)
 */
export function getTopRankedValue(values: PlaceholderHistoryValue[]): string | undefined {
  if (values.length === 0) return undefined;

  const ranked = rankPlaceholderValues(values);
  return ranked[0]?.value;
}

/**
 * Calculate statistics for a placeholder key
 */
export interface PlaceholderKeyStats {
  key: string;
  valueCount: number;
  totalUseCount: number;
  lastUsed: number | undefined;
}

export function calculateKeyStats(key: string, values: PlaceholderHistoryValue[]): PlaceholderKeyStats {
  const totalUseCount = values.reduce((sum, v) => sum + v.useCount, 0);
  const lastUsed = values.length > 0 ? Math.max(...values.map((v) => v.lastUsed)) : undefined;

  return {
    key,
    valueCount: values.length,
    totalUseCount,
    lastUsed,
  };
}

/**
 * Format timestamp to relative time string
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
 * Format absolute date
 */
export function formatAbsoluteDate(timestamp: number): string {
  const date = new Date(timestamp);
  return date.toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
