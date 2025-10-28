import { LocalStorage, getPreferenceValues } from "@raycast/api";
import { Snippet, ExportData, StorageData, PlaceholderHistory, PlaceholderHistoryValue } from "../types";
import { normalizeTags, deduplicateTags, removeRedundantParents } from "./tags";

const STORAGE_KEY = "storage_v2";
const CURRENT_VERSION = 5;

interface Preferences {
  maxPlaceholderHistoryValues?: string;
}

/**
 * Maximum values stored in database (hard limit to prevent unbounded growth)
 * This is separate from the UI display preference
 */
const MAX_STORED_VALUES_PER_KEY = 100;

/**
 * Get the maximum number of placeholder history values for UI display from preferences
 * Note: Storage always keeps up to 100 values, this only affects UI filtering
 */
export function getMaxPlaceholderHistoryValues(): number {
  const preferences = getPreferenceValues<Preferences>();
  const value = parseInt(preferences.maxPlaceholderHistoryValues || "20", 10);
  return isNaN(value) ? 20 : value;
}

/**
 * Migration functions for upgrading data between versions
 */
const MIGRATIONS: Record<number, (data: any) => any> = {
  1: (data: any) => {
    // Migration from v1 to v2
    return {
      ...data,
      version: 2,
    };
  },
  2: (data: any) => {
    // Migration from v2 to v3 - Add isArchived field to all snippets
    return {
      ...data,
      version: 3,
      snippets: data.snippets.map((snippet: any) => ({
        ...snippet,
        isArchived: snippet.isArchived ?? false,
      })),
    };
  },
  3: (data: any) => {
    // Migration from v3 to v4 - Normalize all tags to lowercase and deduplicate
    return {
      ...data,
      version: 4,
      snippets: data.snippets.map((snippet: any) => ({
        ...snippet,
        tags: deduplicateTags(normalizeTags(snippet.tags || [])),
      })),
    };
  },
  4: (data: any) => {
    // Migration from v4 to v5 - Add placeholder history
    return {
      ...data,
      version: 5,
      placeholderHistory: {},
    };
  },
};

/**
 * Load storage data and apply migrations if needed
 */
async function loadStorageData(): Promise<StorageData> {
  const storageJson = await LocalStorage.getItem<string>(STORAGE_KEY);

  if (storageJson) {
    try {
      let data = JSON.parse(storageJson);

      // Apply migrations if data version is older than current version
      while (data.version && data.version < CURRENT_VERSION) {
        const migration = MIGRATIONS[data.version];
        if (migration) {
          console.log(`Migrating data from v${data.version} to v${data.version + 1}`);
          data = migration(data);
        } else {
          console.warn(`No migration found for version ${data.version}`);
          break;
        }
      }

      // Save migrated data if version changed
      if (data.version !== CURRENT_VERSION) {
        data.version = CURRENT_VERSION;
        await saveStorageData(data);
      }

      return data;
    } catch (error) {
      console.error("Error parsing storage data:", error);
    }
  }

  // Return empty storage if nothing found
  return {
    version: CURRENT_VERSION,
    snippets: [],
    tags: [],
    placeholderHistory: {},
  };
}

/**
 * Save storage data
 */
async function saveStorageData(data: StorageData): Promise<void> {
  await LocalStorage.setItem(STORAGE_KEY, JSON.stringify(data));
}

export async function getSnippets(): Promise<Snippet[]> {
  const data = await loadStorageData();
  return data.snippets;
}

export async function getTags(): Promise<string[]> {
  const data = await loadStorageData();

  // Dynamically compute tags from all snippets (ignore stored tags list)
  const tagSet = new Set<string>();
  data.snippets.forEach((snippet) => {
    snippet.tags.forEach((tag) => tagSet.add(tag));
  });

  // Return sorted, unique tags
  return Array.from(tagSet).sort((a, b) => a.localeCompare(b));
}

async function saveSnippets(snippets: Snippet[]): Promise<void> {
  const data = await loadStorageData();
  data.snippets = snippets;
  await saveStorageData(data);
}

async function saveTags(tags: string[]): Promise<void> {
  const data = await loadStorageData();
  data.tags = tags;
  await saveStorageData(data);
}

export async function addSnippet(
  snippet: Omit<Snippet, "id" | "createdAt" | "updatedAt" | "lastUsedAt" | "useCount" | "isFavorite" | "isArchived">,
): Promise<Snippet> {
  const data = await loadStorageData();

  const now = Date.now();
  const newSnippet: Snippet = {
    ...snippet,
    tags: removeRedundantParents(snippet.tags), // Normalize, deduplicate, and remove redundant parents
    id: `snippet-${now}-${Math.random().toString(36).substr(2, 9)}`,
    createdAt: now,
    updatedAt: now,
    lastUsedAt: undefined,
    useCount: 0,
    isFavorite: false,
    isArchived: false,
  };

  data.snippets.push(newSnippet);

  // Tags are now computed dynamically, no need to maintain master list

  await saveStorageData(data);
  return newSnippet;
}

