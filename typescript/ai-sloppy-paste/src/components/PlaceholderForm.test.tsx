import { describe, it, expect, vi, beforeEach } from "vitest";
import { extractPlaceholders } from "../utils/placeholders";
import * as storage from "../utils/storage";

describe("PlaceholderForm storage integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should respect isSaved flag when saving values", async () => {
    const addSpy = vi.spyOn(storage, "addPlaceholderValue");

    // Simulate placeholders with different isSaved flags
    const text = "{{name}} and {{!date}}";
    const placeholders = extractPlaceholders(text);

    // Mock saving logic
    const values = { name: "Alice", date: "2025-10-30" };

    for (const placeholder of placeholders) {
      const value = values[placeholder.key];
      if (placeholder.isSaved && value && value.trim()) {
        await storage.addPlaceholderValue(placeholder.key, value);
      }
    }

    // Verify: name saved, date not saved
    expect(addSpy).toHaveBeenCalledTimes(1);
    expect(addSpy).toHaveBeenCalledWith("name", "Alice");
    expect(addSpy).not.toHaveBeenCalledWith("date", expect.anything());
  });
});
