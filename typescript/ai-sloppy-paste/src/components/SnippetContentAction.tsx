import { Action, Clipboard, closeMainWindow, Icon, showToast, Toast, useNavigation } from "@raycast/api";
import type { Keyboard } from "@raycast/api";
import { Snippet } from "../types";
import { extractPlaceholders, processConditionalBlocks, processSystemPlaceholders } from "../utils/placeholders";
import { pasteWithClipboardRestore } from "../utils/clipboard";
import { incrementUsage } from "../utils/storage";
import { getErrorMessage } from "../utils/errorMessage";
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
    mode === "paste"
      ? { modifiers: ["cmd"], key: "return" }
      : { modifiers: ["cmd", "opt"], key: "return" };

  const config =
    mode === "paste"
      ? {
          title: "Paste Content",
          icon: Icon.ArrowDown,
          formMode: "paste-direct" as const,
          successTitle: "Pasted to frontmost app",
          failTitle: "Failed to paste",
        }
      : {
          title: "Copy Content",
          icon: Icon.Clipboard,
          formMode: "copy" as const,
          successTitle: "Copied to clipboard",
          failTitle: "Failed to copy",
        };

  async function handleAction() {
    const processed = processSystemPlaceholders(snippet.content);
    const placeholders = extractPlaceholders(processed);

    if (placeholders.length > 0) {
      push(
        <PlaceholderForm
          snippet={{ ...snippet, content: processed }}
          placeholders={placeholders}
          mode={config.formMode}
          onComplete={onComplete}
        />,
      );
    } else {
      try {
        const afterBlocks = processConditionalBlocks(processed, {});
        if (mode === "paste") {
          await pasteWithClipboardRestore(afterBlocks);
        } else {
          await Clipboard.copy(afterBlocks);
        }
        await incrementUsage(snippet.id);
        await closeMainWindow();
        showToast({ style: Toast.Style.Success, title: config.successTitle });
        onComplete();
      } catch (error) {
        showToast({ style: Toast.Style.Failure, title: config.failTitle, message: getErrorMessage(error) });
      }
    }
  }

  return <Action title={config.title} icon={config.icon} shortcut={shortcut} onAction={handleAction} />;
}
