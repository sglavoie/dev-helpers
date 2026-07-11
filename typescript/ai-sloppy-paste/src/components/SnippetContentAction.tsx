import { Action, Clipboard, closeMainWindow, Icon, showHUD, showToast, Toast, useNavigation } from "@raycast/api";
import type { Keyboard } from "@raycast/api";
import { Snippet } from "../types";
import { extractPlaceholders, processConditionalBlocks, processSystemPlaceholders } from "../utils/placeholders";
import { pasteSnippet } from "../utils/clipboard";
import { getErrorMessage } from "../utils/errorMessage";
import { runBestEffort, runSnippetAction } from "../utils/snippet-use";
import { PlaceholderForm } from "./PlaceholderForm";

type ContentActionMode = "paste" | "copy";

interface SnippetContentActionProps {
  snippet: Snippet;
  mode: ContentActionMode;
  onComplete: () => void;
}

export function SnippetContentAction({ snippet, mode, onComplete }: SnippetContentActionProps) {
  const { push } = useNavigation();

  const shortcut: Keyboard.Shortcut =
    mode === "paste" ? { modifiers: ["cmd"], key: "return" } : { modifiers: ["cmd", "opt"], key: "return" };

  const config =
    mode === "paste"
      ? {
          title: "Paste Content",
          icon: Icon.ArrowDown,
          formMode: "paste-direct" as const,
          successTitle: "Pasted to frontmost app",
          failTitle: "Failed to paste",
          preparationFailTitle: "Failed to prepare paste",
        }
      : {
          title: "Copy Content",
          icon: Icon.Clipboard,
          formMode: "copy" as const,
          successTitle: "Copied to clipboard",
          failTitle: "Failed to copy",
          preparationFailTitle: "Failed to prepare copy",
        };

  async function handleAction() {
    let processed: string;
    let placeholders;
    try {
      processed = processSystemPlaceholders(snippet.content);
      placeholders = extractPlaceholders(processed);
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: config.preparationFailTitle,
        message: getErrorMessage(error),
      });
      return;
    }

    if (placeholders.length > 0) {
      try {
        push(
          <PlaceholderForm
            snippet={{ ...snippet, content: processed }}
            placeholders={placeholders}
            mode={config.formMode}
            onComplete={onComplete}
          />,
        );
      } catch (error) {
        await showToast({
          style: Toast.Style.Failure,
          title: config.preparationFailTitle,
          message: getErrorMessage(error),
        });
      }
      return;
    } else {
      const didComplete = await runSnippetAction({
        prepare: () => processConditionalBlocks(processed, {}),
        primaryOperation: (content) => (mode === "paste" ? pasteSnippet(content) : Clipboard.copy(content)),
        snippetId: snippet.id,
        onPreparationFailure: (error) =>
          showToast({
            style: Toast.Style.Failure,
            title: config.preparationFailTitle,
            message: getErrorMessage(error),
          }),
        onPrimaryFailure: (error) =>
          showToast({ style: Toast.Style.Failure, title: config.failTitle, message: getErrorMessage(error) }),
      });
      if (!didComplete) return;

      const truncatedTitle = snippet.title.length > 40 ? snippet.title.slice(0, 40) + "…" : snippet.title;
      await runBestEffort(() => closeMainWindow(), "Unable to close Raycast after snippet action");
      await runBestEffort(
        () => showHUD(mode === "paste" ? `✓ Pasted "${truncatedTitle}"` : `✓ Copied "${truncatedTitle}" to clipboard`),
        "Unable to show snippet action HUD",
      );
      await runBestEffort(() => onComplete(), "Unable to refresh after snippet action");
    }
  }

  return <Action title={config.title} icon={config.icon} shortcut={shortcut} onAction={handleAction} />;
}
