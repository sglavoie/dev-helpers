import { describe, expect, it } from "vitest";
import type { ExportData, Snippet, StorageData } from "../types";
import { CURRENT_VERSION } from "./storage-constants";
import { mergeImportedData, replaceImportedData } from "./storage-import-export";

const baseSnippet: Snippet = {
  id: "snippet-1",
  title: "Title",
  content: "Content",
  description: "",
  tags: [],
  createdAt: 1000,
  updatedAt: 1000,
  lastUsedAt: undefined,
  useCount: 0,
  isFavorite: false,
  isArchived: false,
  isPinned: false,
};

function createExportData(snippets: ExportData["snippets"]): ExportData {
  return {
    version: "1.0.0",
    exportedAt: 2000,
    snippets,
    tags: [],
    placeholderHistory: {},
  };
}

describe("storage import/export helpers", () => {
  it("backfills required snippet fields when replacing imported legacy data", () => {
    const legacySnippet = {
      id: "legacy-1",
      title: "Legacy",
      content: "Content",
      category: " Work ",
      createdAt: 1000,
      updatedAt: 1000,
    };

    const data = replaceImportedData(createExportData([legacySnippet as unknown as Snippet]));

    expect(data).toEqual({
      version: CURRENT_VERSION,
      snippets: [
        {
          id: "legacy-1",
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
        },
      ],
      tags: [],
      placeholderHistory: {},
    });
    expect(data.snippets[0]).not.toHaveProperty("category");
  });

  it("preserves explicit imported flags while normalizing duplicate tags during merge", () => {
    const currentData: StorageData = {
      version: CURRENT_VERSION,
      snippets: [baseSnippet],
      tags: [],
      placeholderHistory: {},
    };
    const importedSnippet = {
      ...baseSnippet,
      id: "snippet-2",
      tags: ["Work", " work ", "Personal"],
      useCount: 0,
      isFavorite: true,
      isArchived: true,
      isPinned: true,
    };

    const data = mergeImportedData(currentData, createExportData([importedSnippet]));

    expect(data.snippets).toHaveLength(2);
    expect(data.snippets[1]).toMatchObject({
      id: "snippet-2",
      tags: ["work", "personal"],
      useCount: 0,
      isFavorite: true,
      isArchived: true,
      isPinned: true,
    });
  });
});
