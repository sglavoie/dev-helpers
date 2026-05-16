import {
  Action,
  ActionPanel,
  Color,
  Icon,
  List,
  Toast,
  showToast,
} from "@raycast/api";
import { Snippet } from "../types";
import { toggleArchive } from "../utils/storage";
import { computeSnippetAnalytics, getUnusedTags } from "../utils/analytics";
import { getErrorMessage } from "../utils/errorMessage";
import { extractPlaceholders, getSystemPlaceholderNames } from "../utils/placeholders";
import { AnalyticsDashboard } from "./AnalyticsDashboard";
import { SearchOperatorsHelp } from "./SearchOperatorsHelp";
import { SnippetContentAction } from "./SnippetContentAction";
import {
  CreateSnippetAction,
  DuplicateSnippetAction,
  EditSnippetAction,
  ImportDataAction,
  ManagePlaceholderHistoryAction,
  ManageTagsAction,
  PasteWithLastValuesAction,
  QuickAddTagAction,
  QuickRemoveTagAction,
  SimilarSnippetsAction,
  ToggleArchiveAction,
  ToggleFavoriteAction,
  TogglePinAction,
} from "./SnippetActions";

interface SnippetListItemProps {
  snippet: Snippet;
  allSnippets: Snippet[];
  showingDetail: boolean;
  showNeedsAttention: boolean;
  showOnlyFavorites: boolean;
  showArchivedSnippets: boolean;
  showRecentSection: boolean;
  searchQuery: string;
  visibleTags: string[];
  allTags: string[];
  onToggleDetail: () => void;
  onToggleFavorites: () => void;
  onToggleRecent: () => void;
  onToggleArchived: () => void;
  onToggleNeedsAttention: () => void;
  onLoadData: () => void;
  onDelete: (snippet: Snippet) => void;
  setSearchQuery: (query: string) => void;
  historyAvailable: boolean;
}

