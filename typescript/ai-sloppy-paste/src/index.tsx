import {
  Action,
  ActionPanel,
  Alert,
  Clipboard,
  closeMainWindow,
  Color,
  Form,
  Icon,
  List,
  Toast,
  confirmAlert,
  showToast,
  useNavigation,
} from "@raycast/api";
import { useLocalStorage } from "@raycast/utils";
import { useEffect, useState, useMemo } from "react";
import { Snippet, SnippetFormValues, SortOption, SORT_LABELS } from "./types";
import {
  getSnippets,
  addSnippet,
  updateSnippet,
  deleteSnippet,
  duplicateSnippet,
  toggleFavorite,
  toggleArchive,
  incrementUsage,
  getStorageSize,
  exportData,
} from "./utils/storage";
import { pasteWithClipboardRestore } from "./utils/clipboard";
import { extractPlaceholders } from "./utils/placeholders";
import { validateTitle, validateContent, validateTag, getCharacterInfo, VALIDATION_LIMITS } from "./utils/validation";
import { PlaceholderForm } from "./components/PlaceholderForm";
import { ManageTagsView } from "./components/ManageTagsView";
import { ManagePlaceholderHistoryView } from "./components/ManagePlaceholderHistoryView";
import { ImportForm } from "./components/ImportForm";
import { SearchOperatorsHelp } from "./components/SearchOperatorsHelp";
import { isChildOf, expandTagsWithParents } from "./utils/tags";
import { parseSearchQuery } from "./utils/queryParser";
import { applySearchFilters } from "./utils/searchFilter";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";

/**
 * Note: Search highlighting in markdown detail view has been removed to prevent
 * breaking markdown syntax. Wrapping search matches in ** can corrupt links,
 * code blocks, and other markdown elements. Raycast's built-in search highlighting
 * in the list view provides sufficient visual feedback.
 *
 * Future: Could implement markdown-aware highlighting using a proper parser.
 */

