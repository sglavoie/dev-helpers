/**
 * Tag utilities for handling tag normalization and hierarchy
 */

/**
 * Normalizes a tag to lowercase and trims whitespace
 */
export function normalizeTag(tag: string): string {
  return tag.trim().toLowerCase();
}

/**
 * Normalizes an array of tags
 */
export function normalizeTags(tags: string[]): string[] {
  return tags.map(normalizeTag).filter((t) => t.length > 0);
}

/**
 * Parses a tag path into segments
 * Example: "work/projects/client-a" -> ["work", "projects", "client-a"]
 */
export function parseTagPath(tag: string): string[] {
  return tag.split("/").filter((segment) => segment.length > 0);
}

/**
 * Gets the parent tag from a hierarchical tag
 * Example: "work/projects/client-a" -> "work/projects"
 * Returns null if tag has no parent
 */
export function getParentTag(tag: string): string | null {
  const segments = parseTagPath(tag);
  if (segments.length <= 1) return null;
  return segments.slice(0, -1).join("/");
}

/**
 * Gets all parent tags in the hierarchy
 * Example: "work/projects/client-a" -> ["work", "work/projects"]
 */
export function getAllParentTags(tag: string): string[] {
  const segments = parseTagPath(tag);
  const parents: string[] = [];

  for (let i = 1; i < segments.length; i++) {
    parents.push(segments.slice(0, i).join("/"));
  }

  return parents;
}

/**
 * Gets the depth of a tag in the hierarchy (0-indexed)
 * Example: "work" -> 0, "work/projects" -> 1
 */
export function getTagDepth(tag: string): number {
  return parseTagPath(tag).length - 1;
}

/**
 * Gets the last segment of a tag path (the tag name without parents)
 * Example: "work/projects/client-a" -> "client-a"
 */
export function getTagName(tag: string): string {
  const segments = parseTagPath(tag);
  return segments[segments.length - 1] || tag;
}

/**
 * Gets all child tags of a parent tag from a list of tags
 * Example: getChildTags(["work", "work/projects", "personal"], "work") -> ["work/projects"]
 */
export function getChildTags(tags: string[], parent: string): string[] {
  const normalizedParent = normalizeTag(parent);
  const parentPrefix = `${normalizedParent}/`;

  return tags.filter((tag) => {
    const normalizedTag = normalizeTag(tag);
    // Check if tag starts with parent/ and has exactly one more segment
    if (!normalizedTag.startsWith(parentPrefix)) return false;

    const remainder = normalizedTag.slice(parentPrefix.length);
    // Direct child only (no more slashes)
    return !remainder.includes("/");
  });
}

/**
 * Gets all descendant tags (children, grandchildren, etc.) of a parent tag
 */
export function getDescendantTags(tags: string[], parent: string): string[] {
  const normalizedParent = normalizeTag(parent);
  const parentPrefix = `${normalizedParent}/`;

  return tags.filter((tag) => {
    const normalizedTag = normalizeTag(tag);
    return normalizedTag.startsWith(parentPrefix);
  });
}

/**
 * Checks if a tag is a child of another tag (direct or indirect)
 */
export function isChildOf(tag: string, parent: string): boolean {
  const normalizedTag = normalizeTag(tag);
  const normalizedParent = normalizeTag(parent);

  if (normalizedTag === normalizedParent) return false;

  return normalizedTag.startsWith(`${normalizedParent}/`);
}

/**
 * Checks if a tag is a direct child of another tag
 */
export function isDirectChildOf(tag: string, parent: string): boolean {
  const normalizedTag = normalizeTag(tag);
  const normalizedParent = normalizeTag(parent);
  const parentPrefix = `${normalizedParent}/`;

  if (!normalizedTag.startsWith(parentPrefix)) return false;

  const remainder = normalizedTag.slice(parentPrefix.length);
  return !remainder.includes("/");
}

/**
 * Gets all root tags (tags with no parent) from a list of tags
 */
export function getRootTags(tags: string[]): string[] {
  return tags.filter((tag) => !tag.includes("/"));
}

/**
 * Builds a hierarchical tree structure from a flat list of tags
 * Returns root tags with their children nested
 */
export interface TagNode {
  tag: string;
  name: string;
  depth: number;
  children: TagNode[];
}

