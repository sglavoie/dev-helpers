import { Clipboard } from "@raycast/api";
import { Snippet, Placeholder } from "../types";
import { pasteSnippet } from "./clipboard";
import { replacePlaceholders, processConditionalBlocks } from "./placeholders";
import { PlaceholderValueToRecord } from "./storage";
import { runSnippetAction } from "./snippet-use";

interface PlaceholderFormSubmission {
  snippet: Snippet;
  placeholders: Placeholder[];
  finalValues: Record<string, string>;
  mode: "copy" | "paste" | "paste-direct";
  onPreparationFailure: (error: unknown) => unknown | Promise<unknown>;
  onPrimaryFailure: (error: unknown) => unknown | Promise<unknown>;
}

/**
 * Retains the submitted value and parsed placeholder metadata for the atomic
 * storage operation. recordSnippetUse owns the no-save and blank exclusions.
 */
export function buildTrackedPlaceholderValues(
  placeholders: Placeholder[],
  finalValues: Record<string, string>,
): PlaceholderValueToRecord[] {
  return placeholders.map((placeholder) => ({
    key: placeholder.key,
    value: finalValues[placeholder.key] ?? "",
    isSaved: placeholder.isSaved,
  }));
}

export async function submitPlaceholderForm({
  snippet,
  placeholders,
  finalValues,
  mode,
  onPreparationFailure,
  onPrimaryFailure,
}: PlaceholderFormSubmission): Promise<boolean> {
  return runSnippetAction({
    prepare: () => {
      const afterBlocks = processConditionalBlocks(snippet.content, finalValues);
      return replacePlaceholders(afterBlocks, finalValues, placeholders);
    },
    primaryOperation: (content) => (mode === "paste-direct" ? pasteSnippet(content) : Clipboard.copy(content)),
    snippetId: snippet.id,
    placeholderValues: buildTrackedPlaceholderValues(placeholders, finalValues),
    onPreparationFailure,
    onPrimaryFailure,
  });
}