export async function updateSnippet(id: string, updates: Partial<Omit<Snippet, "id" | "createdAt">>): Promise<void> {
  const data = await loadStorageData();

  const index = data.snippets.findIndex((s) => s.id === id);
  if (index === -1) throw new Error("Snippet not found");

  // Normalize and remove redundant parent tags if they're being updated
  const normalizedUpdates = {
    ...updates,
    ...(updates.tags ? { tags: removeRedundantParents(updates.tags) } : {}),
  };

  data.snippets[index] = {
    ...data.snippets[index],
    ...normalizedUpdates,
    updatedAt: Date.now(),
  };

  // Tags are now computed dynamically, no need to maintain master list

  await saveStorageData(data);
}

export async function deleteSnippet(id: string): Promise<void> {
  const data = await loadStorageData();
  data.snippets = data.snippets.filter((s) => s.id !== id);
  await saveStorageData(data);
}

export async function duplicateSnippet(id: string): Promise<Snippet> {
  const data = await loadStorageData();
  const original = data.snippets.find((s) => s.id === id);

  if (!original) throw new Error("Snippet not found");

  const now = Date.now();
  const duplicate: Snippet = {
    ...original,
    id: `snippet-${now}-${Math.random().toString(36).substr(2, 9)}`,
    title: `${original.title} (Copy)`,
    createdAt: now,
    updatedAt: now,
    lastUsedAt: undefined,
    useCount: 0,
    isFavorite: false,
    isArchived: false,
  };

  data.snippets.push(duplicate);
  await saveStorageData(data);
  return duplicate;
}

export async function toggleFavorite(id: string): Promise<boolean> {
  const data = await loadStorageData();
  const snippet = data.snippets.find((s) => s.id === id);

  if (!snippet) throw new Error("Snippet not found");

  snippet.isFavorite = !snippet.isFavorite;
  snippet.updatedAt = Date.now();

  await saveStorageData(data);
  return snippet.isFavorite;
}

export async function toggleArchive(id: string): Promise<boolean> {
  const data = await loadStorageData();
  const snippet = data.snippets.find((s) => s.id === id);

  if (!snippet) throw new Error("Snippet not found");

  snippet.isArchived = !snippet.isArchived;
  snippet.updatedAt = Date.now();

  await saveStorageData(data);
  return snippet.isArchived;
}

/**
 * Increment usage count and update last used timestamp
 */
export async function incrementUsage(id: string): Promise<void> {
  const data = await loadStorageData();
  const snippet = data.snippets.find((s) => s.id === id);

  if (!snippet) throw new Error("Snippet not found");

  snippet.useCount = (snippet.useCount || 0) + 1;
  snippet.lastUsedAt = Date.now();
  snippet.updatedAt = Date.now();

  await saveStorageData(data);
}

export async function deleteTag(tag: string): Promise<void> {
  const data = await loadStorageData();

  // Remove tag from all snippets that have it
  // (master tags list is computed dynamically, so no need to update it)
  data.snippets = data.snippets.map((snippet) => ({
    ...snippet,
    tags: snippet.tags.filter((t) => t !== tag),
    updatedAt: Date.now(),
  }));

  await saveStorageData(data);
}

export async function renameTag(oldTag: string, newTag: string): Promise<number> {
  const data = await loadStorageData();

  // Update tag in all snippets that have it or any of its descendants
  // (master tags list is computed dynamically, so no need to update it)
  let affectedCount = 0;
  const affectedSnippetIds = new Set<string>();

  data.snippets = data.snippets.map((snippet) => {
    // Check if this snippet has the tag or any descendant tags
    const hasTagOrDescendant = snippet.tags.some((t) => t === oldTag || t.startsWith(`${oldTag}/`));

    if (hasTagOrDescendant) {
      affectedSnippetIds.add(snippet.id);

      // Update all tags: rename parent and update all descendant prefixes
      const updatedTags = snippet.tags.map((t) => {
        if (t === oldTag) {
          // Exact match: rename the parent tag
          return newTag;
        } else if (t.startsWith(`${oldTag}/`)) {
          // Child tag: replace the parent prefix
          return newTag + t.slice(oldTag.length);
        }
        return t;
      });

      // Deduplicate tags in case of conflicts (merge behavior)
      const deduplicatedTags = [...new Set(updatedTags.map((t) => t.toLowerCase()))];

      return {
        ...snippet,
        tags: deduplicatedTags,
        updatedAt: Date.now(),
      };
    }
    return snippet;
  });

  affectedCount = affectedSnippetIds.size;
  await saveStorageData(data);
  return affectedCount;
}

