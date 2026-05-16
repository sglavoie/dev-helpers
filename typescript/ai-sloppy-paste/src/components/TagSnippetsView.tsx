import { Alert, List, Toast, confirmAlert, showToast } from "@raycast/api";
import { useEffect, useState } from "react";
import { useLocalStorage } from "@raycast/utils";
import { Snippet } from "../types";
import { deleteSnippet, getSnippets, getTags } from "../utils/storage";
import { UNTAGGED_SENTINEL, filterSnippetsByTag } from "../utils/tags";
import { SnippetListItem } from "./SnippetListItem";
import { useHistoryAvailability } from "../hooks/useHistoryAvailability";
import { getErrorMessage } from "../utils/errorMessage";

export function TagSnippetsView(props: { tag: string; onUpdated: () => void }) {
  const { tag, onUpdated } = props;
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [allTags, setAllTags] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showArchivedSnippets, setShowArchivedSnippets] = useState(false);
  const { value: showingDetail = false, setValue: setShowingDetail } = useLocalStorage<boolean>(
    "browseByTagDetail",
    false,
  );

  const historyAvailableFor = useHistoryAvailability(snippets);

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    setIsLoading(true);
    try {
      const [loadedTags, loadedSnippets] = await Promise.all([getTags(), getSnippets()]);
      setAllTags(loadedTags);
      setSnippets(loadedSnippets);
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to load snippets",
        message: getErrorMessage(error),
      });
    } finally {
      setIsLoading(false);
    }
  }

  async function handleDelete(snippet: Snippet) {
    const confirmed = await confirmAlert({
      title: "Delete Snippet",
      message: `Are you sure you want to delete "${snippet.title}"?`,
      primaryAction: {
        title: "Delete",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (!confirmed) return;

    try {
      await deleteSnippet(snippet.id);
      await loadData();
      onUpdated();
      showToast({ style: Toast.Style.Success, title: "Snippet deleted" });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to delete snippet",
        message: getErrorMessage(error),
      });
    }
  }

  const filteredByTag = filterSnippetsByTag(snippets, tag);
  const visibleSnippets = showArchivedSnippets
    ? filteredByTag
    : filteredByTag.filter((s) => !s.isArchived);

  const navigationTitle = tag === UNTAGGED_SENTINEL ? "Untagged" : `Tag: ${tag}`;

  return (
    <List
      isLoading={isLoading}
      isShowingDetail={showingDetail as boolean}
      navigationTitle={navigationTitle}
      searchBarPlaceholder="Search snippets in this tag…"
    >
      {visibleSnippets.length === 0 ? (
        <List.EmptyView
          title="No snippets in this tag"
          description="Try a broader filter or toggle archived snippets with Cmd+B"
        />
      ) : (
        visibleSnippets.map((snippet) => (
          <SnippetListItem
            key={snippet.id}
            snippet={snippet}
            allSnippets={snippets}
            showingDetail={showingDetail as boolean}
            showNeedsAttention={false}
            showOnlyFavorites={false}
            showArchivedSnippets={showArchivedSnippets}
            showRecentSection={false}
            searchQuery=""
            visibleTags={allTags}
            allTags={allTags}
            onToggleDetail={() => setShowingDetail(!showingDetail)}
            onToggleArchived={() => setShowArchivedSnippets((v) => !v)}
            onLoadData={async () => {
              await loadData();
              onUpdated();
            }}
            onDelete={handleDelete}
            setSearchQuery={() => {}}
            historyAvailable={historyAvailableFor.has(snippet.id)}
            viewContext="browse"
          />
        ))
      )}
    </List>
  );
}
