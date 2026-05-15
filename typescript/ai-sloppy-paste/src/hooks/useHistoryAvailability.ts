import { useState, useEffect } from "react";
import { Snippet } from "../types";
import { extractPlaceholders, getSystemPlaceholderNames, processSystemPlaceholders } from "../utils/placeholders";
import { getPlaceholderHistoryForKey } from "../utils/storage";
import { getLastUsedValue } from "../utils/placeholderHistory";

export function useHistoryAvailability(snippets: Snippet[]): Set<string> {
  const [available, setAvailable] = useState<Set<string>>(new Set());

  useEffect(() => {
    let cancelled = false;

    async function compute() {
      const systemKeys = new Set(getSystemPlaceholderNames());
      const allKeys = new Set<string>();
      const snippetRequirements = new Map<string, string[]>();

      for (const snippet of snippets) {
        const processed = processSystemPlaceholders(snippet.content);
        const required = extractPlaceholders(processed).filter(
          (p) => p.isRequired && !systemKeys.has(p.key),
        );
        if (required.length === 0) continue;
        const keys = required.map((p) => p.key);
        snippetRequirements.set(snippet.id, keys);
        keys.forEach((k) => allKeys.add(k));
      }

      const historyMap = new Map<string, boolean>();
      await Promise.all(
        [...allKeys].map(async (key) => {
          const history = await getPlaceholderHistoryForKey(key);
          historyMap.set(key, !!getLastUsedValue(history));
        }),
      );

      const result = new Set<string>();
      for (const [snippetId, keys] of snippetRequirements) {
        if (keys.every((k) => historyMap.get(k))) result.add(snippetId);
      }

      if (!cancelled) setAvailable(result);
    }

    compute();
    return () => {
      cancelled = true;
    };
  }, [snippets]);

  return available;
}
