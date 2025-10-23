// Mock implementation of @raycast/api for testing

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

// Export other commonly used mocks
export const showToast = async () => {};
export const closeMainWindow = async () => {};
export const Clipboard = {
  copy: async () => {},
  paste: async () => {},
};
