// Mock implementation of @raycast/api for testing

import { vi } from "vitest";

const storage: Record<string, string> = {};

export const LocalStorage = {
  getItem: async (key: string) => {
    return storage[key];
  },
  setItem: async (key: string, value: string) => {
    storage[key] = value;
  },
  removeItem: async (key: string) => {
    delete storage[key];
  },
  clear: async () => {
    Object.keys(storage).forEach((key) => delete storage[key]);
  },
  // Export storage for test access
  _storage: storage,
};

// Controllable Raycast API mocks used by component and utility tests.
export const getPreferenceValues = vi.fn(() => ({}));
export const showToast = vi.fn(async () => {});
export const showHUD = vi.fn(async () => {});
export const closeMainWindow = vi.fn(async () => {});
export const popToRoot = vi.fn(async () => {});
export const Clipboard = {
  clear: vi.fn(async () => {}),
  copy: vi.fn(async () => {}),
  paste: vi.fn(async () => {}),
  read: vi.fn(async () => ({})),
  readText: vi.fn(async () => undefined),
};
