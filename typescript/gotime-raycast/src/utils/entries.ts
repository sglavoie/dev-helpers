import { useEffect, useMemo, useState } from "react";

/** Absolute path of the gotime CLI the commands shell out to. */
export const GT_BIN = "/Users/sglavoie/.local/bin/gt";

export interface Entry {
  id: string;
  short_id: number;
  keyword: string;
  tags: string[];
  start_time: string;
  end_time: string | null;
  duration: number;
  active: boolean;
  stashed: boolean;
}

/**
 * Reads the JSON output of `gt list`, sorting active entries first and the
 * rest by start time descending.
 */
export function parseEntries(stdout: string): Entry[] {
  const trimmed = stdout.trim();
  if (!trimmed) {
    return [];
  }

  const entries = JSON.parse(trimmed) as Entry[];
  return entries.sort((a, b) => {
    if (a.active === b.active) {
      return (
        new Date(b.start_time).getTime() - new Date(a.start_time).getTime()
      );
    }
    return a.active ? -1 : 1;
  });
}

export function formatRelativeTime(dateString: string | null): string {
  if (!dateString) return "Active";

  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays === 1) return "Yesterday";
  return `${diffDays}d ago`;
}

export function getCurrentDuration(entry: Entry): number {
  if (!entry.active || !entry.start_time) {
    return entry.duration;
  }

  const startTime = new Date(entry.start_time);
  const now = new Date();
  const elapsed = Math.floor((now.getTime() - startTime.getTime()) / 1000);

  return elapsed;
}

/** Extracts the unique keywords and tags used by the entries, for autocomplete. */
export function useKeywordsAndTags(entries: Entry[] | undefined): {
  keywords: string[];
  tags: string[];
} {
  return useMemo(() => {
    if (!entries || !Array.isArray(entries) || entries.length === 0) {
      return { keywords: [], tags: [] };
    }

    const keywordSet = new Set<string>();
    const tagSet = new Set<string>();

    entries.forEach((entry) => {
      keywordSet.add(entry.keyword);
      // Defensive check: handle null/undefined tags during state transitions
      (entry.tags ?? []).forEach((tag) => tagSet.add(tag));
    });

    return {
      keywords: Array.from(keywordSet).sort(),
      tags: Array.from(tagSet).sort(),
    };
  }, [entries]);
}

/** Recomputes the elapsed time of every active entry once a second. */
export function useLiveDurations(
  entries: Entry[] | undefined,
): Map<string, number> {
  const [currentDurations, setCurrentDurations] = useState<Map<string, number>>(
    new Map(),
  );

  useEffect(() => {
    if (!entries || !Array.isArray(entries) || entries.length === 0) return;

    const interval = setInterval(() => {
      const newDurations = new Map<string, number>();
      entries.forEach((entry) => {
        if (entry.active) {
          newDurations.set(entry.id, getCurrentDuration(entry));
        }
      });
      setCurrentDurations(newDurations);
    }, 1000);

    return () => clearInterval(interval);
  }, [entries]);

  return currentDurations;
}
