import type { StorageData } from "../types";
import { deduplicateTags, normalizeTags } from "./tags";
import { CURRENT_VERSION } from "./storage-constants";

type Migration = (data: any) => any;

const MIGRATIONS: Record<number, Migration> = {
  1: (data: any) => ({
    ...data,
    version: 2,
  }),
  2: (data: any) => ({
    ...data,
    version: 3,
    snippets: data.snippets.map((snippet: any) => ({
      ...snippet,
      isArchived: snippet.isArchived ?? false,
    })),
  }),
  3: (data: any) => ({
    ...data,
    version: 4,
    snippets: data.snippets.map((snippet: any) => ({
      ...snippet,
      tags: deduplicateTags(normalizeTags(snippet.tags || [])),
    })),
  }),
  4: (data: any) => ({
    ...data,
    version: 5,
    placeholderHistory: {},
  }),
  5: (data: any) => ({
    ...data,
    version: 6,
    snippets: data.snippets.map((snippet: any) => ({
      ...snippet,
      description: snippet.description ?? "",
    })),
  }),
  6: (data: any) => ({
    ...data,
    version: 7,
    snippets: data.snippets.map((snippet: any) => ({
      ...snippet,
      isPinned: snippet.isPinned ?? false,
    })),
  }),
};

export function migrateStorageData(rawData: any): { data: StorageData; didMigrate: boolean } {
  let data = rawData;
  let didMigrate = false;

  while (data.version && data.version < CURRENT_VERSION) {
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
