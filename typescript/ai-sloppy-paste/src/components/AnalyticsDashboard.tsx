import { Action, ActionPanel, Alert, confirmAlert, Icon, List, showToast, Toast, Color, Clipboard } from "@raycast/api";
import { useEffect, useState, useMemo } from "react";
import { useLocalStorage } from "@raycast/utils";
import { Snippet, TimeRange, TIME_RANGE_LABELS, CleanupSuggestion, AnalyticsSummary } from "../types";
import { deleteSnippet, toggleArchive, deleteTag, getSnippets, getTags } from "../utils/storage";
import { computeAnalyticsSummary, getTopSnippets, computeCleanupSuggestions, getUnusedTags } from "../utils/analytics";
import { formatRelativeTime, formatNumber, computeTagStatistics, TagStatistics } from "../utils/tagStats";

interface AnalyticsDashboardProps {
  onUpdated: () => void;
}

export function AnalyticsDashboard({ onUpdated }: AnalyticsDashboardProps) {
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const { value: timeRange = TimeRange.ThisMonth, setValue: setTimeRange } = useLocalStorage<TimeRange>(
    "analyticsTimeRange",
    TimeRange.ThisMonth,
  );

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    setIsLoading(true);
    try {
      const [loadedSnippets, loadedTags] = await Promise.all([getSnippets(), getTags()]);
      setSnippets(loadedSnippets);
      setTags(loadedTags);
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

  const summary = useMemo(() => computeAnalyticsSummary(snippets, tags), [snippets, tags]);
  const topSnippets = useMemo(() => getTopSnippets(snippets, timeRange, 10), [snippets, timeRange]);
  const tagStats = useMemo(() => computeTagStatistics(snippets, tags), [snippets, tags]);
  const cleanupSuggestions = useMemo(() => computeCleanupSuggestions(snippets, tags), [snippets, tags]);
  const unusedTags = useMemo(() => getUnusedTags(snippets, tags), [snippets, tags]);

  async function handleArchiveSnippet(snippet: Snippet) {
    const action = snippet.isArchived ? "unarchive" : "archive";
    const confirmed = await confirmAlert({
      title: `${snippet.isArchived ? "Unarchive" : "Archive"} Snippet`,
      message: `${snippet.isArchived ? "Unarchive" : "Archive"} "${snippet.title}"?`,
      primaryAction: {
        title: snippet.isArchived ? "Unarchive" : "Archive",
      },
    });

    if (confirmed) {
      try {
        await toggleArchive(snippet.id);
        await loadData();
        onUpdated();
        showToast({
          style: Toast.Style.Success,
          title: `Snippet ${action}d`,
        });
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: `Failed to ${action} snippet`,
          message: String(error),
        });
      }
    }
  }

  async function handleDeleteSnippet(snippet: Snippet) {
    const confirmed = await confirmAlert({
      title: "Delete Snippet",
      message: `Permanently delete "${snippet.title}"? This cannot be undone.`,
      primaryAction: {
        title: "Delete",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (confirmed) {
      try {
        await deleteSnippet(snippet.id);
        await loadData();
        onUpdated();
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

  async function handleDeleteTag(tag: string) {
    const confirmed = await confirmAlert({
      title: "Delete Tag",
      message: `Delete tag "${tag}"? This tag has no snippets associated with it.`,
      primaryAction: {
        title: "Delete",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (confirmed) {
      try {
        await deleteTag(tag);
        await loadData();
        onUpdated();
        showToast({
          style: Toast.Style.Success,
          title: "Tag deleted",
        });
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to delete tag",
          message: String(error),
        });
      }
    }
  }

  async function handleCopyContent(snippet: Snippet) {
    await Clipboard.copy(snippet.content);
    showToast({
      style: Toast.Style.Success,
      title: "Content copied",
    });
  }

  function getUsageColor(useCount: number): Color {
    if (useCount >= 50) return Color.Green;
    if (useCount >= 10) return Color.Blue;
    if (useCount >= 1) return Color.SecondaryText;
    return Color.SecondaryText;
  }

  function renderSnippetActions(snippet: Snippet) {
    return (
      <ActionPanel>
        <ActionPanel.Section title="Snippet Actions">
          <Action title="Copy Content" icon={Icon.Clipboard} onAction={() => handleCopyContent(snippet)} />
          <Action
            title={snippet.isArchived ? "Unarchive Snippet" : "Archive Snippet"}
            icon={Icon.Box}
            onAction={() => handleArchiveSnippet(snippet)}
          />
          <Action
            title="Delete Snippet"
            icon={Icon.Trash}
            style={Action.Style.Destructive}
            onAction={() => handleDeleteSnippet(snippet)}
          />
        </ActionPanel.Section>
      </ActionPanel>
    );
  }

  function renderSuggestionActions(suggestion: CleanupSuggestion) {
    if (suggestion.type === "unused_tag" && suggestion.tag) {
      return (
        <ActionPanel>
          <ActionPanel.Section title="Tag Actions">
            <Action
              title="Delete Tag"
              icon={Icon.Trash}
              style={Action.Style.Destructive}
              onAction={() => handleDeleteTag(suggestion.tag!)}
            />
          </ActionPanel.Section>
        </ActionPanel>
      );
    }

    if (suggestion.snippet) {
      return renderSnippetActions(suggestion.snippet);
    }

    return null;
  }

  const sortedTagStats = [...tagStats].sort((a, b) => b.totalUsageCount - a.totalUsageCount);

  return (
    <List
      isLoading={isLoading}
      navigationTitle="Usage Analytics"
      searchBarPlaceholder="Search analytics..."
      searchBarAccessory={
        <List.Dropdown
          tooltip="Time Range"
          value={timeRange}
          onChange={(newValue) => setTimeRange(newValue as TimeRange)}
        >
          {Object.entries(TIME_RANGE_LABELS).map(([value, label]) => (
            <List.Dropdown.Item key={value} title={label} value={value} />
          ))}
        </List.Dropdown>
      }
    >
      <List.Section title="Summary">
        <List.Item
          icon={Icon.BarChart}
          title="Usage Overview"
          accessories={[
            { tag: { value: `${summary.totalSnippets} snippets`, color: Color.Blue } },
            { tag: { value: `${formatNumber(summary.totalUsageCount)} total uses`, color: Color.Green } },
            ...(summary.staleSnippets > 0
              ? [{ tag: { value: `${summary.staleSnippets} stale`, color: Color.Orange } }]
              : []),
            ...(summary.archivedSnippets > 0 ? [{ text: `${summary.archivedSnippets} archived` }] : []),
          ]}
          actions={
            <ActionPanel>
              <Action.CopyToClipboard
                title="Copy Summary"
                content={`Snippets: ${summary.totalSnippets}, Total Uses: ${summary.totalUsageCount}, Active (30d): ${summary.activeSnippets}, Stale (90d+): ${summary.staleSnippets}, Archived: ${summary.archivedSnippets}`}
              />
            </ActionPanel>
          }
        />
        <List.Item
          icon={Icon.Tag}
          title="Tag Overview"
          accessories={[
            { tag: { value: `${summary.totalTags} tags`, color: Color.Purple } },
            ...(summary.unusedTags > 0
              ? [{ tag: { value: `${summary.unusedTags} unused`, color: Color.Orange } }]
              : []),
          ]}
        />
      </List.Section>

      {topSnippets.length > 0 && (
        <List.Section title={`Top Snippets (${TIME_RANGE_LABELS[timeRange]})`}>
          {topSnippets.map((snippet, index) => (
            <List.Item
              key={snippet.id}
              icon={snippet.isFavorite ? Icon.Star : Icon.Document}
              title={`${index + 1}. ${snippet.title}`}
              accessories={[
                {
                  tag: {
                    value: `${formatNumber(snippet.useCount)} uses`,
                    color: getUsageColor(snippet.useCount),
                  },
                },
                ...(snippet.lastUsedAt ? [{ text: formatRelativeTime(snippet.lastUsedAt) }] : []),
              ]}
              actions={renderSnippetActions(snippet)}
            />
          ))}
        </List.Section>
      )}

      {sortedTagStats.length > 0 && (
        <List.Section title="Tag Insights">
          {sortedTagStats.slice(0, 10).map((stat) => (
            <List.Item
              key={stat.tag}
              icon={unusedTags.includes(stat.tag) ? Icon.Warning : Icon.Tag}
              title={stat.tag}
              accessories={[
                {
                  tag: {
                    value: `${stat.snippetCount} snippet${stat.snippetCount !== 1 ? "s" : ""}`,
                    color: stat.snippetCount > 0 ? Color.Blue : Color.SecondaryText,
                  },
                },
                ...(stat.totalUsageCount > 0
                  ? [{ tag: { value: `${formatNumber(stat.totalUsageCount)} uses`, color: Color.Green } }]
                  : []),
                ...(stat.lastUsedAt
                  ? [{ text: formatRelativeTime(stat.lastUsedAt) }]
                  : stat.snippetCount > 0
                    ? [{ tag: { value: "Never used", color: Color.Orange } }]
                    : []),
              ]}
              actions={
                unusedTags.includes(stat.tag) ? (
                  <ActionPanel>
                    <Action
                      title="Delete Unused Tag"
                      icon={Icon.Trash}
                      style={Action.Style.Destructive}
                      onAction={() => handleDeleteTag(stat.tag)}
                    />
                  </ActionPanel>
                ) : undefined
              }
            />
          ))}
        </List.Section>
      )}

      {cleanupSuggestions.length > 0 && (
        <List.Section title="Cleanup Suggestions">
          {cleanupSuggestions.map((suggestion) => (
            <List.Item
              key={suggestion.id}
              icon={{ source: Icon.Warning, tintColor: Color.Orange }}
              title={suggestion.type === "unused_tag" ? `Tag: "${suggestion.tag}"` : `"${suggestion.snippet?.title}"`}
              subtitle={suggestion.suggestedAction}
              accessories={[
                {
                  tag: {
                    value:
                      suggestion.type === "never_used"
                        ? "Never used"
                        : suggestion.type === "stale"
                          ? "Stale"
                          : "Unused tag",
                    color: Color.Orange,
                  },
                },
                { text: suggestion.reason },
              ]}
              actions={renderSuggestionActions(suggestion)}
            />
          ))}
        </List.Section>
      )}

      {snippets.length === 0 && !isLoading && (
        <List.EmptyView
          icon={Icon.BarChart}
          title="No analytics yet"
          description="Create some snippets and use them to see analytics"
        />
      )}
    </List>
  );
}