export function buildTagTree(tags: string[]): TagNode[] {
  const normalizedTags = [...new Set(normalizeTags(tags))].sort();
  const rootTags = getRootTags(normalizedTags);

  function buildNode(tag: string): TagNode {
    const children = getChildTags(normalizedTags, tag);

    return {
      tag,
      name: getTagName(tag),
      depth: getTagDepth(tag),
      children: children.map(buildNode),
    };
  }

  // Build tree from root tags
  const tree = rootTags.map(buildNode);

  // Collect all tags that are already in the tree
  const tagsInTree = new Set<string>();
  function collectTags(node: TagNode) {
    tagsInTree.add(node.tag);
    node.children.forEach(collectTags);
  }
  tree.forEach(collectTags);

  // Find orphaned nested tags (tags with "/" that aren't in the tree)
  const orphanedTags = normalizedTags.filter((tag) => tag.includes("/") && !tagsInTree.has(tag));

  // For orphaned tags, only add the shallowest ones (those without parents in the orphaned list)
  // This ensures we don't add both "work/projects/client-a" and "work/projects" as separate roots
  const orphanedRoots = orphanedTags.filter((tag) => {
    // Check if any parent of this tag is also in the orphaned list or already in tree
    const parents = getAllParentTags(tag);
    const hasOrphanedParent = parents.some((parent) => orphanedTags.includes(parent));
    const hasTreeParent = parents.some((parent) => tagsInTree.has(parent));
    return !hasOrphanedParent && !hasTreeParent;
  });

  // Add orphaned root tags as root-level nodes (they'll nest properly once parent exists)
  orphanedRoots.forEach((tag) => {
    tree.push(buildNode(tag));
  });

  return tree;
}

/**
 * Flattens a tag tree into a list with depth information
 * Useful for rendering hierarchical lists with indentation
 */
export interface FlatTagNode {
  tag: string;
  name: string;
  depth: number;
  hasChildren: boolean;
}

export function flattenTagTree(tree: TagNode[]): FlatTagNode[] {
  const result: FlatTagNode[] = [];

  function flatten(node: TagNode) {
    result.push({
      tag: node.tag,
      name: node.name,
      depth: node.depth,
      hasChildren: node.children.length > 0,
    });

    node.children.forEach(flatten);
  }

  tree.forEach(flatten);
  return result;
}

/**
 * Deduplicates tags case-insensitively, preserving the first occurrence's casing
 */
export function deduplicateTags(tags: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];

  for (const tag of tags) {
    const normalized = normalizeTag(tag);
    if (!seen.has(normalized)) {
      seen.add(normalized);
      result.push(normalized); // Always use lowercase version
    }
  }

  return result;
}

/**
 * Expands tags to include all parent tags in the hierarchy
 * Example: ["work/projects/client-a"] -> ["work", "work/projects", "work/projects/client-a"]
 * This ensures nested tags always have their parent tags present
 */
export function expandTagsWithParents(tags: string[]): string[] {
  const expandedTags = new Set<string>();

  // Normalize input tags first
  const normalizedTags = normalizeTags(tags);

  for (const tag of normalizedTags) {
    // Add the tag itself
    expandedTags.add(tag);

    // Add all parent tags
    const parents = getAllParentTags(tag);
    parents.forEach((parent) => expandedTags.add(parent));
  }

  // Return as sorted, deduplicated array
  return Array.from(expandedTags).sort((a, b) => a.localeCompare(b));
}

/**
 * Removes redundant parent tags when their children are present
 * Example: ["work", "work/projects"] -> ["work/projects"]
 * Example: ["work", "work/projects", "work/projects/client-a"] -> ["work/projects/client-a"]
 * This keeps only the deepest nested tags, as parent tags are implied by children
 */
export function removeRedundantParents(tags: string[]): string[] {
  // Normalize and deduplicate input first
  const normalizedTags = deduplicateTags(normalizeTags(tags));

  // Filter out tags that have children in the list
  const nonRedundantTags = normalizedTags.filter((tag) => {
    // Check if any other tag starts with this tag + "/"
    const hasChild = normalizedTags.some((otherTag) => otherTag !== tag && otherTag.startsWith(`${tag}/`));
    return !hasChild; // Keep tag only if it has no children
  });

  return nonRedundantTags.sort((a, b) => a.localeCompare(b));
}
