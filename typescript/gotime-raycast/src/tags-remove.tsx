import {
  Action,
  ActionPanel,
  Alert,
  Color,
  Icon,
  List,
  Toast,
  confirmAlert,
  showToast,
} from "@raycast/api";
import { useExec } from "@raycast/utils";
import { execSync } from "child_process";

interface Entry {
  id: string;
  short_id: number;
  keyword: string;
  tags: string[];
  start_time: string;
  end_time: string | null;
  duration: number;
  active: boolean;
  stashed: boolean;
}

interface TagUsage {
  tag: string;
  count: number;
  totalDuration: number;
  activeCount: number;
}

function formatDuration(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  if (secs > 0 || parts.length === 0) parts.push(`${secs}s`);

  return parts.join(" ");
}

export default function Command() {
  const { isLoading, data, error, revalidate } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--days", "3650", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          return [];
        }
        const entries = JSON.parse(trimmed) as Entry[];

        // Calculate tag usage statistics
        const tagMap = new Map<string, TagUsage>();

        entries.forEach((entry) => {
          entry.tags.forEach((tag) => {
            // Skip empty tags
            if (!tag || tag.trim() === "") return;

            const existing = tagMap.get(tag);
            if (existing) {
              existing.count++;
              existing.totalDuration += entry.duration;
              if (entry.active) existing.activeCount++;
            } else {
              tagMap.set(tag, {
                tag,
                count: 1,
                totalDuration: entry.duration,
                activeCount: entry.active ? 1 : 0,
              });
            }
          });
        });

        // Convert to array and sort by count descending
        return Array.from(tagMap.values())
          .filter((tagUsage) => tagUsage.tag && tagUsage.tag.trim() !== "")
          .sort((a, b) => b.count - a.count);
      },
    },
  );

  async function handleRemoveTag(tagUsage: TagUsage) {
    const confirmed = await confirmAlert({
      title: "Remove Tag",
      message: `Remove "${tagUsage.tag}" from all ${tagUsage.count} ${tagUsage.count === 1 ? "entry" : "entries"}?`,
      primaryAction: {
        title: "Remove",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (!confirmed) return;

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Removing tag...",
      });

      execSync(`/Users/sglavoie/.local/bin/gt tags remove ${tagUsage.tag}`, {
        encoding: "utf-8",
      });

      await showToast({
        style: Toast.Style.Success,
        title: "Tag removed",
        message: `Removed "${tagUsage.tag}" from ${tagUsage.count} ${tagUsage.count === 1 ? "entry" : "entries"}`,
      });

      revalidate();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to remove tag",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  if (error) {
    return (
      <List>
        <List.Item title="Error" subtitle={error.message} />
      </List>
    );
  }

  return (
    <List isLoading={isLoading}>
      {!isLoading && (!data || !Array.isArray(data) || data.length === 0) ? (
        <List.EmptyView
          icon={Icon.Tag}
          title="No Tags Found"
          description="No tags available to remove"
        />
      ) : (
        <>
          {Array.isArray(data) &&
            data.map((tagUsage) => {
              // Skip invalid entries
              if (!tagUsage || !tagUsage.tag || tagUsage.tag.trim() === "") {
                return null;
              }

              const hasActive = tagUsage.activeCount > 0;

              return (
                <List.Item
                  key={tagUsage.tag}
                  icon={{
                    source: Icon.Tag,
                    tintColor: hasActive ? Color.Green : Color.Blue,
                  }}
                  title={tagUsage.tag}
                  subtitle={`${tagUsage.count} ${tagUsage.count === 1 ? "entry" : "entries"} • ${formatDuration(tagUsage.totalDuration)}`}
                  accessories={[
                    ...(hasActive
                      ? [
                          {
                            tag: {
                              value: `${tagUsage.activeCount} active`,
                              color: Color.Green,
                            },
                          },
                        ]
                      : []),
                  ]}
                  actions={
                    <ActionPanel>
                      <ActionPanel.Section title="Tag Actions">
                        <Action
                          title="Remove Tag from All Entries"
                          icon={Icon.Trash}
                          style={Action.Style.Destructive}
                          shortcut={{ modifiers: ["cmd"], key: "d" }}
                          onAction={() => handleRemoveTag(tagUsage)}
                        />
                      </ActionPanel.Section>
                      <ActionPanel.Section title="Actions">
                        <Action
                          title="Refresh"
                          icon={Icon.ArrowClockwise}
                          shortcut={{ modifiers: ["cmd"], key: "r" }}
                          onAction={() => revalidate()}
                        />
                      </ActionPanel.Section>
                    </ActionPanel>
                  }
                />
              );
            })}
        </>
      )}
    </List>
  );
}
