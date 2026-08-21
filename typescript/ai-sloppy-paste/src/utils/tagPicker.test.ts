import { describe, it, expect } from "vitest";
import { buildTagRows, filterTagRows, getCreateCandidate, toggleTag } from "./tagPicker";

describe("buildTagRows", () => {
  it("marks selected tags and leaves the rest unselected", () => {
    const rows = buildTagRows(["personal", "work"], ["work"]);

    expect(rows).toEqual([
      { tag: "personal", state: "unselected" },
      { tag: "work", state: "selected" },
    ]);
  });

  it("marks parents of a selected tag as implied", () => {
    const rows = buildTagRows(["work/projects"], ["work/projects"]);

    expect(rows).toEqual([
      { tag: "work", state: "implied", impliedBy: "work/projects" },
      { tag: "work/projects", state: "selected" },
    ]);
  });

  it("keeps a parent selected when it is explicitly selected alongside an unrelated child", () => {
    const rows = buildTagRows(["work", "work/projects"], ["work"]);

    expect(rows).toEqual([
      { tag: "work", state: "selected" },
      { tag: "work/projects", state: "unselected" },
    ]);
  });

  it("includes selected tags that are absent from the known list", () => {
    const rows = buildTagRows(["personal"], ["work/projects/client-a"]);

    expect(rows.map((row) => row.tag)).toEqual(["personal", "work", "work/projects", "work/projects/client-a"]);
    expect(rows.filter((row) => row.state === "implied").map((row) => row.impliedBy)).toEqual([
      "work/projects/client-a",
      "work/projects/client-a",
    ]);
  });

  it("drops a redundant parent from the incoming selection", () => {
    const rows = buildTagRows([], ["work", "work/projects"]);

    expect(rows).toEqual([
      { tag: "work", state: "implied", impliedBy: "work/projects" },
      { tag: "work/projects", state: "selected" },
    ]);
  });

  it("deduplicates known and selected tags", () => {
    const rows = buildTagRows(["work", "work"], ["work"]);

    expect(rows).toEqual([{ tag: "work", state: "selected" }]);
  });
});

describe("filterTagRows", () => {
  const rows = buildTagRows(["work/projects/client-a", "personal/reading"], []);

  it("returns every row for blank search text", () => {
    expect(filterTagRows(rows, "   ")).toHaveLength(rows.length);
  });

  it("matches a mid-path segment", () => {
    expect(filterTagRows(rows, "projects").map((row) => row.tag)).toEqual(["work/projects", "work/projects/client-a"]);
  });

  it("matches case-insensitively", () => {
    expect(filterTagRows(rows, "READING").map((row) => row.tag)).toEqual(["personal/reading"]);
  });
});

describe("getCreateCandidate", () => {
  it("returns null for blank search text", () => {
    expect(getCreateCandidate("   ", ["work"])).toBeNull();
  });

  it("returns null when the tag already exists", () => {
    expect(getCreateCandidate("work", ["work"])).toBeNull();
  });

  it("returns null when the tag exists in a different case", () => {
    expect(getCreateCandidate("Work", ["work"])).toBeNull();
  });

  it("returns the normalized tag for a new name", () => {
    expect(getCreateCandidate("work/new-thing", ["work"])).toEqual({ tag: "work/new-thing" });
  });

  it("converts spaces to dashes", () => {
    expect(getCreateCandidate("Has Spaces", [])).toEqual({ tag: "has-spaces" });
  });

  it("suppresses the candidate when the normalized form already exists", () => {
    expect(getCreateCandidate("Has Spaces", ["has-spaces"])).toBeNull();
  });

  it("reports an error for a tag hierarchy that is too deep", () => {
    expect(getCreateCandidate("a/b/c/d/e/f", [])).toEqual({ error: "Tag hierarchy too deep (max 5 levels)" });
  });

  it("reports an error for unsupported characters", () => {
    const candidate = getCreateCandidate("work!", []);

    expect(candidate).not.toBeNull();
    expect(candidate).toHaveProperty("error");
  });
});

describe("toggleTag", () => {
  it("adds an unselected tag", () => {
    expect(toggleTag(["personal"], "work")).toEqual({ tags: ["personal", "work"], changed: true });
  });

  it("removes a selected tag", () => {
    expect(toggleTag(["personal", "work"], "work")).toEqual({ tags: ["personal"], changed: true });
  });

  it("drops the parent when a child is added", () => {
    expect(toggleTag(["work"], "work/projects")).toEqual({ tags: ["work/projects"], changed: true });
  });

  it("does not write when the tag is only implied", () => {
    expect(toggleTag(["work/projects"], "work")).toEqual({
      tags: ["work/projects"],
      changed: false,
      impliedBy: "work/projects",
    });
  });

  it("normalizes the toggled tag", () => {
    expect(toggleTag(["work"], "WORK")).toEqual({ tags: [], changed: true });
  });

  it("removing a child leaves no implied parent behind", () => {
    expect(toggleTag(["work/projects"], "work/projects")).toEqual({ tags: [], changed: true });
  });
});
