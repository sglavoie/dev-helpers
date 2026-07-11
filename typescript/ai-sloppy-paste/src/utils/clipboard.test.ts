import { Clipboard } from "@raycast/api";
import { describe, expect, it, vi } from "vitest";
import { pasteSnippet } from "./clipboard";

describe("pasteSnippet", () => {
  it("pastes the exact snippet content without reading, copying, clearing, or scheduling a timer", async () => {
    const timer = vi.spyOn(globalThis, "setTimeout");
    const content = "First line\\nSecond line\\n{{placeholder}}";

    await pasteSnippet(content);

    expect(Clipboard.paste).toHaveBeenCalledOnce();
    expect(Clipboard.paste).toHaveBeenCalledWith(content);
    expect(Clipboard.read).not.toHaveBeenCalled();
    expect(Clipboard.copy).not.toHaveBeenCalled();
    expect(Clipboard.clear).not.toHaveBeenCalled();
    expect(timer).not.toHaveBeenCalled();
    timer.mockRestore();
  });

  it("propagates clipboard paste failures", async () => {
    const error = new Error("Clipboard unavailable");
    vi.mocked(Clipboard.paste).mockRejectedValueOnce(error);

    await expect(pasteSnippet("content")).rejects.toThrow(error);
  });
});
