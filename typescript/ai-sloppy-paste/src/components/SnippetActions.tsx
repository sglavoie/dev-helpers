import { Action, Icon, showToast, Toast, useNavigation } from "@raycast/api";
import { Snippet } from "../types";
import { toggleFavorite, duplicateSnippet, toggleArchive, togglePin } from "../utils/storage";
import { findSimilarSnippets } from "../utils/analytics";
import { getErrorMessage } from "../utils/errorMessage";
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
      shortcut={{ modifiers: ["cmd"], key: "n" }}
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
      shortcut={{ modifiers: ["cmd"], key: "e" }}
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
        title: isFavorite ? "Added to Favorites" : "Removed from Favorites",
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
      title={props.snippet.isFavorite ? "Remove from Favorites" : "Add to Favorites"}
      icon={props.snippet.isFavorite ? Icon.StarDisabled : Icon.Star}
      shortcut={{ modifiers: ["cmd", "shift"], key: "f" }}
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

export function ManageTagsAction(props: { onUpdated: () => void }) {
  const { push } = useNavigation();

  return (
    <Action
      title="Manage Tags"
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
      shortcut={{ modifiers: ["cmd", "shift"], key: "h" }}
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
      shortcut={{ modifiers: ["cmd", "shift"], key: "i" }}
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
      title={`Find Similar Snippets (${results.length} found)`}
      icon={Icon.TwoArrowsClockwise}
      target={
        <SimilarSnippetsView target={props.snippet} allSnippets={props.allSnippets} onUpdated={props.onUpdated} />
      }
    />
  );
}
