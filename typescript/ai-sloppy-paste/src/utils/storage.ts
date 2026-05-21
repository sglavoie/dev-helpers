import { LocalStorage, getPreferenceValues } from "@raycast/api";
import type { ExportData, PlaceholderHistory, PlaceholderHistoryValue, Snippet, StorageData } from "../types";
import { CURRENT_VERSION, STORAGE_KEY } from "./storage-constants";
import { createExportData, mergeImportedData, replaceImportedData } from "./storage-import-export";
import { migrateStorageData } from "./storage-migrations";
import {
  addPlaceholderValueToData,
  clearAllPlaceholderHistoryInData,
  clearPlaceholderHistoryForKeyInData,
  deletePlaceholderValueFromData,
  getAllPlaceholderKeysFromData,
  getPlaceholderHistoryForKeyFromData,
  getPlaceholderHistoryFromData,
  updatePlaceholderValueInData,
  updatePlaceholderValueUsageInData,
} from "./storage-placeholder-history";
import { removeRedundantParents } from "./tags";

let cachedData: StorageData | null = null;

interface Preferences {
  maxPlaceholderHistoryValues?: string;
}

export function getMaxPlaceholderHistoryValues(): number {
  const preferences = getPreferenceValues<Preferences>();
  const value = parseInt(preferences.maxPlaceholderHistoryValues || "20", 10);
  return Number.isNaN(value) ? 20 : value;
}

async function loadStorageData(): Promise<StorageData> {
  if (cachedData) {
    return cachedData;
  }

  const storageJson = await LocalStorage.getItem<string>(STORAGE_KEY);
  if (storageJson) {
    try {
      const { data, didMigrate } = migrateStorageData(JSON.parse(storageJson));
      if (didMigrate) {
        await saveStorageData(data);
      }

      cachedData = data;
      return data;
    } catch (error) {
      console.error("Error parsing storage data:", error);
    }
  }

  const emptyData: StorageData = {
    version: CURRENT_VERSION,
    snippets: [],
    tags: [],
    placeholderHistory: {},
  };
  cachedData = emptyData;
  return emptyData;
}

async function saveStorageData(data: StorageData): Promise<void> {
  cachedData = data;
  await LocalStorage.setItem(STORAGE_KEY, JSON.stringify(data));
}

export function invalidateCache(): void {
  cachedData = null;
}

function createSnippetId(now: number): string {
  return `snippet-${now}-${Math.random().toString(36).substr(2, 9)}`;
}

function getSnippetOrThrow(data: StorageData, id: string): Snippet {
  const snippet = data.snippets.find((item) => item.id === id);
  if (!snippet) {
    throw new Error("Snippet not found");
  }
  return snippet;
}

function isTagOrDescendant(tag: string, targetTag: string): boolean {
  return tag === targetTag || tag.startsWith(`${targetTag}/`);
}

function replaceTagPrefix(tag: string, sourceTag: string, targetTag: string): string {
  if (tag === sourceTag) {
    return targetTag;
  }
  return tag.startsWith(`${sourceTag}/`) ? targetTag + tag.slice(sourceTag.length) : tag;
}

function updateMatchingSnippetTags(
  snippets: Snippet[],
  sourceTag: string,
  targetTag: string,
): { snippets: Snippet[]; affectedCount: number } {
  const affectedSnippetIds = new Set<string>();

  return {
    snippets: snippets.map((snippet) => {
      if (!snippet.tags.some((tag) => isTagOrDescendant(tag, sourceTag))) {
        return snippet;
      }

      affectedSnippetIds.add(snippet.id);
      return {
        ...snippet,
        tags: [
          ...new Set(
            snippet.tags.map((tag) => replaceTagPrefix(tag, sourceTag, targetTag)).map((tag) => tag.toLowerCase()),
          ),
        ],
        updatedAt: Date.now(),
      };
    }),
    affectedCount: affectedSnippetIds.size,
  };
}

async function toggleSnippetFlag(id: string, flag: "isFavorite" | "isArchived" | "isPinned"): Promise<boolean> {
  const data = await loadStorageData();
  const snippet = getSnippetOrThrow(data, id);
  snippet[flag] = !snippet[flag];
  snippet.updatedAt = Date.now();
  await saveStorageData(data);
  return snippet[flag];
}

export async function getSnippets(): Promise<Snippet[]> {
  const data = await loadStorageData();
  return [...data.snippets];
}

export async function getTags(): Promise<string[]> {
  const data = await loadStorageData();
  return Array.from(new Set(data.snippets.flatMap((snippet) => snippet.tags))).sort((a, b) => a.localeCompare(b));
}

export async function addSnippet(
  snippet: Omit<
    Snippet,
    "id" | "createdAt" | "updatedAt" | "lastUsedAt" | "useCount" | "isFavorite" | "isArchived" | "isPinned"
  >,
): Promise<Snippet> {
  const data = await loadStorageData();
  const now = Date.now();
  const newSnippet: Snippet = {
    ...snippet,
    tags: removeRedundantParents(snippet.tags),
    id: createSnippetId(now),
    createdAt: now,
    updatedAt: now,
    lastUsedAt: undefined,
    useCount: 0,
    isFavorite: false,
    isArchived: false,
    isPinned: false,
  };

  data.snippets.push(newSnippet);
  await saveStorageData(data);
  return newSnippet;
}

export async function updateSnippet(id: string, updates: Partial<Omit<Snippet, "id" | "createdAt">>): Promise<void> {
  const data = await loadStorageData();
  const index = data.snippets.findIndex((snippet) => snippet.id === id);

  if (index === -1) {
    throw new Error("Snippet not found");
  }

  data.snippets[index] = {
    ...data.snippets[index],
    ...updates,
    ...(updates.tags ? { tags: removeRedundantParents(updates.tags) } : {}),
    updatedAt: Date.now(),
  };

  await saveStorageData(data);
}

