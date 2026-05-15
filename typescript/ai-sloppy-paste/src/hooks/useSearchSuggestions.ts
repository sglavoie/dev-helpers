import { useMemo } from "react";

export interface SearchSuggestion {
  title: string;
  subtitle: string;
  completion: string;
}

const IS_SUGGESTIONS: Array<{ value: string; subtitle: string }> = [
  { value: "favorite", subtitle: "Show only bookmarked snippets" },
  { value: "bookmarked", subtitle: "Show only bookmarked snippets" },
  { value: "archived", subtitle: "Show only archived snippets" },
  { value: "untagged", subtitle: "Show only untagged snippets" },
];

const NOT_SUGGESTIONS: Array<{ value: string; subtitle: string }> = [
  { value: "archived", subtitle: "Exclude archived snippets" },
  { value: "favorite", subtitle: "Exclude bookmarked snippets" },
  { value: "untagged", subtitle: "Exclude untagged snippets" },
];

export function useSearchSuggestions(searchQuery: string, allTags: string[]): SearchSuggestion[] {
  return useMemo(() => {
    const trimmed = searchQuery.trimEnd();

    if (trimmed.endsWith("tag:")) {
      const prefix = trimmed.slice(0, trimmed.length - "tag:".length);
      return allTags.slice(0, 4).map((tag) => ({
        title: `tag:${tag}`,
        subtitle: `Filter to snippets tagged ${tag}`,
        completion: `${prefix}tag:${tag} `,
      }));
    }

    if (trimmed === "is:" || trimmed.endsWith(" is:")) {
      const prefix = trimmed.slice(0, trimmed.length - "is:".length);
      return IS_SUGGESTIONS.slice(0, 4).map(({ value, subtitle }) => ({
        title: `is:${value}`,
        subtitle,
        completion: `${prefix}is:${value} `,
      }));
    }

    if (trimmed === "not:" || trimmed.endsWith(" not:")) {
      const prefix = trimmed.slice(0, trimmed.length - "not:".length);
      const items: SearchSuggestion[] = NOT_SUGGESTIONS.map(({ value, subtitle }) => ({
        title: `not:${value}`,
        subtitle,
        completion: `${prefix}not:${value} `,
      }));
      if (allTags.length > 0) {
        items.push({
          title: `not:tag:${allTags[0]}`,
          subtitle: `Exclude snippets tagged ${allTags[0]}`,
          completion: `${prefix}not:tag:${allTags[0]} `,
        });
      }
      return items.slice(0, 4);
    }

    return [];
  }, [searchQuery, allTags]);
}
