import { Action, closeMainWindow, Icon, Keyboard, showHUD, showToast, Toast, useNavigation } from "@raycast/api";
import { useEffect, useState } from "react";
import { Snippet } from "../types";
import {
  toggleFavorite,
  duplicateSnippet,
  toggleArchive,
  togglePin,
  getPlaceholderHistoryForKey,
} from "../utils/storage";
import { findSimilarSnippets } from "../utils/analytics";
import { getErrorMessage } from "../utils/errorMessage";
import {
  extractPlaceholders,
  processSystemPlaceholders,
  replacePlaceholders,
  processConditionalBlocks,
  getSystemPlaceholderNames,
} from "../utils/placeholders";
import { getLastUsedValue } from "../utils/placeholderHistory";
import { pasteSnippet } from "../utils/clipboard";
import { runBestEffort, runSnippetAction } from "../utils/snippet-use";
import { SnippetForm } from "./SnippetForm";
import { ManageTagsView } from "./ManageTagsView";
import { ManagePlaceholderHistoryView } from "./ManagePlaceholderHistoryView";
import { ImportForm } from "./ImportForm";
import { QuickTagForm } from "./QuickTagForm";
import { SimilarSnippetsView } from "./SimilarSnippetsView";

export function CreateSnippetAction(props: { onCreated: () => void; tags: string[] }) {
  const { push } = useNavigation();

  return (
    <Action
      title="Create Snippet"
      icon={Icon.Plus}
      shortcut={Keyboard.Shortcut.Common.New}
      onAction={() => {
        push(<SnippetForm onSubmit={props.onCreated} tags={props.tags} />);
      }}
    />
  );
}

export function EditSnippetAction(props: { snippet: Snippet; onEdited: () => void; tags: string[] }) {
  const { push } = useNavigation();

  return (
    <Action
      title="Edit Snippet"
      icon={Icon.Pencil}
      shortcut={Keyboard.Shortcut.Common.Edit}
      onAction={() => {
        push(<SnippetForm snippet={props.snippet} onSubmit={props.onEdited} tags={props.tags} />);
      }}
    />
  );
}

export function ToggleFavoriteAction(props: { snippet: Snippet; onToggled: () => void }) {
  async function handleToggle() {
    try {
      const isFavorite = await toggleFavorite(props.snippet.id);
      showToast({
        style: Toast.Style.Success,
        title: isFavorite ? "Bookmarked" : "Removed bookmark",
      });
      props.onToggled();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to toggle favorite",
        message: getErrorMessage(error),
      });
    }
  }

  return (
    <Action
      title={props.snippet.isFavorite ? "Remove Bookmark" : "Add Bookmark"}
      icon={props.snippet.isFavorite ? Icon.StarDisabled : Icon.Star}
      shortcut={{ modifiers: ["cmd", "shift"], key: "v" }}
      onAction={handleToggle}
    />
  );
}

export function DuplicateSnippetAction(props: { snippet: Snippet; onDuplicated: () => void }) {
  async function handleDuplicate() {
    try {
      await duplicateSnippet(props.snippet.id);
      showToast({
        style: Toast.Style.Success,
        title: "Snippet duplicated",
        message: `Created "${props.snippet.title} (Copy)"`,
      });
      props.onDuplicated();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to duplicate snippet",
        message: getErrorMessage(error),
      });
    }
  }

  return (
    <Action
      title="Duplicate Snippet"
      icon={Icon.CopyClipboard}
      shortcut={{ modifiers: ["cmd", "shift"], key: "d" }}
      onAction={handleDuplicate}
    />
  );
}

export function ToggleArchiveAction(props: { snippet: Snippet; onToggled: () => void }) {
  async function handleToggle() {
    try {
      const isArchived = await toggleArchive(props.snippet.id);
      showToast({
        style: Toast.Style.Success,
        title: isArchived ? "Snippet archived" : "Snippet unarchived",
      });
      props.onToggled();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to toggle archive",
        message: getErrorMessage(error),
      });
    }
  }

  return (
    <Action
      title={props.snippet.isArchived ? "Unarchive Snippet" : "Archive Snippet"}
      icon={props.snippet.isArchived ? Icon.Tray : Icon.Box}
      shortcut={{ modifiers: ["cmd", "shift"], key: "a" }}
      onAction={handleToggle}
    />
  );
}

