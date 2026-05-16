import {
  Action,
  ActionPanel,
  Icon,
  List,
} from "@raycast/api";
import { useLocalStorage } from "@raycast/utils";
import { Snippet, SortOption, SORT_LABELS } from "./types";
import { SnippetListItem } from "./components/SnippetListItem";
import { BrowseByTagView } from "./components/BrowseByTagView";
import { CreateSnippetAction, ImportDataAction } from "./components/SnippetActions";
import { SearchOperatorsHelp } from "./components/SearchOperatorsHelp";
import { useSnippets } from "./hooks/useSnippets";
import { useSnippetFiltering } from "./hooks/useSnippetFiltering";
import { useSearchSuggestions } from "./hooks/useSearchSuggestions";
import { useHistoryAvailability } from "./hooks/useHistoryAvailability";

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
    hasStructuredOperators,
  } = useSnippetFiltering(snippets, sortOption as SortOption, showRecentSection as boolean);

  const suggestions = useSearchSuggestions(searchQuery, allTags);
  const historyAvailableFor = useHistoryAvailability(snippets);

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
    setSearchQuery,
  };

  const searchBarPlaceholder = (() => {
    if (showOnlyFavorites) return '★ Bookmarked — Cmd+Shift+F to show all';
    if (showArchivedSnippets) return '⊟ Archived — Cmd+B to show all';
    if (showNeedsAttention) return '⚠ Needs Attention — Cmd+Shift+N to show all';
    if (hasStructuredOperators) return 'Operators active — Cmd+/ for syntax help';
    return 'Search… (tag:, is:favorite, not:, "exact") — Cmd+/ for help';
  })();

  const hasPinnedOrRecent = pinnedSnippets.length > 0 || recentSnippets.length > 0;
  const mainSectionTitle = showArchivedSnippets
    ? `⊟ Archived (${filtered.length})`
    : showNeedsAttention
    ? `⚠ Needs Attention (${filtered.length})`
    : showOnlyFavorites
    ? `★ Bookmarked (${filtered.length})`
    : hasPinnedOrRecent
    ? 'All Snippets'
    : undefined;
  const displayTitle =
    hasStructuredOperators && !showArchivedSnippets && !showNeedsAttention && !showOnlyFavorites
      ? `${mainSectionTitle ?? 'All Snippets'} — operators active`
      : mainSectionTitle;

  return (
    <List
      isLoading={isLoading}
      isShowingDetail={showingDetail as boolean}
      filtering={false}
      searchText={searchQuery}
      onSearchTextChange={setSearchQuery}
      searchBarPlaceholder={searchBarPlaceholder}
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
              ? "No bookmarks yet"
              : showNeedsAttention
                ? "No snippets need attention"
                : "No snippets yet"
          }
          description={
            showOnlyFavorites
              ? "Bookmark snippets with ⌘+Shift+V or press ⌘+Shift+F to view all bookmarks"
              : showNeedsAttention
                ? "All snippets are in good shape. Press ⌘+Shift+N to return to the full list"
                : "Press ⌘+N to create your first snippet"
          }
          actions={
            <ActionPanel>
              <CreateSnippetAction onCreated={loadData} tags={allTags} />
              <Action
                title={showOnlyFavorites ? "Show All Snippets" : "Show Bookmarked"}
                icon={Icon.Star}
                shortcut={{ modifiers: ["cmd", "shift"], key: "f" }}
                onAction={() => setShowOnlyFavorites(!showOnlyFavorites)}
              />
              <ActionPanel.Section title="Tools">
                <Action.Push
                  title="Browse by Tag"
                  icon={Icon.Folder}
                  shortcut={{ modifiers: ["cmd", "shift"], key: "g" }}
                  target={<BrowseByTagView onUpdated={loadData} />}
                />
              </ActionPanel.Section>
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
                <Action title="Export All Snippets" icon={Icon.Download} onAction={handleExport} />
                <Action title="View Storage Info" icon={Icon.HardDrive} onAction={handleShowStorageInfo} />
              </ActionPanel.Section>
            </ActionPanel>
          }
        />
      ) : (
        <>
          {suggestions.length > 0 && (
            <List.Section title="Search Suggestions">
              {suggestions.map((suggestion) => (
                <List.Item
                  key={suggestion.title}
                  title={suggestion.title}
                  subtitle={suggestion.subtitle}
                  icon={Icon.MagnifyingGlass}
                  actions={
                    <ActionPanel>
                      <Action
                        title="Apply Filter"
                        icon={Icon.MagnifyingGlass}
                        onAction={() => setSearchQuery(suggestion.completion)}
                      />
                    </ActionPanel>
                  }
                />
              ))}
            </List.Section>
          )}
          {pinnedSnippets.length > 0 && !showArchivedSnippets && !showNeedsAttention && (
            <List.Section title="Pinned" subtitle={`${pinnedSnippets.length} snippets`}>
              {pinnedSnippets.map((snippet: Snippet) => (
                <SnippetListItem key={snippet.id} snippet={snippet} historyAvailable={historyAvailableFor.has(snippet.id)} {...sharedItemProps} />
              ))}
            </List.Section>
          )}
          {recentSnippets.length > 0 && !showArchivedSnippets && !showNeedsAttention && snippets.filter((s) => !s.isArchived).length >= 3 && (
            <List.Section title="Recently Used" subtitle={`${recentSnippets.length} snippets`}>
              {recentSnippets.map((snippet: Snippet) => (
                <SnippetListItem key={snippet.id} snippet={snippet} historyAvailable={historyAvailableFor.has(snippet.id)} {...sharedItemProps} />
              ))}
            </List.Section>
          )}
          <List.Section
            title={displayTitle}
          >
            {sortedSnippets.map((snippet: Snippet) => (
              <SnippetListItem key={snippet.id} snippet={snippet} historyAvailable={historyAvailableFor.has(snippet.id)} {...sharedItemProps} />
            ))}
          </List.Section>
        </>
      )}
    </List>
  );
}