export async function deleteSnippet(id: string): Promise<void> {
  const data = await loadStorageData();
  data.snippets = data.snippets.filter((snippet) => snippet.id !== id);
  await saveStorageData(data);
}

export async function duplicateSnippet(id: string): Promise<Snippet> {
  const data = await loadStorageData();
  const original = getSnippetOrThrow(data, id);
  const now = Date.now();
  const duplicate: Snippet = {
    ...original,
    id: createSnippetId(now),
    title: `${original.title} (Copy)`,
    createdAt: now,
    updatedAt: now,
    lastUsedAt: undefined,
    useCount: 0,
    isFavorite: false,
    isArchived: false,
    isPinned: false,
  };

  data.snippets.push(duplicate);
  await saveStorageData(data);
  return duplicate;
}

export async function toggleFavorite(id: string): Promise<boolean> {
  return toggleSnippetFlag(id, "isFavorite");
}

export async function toggleArchive(id: string): Promise<boolean> {
  return toggleSnippetFlag(id, "isArchived");
}

export async function togglePin(id: string): Promise<boolean> {
  return toggleSnippetFlag(id, "isPinned");
}

export async function incrementUsage(id: string): Promise<void> {
  const data = await loadStorageData();
  const snippet = getSnippetOrThrow(data, id);
  snippet.useCount = (snippet.useCount || 0) + 1;
  snippet.lastUsedAt = Date.now();
  snippet.updatedAt = Date.now();
  await saveStorageData(data);
}

export async function deleteTag(tag: string): Promise<void> {
  const data = await loadStorageData();
  data.snippets = data.snippets.map((snippet) =>
    !snippet.tags.includes(tag)
      ? snippet
      : {
          ...snippet,
          tags: snippet.tags.filter((item) => item !== tag),
          updatedAt: Date.now(),
        },
  );
  await saveStorageData(data);
}

export async function renameTag(oldTag: string, newTag: string): Promise<number> {
  const data = await loadStorageData();
  const result = updateMatchingSnippetTags(data.snippets, oldTag, newTag);
  data.snippets = result.snippets;
  await saveStorageData(data);
  return result.affectedCount;
}

export async function mergeTags(sourceTag: string, targetTag: string): Promise<number> {
  if (sourceTag === targetTag) {
    throw new Error("Cannot merge a tag with itself");
  }

  const data = await loadStorageData();
  const result = updateMatchingSnippetTags(data.snippets, sourceTag, targetTag);
  data.snippets = result.snippets;
  await saveStorageData(data);
  return result.affectedCount;
}

export async function getPlaceholderHistory(): Promise<PlaceholderHistory> {
  return getPlaceholderHistoryFromData(await loadStorageData());
}

export async function getPlaceholderHistoryForKey(key: string): Promise<PlaceholderHistoryValue[]> {
  return getPlaceholderHistoryForKeyFromData(await loadStorageData(), key);
}

export async function getAllPlaceholderKeys(): Promise<string[]> {
  return getAllPlaceholderKeysFromData(await loadStorageData());
}

export async function addPlaceholderValue(key: string, value: string): Promise<void> {
  const data = await loadStorageData();
  if (!addPlaceholderValueToData(data, key, value)) {
    return;
  }
  await saveStorageData(data);
}

export async function updatePlaceholderValueUsage(key: string, value: string): Promise<void> {
  const data = await loadStorageData();
  if (!updatePlaceholderValueUsageInData(data, key, value)) {
    return;
  }
  await saveStorageData(data);
}

export async function deletePlaceholderValue(key: string, value: string): Promise<void> {
  const data = await loadStorageData();
  if (!deletePlaceholderValueFromData(data, key, value)) {
    return;
  }
  await saveStorageData(data);
}

export async function updatePlaceholderValue(key: string, oldValue: string, newValue: string): Promise<void> {
  const data = await loadStorageData();
  updatePlaceholderValueInData(data, key, oldValue, newValue);
  await saveStorageData(data);
}

export async function clearPlaceholderHistoryForKey(key: string): Promise<void> {
  const data = await loadStorageData();
  clearPlaceholderHistoryForKeyInData(data, key);
  await saveStorageData(data);
}

export async function clearAllPlaceholderHistory(): Promise<void> {
  const data = await loadStorageData();
  clearAllPlaceholderHistoryInData(data);
  await saveStorageData(data);
}

export async function exportData(): Promise<ExportData> {
  return createExportData(await loadStorageData());
}

export async function importData(importedData: ExportData, merge = false): Promise<void> {
  const currentData = await loadStorageData();
  const nextData = merge ? mergeImportedData(currentData, importedData) : replaceImportedData(importedData);
  invalidateCache();
  await saveStorageData(nextData);
}

export async function clearAllData(): Promise<void> {
  invalidateCache();
  await LocalStorage.removeItem(STORAGE_KEY);
}

export async function getStorageSize(): Promise<{ bytes: number; formatted: string; percentage: number }> {
  const storageJson = (await LocalStorage.getItem<string>(STORAGE_KEY)) || "{}";
  const totalBytes = new Blob([storageJson]).size;

  let formatted: string;
  if (totalBytes < 1024) {
    formatted = `${totalBytes} bytes`;
  } else if (totalBytes < 1024 * 1024) {
    formatted = `${(totalBytes / 1024).toFixed(1)} KB`;
  } else {
    formatted = `${(totalBytes / (1024 * 1024)).toFixed(2)} MB`;
  }

  return {
    bytes: totalBytes,
    formatted,
    percentage: (totalBytes / (5 * 1024 * 1024)) * 100,
  };
}
