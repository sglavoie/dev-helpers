import { Clipboard, LocalStorage } from "@raycast/api";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Placeholder } from "../types";
import { addSnippet, getPlaceholderHistoryForKey, getSnippets } from "../utils/storage";
import { buildTrackedPlaceholderValues, submitPlaceholderForm } from "./PlaceholderForm";

const placeholders: Placeholder[] = [
  { key: "required", isRequired: true, isSaved: true },
  { key: "optional", defaultValue: "fallback", isRequired: false, isSaved: true },
  { key: "wrapped", isRequired: false, isSaved: true, prefixWrapper: "[", suffixWrapper: "]" },
  { key: "guard", isRequired: false, isSaved: false, isGuardOnly: true },
  { key: "private", isRequired: false, isSaved: false },
  { key: "blank", isRequired: false, isSaved: true },
];

const submittedValues = {
  required: "Ada",
  optional: "fallback",
  wrapped: "",
  guard: "true",
  private: "secret",
  blank: "  ",
};

describe("PlaceholderForm submission", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("preserves normalized values and parsed metadata for batched tracking", () => {
    expect(buildTrackedPlaceholderValues(placeholders, submittedValues)).toEqual([
      { key: "required", value: "Ada", isSaved: true },
      { key: "optional", value: "fallback", isSaved: true },
      { key: "wrapped", value: "", isSaved: true },
      { key: "guard", value: "true", isSaved: false },
      { key: "private", value: "secret", isSaved: false },
      { key: "blank", value: "  ", isSaved: true },
    ]);
  });

  it("copies once and records the snippet and eligible values with one post-copy write", async () => {
    const snippet = await addSnippet({ title: "Test", content: "Hello {{required}}", tags: [] });
    const setItemSpy = vi.spyOn(LocalStorage, "setItem");
    vi.mocked(Clipboard.copy).mockResolvedValueOnce();

    await expect(
      submitPlaceholderForm({
        snippet,
        placeholders,
        finalValues: submittedValues,
        mode: "copy",
        onPreparationFailure: vi.fn(),
        onPrimaryFailure: vi.fn(),
      }),
    ).resolves.toBe(true);

    expect(Clipboard.copy).toHaveBeenCalledOnce();
    expect(Clipboard.copy).toHaveBeenCalledWith("Hello Ada");
    expect(setItemSpy).toHaveBeenCalledOnce();
    expect(await getSnippets()).toMatchObject([{ id: snippet.id, useCount: 1 }]);
    expect(await getPlaceholderHistoryForKey("required")).toMatchObject([{ value: "Ada", useCount: 1 }]);
    expect(await getPlaceholderHistoryForKey("optional")).toMatchObject([{ value: "fallback", useCount: 1 }]);
    expect(await getPlaceholderHistoryForKey("wrapped")).toEqual([]);
    expect(await getPlaceholderHistoryForKey("guard")).toEqual([]);
    expect(await getPlaceholderHistoryForKey("private")).toEqual([]);
    expect(await getPlaceholderHistoryForKey("blank")).toEqual([]);
  });

  it("does not track a submission when copying fails", async () => {
    const snippet = await addSnippet({ title: "Test", content: "Hello {{required}}", tags: [] });
    const setItemSpy = vi.spyOn(LocalStorage, "setItem");
    const clipboardError = new Error("Clipboard unavailable");
    const onPrimaryFailure = vi.fn();
    vi.mocked(Clipboard.copy).mockRejectedValueOnce(clipboardError);

    await expect(
      submitPlaceholderForm({
        snippet,
        placeholders,
        finalValues: submittedValues,
        mode: "copy",
        onPreparationFailure: vi.fn(),
        onPrimaryFailure,
      }),
    ).resolves.toBe(false);

    expect(Clipboard.copy).toHaveBeenCalledOnce();
    expect(onPrimaryFailure).toHaveBeenCalledWith(clipboardError);
    expect(setItemSpy).not.toHaveBeenCalled();
    expect(await getSnippets()).toMatchObject([{ id: snippet.id, useCount: 0 }]);
    expect(await getPlaceholderHistoryForKey("required")).toEqual([]);
  });
});