export async function mergeTags(sourceTag: string, targetTag: string): Promise<number> {
  const data = await loadStorageData();

  if (sourceTag === targetTag) {
    throw new Error("Cannot merge a tag with itself");
  }

  // Merge sourceTag into targetTag - replace all instances of sourceTag with targetTag
  // and remove duplicates
  let affectedCount = 0;
  data.snippets = data.snippets.map((snippet) => {
    if (snippet.tags.includes(sourceTag)) {
      affectedCount++;
      const newTags = snippet.tags.map((t) => (t === sourceTag ? targetTag : t));
      // Remove duplicates (in case snippet already had targetTag)
      const uniqueTags = Array.from(new Set(newTags));
      return {
        ...snippet,
        tags: uniqueTags,
        updatedAt: Date.now(),
      };
    }
    return snippet;
  });

  await saveStorageData(data);
  return affectedCount;
}

/**
 * ========================================
 * Placeholder History Management Functions
 * ========================================
 */

/**
 * Get all placeholder history
 */
export async function getPlaceholderHistory(): Promise<PlaceholderHistory> {
  const data = await loadStorageData();
  return data.placeholderHistory || {};
}

/**
 * Get placeholder history for a specific key
 */
export async function getPlaceholderHistoryForKey(key: string): Promise<PlaceholderHistoryValue[]> {
  const history = await getPlaceholderHistory();
  return history[key] || [];
}

/**
 * Get all placeholder keys that have history
 */
export async function getAllPlaceholderKeys(): Promise<string[]> {
  const history = await getPlaceholderHistory();
  return Object.keys(history).sort((a, b) => a.localeCompare(b));
}

/**
 * Add or update a placeholder value in history
 * Implements LRU eviction when limit (20) is reached
 */
export async function addPlaceholderValue(key: string, value: string): Promise<void> {
  if (!key || !value.trim()) return;

  const data = await loadStorageData();
  const history = data.placeholderHistory || {};

  if (!history[key]) {
    history[key] = [];
  }

  const now = Date.now();

  // Check if value already exists
  const existingIndex = history[key].findIndex((item) => item.value === value);

  if (existingIndex !== -1) {
    // Update existing value's usage stats
    history[key][existingIndex].useCount += 1;
    history[key][existingIndex].lastUsed = now;
  } else {
    // Add new value
    const newValue: PlaceholderHistoryValue = {
      value,
      useCount: 1,
      lastUsed: now,
      createdAt: now,
    };

    history[key].push(newValue);

    // Enforce storage limit: always keep up to 100 values regardless of UI preference
    // This prevents data loss when users change their preference setting
    if (history[key].length > MAX_STORED_VALUES_PER_KEY) {
      // Sort by lastUsed (oldest first) and remove the oldest
      history[key].sort((a, b) => a.lastUsed - b.lastUsed);
      history[key].shift(); // Remove first (oldest)
    }
  }

  data.placeholderHistory = history;
  await saveStorageData(data);
}

/**
 * Update usage stats for a placeholder value (increment useCount, update lastUsed)
 */
export async function updatePlaceholderValueUsage(key: string, value: string): Promise<void> {
  const data = await loadStorageData();
  const history = data.placeholderHistory || {};

  if (!history[key]) return;

  const valueIndex = history[key].findIndex((item) => item.value === value);
  if (valueIndex !== -1) {
    history[key][valueIndex].useCount += 1;
    history[key][valueIndex].lastUsed = Date.now();

    data.placeholderHistory = history;
    await saveStorageData(data);
  }
}

/**
 * Delete a specific value from placeholder history
 */
export async function deletePlaceholderValue(key: string, value: string): Promise<void> {
  const data = await loadStorageData();
  const history = data.placeholderHistory || {};

  if (!history[key]) return;

  history[key] = history[key].filter((item) => item.value !== value);

  // Remove key entirely if no values left
  if (history[key].length === 0) {
    delete history[key];
  }

  data.placeholderHistory = history;
  await saveStorageData(data);
}

/**
 * Update/rename a placeholder value
 */
export async function updatePlaceholderValue(key: string, oldValue: string, newValue: string): Promise<void> {
  if (!newValue.trim()) {
    throw new Error("New value cannot be empty");
  }

  const data = await loadStorageData();
  const history = data.placeholderHistory || {};

  if (!history[key]) {
    throw new Error("Placeholder key not found");
  }

  const valueIndex = history[key].findIndex((item) => item.value === oldValue);
  if (valueIndex === -1) {
    throw new Error("Value not found");
  }

  // Check for duplicate
  const duplicateIndex = history[key].findIndex((item) => item.value === newValue);
  if (duplicateIndex !== -1 && duplicateIndex !== valueIndex) {
    throw new Error("A value with this name already exists");
  }

  history[key][valueIndex].value = newValue;

  data.placeholderHistory = history;
  await saveStorageData(data);
}

