import { describe, expect, it } from "vitest";
import { CURRENT_VERSION } from "./storage-constants";
import { migrateStorageData } from "./storage-migrations";

describe("storage migrations", () => {
  it("migrates v1 category data into normalized tags without losing required snippet fields", () => {
    const { data, didMigrate } = migrateStorageData({
      version: 1,
      snippets: [
        {
          id: "snippet-1",
          title: "Legacy",
          content: "Content",
          category: " Work ",
          createdAt: 1000,
          updatedAt: 1000,
        },
      ],
      tags: [],
    });

    expect(didMigrate).toBe(true);
    expect(data.version).toBe(CURRENT_VERSION);
    expect(data.placeholderHistory).toEqual({});
    expect(data.snippets[0]).toEqual({
      id: "snippet-1",
      title: "Legacy",
      content: "Content",
      createdAt: 1000,
      updatedAt: 1000,
      description: "",
      tags: ["work"],
      useCount: 0,
      isFavorite: false,
      isArchived: false,
      isPinned: false,
    });
    expect(data.snippets[0]).not.toHaveProperty("category");
  });

  it("repairs v6 snippets while preserving explicit current fields", () => {
    const { data } = migrateStorageData({
      version: 6,
      snippets: [
        {
          id: "snippet-1",
          title: "Existing",
          content: "Content",
          tags: ["Personal", "personal"],
          createdAt: 1000,
          updatedAt: 1000,
          useCount: 0,
          isFavorite: true,
          isArchived: true,
          description: "Already described",
        },
      ],
      tags: [],
      placeholderHistory: {},
    });

    expect(data.version).toBe(CURRENT_VERSION);
    expect(data.snippets[0]).toMatchObject({
      tags: ["personal"],
      useCount: 0,
      isFavorite: true,
      isArchived: true,
      isPinned: false,
      description: "Already described",
    });
  });
});