export function QuickAddTagAction(props: { snippet: Snippet; availableTags: string[]; onUpdated: () => void }) {
  const { push } = useNavigation();

  return (
    <Action
      title="Quick Add Tag"
      icon={Icon.PlusCircle}
      shortcut={{ modifiers: ["cmd", "shift"], key: "t" }}
      onAction={() => {
        push(
          <QuickTagForm
            snippet={props.snippet}
            availableTags={props.availableTags}
            mode="add"
            onUpdated={props.onUpdated}
          />,
        );
      }}
    />
  );
}

export function QuickRemoveTagAction(props: { snippet: Snippet; onUpdated: () => void }) {
  const { push } = useNavigation();

  if (props.snippet.tags.length === 0) {
    return null;
  }

  return (
    <Action
      title="Quick Remove Tag"
      icon={Icon.MinusCircle}
      shortcut={{ modifiers: ["cmd", "opt"], key: "t" }}
      onAction={() => {
        push(<QuickTagForm snippet={props.snippet} availableTags={[]} mode="remove" onUpdated={props.onUpdated} />);
      }}
    />
  );
}

export function TogglePinAction(props: { snippet: Snippet; onToggled: () => void }) {
  async function handleToggle() {
    try {
      const isPinned = await togglePin(props.snippet.id);
      showToast({
        style: Toast.Style.Success,
        title: isPinned ? "Snippet pinned" : "Snippet unpinned",
      });
      props.onToggled();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to toggle pin",
        message: getErrorMessage(error),
      });
    }
  }

  return (
    <Action
      title={props.snippet.isPinned ? "Unpin Snippet" : "Pin Snippet"}
      icon={props.snippet.isPinned ? Icon.PinDisabled : Icon.Pin}
      shortcut={{ modifiers: ["cmd", "shift"], key: "p" }}
      onAction={handleToggle}
    />
  );
}

export function ManageTagsAction(props: { onUpdated: () => void; unusedCount?: number }) {
  const { push } = useNavigation();
  const title = props.unusedCount ? `Manage Tags (${props.unusedCount} unused)` : "Manage Tags";

  return (
    <Action
      title={title}
      icon={Icon.Tag}
      shortcut={{ modifiers: ["cmd"], key: "t" }}
      onAction={() => {
        push(<ManageTagsView onUpdated={props.onUpdated} />);
      }}
    />
  );
}

export function ManagePlaceholderHistoryAction(props: { onUpdated: () => void }) {
  const { push } = useNavigation();

  return (
    <Action
      title="Manage Placeholder History"
      icon={Icon.Clock}
      onAction={() => {
        push(<ManagePlaceholderHistoryView onUpdated={props.onUpdated} />);
      }}
    />
  );
}

export function ImportDataAction(props: { onImported: () => void }) {
  const { push } = useNavigation();

  return (
    <Action
      title="Import Snippets"
      icon={Icon.Upload}
      onAction={() => {
        push(<ImportForm onImported={props.onImported} />);
      }}
    />
  );
}

export function SimilarSnippetsAction(props: { snippet: Snippet; allSnippets: Snippet[]; onUpdated: () => void }) {
  const results = findSimilarSnippets(props.snippet, props.allSnippets);
  if (results.length === 0) return null;
  return (
    <Action.Push
      title={`Find Similar Snippets (${results.length} Found)`}
      icon={Icon.TwoArrowsClockwise}
      target={
        <SimilarSnippetsView target={props.snippet} allSnippets={props.allSnippets} onUpdated={props.onUpdated} />
      }
    />
  );
}

