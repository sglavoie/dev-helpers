import { describe, it, expect, beforeEach } from "vitest";
import { addSnippet, getTags, deleteTag, renameTag, mergeTags, clearAllData, getSnippets } from "./storage";

beforeEach(async () => {
  // Clear storage before each test
  await clearAllData();
});

describe("Tag Operations", () => {
  describe("getTags", () => {
    it("should return empty array when no snippets exist", async () => {
      const tags = await getTags();
      expect(tags).toEqual([]);
    });

    it("should return unique tags from all snippets", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work", "important"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["personal", "work"] });

      const tags = await getTags();
      expect(tags).toEqual(["important", "personal", "work"]); // Sorted alphabetically
    });

    it("should handle snippets with no tags", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: [] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["work"] });

      const tags = await getTags();
      expect(tags).toEqual(["work"]);
    });

    it("should return sorted tags", async () => {
      await addSnippet({ title: "Test", content: "Content", tags: ["zebra", "apple", "banana"] });

      const tags = await getTags();
      expect(tags).toEqual(["apple", "banana", "zebra"]);
    });
  });

  describe("Redundant parent tag removal", () => {
    it("should remove redundant parent when adding snippet with parent and child", async () => {
      const snippet = await addSnippet({
        title: "Test",
        content: "Content",
        tags: ["work", "work/projects"],
      });

      expect(snippet.tags).toEqual(["work/projects"]);
    });

    it("should keep only deepest nested tag", async () => {
      const snippet = await addSnippet({
        title: "Test",
        content: "Content",
        tags: ["work", "work/projects", "work/projects/client-a"],
      });

      expect(snippet.tags).toEqual(["work/projects/client-a"]);
    });

    it("should preserve orphaned nested tag without parent", async () => {
      const snippet = await addSnippet({
        title: "Test",
        content: "Content",
        tags: ["work/projects"],
      });

      expect(snippet.tags).toEqual(["work/projects"]);
    });

    it("should include only orphaned nested tag in getTags() result", async () => {
      await addSnippet({
        title: "Test",
        content: "Content",
        tags: ["work/projects"],
      });

      const tags = await getTags();
      expect(tags).toEqual(["work/projects"]);
    });

    it("should handle multiple nested tags from different hierarchies", async () => {
      const snippet = await addSnippet({
        title: "Test",
        content: "Content",
        tags: ["work/projects", "personal/notes"],
      });

      expect(snippet.tags).toEqual(["personal/notes", "work/projects"]);
    });

    it("should handle flat tags without modification", async () => {
      const snippet = await addSnippet({
        title: "Test",
        content: "Content",
        tags: ["work", "personal"],
      });

      expect(snippet.tags).toEqual(["personal", "work"]);
    });
  });

  describe("deleteTag", () => {
    it("should remove tag from all snippets", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work", "important"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["work", "personal"] });

      await deleteTag("work");

      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["important"]);
      expect(snippets[1].tags).toEqual(["personal"]);
    });

    it("should not affect snippets without the tag", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["personal"] });

      await deleteTag("work");

      const snippets = await getSnippets();
      expect(snippets[1].tags).toEqual(["personal"]);
    });

    it("should update tags list after deletion", async () => {
      await addSnippet({ title: "Test", content: "Content", tags: ["work", "personal"] });

      await deleteTag("work");

      const tags = await getTags();
      expect(tags).toEqual(["personal"]);
    });
  });

  describe("renameTag", () => {
    it("should rename tag in all snippets", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["work", "important"] });

      const affectedCount = await renameTag("work", "office");

      expect(affectedCount).toBe(2);
      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["office"]);
      expect(snippets[1].tags).toEqual(["important", "office"]); // Tags are sorted alphabetically
    });

    it("should rename parent tag and all child tags", async () => {
      await addSnippet({
        title: "Test",
        content: "Content",
        tags: ["work", "work/projects", "work/projects/client-a"],
      });

      const affectedCount = await renameTag("work", "office");

      expect(affectedCount).toBe(1);
      const snippets = await getSnippets();
      // Only deepest tag is kept (redundant parents removed)
      expect(snippets[0].tags).toEqual(["office/projects/client-a"]);
    });

    it("should rename child tag without affecting parent", async () => {
      await addSnippet({ title: "Test", content: "Content", tags: ["work", "work/projects"] });

      const affectedCount = await renameTag("work/projects", "work/tasks");

      expect(affectedCount).toBe(1);
      const snippets = await getSnippets();
      // Only deepest tag is kept (redundant parent removed)
      expect(snippets[0].tags).toEqual(["work/tasks"]);
    });

    it("should count all snippets with parent or child tags", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["work/admin"] });
      await addSnippet({ title: "Test 3", content: "Content 3", tags: ["personal"] });

      const affectedCount = await renameTag("work", "office");

      expect(affectedCount).toBe(2);
      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["office"]);
      expect(snippets[1].tags).toEqual(["office/admin"]); // Only child tag, no parent auto-created
      expect(snippets[2].tags).toEqual(["personal"]);
    });

    it("should deduplicate tags when renaming creates conflicts", async () => {
      await addSnippet({ title: "Test", content: "Content", tags: ["work/admin", "office/admin"] });

      const affectedCount = await renameTag("work", "office");

      expect(affectedCount).toBe(1);
      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["office/admin"]); // Deduplicated, only one tag remains
    });

    it("should return count of affected snippets", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["personal"] });

      const affectedCount = await renameTag("work", "office");

      expect(affectedCount).toBe(1);
    });

    it("should not affect snippets without the tag", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["personal"] });

      await renameTag("work", "office");

      const snippets = await getSnippets();
      expect(snippets[1].tags).toEqual(["personal"]);
    });
  });

  describe("mergeTags", () => {
    it("should merge source tag into target tag", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["office"] });

      const affectedCount = await mergeTags("work", "office");

      expect(affectedCount).toBe(1);
      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["office"]);
      expect(snippets[1].tags).toEqual(["office"]);
    });

    it("should remove duplicates when merging", async () => {
      await addSnippet({ title: "Test", content: "Content", tags: ["work", "office"] });

      await mergeTags("work", "office");

      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["office"]); // No duplicates
    });

    it("should throw error when merging tag with itself", async () => {
      await addSnippet({ title: "Test", content: "Content", tags: ["work"] });

      await expect(mergeTags("work", "work")).rejects.toThrow("Cannot merge a tag with itself");
    });

    it("should handle multiple snippets with both tags", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work", "important"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["office", "important"] });
      await addSnippet({ title: "Test 3", content: "Content 3", tags: ["work", "office"] });

      const affectedCount = await mergeTags("work", "office");

      expect(affectedCount).toBe(2); // Test 1 and Test 3
      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["important", "office"]); // Tags are sorted alphabetically
      expect(snippets[1].tags).toEqual(["important", "office"]); // Tags are sorted alphabetically
      expect(snippets[2].tags).toEqual(["office"]); // Duplicate removed
    });

    it("should update tags list after merge", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["office"] });

      await mergeTags("work", "office");

      const tags = await getTags();
      expect(tags).toEqual(["office"]); // "work" should be gone
    });

    it("should not affect snippets without source tag", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["personal"] });

      await mergeTags("work", "office");

      const snippets = await getSnippets();
      expect(snippets[1].tags).toEqual(["personal"]); // Unchanged
    });
  });

  describe("Tag edge cases", () => {
    it("should handle empty tag arrays", async () => {
      await addSnippet({ title: "Test", content: "Content", tags: [] });

      const tags = await getTags();
      expect(tags).toEqual([]);
    });

    it("should handle multiple operations in sequence", async () => {
      await addSnippet({ title: "Test 1", content: "Content 1", tags: ["work", "urgent"] });
      await addSnippet({ title: "Test 2", content: "Content 2", tags: ["work", "important"] });

      await renameTag("work", "office");
      await deleteTag("urgent");
      await mergeTags("important", "office");

      const tags = await getTags();
      expect(tags).toEqual(["office"]);

      const snippets = await getSnippets();
      expect(snippets[0].tags).toEqual(["office"]);
      expect(snippets[1].tags).toEqual(["office"]);
    });
  });
});
