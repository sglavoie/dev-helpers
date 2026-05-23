import type { ExportData, Snippet, StorageData } from "../types";
import { deduplicateTags, normalizeTags } from "./tags";
import { CURRENT_VERSION } from "./storage-constants";
import { mergePlaceholderHistory } from "./storage-placeholder-history";

type ImportableSnippet = Omit<Snippet, "description" | "tags" | "useCount" | "isFavorite" | "isArchived" | "isPinned"> &
  Partial<Pick<Snippet, "description" | "tags" | "useCount" | "isFavorite" | "isArchived" | "isPinned">> & {
    category?: string;
  };

function getImportedTags(snippet: ImportableSnippet): string[] {
  if (Array.isArray(snippet.tags)) {
    return snippet.tags;
  }
  return typeof snippet.category === "string" ? [snippet.category] : [];
}

function sanitizeImportedSnippet(snippet: ImportableSnippet): Snippet {
  const snippetWithoutLegacyCategory = { ...snippet };
  delete snippetWithoutLegacyCategory.category;

  return {
    ...snippetWithoutLegacyCategory,
    useCount: snippet.useCount ?? 0,
    tags: deduplicateTags(normalizeTags(getImportedTags(snippet))),
    isFavorite: snippet.isFavorite ?? false,
    isArchived: snippet.isArchived ?? false,
    isPinned: snippet.isPinned ?? false,
    description: snippet.description ?? "",
  };
}

export function createExportData(data: StorageData): ExportData {
  return {
    version: `${CURRENT_VERSION}.0.0`,
    exportedAt: Date.now(),
    snippets: data.snippets,
    tags: data.tags,
    placeholderHistory: data.placeholderHistory || {},
  };
}

export function mergeImportedData(currentData: StorageData, importedData: ExportData): StorageData {
  const existingIds = new Set(currentData.snippets.map((snippet) => snippet.id));
  const newSnippets = importedData.snippets
    .filter((snippet) => !existingIds.has(snippet.id))
    .map(sanitizeImportedSnippet);

  return {
    ...currentData,
    snippets: [...currentData.snippets, ...newSnippets],
    placeholderHistory: importedData.placeholderHistory
      ? mergePlaceholderHistory(currentData.placeholderHistory, importedData.placeholderHistory)
      : currentData.placeholderHistory || {},
  };
}

export function replaceImportedData(importedData: ExportData): StorageData {
  return {
    version: CURRENT_VERSION,
    snippets: importedData.snippets.map(sanitizeImportedSnippet),
    tags: [],
    placeholderHistory: importedData.placeholderHistory || {},
  };
}
