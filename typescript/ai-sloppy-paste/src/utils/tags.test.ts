import { describe, it, expect } from "vitest";
import {
  normalizeTag,
  normalizeTags,
  parseTagPath,
  getParentTag,
  getAllParentTags,
  getTagDepth,
  getTagName,
  getChildTags,
  getDescendantTags,
  isChildOf,
  isDirectChildOf,
  getRootTags,
  buildTagTree,
  flattenTagTree,
  deduplicateTags,
  expandTagsWithParents,
  removeRedundantParents,
} from "./tags";

describe("normalizeTag", () => {
  it("should convert tags to lowercase", () => {
    expect(normalizeTag("Work")).toBe("work");
    expect(normalizeTag("PROJECTS")).toBe("projects");
  });

  it("should trim whitespace", () => {
    expect(normalizeTag("  work  ")).toBe("work");
    expect(normalizeTag("\tprojects\n")).toBe("projects");
  });

  it("should handle mixed case and whitespace", () => {
    expect(normalizeTag("  Work/Projects  ")).toBe("work/projects");
  });
});

describe("normalizeTags", () => {
  it("should normalize array of tags", () => {
    expect(normalizeTags(["Work", "PROJECTS", "  personal  "])).toEqual(["work", "projects", "personal"]);
  });

  it("should filter out empty tags after trimming", () => {
    expect(normalizeTags(["work", "  ", "", "projects"])).toEqual(["work", "projects"]);
  });
});

describe("parseTagPath", () => {
  it("should split tag path into segments", () => {
    expect(parseTagPath("work/projects/client-a")).toEqual(["work", "projects", "client-a"]);
    expect(parseTagPath("work")).toEqual(["work"]);
  });

  it("should filter out empty segments", () => {
    expect(parseTagPath("work//projects")).toEqual(["work", "projects"]);
  });
});

describe("getParentTag", () => {
  it("should return parent tag", () => {
    expect(getParentTag("work/projects/client-a")).toBe("work/projects");
    expect(getParentTag("work/projects")).toBe("work");
  });

  it("should return null for root tags", () => {
    expect(getParentTag("work")).toBeNull();
  });
});

describe("getAllParentTags", () => {
  it("should return all parent tags in hierarchy", () => {
    expect(getAllParentTags("work/projects/client-a")).toEqual(["work", "work/projects"]);
    expect(getAllParentTags("work/projects")).toEqual(["work"]);
  });

  it("should return empty array for root tags", () => {
    expect(getAllParentTags("work")).toEqual([]);
  });
});

describe("getTagDepth", () => {
  it("should return correct depth", () => {
    expect(getTagDepth("work")).toBe(0);
    expect(getTagDepth("work/projects")).toBe(1);
    expect(getTagDepth("work/projects/client-a")).toBe(2);
  });
});

describe("getTagName", () => {
  it("should return last segment of tag path", () => {
    expect(getTagName("work/projects/client-a")).toBe("client-a");
    expect(getTagName("work/projects")).toBe("projects");
    expect(getTagName("work")).toBe("work");
  });
});

describe("getChildTags", () => {
  const tags = ["work", "work/projects", "work/personal", "work/projects/client-a", "personal"];

  it("should return direct children only", () => {
    expect(getChildTags(tags, "work")).toEqual(["work/projects", "work/personal"]);
  });

  it("should not return grandchildren", () => {
    const children = getChildTags(tags, "work");
    expect(children).not.toContain("work/projects/client-a");
  });

  it("should return empty array if no children", () => {
    expect(getChildTags(tags, "personal")).toEqual([]);
  });

  it("should be case-insensitive", () => {
    expect(getChildTags(tags, "WORK")).toEqual(["work/projects", "work/personal"]);
  });
});

describe("getDescendantTags", () => {
  const tags = ["work", "work/projects", "work/personal", "work/projects/client-a", "personal"];

  it("should return all descendants (children and grandchildren)", () => {
    expect(getDescendantTags(tags, "work")).toEqual(["work/projects", "work/personal", "work/projects/client-a"]);
  });

  it("should return empty array if no descendants", () => {
    expect(getDescendantTags(tags, "personal")).toEqual([]);
  });
});

describe("isChildOf", () => {
  it("should return true for direct children", () => {
    expect(isChildOf("work/projects", "work")).toBe(true);
  });

  it("should return true for indirect children (grandchildren)", () => {
    expect(isChildOf("work/projects/client-a", "work")).toBe(true);
  });

  it("should return false for same tag", () => {
    expect(isChildOf("work", "work")).toBe(false);
  });

  it("should return false for unrelated tags", () => {
    expect(isChildOf("personal", "work")).toBe(false);
  });

  it("should be case-insensitive", () => {
    expect(isChildOf("WORK/PROJECTS", "work")).toBe(true);
  });
});

