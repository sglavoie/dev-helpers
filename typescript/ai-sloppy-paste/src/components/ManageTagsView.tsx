import { Action, ActionPanel, Alert, confirmAlert, Icon, List, showToast, Toast, useNavigation } from "@raycast/api";
import { useEffect, useState } from "react";
import { useLocalStorage } from "@raycast/utils";
import { RenameTagForm } from "./RenameTagForm";
import { MergeTagsForm } from "./MergeTagsForm";
import { deleteTag, getTags, getSnippets } from "../utils/storage";
import { buildTagTree, flattenTagTree } from "../utils/tags";
import {
  computeTagStatistics,
  sortTagStatistics,
  formatRelativeTime,
  formatNumber,
  TagSortOption,
  TAG_SORT_LABELS,
  TagStatistics,
} from "../utils/tagStats";
import { Snippet } from "../types";

export function ManageTagsView(props: { onUpdated: () => void }) {
  const { push } = useNavigation();
  const [tags, setTags] = useState<string[]>([]);
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [tagStats, setTagStats] = useState<TagStatistics[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const { value: sortOption = TagSortOption.NameAsc, setValue: setSortOption } = useLocalStorage<TagSortOption>(
    "tagSortOption",
    TagSortOption.NameAsc,
  );

  useEffect(() => {
    loadTags();
  }, []);

  // Recompute sorted stats when sortOption changes
  useEffect(() => {
    if (tagStats.length > 0) {
      const sorted = sortTagStatistics(tagStats, sortOption);
      setTagStats(sorted);
    }
  }, [sortOption]);

  async function loadTags() {
    setIsLoading(true);
    try {
      const [loadedTags, loadedSnippets] = await Promise.all([getTags(), getSnippets()]);
      setTags(loadedTags);
      setSnippets(loadedSnippets);

      // Compute statistics
      const stats = computeTagStatistics(loadedSnippets, loadedTags);
      const sorted = sortTagStatistics(stats, sortOption);
      setTagStats(sorted);
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to load tags",
        message: String(error),
      });
    } finally {
      setIsLoading(false);
    }
  }

  async function handleDeleteTag(tag: string) {
    const confirmed = await confirmAlert({
      title: "Delete Tag",
      message: `Delete "${tag}"? This tag will be removed from all snippets.`,
      primaryAction: {
        title: "Delete",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (confirmed) {
      try {
        await deleteTag(tag);
        await loadTags(); // Refresh tags list
        props.onUpdated(); // Also update parent
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

  async function handleTagRenamed() {
    await loadTags(); // Refresh tags list
    props.onUpdated(); // Also update parent
  }

  // Create a map of tag -> stats for quick lookup
  const statsMap = new Map<string, TagStatistics>();
  tagStats.forEach((stat) => statsMap.set(stat.tag, stat));

  // Determine display mode: hierarchical tree for name sort, flat list for stat sorts
  const useHierarchy = sortOption === TagSortOption.NameAsc;
  const displayTags = useHierarchy
    ? flattenTagTree(buildTagTree(tags))
    : tagStats.map((stat) => ({
        tag: stat.tag,
        name: stat.tag.split("/").pop() || stat.tag,
        depth: 0,
        hasChildren: false,
      }));

  // Helper to build accessories for a tag
  function getTagAccessories(tag: string): List.Item.Accessory[] {
    const stat = statsMap.get(tag);
    if (!stat) return [];

    const accessories: List.Item.Accessory[] = [];

    // Snippet count
    if (stat.snippetCount > 0) {
      accessories.push({
        tag: { value: `${stat.snippetCount} snippet${stat.snippetCount !== 1 ? "s" : ""}` },
      });
    } else {
      accessories.push({
        tag: { value: "0 snippets", color: "#999" },
      });
    }

    // Usage count (if > 0)
    if (stat.totalUsageCount > 0) {
      accessories.push({
        tag: { value: `${formatNumber(stat.totalUsageCount)} uses`, color: "#00aa00" },
      });
    }

    // Last used
    if (stat.lastUsedAt) {
      accessories.push({
        text: formatRelativeTime(stat.lastUsedAt),
        tooltip: `Last used: ${new Date(stat.lastUsedAt).toLocaleString()}`,
      });
    } else if (stat.snippetCount > 0) {
      // Has snippets but never used
      accessories.push({
        tag: { value: "Never used", color: "#ff9900" },
      });
    }

    return accessories;
  }

  return (
    <List
      isLoading={isLoading}
      navigationTitle="Manage Tags"
      searchBarPlaceholder="Search tags..."
      searchBarAccessory={
        <List.Dropdown
          tooltip="Sort By"
          value={sortOption}
          onChange={(newValue) => setSortOption(newValue as TagSortOption)}
        >
          {Object.entries(TAG_SORT_LABELS).map(([value, label]) => (
            <List.Dropdown.Item key={value} title={label} value={value} />
          ))}
        </List.Dropdown>
      }
    >
      {tags.length === 0 ? (
        <List.EmptyView
          icon={Icon.Tag}
          title="No tags yet"
          description="Tags will appear here as you create snippets with tags"
        />
      ) : (
        displayTags.map((tagItem) => {
          const stat = statsMap.get(tagItem.tag);
          return (
            <List.Item
              key={tagItem.tag}
              icon={tagItem.hasChildren ? Icon.Folder : Icon.Tag}
              title={useHierarchy ? `${"    ".repeat(tagItem.depth)}${tagItem.name}` : tagItem.tag}
              subtitle={useHierarchy && tagItem.depth > 0 ? tagItem.tag : undefined}
              accessories={getTagAccessories(tagItem.tag)}
              actions={
                <ActionPanel>
                  <ActionPanel.Section title="Tag Actions">
                    <Action
                      title="Rename Tag"
                      icon={Icon.Pencil}
                      onAction={() => {
                        push(<RenameTagForm tag={tagItem.tag} onRenamed={handleTagRenamed} />);
                      }}
                    />
                    <Action
                      title={`Delete Tag${stat && stat.snippetCount > 0 ? ` (${stat.snippetCount} snippets)` : ""}`}
                      icon={Icon.Trash}
                      style={Action.Style.Destructive}
                      onAction={() => handleDeleteTag(tagItem.tag)}
                    />
                  </ActionPanel.Section>
                  <ActionPanel.Section title="Bulk Actions">
                    <Action
                      title="Merge Tags"
                      icon={Icon.Link}
                      shortcut={{ modifiers: ["cmd"], key: "m" }}
                      onAction={() => {
                        push(<MergeTagsForm tags={tags} onMerged={handleTagRenamed} />);
                      }}
                    />
                  </ActionPanel.Section>
                </ActionPanel>
              }
            />
          );
        })
      )}
    </List>
  );
}
