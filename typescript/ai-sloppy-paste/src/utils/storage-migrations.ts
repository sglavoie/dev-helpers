import type { StorageData } from "../types";
import { deduplicateTags, normalizeTags } from "./tags";
import { CURRENT_VERSION } from "./storage-constants";

type LegacySnippet = Partial<Omit<StorageData["snippets"][number], "tags">> & {
  category?: unknown;
  tags?: unknown;
  [key: string]: unknown;
};

type LegacyStorageData = Partial<Omit<StorageData, "snippets" | "version">> & {
  version?: number;
  snippets?: LegacySnippet[];
  [key: string]: unknown;
};

type Migration = (data: LegacyStorageData) => LegacyStorageData;

function isLegacyStorageData(data: unknown): data is LegacyStorageData {
  return typeof data === "object" && data !== null;
}

function getSnippets(data: LegacyStorageData): LegacySnippet[] {
  return Array.isArray(data.snippets) ? data.snippets : [];
}

function getSnippetTags(snippet: LegacySnippet): string[] {
  if (Array.isArray(snippet.tags)) {
    return snippet.tags.filter((tag): tag is string => typeof tag === "string");
  }
  return typeof snippet.category === "string" ? [snippet.category] : [];
}

function normalizeSnippetShape(snippet: LegacySnippet): LegacySnippet {
  const snippetWithoutLegacyCategory = { ...snippet };
  delete snippetWithoutLegacyCategory.category;

  return {
    ...snippetWithoutLegacyCategory,
    tags: deduplicateTags(normalizeTags(getSnippetTags(snippet))),
    useCount: snippet.useCount ?? 0,
    isFavorite: snippet.isFavorite ?? false,
    isArchived: snippet.isArchived ?? false,
    isPinned: snippet.isPinned ?? false,
    description: snippet.description ?? "",
  };
}

const MIGRATIONS: Record<number, Migration> = {
  1: (data) => ({
    ...data,
    version: 2,
    snippets: getSnippets(data).map(normalizeSnippetShape),
  }),
  2: (data) => ({
    ...data,
    version: 3,
    snippets: getSnippets(data).map((snippet) => ({
      ...snippet,
      isArchived: snippet.isArchived ?? false,
    })),
  }),
  3: (data) => ({
    ...data,
    version: 4,
    snippets: getSnippets(data).map((snippet) => ({
      ...snippet,
      tags: deduplicateTags(normalizeTags(getSnippetTags(snippet))),
    })),
  }),
  4: (data) => ({
    ...data,
    version: 5,
    placeholderHistory: {},
  }),
  5: (data) => ({
    ...data,
    version: 6,
    snippets: getSnippets(data).map((snippet) => ({
      ...snippet,
      description: snippet.description ?? "",
    })),
  }),
  6: (data) => ({
    ...data,
    version: 7,
    snippets: getSnippets(data).map(normalizeSnippetShape),
  }),
};

export function migrateStorageData(rawData: unknown): { data: StorageData; didMigrate: boolean } {
  let data: LegacyStorageData = isLegacyStorageData(rawData) ? rawData : {};
  let didMigrate = false;

  while (typeof data.version === "number" && data.version < CURRENT_VERSION) {
    const migration = MIGRATIONS[data.version];
    if (!migration) {
      console.warn(`No migration found for version ${data.version}`);
      break;
    }

    console.log(`Migrating data from v${data.version} to v${data.version + 1}`);
    data = migration(data);
    didMigrate = true;
  }

  if (data.version !== CURRENT_VERSION) {
    data = {
      ...data,
      version: CURRENT_VERSION,
    };
    didMigrate = true;
  }

  return { data: data as StorageData, didMigrate };
}