/**
 * Clear all values for a specific placeholder key
 */
export async function clearPlaceholderHistoryForKey(key: string): Promise<void> {
  const data = await loadStorageData();
  const history = data.placeholderHistory || {};

  delete history[key];

  data.placeholderHistory = history;
  await saveStorageData(data);
}

/**
 * Clear all placeholder history
 */
export async function clearAllPlaceholderHistory(): Promise<void> {
  const data = await loadStorageData();
  data.placeholderHistory = {};
  await saveStorageData(data);
}

export async function exportData(): Promise<ExportData> {
  const data = await loadStorageData();

  return {
    version: `${CURRENT_VERSION}.0.0`,
    exportedAt: Date.now(),
    snippets: data.snippets,
    tags: data.tags,
    placeholderHistory: data.placeholderHistory || {},
  };
}

export async function importData(importedData: ExportData, merge: boolean = false): Promise<void> {
  const currentData = await loadStorageData();

  if (merge) {
    // Merge snippets (avoid duplicates by ID)
    const existingIds = new Set(currentData.snippets.map((s) => s.id));
    const newSnippets = importedData.snippets.filter((s) => !existingIds.has(s.id));

    // Ensure imported snippets have required fields
    const sanitizedSnippets = newSnippets.map((s) => ({
      ...s,
      useCount: s.useCount || 0,
      tags: s.tags || [],
      isArchived: s.isArchived ?? false,
    }));

    currentData.snippets = [...currentData.snippets, ...sanitizedSnippets];

    // Tags are computed dynamically, no need to merge master list
    // Keep data.tags for backward compatibility but it won't be used

    // Merge placeholder history
    if (importedData.placeholderHistory) {
      const mergedHistory = { ...currentData.placeholderHistory };

      for (const [key, values] of Object.entries(importedData.placeholderHistory)) {
        if (!mergedHistory[key]) {
          mergedHistory[key] = values;
        } else {
          // Merge values, avoiding duplicates by value string
          const existingValues = new Set(mergedHistory[key].map((v) => v.value));
          const newValues = values.filter((v) => !existingValues.has(v.value));
          mergedHistory[key] = [...mergedHistory[key], ...newValues];

          // Enforce storage limit per key (always keep up to 100 values)
          if (mergedHistory[key].length > MAX_STORED_VALUES_PER_KEY) {
            // Sort by lastUsed and keep top 100 most recently used
            mergedHistory[key].sort((a, b) => b.lastUsed - a.lastUsed);
            mergedHistory[key] = mergedHistory[key].slice(0, MAX_STORED_VALUES_PER_KEY);
          }
        }
      }

      currentData.placeholderHistory = mergedHistory;
    }

    await saveStorageData(currentData);
  } else {
    // Replace all data - ensure imported snippets are properly formatted
    const sanitizedSnippets = importedData.snippets.map((s) => ({
      ...s,
      useCount: s.useCount || 0,
      tags: s.tags || [],
      isArchived: s.isArchived ?? false,
    }));

    const newData: StorageData = {
      version: CURRENT_VERSION,
      snippets: sanitizedSnippets,
      tags: [], // Empty tags list - will be computed dynamically
      placeholderHistory: importedData.placeholderHistory || {},
    };

    await saveStorageData(newData);
  }
}

export async function clearAllData(): Promise<void> {
  await LocalStorage.removeItem(STORAGE_KEY);
}

/**
 * Calculate total storage size in bytes
 */
export async function getStorageSize(): Promise<{ bytes: number; formatted: string; percentage: number }> {
  const storageJson = (await LocalStorage.getItem<string>(STORAGE_KEY)) || "{}";

  // Calculate size in bytes using Blob for accurate byte count
  const totalBytes = new Blob([storageJson]).size;

  // Format size
  let formatted: string;
  if (totalBytes < 1024) {
    formatted = `${totalBytes} bytes`;
  } else if (totalBytes < 1024 * 1024) {
    formatted = `${(totalBytes / 1024).toFixed(1)} KB`;
  } else {
    formatted = `${(totalBytes / (1024 * 1024)).toFixed(2)} MB`;
  }

  // Estimate percentage (LocalStorage typically has 5-10MB limit, we'll use 5MB as conservative estimate)
  const estimatedLimit = 5 * 1024 * 1024; // 5MB in bytes
  const percentage = (totalBytes / estimatedLimit) * 100;

  return { bytes: totalBytes, formatted, percentage };
}
