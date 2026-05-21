import type { PlaceholderHistory, PlaceholderHistoryValue, StorageData } from "../types";
import { MAX_STORED_VALUES_PER_KEY } from "./storage-constants";

export function getPlaceholderHistoryFromData(data: StorageData): PlaceholderHistory {
  return data.placeholderHistory || {};
}

export function getPlaceholderHistoryForKeyFromData(data: StorageData, key: string): PlaceholderHistoryValue[] {
  return getPlaceholderHistoryFromData(data)[key] || [];
}

export function getAllPlaceholderKeysFromData(data: StorageData): string[] {
  return Object.keys(getPlaceholderHistoryFromData(data)).sort((a, b) => a.localeCompare(b));
}

export function addPlaceholderValueToData(data: StorageData, key: string, value: string): boolean {
  if (!key || !value.trim()) {
    return false;
  }

  const history = data.placeholderHistory || {};
  if (!history[key]) {
    history[key] = [];
  }

  const now = Date.now();
  const existingIndex = history[key].findIndex((item) => item.value === value);

  if (existingIndex !== -1) {
    history[key][existingIndex].useCount += 1;
    history[key][existingIndex].lastUsed = now;
  } else {
    history[key].push({
      value,
      useCount: 1,
      lastUsed: now,
      createdAt: now,
    });

    if (history[key].length > MAX_STORED_VALUES_PER_KEY) {
      history[key].sort((a, b) => a.lastUsed - b.lastUsed);
      history[key].shift();
    }
  }

  data.placeholderHistory = history;
  return true;
}

export function updatePlaceholderValueUsageInData(data: StorageData, key: string, value: string): boolean {
  const history = data.placeholderHistory || {};
  if (!history[key]) {
    return false;
  }

  const valueIndex = history[key].findIndex((item) => item.value === value);
  if (valueIndex === -1) {
    return false;
  }

  history[key][valueIndex].useCount += 1;
  history[key][valueIndex].lastUsed = Date.now();
  data.placeholderHistory = history;
  return true;
}

export function deletePlaceholderValueFromData(data: StorageData, key: string, value: string): boolean {
  const history = data.placeholderHistory || {};
  if (!history[key]) {
    return false;
  }

  history[key] = history[key].filter((item) => item.value !== value);
  if (history[key].length === 0) {
    delete history[key];
  }

  data.placeholderHistory = history;
  return true;
}

export function updatePlaceholderValueInData(
  data: StorageData,
  key: string,
  oldValue: string,
  newValue: string,
): boolean {
  if (!newValue.trim()) {
    throw new Error("New value cannot be empty");
  }

  const history = data.placeholderHistory || {};
  if (!history[key]) {
    throw new Error("Placeholder key not found");
  }

  const valueIndex = history[key].findIndex((item) => item.value === oldValue);
  if (valueIndex === -1) {
    throw new Error("Value not found");
  }

  const duplicateIndex = history[key].findIndex((item) => item.value === newValue);
  if (duplicateIndex !== -1 && duplicateIndex != valueIndex) {
    throw new Error("A value with this name already exists");
  }

  history[key][valueIndex].value = newValue;
  data.placeholderHistory = history;
  return true;
}

export function clearPlaceholderHistoryForKeyInData(data: StorageData, key: string): boolean {
  const history = data.placeholderHistory || {};
  delete history[key];
  data.placeholderHistory = history;
  return true;
}

export function clearAllPlaceholderHistoryInData(data: StorageData): boolean {
  data.placeholderHistory = {};
  return true;
}

export function mergePlaceholderHistory(
  currentHistory: PlaceholderHistory | undefined,
  importedHistory: PlaceholderHistory,
): PlaceholderHistory {
  const mergedHistory: PlaceholderHistory = { ...(currentHistory || {}) };

  for (const [key, values] of Object.entries(importedHistory)) {
    if (!mergedHistory[key]) {
      mergedHistory[key] = values;
      continue;
    }

    const existingValues = new Set(mergedHistory[key].map((item) => item.value));
    const newValues = values.filter((item) => !existingValues.has(item.value));
    mergedHistory[key] = [...mergedHistory[key], ...newValues];

    if (mergedHistory[key].length > MAX_STORED_VALUES_PER_KEY) {
      mergedHistory[key].sort((a, b) => b.lastUsed - a.lastUsed);
      mergedHistory[key] = mergedHistory[key].slice(0, MAX_STORED_VALUES_PER_KEY);
    }
  }

  return mergedHistory;
}
