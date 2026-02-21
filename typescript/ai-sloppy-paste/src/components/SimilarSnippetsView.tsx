import { Action, ActionPanel, Alert, Clipboard, Color, confirmAlert, Icon, List, showToast, Toast } from "@raycast/api";
import { useState, useMemo } from "react";
import { Snippet } from "../types";
import { findSimilarSnippets } from "../utils/analytics";
import { deleteSnippet, toggleArchive } from "../utils/storage";

interface SimilarSnippetsViewProps {
  target: Snippet;
  allSnippets: Snippet[];
  onUpdated: () => void;
}

export function SimilarSnippetsView({ target, allSnippets, onUpdated }: SimilarSnippetsViewProps) {
  const [localSnippets, setLocalSnippets] = useState(allSnippets);

  const results = useMemo(() => findSimilarSnippets(target, localSnippets), [target, localSnippets]);

  async function handleArchive(snippet: Snippet) {
    try {
      await toggleArchive(snippet.id);
      setLocalSnippets((prev) => prev.map((s) => (s.id === snippet.id ? { ...s, isArchived: true } : s)));
      onUpdated();
      showToast({ style: Toast.Style.Success, title: "Snippet archived" });
    } catch (error) {
      showToast({ style: Toast.Style.Failure, title: "Failed to archive snippet", message: String(error) });
    }
  }

  async function handleDelete(snippet: Snippet) {
    const confirmed = await confirmAlert({
      title: "Delete Snippet",
      message: `Permanently delete "${snippet.title}"? This cannot be undone.`,
      primaryAction: { title: "Delete", style: Alert.ActionStyle.Destructive },
    });
    if (confirmed) {
      try {
        await deleteSnippet(snippet.id);
        setLocalSnippets((prev) => prev.filter((s) => s.id !== snippet.id));
        onUpdated();
        showToast({ style: Toast.Style.Success, title: "Snippet deleted" });
      } catch (error) {
        showToast({ style: Toast.Style.Failure, title: "Failed to delete snippet", message: String(error) });
      }
    }
  }

  function getMatchColor(score: number): Color {
    if (score >= 0.6) return Color.Red;
    if (score >= 0.45) return Color.Orange;
    return Color.SecondaryText;
  }

  return (
    <List navigationTitle={`Similar to: ${target.title}`}>
      {results.length === 0 ? (
        <List.EmptyView icon={Icon.Checkmark} title="No similar snippets found" />
      ) : (
        results.map(({ snippet, score }) => (
          <List.Item
            key={snippet.id}
            icon={snippet.isPinned ? Icon.Pin : snippet.isFavorite ? Icon.Star : Icon.Document}
            title={snippet.title}
            subtitle={snippet.content.slice(0, 60)}
            accessories={[
              {
                tag: {
                  value: `${Math.round(score * 100)}% match`,
                  color: getMatchColor(score),
                },
              },
            ]}
            actions={
              <ActionPanel>
                <ActionPanel.Section title="Snippet Actions">
                  <Action
                    title="Copy Content"
                    icon={Icon.Clipboard}
                    onAction={async () => {
                      await Clipboard.copy(snippet.content);
                      showToast({ style: Toast.Style.Success, title: "Content copied" });
                    }}
                  />
                  <Action title="Archive Snippet" icon={Icon.Box} onAction={() => handleArchive(snippet)} />
                  <Action
                    title="Delete Snippet"
                    icon={Icon.Trash}
                    style={Action.Style.Destructive}
                    onAction={() => handleDelete(snippet)}
                  />
                </ActionPanel.Section>
              </ActionPanel>
            }
          />
        ))
      )}
    </List>
  );
}
