import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Clipboard } from "@raycast/api";
import { recordSnippetUse } from "./storage";
import { copySnippetContent, recordSnippetUseBestEffort, runSnippetAction } from "./snippet-use";

vi.mock("./storage", () => ({
  recordSnippetUse: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("recordSnippetUseBestEffort", () => {
  it("logs only safe metadata when tracking fails", async () => {
    const error = new Error("contains snippet content and a placeholder value");
    const logSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(recordSnippetUse).mockRejectedValueOnce(error);

    await expect(
      recordSnippetUseBestEffort("snippet-123", [{ key: "name", value: "Ada", isSaved: true }]),
    ).resolves.toBeUndefined();

    expect(logSpy).toHaveBeenCalledOnce();
    expect(logSpy).toHaveBeenCalledWith("Unable to record snippet use: snippet-123");
    expect(logSpy).not.toHaveBeenCalledWith(expect.stringContaining("Ada"));
  });
});

describe("runSnippetAction", () => {
  it("stops at a preparation failure before clipboard or tracking", async () => {
    const preparationError = new Error("Could not prepare");
    const primaryOperation = vi.fn();
    const onPreparationFailure = vi.fn();

    await expect(
      runSnippetAction({
        prepare: () => {
          throw preparationError;
        },
        primaryOperation,
        snippetId: "snippet-123",
        onPreparationFailure,
        onPrimaryFailure: vi.fn(),
      }),
    ).resolves.toBe(false);

    expect(onPreparationFailure).toHaveBeenCalledWith(preparationError);
    expect(primaryOperation).not.toHaveBeenCalled();
    expect(recordSnippetUse).not.toHaveBeenCalled();
  });

  it("stops at a clipboard failure before tracking", async () => {
    const clipboardError = new Error("Clipboard unavailable");
    const onPrimaryFailure = vi.fn();

    await expect(
      runSnippetAction({
        prepare: () => "prepared content",
        primaryOperation: vi.fn().mockRejectedValueOnce(clipboardError),
        snippetId: "snippet-123",
        onPreparationFailure: vi.fn(),
        onPrimaryFailure,
      }),
    ).resolves.toBe(false);

    expect(onPrimaryFailure).toHaveBeenCalledWith(clipboardError);
    expect(recordSnippetUse).not.toHaveBeenCalled();
  });

  it("tracks exactly once after the one successful primary operation", async () => {
    const primaryOperation = vi.fn().mockResolvedValue(undefined);
    vi.mocked(recordSnippetUse).mockResolvedValueOnce(undefined);

    await expect(
      runSnippetAction({
        prepare: () => "prepared content",
        primaryOperation,
        snippetId: "snippet-123",
        placeholderValues: [{ key: "name", value: "Ada", isSaved: true }],
        onPreparationFailure: vi.fn(),
        onPrimaryFailure: vi.fn(),
      }),
    ).resolves.toBe(true);

    expect(primaryOperation).toHaveBeenCalledOnce();
    expect(recordSnippetUse).toHaveBeenCalledOnce();
    expect(recordSnippetUse).toHaveBeenCalledWith("snippet-123", [{ key: "name", value: "Ada", isSaved: true }]);
  });
});

describe("copySnippetContent", () => {
  it("copies stored content and records one use", async () => {
    vi.mocked(recordSnippetUse).mockResolvedValueOnce(undefined);

    await expect(
      copySnippetContent({
        snippetId: "snippet-123",
        content: "stored snippet content",
        onPrimaryFailure: vi.fn(),
      }),
    ).resolves.toBe(true);

    expect(Clipboard.copy).toHaveBeenCalledOnce();
    expect(Clipboard.copy).toHaveBeenCalledWith("stored snippet content");
    expect(recordSnippetUse).toHaveBeenCalledOnce();
    expect(recordSnippetUse).toHaveBeenCalledWith("snippet-123", []);
  });

  it("reports a clipboard failure without recording a use", async () => {
    const error = new Error("Clipboard unavailable");
    const onPrimaryFailure = vi.fn();
    vi.mocked(Clipboard.copy).mockRejectedValueOnce(error);

    await expect(
      copySnippetContent({
        snippetId: "snippet-123",
        content: "stored snippet content",
        onPrimaryFailure,
      }),
    ).resolves.toBe(false);

    expect(onPrimaryFailure).toHaveBeenCalledWith(error);
    expect(recordSnippetUse).not.toHaveBeenCalled();
  });
});
