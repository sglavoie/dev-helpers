import { Action, ActionPanel, Alert, confirmAlert, Icon, List, showToast, Toast, useNavigation } from "@raycast/api";
import { useEffect, useState } from "react";
import { useLocalStorage } from "@raycast/utils";
import { PlaceholderHistoryDetailView } from "./PlaceholderHistoryDetailView";
import { getAllPlaceholderKeys, getPlaceholderHistory, clearPlaceholderHistoryForKey } from "../utils/storage";
import { calculateKeyStats, formatRelativeTime, PlaceholderKeyStats } from "../utils/placeholderHistory";

enum SortOption {
  NameAsc = "name-asc",
  ValueCountDesc = "value-count-desc",
  UsageDesc = "usage-desc",
  LastUsedDesc = "last-used-desc",
}

const SORT_LABELS: Record<SortOption, string> = {
  [SortOption.NameAsc]: "Name (A-Z)",
  [SortOption.ValueCountDesc]: "Most Values",
  [SortOption.UsageDesc]: "Most Used",
  [SortOption.LastUsedDesc]: "Recently Used",
};

export function ManagePlaceholderHistoryView(props: { onUpdated?: () => void }) {
  const { push } = useNavigation();
  const [keys, setKeys] = useState<string[]>([]);
  const [stats, setStats] = useState<PlaceholderKeyStats[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const { value: sortOption = SortOption.NameAsc, setValue: setSortOption } = useLocalStorage<SortOption>(
    "placeholderHistorySortOption",
    SortOption.NameAsc,
  );

  useEffect(() => {
    loadHistory();
  }, []);

  // Recompute sorted stats when sortOption changes
  useEffect(() => {
    if (stats.length > 0) {
      const sorted = sortStats(stats, sortOption);
      setStats(sorted);
    }
  }, [sortOption]);

  async function loadHistory() {
    setIsLoading(true);
    try {
      const [loadedKeys, history] = await Promise.all([getAllPlaceholderKeys(), getPlaceholderHistory()]);
      setKeys(loadedKeys);

      // Compute statistics for each key
      const keyStats = loadedKeys.map((key) => {
        const values = history[key] || [];
        return calculateKeyStats(key, values);
      });

      const sorted = sortStats(keyStats, sortOption);
      setStats(sorted);
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to load placeholder history",
        message: String(error),
      });
    } finally {
      setIsLoading(false);
    }
  }

  function sortStats(stats: PlaceholderKeyStats[], option: SortOption): PlaceholderKeyStats[] {
    const sorted = [...stats];

    switch (option) {
      case SortOption.NameAsc:
        sorted.sort((a, b) => a.key.localeCompare(b.key));
        break;
      case SortOption.ValueCountDesc:
        sorted.sort((a, b) => b.valueCount - a.valueCount);
        break;
      case SortOption.UsageDesc:
        sorted.sort((a, b) => b.totalUseCount - a.totalUseCount);
        break;
      case SortOption.LastUsedDesc:
        sorted.sort((a, b) => {
          if (!a.lastUsed && !b.lastUsed) return 0;
          if (!a.lastUsed) return 1;
          if (!b.lastUsed) return -1;
          return b.lastUsed - a.lastUsed;
        });
        break;
    }

    return sorted;
  }

  async function handleClearKey(key: string, valueCount: number) {
    const confirmed = await confirmAlert({
      title: "Clear Placeholder History",
      message: `Clear all ${valueCount} value${valueCount !== 1 ? "s" : ""} for "${key}"?`,
      primaryAction: {
        title: "Clear",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (confirmed) {
      try {
        await clearPlaceholderHistoryForKey(key);
        await loadHistory();
        props.onUpdated?.();
        showToast({
          style: Toast.Style.Success,
          title: "History cleared",
          message: `Cleared ${valueCount} value${valueCount !== 1 ? "s" : ""} for "${key}"`,
        });
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to clear history",
          message: String(error),
        });
      }
    }
  }

  async function handleHistoryUpdated() {
    await loadHistory();
    props.onUpdated?.();
  }

  function getAccessories(stat: PlaceholderKeyStats): List.Item.Accessory[] {
    const accessories: List.Item.Accessory[] = [];

    // Value count (show storage limit of 100, not UI preference)
    accessories.push({
      tag: { value: `${stat.valueCount}/100 values` },
    });

    // Total usage
    if (stat.totalUseCount > 0) {
      accessories.push({
        tag: { value: `${stat.totalUseCount} uses`, color: "#00aa00" },
      });
    }

    // Last used
    if (stat.lastUsed) {
      accessories.push({
        text: formatRelativeTime(stat.lastUsed),
        tooltip: `Last used: ${new Date(stat.lastUsed).toLocaleString()}`,
      });
    } else {
      accessories.push({
        tag: { value: "Never used", color: "#999" },
      });
    }

    return accessories;
  }

  return (
    <List
      isLoading={isLoading}
      navigationTitle="Manage Placeholder History"
      searchBarPlaceholder="Search placeholder keys..."
      searchBarAccessory={
        <List.Dropdown
          tooltip="Sort By"
          value={sortOption}
          onChange={(newValue) => setSortOption(newValue as SortOption)}
        >
          {Object.entries(SORT_LABELS).map(([value, label]) => (
            <List.Dropdown.Item key={value} title={label} value={value} />
          ))}
        </List.Dropdown>
      }
    >
      {keys.length === 0 ? (
        <List.EmptyView
          icon={Icon.Clock}
          title="No placeholder history"
          description="Placeholder values will be saved here as you use them in snippets"
        />
      ) : (
        stats.map((stat) => (
          <List.Item
            key={stat.key}
            icon={Icon.Text}
            title={stat.key}
            accessories={getAccessories(stat)}
            actions={
              <ActionPanel>
                <ActionPanel.Section title="History Actions">
                  <Action
                    title="View Values"
                    icon={Icon.Eye}
                    onAction={() => {
                      push(<PlaceholderHistoryDetailView placeholderKey={stat.key} onUpdated={handleHistoryUpdated} />);
                    }}
                  />
                  <Action
                    title={`Clear All Values (${stat.valueCount})`}
                    icon={Icon.Trash}
                    style={Action.Style.Destructive}
                    shortcut={{ modifiers: ["cmd"], key: "delete" }}
                    onAction={() => handleClearKey(stat.key, stat.valueCount)}
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