export default function Command() {
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [selectedTag, setSelectedTag] = useState<string>("All");
  const [isLoading, setIsLoading] = useState(true);
  const { value: showingDetail = false, setValue: setShowingDetail } = useLocalStorage<boolean>("showingDetail", false);
  const { value: sortOption = SortOption.UpdatedDesc, setValue: setSortOption } = useLocalStorage<SortOption>(
    "sortOption",
    SortOption.UpdatedDesc,
  );
  const { value: showRecentSection = true, setValue: setShowRecentSection } = useLocalStorage<boolean>(
    "showRecentSection",
    true,
  );
  const [showOnlyFavorites, setShowOnlyFavorites] = useState(false);
  const [showArchivedSnippets, setShowArchivedSnippets] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  // Parse search query for operators
  const parsedQuery = useMemo(() => parseSearchQuery(searchQuery), [searchQuery]);

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    setIsLoading(true);
    try {
      const loadedSnippets = await getSnippets();
      setSnippets(loadedSnippets);
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to load data",
        message: String(error),
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

    if (confirmed) {
      try {
        await deleteSnippet(snippet.id);
        await loadData();
        showToast({
          style: Toast.Style.Success,
          title: "Snippet deleted",
        });
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to delete snippet",
          message: String(error),
        });
      }
    }
  }

  async function handleExport() {
    try {
      const data = await exportData();
      const downloadsPath = path.join(os.homedir(), "Downloads");
      const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
      const filename = `ai-sloppy-paste-${timestamp}.json`;
      const filepath = path.join(downloadsPath, filename);

      fs.writeFileSync(filepath, JSON.stringify(data, null, 2));

      showToast({
        style: Toast.Style.Success,
        title: "Export successful",
        message: `Saved to ${filename}`,
      });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Export failed",
        message: String(error),
      });
    }
  }

  async function handleShowStorageInfo() {
    try {
      const storage = await getStorageSize();
      let message = `Using ${storage.formatted} (${storage.percentage.toFixed(1)}% of estimated 5MB limit)`;

      if (storage.percentage > 90) {
        message += "\n⚠️ Approaching storage limit!";
      } else if (storage.percentage > 75) {
        message += "\n⚠️ Storage usage is high";
      }

      await confirmAlert({
        title: "Storage Information",
        message: message,
        primaryAction: {
          title: "OK",
          style: Alert.ActionStyle.Default,
        },
      });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to get storage info",
        message: String(error),
      });
    }
  }

  // Apply filtering: always respect current view (archived/non-archived), then apply search or UI filters
  const filtered = useMemo(() => {
    // Always filter by archive status first based on current view
    const baseSnippets = showArchivedSnippets
      ? snippets.filter((s) => s.isArchived)
      : snippets.filter((s) => !s.isArchived);

    // If search operators are present, apply them to the view-filtered snippets
    if (parsedQuery.hasOperators) {
      return applySearchFilters(baseSnippets, parsedQuery);
    }

    // Otherwise, use traditional UI filter pipeline
    // 1. Filter by tag - support hierarchical filtering
    const tagFiltered =
      selectedTag === "All"
        ? baseSnippets
        : baseSnippets.filter((s) => s.tags.some((tag) => tag === selectedTag || isChildOf(tag, selectedTag)));

    // 2. Filter by favorites
    const favoritesFiltered = showOnlyFavorites ? tagFiltered.filter((s) => s.isFavorite) : tagFiltered;

    return favoritesFiltered;
  }, [snippets, parsedQuery, showArchivedSnippets, selectedTag, showOnlyFavorites]);

  // Compute tags from currently visible snippets
  const visibleTags = useMemo(() => {
    const tagSet = new Set<string>();
    filtered.forEach((snippet) => {
      snippet.tags.forEach((tag) => tagSet.add(tag));
    });
    const tags = Array.from(tagSet);
    // Expand to include parent tags so "tag1" appears even if only "tag1/child1" exists
    return expandTagsWithParents(tags);
  }, [filtered]);

  // Get recently used snippets (top 5 with lastUsedAt)
  const recentSnippets = showRecentSection
    ? [...filtered]
        .filter((s) => s.lastUsedAt)
        .sort((a, b) => (b.lastUsedAt || 0) - (a.lastUsedAt || 0))
        .slice(0, 5)
    : [];

  // Get remaining snippets (exclude recent ones from main list if showing recent section)
  const recentIds = new Set(recentSnippets.map((s) => s.id));
  const remainingSnippets = showRecentSection ? filtered.filter((s) => !recentIds.has(s.id)) : filtered;

  // Sort remaining snippets based on selected option
  const sortedSnippets = [...remainingSnippets].sort((a, b) => {
    switch (sortOption) {
      case SortOption.UpdatedDesc:
        return b.updatedAt - a.updatedAt;
      case SortOption.MostUsedDesc:
        return (b.useCount || 0) - (a.useCount || 0);
      case SortOption.MostUsedAsc:
        return (a.useCount || 0) - (b.useCount || 0);
      case SortOption.Alphabetical:
        return a.title.localeCompare(b.title);
      case SortOption.LastUsed:
        // Put never-used snippets at the end
        if (!a.lastUsedAt && !b.lastUsedAt) return b.updatedAt - a.updatedAt;
        if (!a.lastUsedAt) return 1;
        if (!b.lastUsedAt) return -1;
        return b.lastUsedAt - a.lastUsedAt;
      case SortOption.CreatedDesc:
        return b.createdAt - a.createdAt;
      default:
        return b.updatedAt - a.updatedAt;
    }
  });

  return (
    <List
      isLoading={isLoading}
      isShowingDetail={showingDetail}
      filtering={false}
      onSearchTextChange={setSearchQuery}
      searchBarPlaceholder='Search or use: tag:work, is:favorite, not:archived, "exact"'
      searchBarAccessory={
        <>
          <List.Dropdown
            tooltip="Sort By"
            value={sortOption}
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
      {sortedSnippets.length === 0 && recentSnippets.length === 0 ? (
        <List.EmptyView
          icon={showOnlyFavorites ? Icon.Star : Icon.Document}
          title={showOnlyFavorites ? "No favorites yet" : "No snippets yet"}
          description={
            showOnlyFavorites
              ? "Mark snippets as favorites with ⌘+Shift+F or press ⌘+F to view all snippets"
              : "Press ⌘+N to create your first snippet"
          }
          actions={
            <ActionPanel>
              <CreateSnippetAction onCreated={loadData} tags={visibleTags} />
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
            </ActionPanel>
          }
        />
      ) : (
        <>
          {recentSnippets.length > 0 && !showArchivedSnippets && (
            <List.Section title="Recently Used" subtitle={`${recentSnippets.length} snippets`}>
              {recentSnippets.map((snippet) => renderSnippetItem(snippet))}
            </List.Section>
          )}
          <List.Section
            title={showArchivedSnippets ? "Archived Snippets" : recentSnippets.length > 0 ? "All Snippets" : undefined}
          >
            {sortedSnippets.map((snippet) => renderSnippetItem(snippet))}
          </List.Section>
        </>
      )}
    </List>
  );

  function renderSnippetItem(snippet: Snippet) {
    return (
      <List.Item
        key={snippet.id}
        icon={snippet.isFavorite ? Icon.Star : Icon.Document}
        title={snippet.title}
        subtitle={showingDetail ? undefined : snippet.content}
        keywords={[...snippet.tags, snippet.content]}
        accessories={
          showingDetail
            ? undefined
            : [
                ...(snippet.tags.length > 0
                  ? snippet.tags.slice(0, 3).map((tag) => ({ tag: { value: tag, color: Color.Blue } }))
                  : [{ tag: { value: "untagged", color: Color.SecondaryText } }]),
                ...(snippet.tags.length > 3
                  ? [{ tag: { value: `+${snippet.tags.length - 3}`, color: Color.SecondaryText } }]
                  : []),
                {
                  date: new Date(snippet.updatedAt),
                  tooltip: `Updated: ${new Date(snippet.updatedAt).toLocaleString()}`,
                },
              ]
        }
        detail={
          <List.Item.Detail
            markdown={snippet.content}
            metadata={
              <List.Item.Detail.Metadata>
                <List.Item.Detail.Metadata.TagList title="Tags">
                  {snippet.tags.length > 0 ? (
                    snippet.tags.map((tag) => (
                      <List.Item.Detail.Metadata.TagList.Item key={tag} text={tag} color={Color.Blue} />
                    ))
                  ) : (
                    <List.Item.Detail.Metadata.TagList.Item text="untagged" color={Color.SecondaryText} />
                  )}
                </List.Item.Detail.Metadata.TagList>
                <List.Item.Detail.Metadata.Separator />
                {snippet.description && (
                  <>
                    <List.Item.Detail.Metadata.Label title="Description" text={snippet.description} />
                    <List.Item.Detail.Metadata.Separator />
                  </>
                )}
                <List.Item.Detail.Metadata.Label title="Created" text={new Date(snippet.createdAt).toLocaleString()} />
                <List.Item.Detail.Metadata.Label title="Updated" text={new Date(snippet.updatedAt).toLocaleString()} />
                {snippet.lastUsedAt && (
                  <List.Item.Detail.Metadata.Label
                    title="Last Used"
                    text={new Date(snippet.lastUsedAt).toLocaleString()}
                  />
                )}
                <List.Item.Detail.Metadata.Label title="Use Count" text={`${snippet.useCount || 0} times`} />
              </List.Item.Detail.Metadata>
            }
          />
        }
        actions={
          <ActionPanel>
            <ActionPanel.Section title="Clipboard">
              <CopyContentAction snippet={snippet} onComplete={loadData} />
              <PasteContentAction snippet={snippet} onComplete={loadData} />
              <Action.CopyToClipboard
                title="Copy Title"
                content={snippet.title}
                shortcut={{ modifiers: ["cmd"], key: "c" }}
              />
            </ActionPanel.Section>
            <ActionPanel.Section title="View">
              <Action
                title="Toggle Detail View"
                icon={Icon.AppWindowSidebarLeft}
                shortcut={{ modifiers: ["cmd"], key: "d" }}
                onAction={() => setShowingDetail(!showingDetail)}
              />
              <Action
                title={showOnlyFavorites ? "Show All Snippets" : "Show Favorites"}
                icon={Icon.Star}
                shortcut={{ modifiers: ["cmd"], key: "f" }}
                onAction={() => setShowOnlyFavorites(!showOnlyFavorites)}
              />
              <Action
                title={showRecentSection ? "Hide Recent Section" : "Show Recent Section"}
                icon={Icon.Clock}
                shortcut={{ modifiers: ["cmd"], key: "r" }}
                onAction={() => setShowRecentSection(!showRecentSection)}
              />
              <Action
                title={showArchivedSnippets ? "Hide Archived Snippets" : "Show Archived Snippets"}
                icon={Icon.Box}
                shortcut={{ modifiers: ["cmd"], key: "b" }}
                onAction={() => setShowArchivedSnippets(!showArchivedSnippets)}
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
            <ActionPanel.Section title="Manage">
              <ToggleFavoriteAction snippet={snippet} onToggled={loadData} />
              <EditSnippetAction snippet={snippet} onEdited={loadData} tags={visibleTags} />
              <DuplicateSnippetAction snippet={snippet} onDuplicated={loadData} />
              <ToggleArchiveAction snippet={snippet} onToggled={loadData} />
              <CreateSnippetAction onCreated={loadData} tags={visibleTags} />
              <Action
                title="Delete Snippet"
                icon={Icon.Trash}
                style={Action.Style.Destructive}
                shortcut={{ modifiers: ["ctrl"], key: "x" }}
                onAction={() => handleDelete(snippet)}
              />
            </ActionPanel.Section>
            <ActionPanel.Section title="Tags">
              <ManageTagsAction onUpdated={loadData} />
            </ActionPanel.Section>
            <ActionPanel.Section title="Placeholder History">
              <ManagePlaceholderHistoryAction onUpdated={loadData} />
            </ActionPanel.Section>
            <ActionPanel.Section title="Data">
              <Action
                title="Export All Snippets"
                icon={Icon.Download}
                shortcut={{ modifiers: ["cmd", "shift"], key: "e" }}
                onAction={handleExport}
              />
              <ImportDataAction onImported={loadData} />
              <Action
                title="View Storage Info"
                icon={Icon.HardDrive}
                shortcut={{ modifiers: ["cmd", "shift"], key: "s" }}
                onAction={handleShowStorageInfo}
              />
            </ActionPanel.Section>
          </ActionPanel>
        }
      />
    );
  }
}

