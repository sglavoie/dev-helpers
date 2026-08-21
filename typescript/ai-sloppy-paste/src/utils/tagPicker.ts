/**
 * Logic backing the unified tag picker: row states, filtering, and toggling.
 */

import { expandTagsWithParents, getAllParentTags, normalizeTag, removeRedundantParents } from "./tags";
import { validateTag } from "./validation";

/**
 * "implied" means the tag is not stored on the snippet but is covered by a
 * descendant that is (storage drops redundant parents).
 */
export type TagRowState = "selected" | "implied" | "unselected";

export interface TagRow {
  tag: string;
  state: TagRowState;
  impliedBy?: string;
}

/**
 * Builds the full row list: known and selected tags expanded with their parents,
 * deduplicated and sorted, each annotated with its selection state.
 */
export function buildTagRows(knownTags: string[], selectedTags: string[]): TagRow[] {
  const selected = removeRedundantParents(selectedTags);
  const selectedSet = new Set(selected);

  const impliedBy = new Map<string, string>();
  for (const tag of selected) {
    for (const parent of getAllParentTags(tag)) {
      if (!selectedSet.has(parent) && !impliedBy.has(parent)) {
        impliedBy.set(parent, tag);
      }
    }
  }

  return expandTagsWithParents([...knownTags, ...selected]).map((tag) => {
    if (selectedSet.has(tag)) {
      return { tag, state: "selected" };
    }

    const source = impliedBy.get(tag);
    if (source) {
      return { tag, state: "implied", impliedBy: source };
    }

    return { tag, state: "unselected" };
  });
}

/**
 * Case-insensitive substring match against the full tag path.
 */
export function filterTagRows(rows: TagRow[], searchText: string): TagRow[] {
  const query = searchText.trim().toLowerCase();
  if (query.length === 0) return rows;

  return rows.filter((row) => row.tag.toLowerCase().includes(query));
}

/**
 * Returns the tag the search text would create, a validation error to display,
 * or null when the text is blank or already matches a known tag.
 */
export function getCreateCandidate(
  searchText: string,
  knownTags: string[],
): { tag: string } | { error: string } | null {
  const trimmed = searchText.trim();
  if (trimmed.length === 0) return null;

  const validation = validateTag(trimmed);
  if (!validation.isValid) {
    return { error: validation.error ?? "Invalid tag name" };
  }

  const tag = validation.normalizedValue ?? normalizeTag(trimmed);
  if (knownTags.some((known) => normalizeTag(known) === tag)) return null;

  return { tag };
}

/**
 * Adds or removes a tag, mirroring the `removeRedundantParents` normalization
 * that storage applies. Toggling a tag that is only implied is a no-op and
 * reports the descendant responsible.
 */
export function toggleTag(
  selectedTags: string[],
  tag: string,
): { tags: string[]; changed: boolean; impliedBy?: string } {
  const normalized = normalizeTag(tag);
  const current = removeRedundantParents(selectedTags);

  if (current.includes(normalized)) {
    return { tags: removeRedundantParents(current.filter((t) => t !== normalized)), changed: true };
  }

  const descendant = current.find((t) => t.startsWith(`${normalized}/`));
  if (descendant) {
    return { tags: current, changed: false, impliedBy: descendant };
  }

  return { tags: removeRedundantParents([...current, normalized]), changed: true };
}