describe("isDirectChildOf", () => {
  it("should return true for direct children", () => {
    expect(isDirectChildOf("work/projects", "work")).toBe(true);
  });

  it("should return false for indirect children (grandchildren)", () => {
    expect(isDirectChildOf("work/projects/client-a", "work")).toBe(false);
  });

  it("should return false for same tag", () => {
    expect(isDirectChildOf("work", "work")).toBe(false);
  });
});

describe("getRootTags", () => {
  it("should return only root-level tags", () => {
    const tags = ["work", "work/projects", "personal", "dev/backend"];
    expect(getRootTags(tags)).toEqual(["work", "personal"]);
  });
});

describe("buildTagTree", () => {
  it("should build hierarchical tree structure", () => {
    const tags = ["work", "work/projects", "work/personal", "personal"];
    const tree = buildTagTree(tags);

    expect(tree).toHaveLength(2); // "work" and "personal"
    // Tags are sorted alphabetically, so "personal" comes before "work"
    expect(tree[0].tag).toBe("personal");
    expect(tree[0].children).toHaveLength(0);
    expect(tree[1].tag).toBe("work");
    expect(tree[1].children).toHaveLength(2);
  });

  it("should handle case-insensitive normalization", () => {
    const tags = ["Work", "work/Projects", "PERSONAL"];
    const tree = buildTagTree(tags);

    expect(tree).toHaveLength(2);
    // Tags are sorted alphabetically, so "personal" comes before "work"
    expect(tree[0].tag).toBe("personal");
    expect(tree[1].tag).toBe("work");
  });

  it("should deduplicate tags", () => {
    const tags = ["work", "Work", "WORK"];
    const tree = buildTagTree(tags);

    expect(tree).toHaveLength(1);
  });

  it("should handle orphaned nested tag as root-level item", () => {
    const tags = ["work/projects", "personal"];
    const tree = buildTagTree(tags);

    expect(tree).toHaveLength(2);
    expect(tree[0].tag).toBe("personal");
    expect(tree[0].depth).toBe(0);
    expect(tree[1].tag).toBe("work/projects"); // Shown at root level
    expect(tree[1].depth).toBe(1); // Still has depth 1 (one parent level)
    expect(tree[1].children).toHaveLength(0);
  });

  it("should nest orphaned tag once parent is added", () => {
    const tagsWithoutParent = ["work/projects"];
    const treeWithoutParent = buildTagTree(tagsWithoutParent);
    expect(treeWithoutParent).toHaveLength(1);
    expect(treeWithoutParent[0].tag).toBe("work/projects");

    const tagsWithParent = ["work", "work/projects"];
    const treeWithParent = buildTagTree(tagsWithParent);
    expect(treeWithParent).toHaveLength(1);
    expect(treeWithParent[0].tag).toBe("work");
    expect(treeWithParent[0].children).toHaveLength(1);
    expect(treeWithParent[0].children[0].tag).toBe("work/projects");
  });

  it("should handle multiple orphaned nested tags", () => {
    const tags = ["work/projects", "work/admin", "personal"];
    const tree = buildTagTree(tags);

    expect(tree).toHaveLength(3);
    expect(tree[0].tag).toBe("personal");
    expect(tree[1].tag).toBe("work/admin");
    expect(tree[2].tag).toBe("work/projects");
  });

  it("should handle deeply nested orphaned tag", () => {
    const tags = ["work/projects/client-a"];
    const tree = buildTagTree(tags);

    expect(tree).toHaveLength(1);
    expect(tree[0].tag).toBe("work/projects/client-a");
    expect(tree[0].depth).toBe(2); // Two parent levels
    expect(tree[0].children).toHaveLength(0);
  });

  it("should handle orphaned tags with common parent", () => {
    const tags = ["work/projects/client-a", "work/projects/client-b"];
    const tree = buildTagTree(tags);

    // Both tags are orphaned (work/projects doesn't exist), so both appear as roots
    expect(tree).toHaveLength(2);
    expect(tree[0].tag).toBe("work/projects/client-a");
    expect(tree[0].children).toHaveLength(0);
    expect(tree[1].tag).toBe("work/projects/client-b");
    expect(tree[1].children).toHaveLength(0);

    // Once parent is added, they should nest properly
    const tagsWithParent = ["work/projects", "work/projects/client-a", "work/projects/client-b"];
    const treeWithParent = buildTagTree(tagsWithParent);
    expect(treeWithParent).toHaveLength(1);
    expect(treeWithParent[0].tag).toBe("work/projects");
    expect(treeWithParent[0].children).toHaveLength(2);
  });
});