export function SnippetListItem({
  snippet,
  allSnippets,
  showingDetail,
  showNeedsAttention,
  showOnlyFavorites,
  showArchivedSnippets,
  showRecentSection,
  searchQuery,
  visibleTags,
  allTags,
  onToggleDetail,
  onToggleFavorites,
  onToggleRecent,
  onToggleArchived,
  onToggleNeedsAttention,
  onLoadData,
  onDelete,
  setSearchQuery,
  historyAvailable,
}: SnippetListItemProps) {
  const primaryIcon = snippet.isPinned ? Icon.Pin : snippet.isFavorite ? Icon.Star : Icon.Document;
  const analytics = computeSnippetAnalytics(snippet);
  const systemKeys = new Set(getSystemPlaceholderNames());
  const requiredInputCount = extractPlaceholders(snippet.content).filter(
    (p) => p.isRequired && !systemKeys.has(p.key),
  ).length;
  const unusedTagCount = getUnusedTags(allSnippets, allTags).length;

  return (
    <List.Item
      key={snippet.id}
      icon={primaryIcon}
      title={snippet.title}
      subtitle={
        showingDetail
          ? undefined
          : showNeedsAttention
            ? (analytics.stalenessReason ?? snippet.content)
            : snippet.content
      }
      keywords={[
        ...snippet.title.toLowerCase().split(/\W+/).filter(Boolean),
        ...snippet.tags.map((t) => t.toLowerCase()),
        ...snippet.content.toLowerCase().split(/\W+/).filter(Boolean),
        searchQuery.toLowerCase(),
      ]}
      accessories={
        showingDetail
          ? undefined
          : [
              ...(analytics.isStale && !snippet.isArchived
                ? [
                    {
                      text: { value: "stale", color: Color.SecondaryText },
                      tooltip: analytics.stalenessReason ?? "Stale snippet",
                    },
                  ]
                : []),
              ...(requiredInputCount > 0
                ? [
                    {
                      text: { value: `⌨ ${requiredInputCount}`, color: historyAvailable ? Color.Green : Color.SecondaryText },
                      tooltip: historyAvailable
                        ? "Cmd+Shift+Return: paste with last values"
                        : `${requiredInputCount} required placeholder${requiredInputCount > 1 ? "s" : ""}`,
                    },
                  ]
                : []),
              ...(snippet.isPinned && snippet.isFavorite ? [{ icon: Icon.Star, tooltip: "Bookmarked" }] : []),
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
          <ActionPanel.Section>
            <SnippetContentAction snippet={snippet} mode="paste" onComplete={onLoadData} />
            <SnippetContentAction snippet={snippet} mode="copy" onComplete={onLoadData} />
            <PasteWithLastValuesAction snippet={snippet} onComplete={onLoadData} />
            <Action.CopyToClipboard
              title="Copy Title"
              content={snippet.title}
              shortcut={{ modifiers: ["cmd"], key: "c" }}
            />
            <EditSnippetAction snippet={snippet} onEdited={onLoadData} tags={allTags} />
            <CreateSnippetAction onCreated={onLoadData} tags={allTags} />
            <TogglePinAction snippet={snippet} onToggled={onLoadData} />
            <Action
              title="Delete Snippet"
              icon={Icon.Trash}
              style={Action.Style.Destructive}
              shortcut={{ modifiers: ["ctrl"], key: "x" }}
              onAction={() => onDelete(snippet)}
            />
          </ActionPanel.Section>
          <ActionPanel.Section title="Organize">
            <ToggleFavoriteAction snippet={snippet} onToggled={onLoadData} />
            <SimilarSnippetsAction snippet={snippet} allSnippets={allSnippets} onUpdated={onLoadData} />
            <DuplicateSnippetAction snippet={snippet} onDuplicated={onLoadData} />
            <ToggleArchiveAction snippet={snippet} onToggled={onLoadData} />
            <QuickAddTagAction snippet={snippet} availableTags={allTags} onUpdated={onLoadData} />
            <QuickRemoveTagAction snippet={snippet} onUpdated={onLoadData} />
            {analytics.isStale && (
              <Action
                title={`Archive — ${analytics.stalenessReason}`}
                icon={{ source: Icon.Trash, tintColor: Color.Orange }}
                shortcut={{ modifiers: ["cmd", "shift"], key: "u" }}
                onAction={async () => {
                  try {
                    await toggleArchive(snippet.id);
                    showToast({ style: Toast.Style.Success, title: "Snippet archived" });
                    onLoadData();
                  } catch (error) {
                    showToast({
                      style: Toast.Style.Failure,
                      title: "Failed to archive snippet",
                      message: getErrorMessage(error),
                    });
                  }
                }}
              />
            )}
          </ActionPanel.Section>
          <ActionPanel.Section title="Tools">
            <ManageTagsAction onUpdated={onLoadData} unusedCount={unusedTagCount} />
            <Action.Push
              title="View Usage Analytics"
              icon={Icon.BarChart}
              shortcut={{ modifiers: ["cmd", "shift"], key: "y" }}
              target={<AnalyticsDashboard onUpdated={onLoadData} />}
            />
            <ManagePlaceholderHistoryAction onUpdated={onLoadData} />
            <ImportDataAction onImported={onLoadData} />
          </ActionPanel.Section>
          <ActionPanel.Section title="View">
            <Action
              title="Toggle Detail View"
              icon={Icon.AppWindowSidebarLeft}
              shortcut={{ modifiers: ["cmd"], key: "d" }}
              onAction={onToggleDetail}
            />
            <Action
              title={showOnlyFavorites ? "Show All Snippets" : "Show Bookmarked"}
              icon={Icon.Star}
              shortcut={{ modifiers: ["cmd", "shift"], key: "f" }}
              onAction={onToggleFavorites}
            />
            <Action
              title={showRecentSection ? "Hide Recent Section" : "Show Recent Section"}
              icon={Icon.Clock}
              shortcut={{ modifiers: ["cmd"], key: "r" }}
              onAction={onToggleRecent}
            />
            <Action
              title={showArchivedSnippets ? "Hide Archived Snippets" : "Show Archived Snippets"}
              icon={Icon.Box}
              shortcut={{ modifiers: ["cmd"], key: "b" }}
              onAction={onToggleArchived}
            />
            <Action
              title={showNeedsAttention ? "Show All Snippets" : "Show Snippets Needing Attention"}
              icon={Icon.Warning}
              shortcut={{ modifiers: ["cmd", "shift"], key: "n" }}
              onAction={onToggleNeedsAttention}
            />
            <Action.Push
              title="Search Operators Help"
              icon={Icon.QuestionMark}
              shortcut={{ modifiers: ["cmd"], key: "/" }}
              target={<SearchOperatorsHelp />}
            />
          </ActionPanel.Section>
          {snippet.tags.length > 0 && (
            <ActionPanel.Section title="Filter by Tag">
              {snippet.tags.slice(0, 5).map((tag) => (
                <Action
                  key={tag}
                  title={`Filter by tag: ${tag}`}
                  icon={Icon.Tag}
                  onAction={() => setSearchQuery(`tag:${tag}`)}
                />
              ))}
            </ActionPanel.Section>
          )}
        </ActionPanel>
      }
    />
  );
}
