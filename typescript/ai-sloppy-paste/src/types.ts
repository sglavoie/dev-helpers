export interface Snippet {
  id: string;
  title: string;
  content: string;
  tags: string[];
  createdAt: number;
  updatedAt: number;
  lastUsedAt?: number;
  useCount: number;
  isFavorite: boolean;
  isArchived: boolean;
}

export interface StorageData {
  version: number;
  snippets: Snippet[];
  tags: string[];
}

export interface SnippetFormValues {
  title: string;
  content: string;
  tags: string; // Comma-separated tag string from TextField
}

export interface ExportData {
  version: string;
  exportedAt: number;
  snippets: Snippet[];
  tags: string[];
}

export interface Placeholder {
  key: string;
  defaultValue?: string;
  isRequired: boolean;
}

export enum SortOption {
  UpdatedDesc = "updated-desc",
  MostUsedDesc = "most-used-desc",
  MostUsedAsc = "most-used-asc",
  Alphabetical = "alphabetical",
  LastUsed = "last-used",
  CreatedDesc = "created-desc",
}

export const SORT_LABELS: Record<SortOption, string> = {
  [SortOption.UpdatedDesc]: "Recently Updated",
  [SortOption.MostUsedDesc]: "Most Used",
  [SortOption.MostUsedAsc]: "Least Used",
  [SortOption.Alphabetical]: "Alphabetical (A-Z)",
  [SortOption.LastUsed]: "Recently Used",
  [SortOption.CreatedDesc]: "Recently Created",
};
