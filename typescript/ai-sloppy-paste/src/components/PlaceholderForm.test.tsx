import { Clipboard, LocalStorage } from "@raycast/api";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Placeholder } from "../types";
import { addSnippet, getPlaceholderHistoryForKey, getSnippets } from "../utils/storage";
import {
  buildRequiredPlaceholderErrors,
  buildTrackedPlaceholderValues,
  CUSTOM_VALUE_MARKER,
  getAuthoredChoiceOptionId,
  getAuthoredChoiceValue,
  initializeAuthoredChoiceState,
  resolveAuthoredChoiceSelection,
  submitPlaceholderForm,
} from "./PlaceholderForm";

function makeChoicePlaceholder(overrides: Partial<Placeholder> = {}): Placeholder & { choices: string[] } {
  return {
    key: "tone",
    choices: ["Formal", "Casual", "Technical"],
    isRequired: true,
    isSaved: true,
    ...overrides,
  };
}

describe("authored choice form state", () => {
  it("selects the first choice when no explicit default exists", () => {
    expect(initializeAuthoredChoiceState(makeChoicePlaceholder())).toMatchObject({
      formValue: "Formal",
      dropdownSelection: getAuthoredChoiceOptionId(0),
      customValue: "",
      useCustomInput: false,
    });
  });

  it("selects an exact matching explicit default", () => {
    expect(
      initializeAuthoredChoiceState(makeChoicePlaceholder({ defaultValue: "Casual", isRequired: false })),
    ).toMatchObject({
      formValue: "Casual",
      dropdownSelection: getAuthoredChoiceOptionId(1),
      useCustomInput: false,
    });
  });

  it.each(["Conversational", ""])("uses Custom for the explicit out-of-list default %j", (defaultValue) => {
    expect(initializeAuthoredChoiceState(makeChoicePlaceholder({ defaultValue, isRequired: false }))).toMatchObject({
      formValue: defaultValue,
      dropdownSelection: CUSTOM_VALUE_MARKER,
      customValue: defaultValue,
      useCustomInput: true,
    });
  });

  it("uses collision-safe IDs while preserving authored order and exact values", () => {
    const choices = [CUSTOM_VALUE_MARKER, getAuthoredChoiceOptionId(0), "A|B"];
    const ids = choices.map((_, index) => getAuthoredChoiceOptionId(index));

    expect(ids).toEqual([getAuthoredChoiceOptionId(0), getAuthoredChoiceOptionId(1), getAuthoredChoiceOptionId(2)]);
    expect(ids).not.toContain(CUSTOM_VALUE_MARKER);
    expect(ids.map((id) => getAuthoredChoiceValue(choices, id))).toEqual(choices);
    expect(getAuthoredChoiceValue(choices, "not-an-option-id")).toBeUndefined();
  });

  it("preserves custom text while switching between authored and Custom modes", () => {
    const choices = ["Formal", "Casual"];
    const custom = resolveAuthoredChoiceSelection(choices, CUSTOM_VALUE_MARKER, "Conversational");
    expect(custom).toMatchObject({ formValue: "Conversational", customValue: "Conversational", useCustomInput: true });

    const authored = resolveAuthoredChoiceSelection(choices, getAuthoredChoiceOptionId(1), custom?.customValue ?? "");
    expect(authored).toMatchObject({ formValue: "Casual", customValue: "Conversational", useCustomInput: false });

    expect(resolveAuthoredChoiceSelection(choices, CUSTOM_VALUE_MARKER, authored?.customValue ?? "")).toMatchObject({
      formValue: "Conversational",
      customValue: "Conversational",
      useCustomInput: true,
    });
  });

  it("keeps a wrapper without a default disabled while retaining its first choice", () => {
    expect(
      initializeAuthoredChoiceState(
        makeChoicePlaceholder({ isRequired: false, prefixWrapper: "(", suffixWrapper: ")" }),
      ),
    ).toMatchObject({
      formValue: "Formal",
      dropdownSelection: getAuthoredChoiceOptionId(0),
      enabledOptional: false,
    });
  });

  it("enables a wrapper with a non-empty default", () => {
    expect(
      initializeAuthoredChoiceState(
        makeChoicePlaceholder({
          defaultValue: "Casual",
          isRequired: false,
          prefixWrapper: "(",
          suffixWrapper: ")",
        }),
      ),
    ).toMatchObject({
      formValue: "Casual",
      dropdownSelection: getAuthoredChoiceOptionId(1),
      enabledOptional: true,
    });
  });

  it("reports a required field error when Custom is blank", () => {
    const placeholder = makeChoicePlaceholder();
    const custom = resolveAuthoredChoiceSelection(placeholder.choices, CUSTOM_VALUE_MARKER, "");

    expect(buildRequiredPlaceholderErrors([placeholder], { tone: custom?.formValue ?? "" })).toEqual({
      tone: "This field is required",
    });
  });
});

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

  it("records authored and custom choice values while honoring the no-save flag", async () => {
    const snippet = await addSnippet({
      title: "Choices",
      content: "{{tone[Formal|Casual]}} / {{detail[Short|Long]|Other}} / {{!private[Yes|No]|No}}",
      tags: [],
    });
    const choicePlaceholders: Placeholder[] = [
      { key: "tone", choices: ["Formal", "Casual"], isRequired: true, isSaved: true },
      { key: "detail", choices: ["Short", "Long"], defaultValue: "Other", isRequired: false, isSaved: true },
      { key: "private", choices: ["Yes", "No"], defaultValue: "No", isRequired: false, isSaved: false },
    ];
    vi.mocked(Clipboard.copy).mockResolvedValueOnce();

    await expect(
      submitPlaceholderForm({
        snippet,
        placeholders: choicePlaceholders,
        finalValues: { tone: "Formal", detail: "Conversational", private: "Yes" },
        mode: "copy",
        onPreparationFailure: vi.fn(),
        onPrimaryFailure: vi.fn(),
      }),
    ).resolves.toBe(true);

    expect(Clipboard.copy).toHaveBeenCalledWith("Formal / Conversational / Yes");
    expect(await getPlaceholderHistoryForKey("tone")).toMatchObject([{ value: "Formal", useCount: 1 }]);
    expect(await getPlaceholderHistoryForKey("detail")).toMatchObject([{ value: "Conversational", useCount: 1 }]);
    expect(await getPlaceholderHistoryForKey("private")).toEqual([]);
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
