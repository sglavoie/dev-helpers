import { Action, ActionPanel, Color, Icon, List, showToast, Toast } from "@raycast/api";
import { useEffect, useState } from "react";
import { useLocalStorage } from "@raycast/utils";
import { getTags, getSnippets } from "../utils/storage";
import { buildTagTree, flattenTagTree, UNTAGGED_SENTINEL } from "../utils/tags";
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
import { getErrorMessage } from "../utils/errorMessage";
import { TagSnippetsView } from "./TagSnippetsView";

export function BrowseByTagView(props: { onUpdated: () => void }) {
  const [tags, setTags] = useState<string[]>([]);
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [tagStats, setTagStats] = useState<TagStatistics[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const { value: sortOption = TagSortOption.LastUsedAsc, setValue: setSortOption } = useLocalStorage<TagSortOption>(
    "browseByTagSort",
    TagSortOption.LastUsedAsc,
  );

  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    if (tagStats.length > 0) {
      const sorted = sortTagStatistics(tagStats, sortOption);
      setTagStats(sorted);
    }
  }, [sortOption]);

  async function loadData() {
    setIsLoading(true);
    try {
      const [loadedTags, loadedSnippets] = await Promise.all([getTags(), getSnippets()]);
      setTags(loadedTags);
      setSnippets(loadedSnippets);

      const stats = computeTagStatistics(loadedSnippets, loadedTags);
      const sorted = sortTagStatistics(stats, sortOption);
      setTagStats(sorted);
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to load tags",
        message: getErrorMessage(error),
      });
    } finally {
      setIsLoading(false);
    }
  }

  async function handleRefresh() {
    await loadData();
    props.onUpdated();
  }

  const untaggedSnippets = snippets.filter((s) => s.tags.length === 0);
  const untaggedLastUsedAt = untaggedSnippets.reduce<number | undefined>((acc, snippet) => {
    if (snippet.lastUsedAt === undefined) return acc;
    if (acc === undefined || snippet.lastUsedAt > acc) return snippet.lastUsedAt;
    return acc;
  }, undefined);
  const untaggedStats: TagStatistics = {
    tag: UNTAGGED_SENTINEL,
    snippetCount: untaggedSnippets.length,
    lastUsedAt: untaggedLastUsedAt,
    totalUsageCount: untaggedSnippets.reduce((sum, s) => sum + (s.useCount || 0), 0),
    neverUsed: untaggedLastUsedAt === undefined,
  };

  const statsMap = new Map<string, TagStatistics>();
  tagStats.forEach((stat) => statsMap.set(stat.tag, stat));
  statsMap.set(UNTAGGED_SENTINEL, untaggedStats);

  const useHierarchy = sortOption === TagSortOption.NameAsc;
  const displayTags = useHierarchy
    ? flattenTagTree(buildTagTree(tags))
    : tagStats.map((stat) => ({
        tag: stat.tag,
        name: stat.tag.split("/").pop() || stat.tag,
        depth: 0,
        hasChildren: false,
      }));

  function getTagAccessories(tag: string): List.Item.Accessory[] {
    const stat = statsMap.get(tag);
    if (!stat) return [];

    const accessories: List.Item.Accessory[] = [];

    if (stat.snippetCount > 0) {
      accessories.push({
        tag: { value: `${stat.snippetCount} snippet${stat.snippetCount !== 1 ? "s" : ""}` },
      });
    } else {
      accessories.push({
        tag: { value: "0 snippets", color: Color.SecondaryText },
      });
    }

    if (stat.totalUsageCount > 0) {
      accessories.push({
        tag: { value: `${formatNumber(stat.totalUsageCount)} uses`, color: Color.Green },
      });
    }

    if (stat.lastUsedAt !== undefined) {
      accessories.push({
        text: formatRelativeTime(stat.lastUsedAt),
        tooltip: `Last used: ${new Date(stat.lastUsedAt).toLocaleString()}`,
      });
    } else if (stat.snippetCount > 0) {
      accessories.push({
        tag: { value: "Never used", color: Color.Orange },
      });
    }

    return accessories;
  }

  const hasContent = tags.length > 0 || untaggedSnippets.length > 0;

  return (
    <List
      isLoading={isLoading}
      navigationTitle="Browse by Tag"
      searchBarPlaceholder="Search tags…"
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
      {!hasContent ? (
        <List.EmptyView
          icon={Icon.Tag}
          title="No tags yet"
          description="Tags will appear here as you create snippets with tags"
        />
      ) : (
        <>
          {displayTags.map((tagItem) => (
            <List.Item
              key={tagItem.tag}
              icon={tagItem.hasChildren ? Icon.Folder : Icon.Tag}
              title={useHierarchy ? `${"    ".repeat(tagItem.depth)}${tagItem.name}` : tagItem.tag}
              subtitle={useHierarchy && tagItem.depth > 0 ? tagItem.tag : undefined}
              accessories={getTagAccessories(tagItem.tag)}
              actions={
                <ActionPanel>
                  <Action.Push
                    title="Browse Snippets"
                    icon={Icon.ArrowRight}
                    target={<TagSnippetsView tag={tagItem.tag} onUpdated={handleRefresh} />}
                  />
                </ActionPanel>
              }
            />
          ))}
          {untaggedSnippets.length > 0 && (
            <List.Item
              key={UNTAGGED_SENTINEL}
              icon={Icon.QuestionMarkCircle}
              title="Untagged"
              accessories={getTagAccessories(UNTAGGED_SENTINEL)}
              actions={
                <ActionPanel>
                  <Action.Push
                    title="Browse Snippets"
                    icon={Icon.ArrowRight}
                    target={<TagSnippetsView tag={UNTAGGED_SENTINEL} onUpdated={handleRefresh} />}
                  />
                </ActionPanel>
              }
            />
          )}
        </>
      )}
    </List>
  );
}