function CreateSnippetAction(props: { onCreated: () => void; tags: string[] }) {
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

function EditSnippetAction(props: { snippet: Snippet; onEdited: () => void; tags: string[] }) {
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

function ToggleFavoriteAction(props: { snippet: Snippet; onToggled: () => void }) {
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
        message: String(error),
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

function DuplicateSnippetAction(props: { snippet: Snippet; onDuplicated: () => void }) {
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
        message: String(error),
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

function ToggleArchiveAction(props: { snippet: Snippet; onToggled: () => void }) {
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
        message: String(error),
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

function SnippetForm(props: { snippet?: Snippet; onSubmit: () => void; tags: string[] }) {
  const { pop } = useNavigation();
  const [titleError, setTitleError] = useState<string | undefined>();
  const [contentError, setContentError] = useState<string | undefined>();
  const [tagsError, setTagsError] = useState<string | undefined>();
  const [titleCharInfo, setTitleCharInfo] = useState("");
  const [contentCharInfo, setContentCharInfo] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>(props.snippet?.tags || []);
  const [newTagInput, setNewTagInput] = useState<string>("");
  const [newTagError, setNewTagError] = useState<string | undefined>();

  // Initialize character counts for edit mode
  useEffect(() => {
    if (props.snippet) {
      const titleInfo = getCharacterInfo(props.snippet.title, VALIDATION_LIMITS.TITLE_MAX_LENGTH);
      setTitleCharInfo(titleInfo.info);
      const contentInfo = getCharacterInfo(props.snippet.content, VALIDATION_LIMITS.CONTENT_MAX_LENGTH);
      setContentCharInfo(contentInfo.info);
    }
  }, [props.snippet]);

  // Handle adding a new tag
  function handleAddTag() {
    const trimmedTag = newTagInput.trim();

    if (!trimmedTag) {
      return;
    }

    // Validate the tag
    const tagValidation = validateTag(trimmedTag);
    if (!tagValidation.isValid) {
      setNewTagError(tagValidation.error);
      return;
    }

    // Check if tag already exists
    if (selectedTags.includes(trimmedTag)) {
      setNewTagError("Tag already added");
      return;
    }

    // Add the tag
    setSelectedTags([...selectedTags, trimmedTag]);
    setNewTagInput("");
    setNewTagError(undefined);

    showToast({
      style: Toast.Style.Success,
      title: "Tag added",
      message: trimmedTag,
    });
  }

  async function handleSubmit(values: SnippetFormValues) {
    // Validation
    const titleValidation = validateTitle(values.title);
    if (!titleValidation.isValid) {
      setTitleError(titleValidation.error);
      return;
    }

    const contentValidation = validateContent(values.content);
    if (!contentValidation.isValid) {
      setContentError(contentValidation.error);
      return;
    }

    // Use selectedTags for final submission
    const finalTags = selectedTags;

    try {
      if (props.snippet) {
        // Update existing snippet
        await updateSnippet(props.snippet.id, {
          title: values.title.trim(),
          content: values.content.trim(),
          description: values.description?.trim() || "",
          tags: finalTags,
        });
        showToast({
          style: Toast.Style.Success,
          title: "Snippet updated",
        });
      } else {
        // Create new snippet
        await addSnippet({
          title: values.title.trim(),
          content: values.content.trim(),
          description: values.description?.trim() || "",
          tags: finalTags,
        });
        showToast({
          style: Toast.Style.Success,
          title: "Snippet created",
        });
      }
      props.onSubmit();
      pop();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: props.snippet ? "Failed to update snippet" : "Failed to create snippet",
        message: String(error),
      });
    }
  }

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm title={props.snippet ? "Update Snippet" : "Create Snippet"} onSubmit={handleSubmit} />
          <Action
            title="Add Tag"
            icon={Icon.Plus}
            shortcut={{ modifiers: ["cmd"], key: "t" }}
            onAction={handleAddTag}
          />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="title"
        title="Title"
        placeholder="Enter snippet title"
        defaultValue={props.snippet?.title || ""}
        error={titleError}
        info={titleCharInfo}
        onChange={(value) => {
          setTitleError(undefined);
          const charInfo = getCharacterInfo(value, VALIDATION_LIMITS.TITLE_MAX_LENGTH);
          setTitleCharInfo(charInfo.info);
        }}
      />
      <Form.TextArea
        id="content"
        title="Content"
        placeholder="Enter snippet content"
        defaultValue={props.snippet?.content || ""}
        error={contentError}
        info={contentCharInfo}
        enableMarkdown={true}
        onChange={(value) => {
          setContentError(undefined);
          const charInfo = getCharacterInfo(value, VALIDATION_LIMITS.CONTENT_MAX_LENGTH);
          setContentCharInfo(charInfo.info);
        }}
      />
      <Form.Description text="Placeholders: {{key}} (required) | {{key|default}} (optional) | {{prefix:key:suffix}} (wrappers) | {{!key}} (no history save)" />
      <Form.TextArea
        id="description"
        title="Description"
        placeholder="Optional description for this snippet..."
        defaultValue={props.snippet?.description || ""}
        enableMarkdown={true}
      />
      <Form.TagPicker
        id="tags"
        title="Tags"
        value={selectedTags}
        onChange={setSelectedTags}
        placeholder="Select tags to add to this snippet"
        error={tagsError}
      >
        {(() => {
          // Combine existing tags with any newly created tags that aren't in the list yet
          const allTags = Array.from(new Set([...props.tags, ...selectedTags])).sort();

          return allTags.length > 0 ? (
            allTags.map((tag) => {
              // Calculate display: show hierarchy with visual indentation
              const parts = tag.split("/");
              const depth = parts.length - 1;
              const name = parts[parts.length - 1];
              const indent = "  ".repeat(depth); // 2 spaces per level

              return <Form.TagPicker.Item key={tag} value={tag} title={`${indent}${tag}`} icon={Icon.Tag} />;
            })
          ) : (
            <Form.TagPicker.Item value="" title="No tags available" />
          );
        })()}
      </Form.TagPicker>
      <Form.TextField
        id="newTag"
        title="Add New Tag"
        placeholder="e.g., work/projects or personal"
        value={newTagInput}
        error={newTagError}
        info="Type a tag name and press Cmd+T to add it"
        onChange={(value) => {
          setNewTagInput(value);
          setNewTagError(undefined);
        }}
      />
      <Form.Description text="💡 Press Cmd+T to add tags. Tags appear as badges above - click to remove. Use slashes for hierarchy (e.g., work/projects). No spaces - use dashes (e.g., my-project)." />
    </Form>
  );
}

function ManageTagsAction(props: { onUpdated: () => void }) {
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

function ManagePlaceholderHistoryAction(props: { onUpdated: () => void }) {
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

function ImportDataAction(props: { onImported: () => void }) {
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

function CopyContentAction(props: { snippet: Snippet; onComplete: () => void }) {
  const { push } = useNavigation();

  async function handlePaste() {
    const placeholders = extractPlaceholders(props.snippet.content);

    if (placeholders.length > 0) {
      // Has placeholders - show form
      push(
        <PlaceholderForm
          snippet={props.snippet}
          placeholders={placeholders}
          mode="paste-direct"
          onComplete={props.onComplete}
        />,
      );
    } else {
      // No placeholders - paste directly
      try {
        await pasteWithClipboardRestore(props.snippet.content);
        await incrementUsage(props.snippet.id);
        await closeMainWindow();
        showToast({
          style: Toast.Style.Success,
          title: "Pasted to frontmost app",
        });
        props.onComplete();
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to paste",
          message: String(error),
        });
      }
    }
  }

  return (
    <Action
      title="Paste Content"
      icon={Icon.ArrowDown}
      shortcut={{ modifiers: ["cmd"], key: "return" }}
      onAction={handlePaste}
    />
  );
}

function PasteContentAction(props: { snippet: Snippet; onComplete: () => void }) {
  const { push } = useNavigation();

  async function handleCopy() {
    const placeholders = extractPlaceholders(props.snippet.content);

    if (placeholders.length > 0) {
      // Has placeholders - show form with copy mode
      push(
        <PlaceholderForm
          snippet={props.snippet}
          placeholders={placeholders}
          mode="copy"
          onComplete={props.onComplete}
        />,
      );
    } else {
      // No placeholders - copy directly
      try {
        await Clipboard.copy(props.snippet.content);
        await incrementUsage(props.snippet.id);
        await closeMainWindow();
        showToast({
          style: Toast.Style.Success,
          title: "Copied to clipboard",
        });
        props.onComplete();
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to copy",
          message: String(error),
        });
      }
    }
  }

  return (
    <Action
      title="Copy Content"
      icon={Icon.Clipboard}
      shortcut={{ modifiers: ["cmd", "opt"], key: "return" }}
      onAction={handleCopy}
    />
  );
}