export function PasteWithLastValuesAction(props: { snippet: Snippet; onComplete: () => void }) {
  const [hasRequiredPlaceholders, setHasRequiredPlaceholders] = useState(false);
  const [isAvailable, setIsAvailable] = useState(false);

  useEffect(() => {
    const systemKeys = new Set(getSystemPlaceholderNames());
    const processed = processSystemPlaceholders(props.snippet.content);
    const required = extractPlaceholders(processed).filter((p) => p.isRequired && !systemKeys.has(p.key));

    if (required.length === 0) {
      setHasRequiredPlaceholders(false);
      setIsAvailable(false);
      return;
    }

    setHasRequiredPlaceholders(true);
    let cancelled = false;
    (async () => {
      for (const placeholder of required) {
        const values = await getPlaceholderHistoryForKey(placeholder.key);
        if (cancelled) return;
        if (!getLastUsedValue(values)) {
          setIsAvailable(false);
          return;
        }
      }
      if (!cancelled) setIsAvailable(true);
    })();

    return () => {
      cancelled = true;
    };
  }, [props.snippet.content]);

  if (!hasRequiredPlaceholders) return null;

  const title = isAvailable ? "Paste with Last Values" : "Paste with Last Values (no history yet)";

  async function handlePaste() {
    const didComplete = await runSnippetAction({
      prepare: async () => {
        const systemKeys = new Set(getSystemPlaceholderNames());
        const processed = processSystemPlaceholders(props.snippet.content);
        const allPlaceholders = extractPlaceholders(processed);
        const required = allPlaceholders.filter((p) => p.isRequired && !systemKeys.has(p.key));
        const finalValues: Record<string, string> = {};

        for (const placeholder of required) {
          const values = await getPlaceholderHistoryForKey(placeholder.key);
          const lastValue = getLastUsedValue(values);
          if (lastValue === undefined) {
            throw new MissingPlaceholderHistoryError(placeholder.key);
          }
          finalValues[placeholder.key] = lastValue;
        }

        for (const placeholder of allPlaceholders) {
          if (!placeholder.isRequired && placeholder.defaultValue !== undefined && !(placeholder.key in finalValues)) {
            finalValues[placeholder.key] = placeholder.defaultValue;
          }
        }

        const afterBlocks = processConditionalBlocks(processed, finalValues);
        return {
          content: replacePlaceholders(afterBlocks, finalValues, allPlaceholders),
          placeholderValues: allPlaceholders
            .filter((placeholder) => !systemKeys.has(placeholder.key))
            .map((placeholder) => ({
              key: placeholder.key,
              value: finalValues[placeholder.key] ?? "",
              isSaved: placeholder.isSaved,
            })),
        };
      },
      primaryOperation: ({ content }) => pasteSnippet(content),
      snippetId: props.snippet.id,
      onPreparationFailure: (error) => {
        if (error instanceof MissingPlaceholderHistoryError) {
          return showToast({
            style: Toast.Style.Failure,
            title: "Missing history",
            message: `No history for {{${error.placeholderKey}}}`,
          });
        }
        return showToast({
          style: Toast.Style.Failure,
          title: "Failed to prepare paste",
          message: getErrorMessage(error),
        });
      },
      onPrimaryFailure: (error) =>
        showToast({ style: Toast.Style.Failure, title: "Failed to paste", message: getErrorMessage(error) }),
    });
    if (!didComplete) return;

    const truncatedTitle =
      props.snippet.title.length > 40 ? props.snippet.title.slice(0, 40) + "…" : props.snippet.title;
    await runBestEffort(() => closeMainWindow(), "Unable to close Raycast after last-values paste");
    await runBestEffort(
      () => showHUD(`✓ Pasted "${truncatedTitle}" with last values`),
      "Unable to show last-values HUD",
    );
    await runBestEffort(() => props.onComplete(), "Unable to refresh after last-values paste");
  }

  return (
    <Action
      title={title}
      icon={Icon.ArrowDownCircle}
      shortcut={{ modifiers: ["cmd", "shift"], key: "return" }}
      onAction={
        isAvailable
          ? handlePaste
          : () =>
              showToast({
                style: Toast.Style.Animated,
                title: "No history yet",
                message: "Use Paste (Cmd+Return) once to build history for this snippet.",
              })
      }
    />
  );
}

class MissingPlaceholderHistoryError extends Error {
  constructor(readonly placeholderKey: string) {
    super(`No history for {{${placeholderKey}}}`);
  }
}