describe("flattenTagTree", () => {
  it("should flatten tree to list with depth info", () => {
    const tags = ["work", "work/projects", "work/personal", "personal"];
    const tree = buildTagTree(tags);
    const flattened = flattenTagTree(tree);

    expect(flattened).toHaveLength(4);
    // Tags are sorted alphabetically, so "personal" comes first, then "work" with its children
    expect(flattened[0]).toEqual({
      tag: "personal",
      name: "personal",
      depth: 0,
      hasChildren: false,
    });
    expect(flattened[1]).toEqual({
      tag: "work",
      name: "work",
      depth: 0,
      hasChildren: true,
    });
  });
});

describe("deduplicateTags", () => {
  it("should remove case-insensitive duplicates", () => {
    expect(deduplicateTags(["work", "Work", "WORK"])).toEqual(["work"]);
  });

  it("should preserve first occurrence's casing (normalized to lowercase)", () => {
    expect(deduplicateTags(["Work", "PROJECTS", "work"])).toEqual(["work", "projects"]);
  });

  it("should handle hierarchical tags", () => {
    expect(deduplicateTags(["work/projects", "Work/Projects", "personal"])).toEqual(["work/projects", "personal"]);
  });
});

describe("expandTagsWithParents", () => {
  it("should add parent tags for nested tags", () => {
    const result = expandTagsWithParents(["work/projects"]);
    expect(result).toEqual(["work", "work/projects"]);
  });

  it("should handle multiple levels of nesting", () => {
    const result = expandTagsWithParents(["work/projects/client-a"]);
    expect(result).toEqual(["work", "work/projects", "work/projects/client-a"]);
  });

  it("should deduplicate when parent already exists in input", () => {
    const result = expandTagsWithParents(["work", "work/projects"]);
    expect(result).toEqual(["work", "work/projects"]);
  });

  it("should handle flat tags without modification", () => {
    const result = expandTagsWithParents(["work", "personal"]);
    expect(result).toEqual(["personal", "work"]);
  });

  it("should handle mixed flat and nested tags", () => {
    const result = expandTagsWithParents(["personal", "work/projects/client-a"]);
    expect(result).toEqual(["personal", "work", "work/projects", "work/projects/client-a"]);
  });

  it("should normalize and deduplicate tags", () => {
    const result = expandTagsWithParents(["Work/Projects", "work", "WORK/PROJECTS"]);
    expect(result).toEqual(["work", "work/projects"]);
  });

  it("should handle empty array", () => {
    const result = expandTagsWithParents([]);
    expect(result).toEqual([]);
  });

  it("should handle multiple nested tags from different hierarchies", () => {
    const result = expandTagsWithParents(["work/projects", "personal/notes"]);
    expect(result).toEqual(["personal", "personal/notes", "work", "work/projects"]);
  });
});

describe("removeRedundantParents", () => {
  it("should remove parent when child is present", () => {
    const result = removeRedundantParents(["work", "work/projects"]);
    expect(result).toEqual(["work/projects"]);
  });

  it("should keep only deepest nested tag", () => {
    const result = removeRedundantParents(["work", "work/projects", "work/projects/client-a"]);
    expect(result).toEqual(["work/projects/client-a"]);
  });

  it("should preserve tags without children", () => {
    const result = removeRedundantParents(["work", "personal"]);
    expect(result).toEqual(["personal", "work"]);
  });

  it("should handle mixed hierarchies", () => {
    const result = removeRedundantParents(["work", "work/projects", "personal", "personal/notes"]);
    expect(result).toEqual(["personal/notes", "work/projects"]);
  });

  it("should handle multiple children of same parent", () => {
    const result = removeRedundantParents(["work", "work/projects", "work/admin"]);
    expect(result).toEqual(["work/admin", "work/projects"]);
  });

  it("should normalize and deduplicate before removing parents", () => {
    const result = removeRedundantParents(["Work", "work/Projects", "WORK/PROJECTS"]);
    expect(result).toEqual(["work/projects"]);
  });

  it("should handle empty array", () => {
    const result = removeRedundantParents([]);
    expect(result).toEqual([]);
  });

  it("should handle single nested tag without parent", () => {
    const result = removeRedundantParents(["work/projects"]);
    expect(result).toEqual(["work/projects"]);
  });

  it("should handle complex multi-level hierarchy", () => {
    const result = removeRedundantParents([
      "root",
      "root/level1",
      "root/level1/level2",
      "root/level1/level2/level3",
      "other",
    ]);
    expect(result).toEqual(["other", "root/level1/level2/level3"]);
  });
});
