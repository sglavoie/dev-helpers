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
import { computeSnippetAnalytics } from "../utils/analytics";
import { getErrorMessage } from "../utils/errorMessage";
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
  onToggleDetail: () => void;
  onToggleFavorites: () => void;
  onToggleRecent: () => void;
  onToggleArchived: () => void;
  onToggleNeedsAttention: () => void;
  onLoadData: () => void;
  onDelete: (snippet: Snippet) => void;
  onExport: () => void;
  onShowStorageInfo: () => void;
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
  onToggleDetail,
  onToggleFavorites,
  onToggleRecent,
  onToggleArchived,
  onToggleNeedsAttention,
  onLoadData,
  onDelete,
  onExport,
  onShowStorageInfo,
}: SnippetListItemProps) {
  const primaryIcon = snippet.isPinned ? Icon.Pin : snippet.isFavorite ? Icon.Star : Icon.Document;
  const analytics = computeSnippetAnalytics(snippet);

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
              ...(analytics.isStale
                ? [
                    {
                      icon: { source: Icon.ExclamationMark, tintColor: Color.Orange },
                      tooltip: analytics.stalenessReason ?? "Stale snippet",
                    },
                  ]
                : []),
              ...(snippet.isPinned && snippet.isFavorite ? [{ icon: Icon.Star, tooltip: "Favorite" }] : []),
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
            <SnippetContentAction snippet={snippet} mode="paste" onComplete={onLoadData} />
            <SnippetContentAction snippet={snippet} mode="copy" onComplete={onLoadData} />
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
              onAction={onToggleDetail}
            />
            <Action
              title={showOnlyFavorites ? "Show All Snippets" : "Show Favorites"}
              icon={Icon.Star}
              shortcut={{ modifiers: ["cmd"], key: "f" }}
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
            <TogglePinAction snippet={snippet} onToggled={onLoadData} />
            <ToggleFavoriteAction snippet={snippet} onToggled={onLoadData} />
            <EditSnippetAction snippet={snippet} onEdited={onLoadData} tags={visibleTags} />
            <DuplicateSnippetAction snippet={snippet} onDuplicated={onLoadData} />
            <ToggleArchiveAction snippet={snippet} onToggled={onLoadData} />
            <CreateSnippetAction onCreated={onLoadData} tags={visibleTags} />
            <Action
              title="Delete Snippet"
              icon={Icon.Trash}
              style={Action.Style.Destructive}
              shortcut={{ modifiers: ["ctrl"], key: "x" }}
              onAction={() => onDelete(snippet)}
            />
          </ActionPanel.Section>
          {analytics.isStale && (
            <ActionPanel.Section title="Cleanup Suggestion">
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
            </ActionPanel.Section>
          )}
          <ActionPanel.Section title="Tags">
            <QuickAddTagAction snippet={snippet} availableTags={visibleTags} onUpdated={onLoadData} />
            <QuickRemoveTagAction snippet={snippet} onUpdated={onLoadData} />
            <ManageTagsAction onUpdated={onLoadData} />
          </ActionPanel.Section>
          <ActionPanel.Section title="Placeholder History">
            <ManagePlaceholderHistoryAction onUpdated={onLoadData} />
          </ActionPanel.Section>
          <ActionPanel.Section title="Analytics">
            <Action.Push
              title="View Usage Analytics"
              icon={Icon.BarChart}
              shortcut={{ modifiers: ["cmd", "shift"], key: "a" }}
              target={<AnalyticsDashboard onUpdated={onLoadData} />}
            />
            <SimilarSnippetsAction snippet={snippet} allSnippets={allSnippets} onUpdated={onLoadData} />
          </ActionPanel.Section>
          <ActionPanel.Section title="Data">
            <Action
              title="Export All Snippets"
              icon={Icon.Download}
              shortcut={{ modifiers: ["cmd", "shift"], key: "e" }}
              onAction={onExport}
            />
            <ImportDataAction onImported={onLoadData} />
            <Action
              title="View Storage Info"
              icon={Icon.HardDrive}
              shortcut={{ modifiers: ["cmd", "shift"], key: "s" }}
              onAction={onShowStorageInfo}
            />
          </ActionPanel.Section>
        </ActionPanel>
      }
    />
  );
}
