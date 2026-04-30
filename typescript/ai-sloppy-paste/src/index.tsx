import {
  Action,
  ActionPanel,
  Icon,
  List,
} from "@raycast/api";
import { useLocalStorage } from "@raycast/utils";
import { Snippet, SortOption, SORT_LABELS } from "./types";
import { SnippetListItem } from "./components/SnippetListItem";
import { CreateSnippetAction, ImportDataAction } from "./components/SnippetActions";
import { SearchOperatorsHelp } from "./components/SearchOperatorsHelp";
import { useSnippets } from "./hooks/useSnippets";
import { useSnippetFiltering } from "./hooks/useSnippetFiltering";

/**
 * Note: Search highlighting in markdown detail view has been removed to prevent
 * breaking markdown syntax. Wrapping search matches in ** can corrupt links,
 * code blocks, and other markdown elements. Raycast's built-in search highlighting
 * in the list view provides sufficient visual feedback.
 *
 * Future: Could implement markdown-aware highlighting using a proper parser.
 */

export default function Command() {
  const { snippets, isLoading, loadData, handleDelete, handleExport, handleShowStorageInfo } = useSnippets();
  const { value: showingDetail = false, setValue: setShowingDetail } = useLocalStorage<boolean>("showingDetail", false);
  const { value: sortOption = SortOption.UpdatedDesc, setValue: setSortOption } = useLocalStorage<SortOption>(
    "sortOption",
    SortOption.UpdatedDesc,
  );
  const { value: showRecentSection = true, setValue: setShowRecentSection } = useLocalStorage<boolean>(
    "showRecentSection",
    true,
  );

  const {
    searchQuery,
    setSearchQuery,
    selectedTag,
    setSelectedTag,
    showOnlyFavorites,
    setShowOnlyFavorites,
    showArchivedSnippets,
    setShowArchivedSnippets,
    showNeedsAttention,
    setShowNeedsAttention,
    filtered,
    visibleTags,
    allTags,
    pinnedSnippets,
    recentSnippets,
    sortedSnippets,
  } = useSnippetFiltering(snippets, sortOption as SortOption, showRecentSection as boolean);

  const sharedItemProps = {
    allSnippets: snippets,
    showingDetail: showingDetail as boolean,
    showNeedsAttention,
    showOnlyFavorites,
    showArchivedSnippets,
    showRecentSection: showRecentSection as boolean,
    searchQuery,
    visibleTags,
    allTags,
    onToggleDetail: () => setShowingDetail(!showingDetail),
    onToggleFavorites: () => setShowOnlyFavorites(!showOnlyFavorites),
    onToggleRecent: () => setShowRecentSection(!showRecentSection),
    onToggleArchived: () => setShowArchivedSnippets(!showArchivedSnippets),
    onToggleNeedsAttention: () => setShowNeedsAttention((v) => !v),
    onLoadData: loadData,
    onDelete: handleDelete,
    onExport: handleExport,
    onShowStorageInfo: handleShowStorageInfo,
  };

  return (
    <List
      isLoading={isLoading}
      isShowingDetail={showingDetail as boolean}
      filtering={false}
      onSearchTextChange={setSearchQuery}
      searchBarPlaceholder='Search or use: tag:work, is:favorite, not:archived, "exact"'
      searchBarAccessory={
        <>
          <List.Dropdown
            tooltip="Sort By"
            value={sortOption as string}
            onChange={(newValue) => setSortOption(newValue as SortOption)}
          >
            {Object.entries(SORT_LABELS).map(([value, label]) => (
              <List.Dropdown.Item key={value} title={label} value={value} />
            ))}
          </List.Dropdown>
          <List.Dropdown tooltip="Filter by Tag" value={selectedTag} onChange={setSelectedTag}>
            <List.Dropdown.Item title="All Tags" value="All" />
            {visibleTags.length > 0 && (
              <List.Dropdown.Section title="Tags">
                {visibleTags.map((tag) => (
                  <List.Dropdown.Item key={tag} title={tag} value={tag} />
                ))}
              </List.Dropdown.Section>
            )}
          </List.Dropdown>
        </>
      }
    >
      {sortedSnippets.length === 0 && recentSnippets.length === 0 && pinnedSnippets.length === 0 ? (
        <List.EmptyView
          icon={showOnlyFavorites ? Icon.Star : showNeedsAttention ? Icon.Checkmark : Icon.Document}
          title={
            showOnlyFavorites
              ? "No favorites yet"
              : showNeedsAttention
                ? "No snippets need attention"
                : "No snippets yet"
          }
          description={
            showOnlyFavorites
              ? "Mark snippets as favorites with ⌘+Shift+F or press ⌘+F to view all snippets"
              : showNeedsAttention
                ? "All snippets are in good shape. Press ⌘+Shift+N to return to the full list"
                : "Press ⌘+N to create your first snippet"
          }
          actions={
            <ActionPanel>
              <CreateSnippetAction onCreated={loadData} tags={allTags} />
              <Action
                title={showOnlyFavorites ? "Show All Snippets" : "Show Favorites"}
                icon={Icon.Star}
                shortcut={{ modifiers: ["cmd"], key: "f" }}
                onAction={() => setShowOnlyFavorites(!showOnlyFavorites)}
              />
              <ActionPanel.Section title="Help">
                <Action.Push
                  title="Search Operators Help"
                  icon={Icon.QuestionMark}
                  shortcut={{ modifiers: ["cmd"], key: "/" }}
                  target={<SearchOperatorsHelp />}
                />
              </ActionPanel.Section>
              <ActionPanel.Section title="Data">
                <ImportDataAction onImported={loadData} />
              </ActionPanel.Section>
            </ActionPanel>
          }
        />
      ) : (
        <>
          {pinnedSnippets.length > 0 && !showArchivedSnippets && !showNeedsAttention && (
            <List.Section title="Pinned" subtitle={`${pinnedSnippets.length} snippets`}>
              {pinnedSnippets.map((snippet: Snippet) => (
                <SnippetListItem key={snippet.id} snippet={snippet} {...sharedItemProps} />
              ))}
            </List.Section>
          )}
          {recentSnippets.length > 0 && !showArchivedSnippets && !showNeedsAttention && (
            <List.Section title="Recently Used" subtitle={`${recentSnippets.length} snippets`}>
              {recentSnippets.map((snippet: Snippet) => (
                <SnippetListItem key={snippet.id} snippet={snippet} {...sharedItemProps} />
              ))}
            </List.Section>
          )}
          <List.Section
            title={
              showArchivedSnippets
                ? "Archived Snippets"
                : showNeedsAttention
                  ? `Needs Attention (${filtered.length})`
                  : pinnedSnippets.length > 0 || recentSnippets.length > 0
                    ? "All Snippets"
                    : undefined
            }
          >
            {sortedSnippets.map((snippet: Snippet) => (
              <SnippetListItem key={snippet.id} snippet={snippet} {...sharedItemProps} />
            ))}
          </List.Section>
        </>
      )}
    </List>
  );
}
