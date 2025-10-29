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
import { useState, useEffect } from "react";
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

function formatRelativeTime(dateString: string | null): string {
  if (!dateString) return "Active";

  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays === 1) return "Yesterday";
  return `${diffDays}d ago`;
}

function getCurrentDuration(entry: Entry): number {
  if (!entry.active || !entry.start_time) {
    return entry.duration;
  }

  const startTime = new Date(entry.start_time);
  const now = new Date();
  const elapsed = Math.floor((now.getTime() - startTime.getTime()) / 1000);

  return elapsed;
}

export default function Command() {
  const [currentDurations, setCurrentDurations] = useState<Map<string, number>>(
    new Map(),
  );
  const { isLoading, data, error, revalidate } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--days", "7", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          return [];
        }
        const entries = JSON.parse(trimmed) as Entry[];
        // Sort to show active timers first
        return entries.sort((a, b) => {
          if (a.active === b.active) return 0;
          return a.active ? -1 : 1;
        });
      },
    },
  );

  // Update durations every second for active timers
  useEffect(() => {
    if (!data || data.length === 0) return;

    const interval = setInterval(() => {
      const newDurations = new Map<string, number>();
      data.forEach((entry) => {
        if (entry.active) {
          newDurations.set(entry.id, getCurrentDuration(entry));
        }
      });
      setCurrentDurations(newDurations);
    }, 1000);

    return () => clearInterval(interval);
  }, [data]);

  async function handleDeleteTimer(entry: Entry) {
    const duration =
      currentDurations.get(entry.id) || getCurrentDuration(entry);
    const confirmed = await confirmAlert({
      title: "Delete Timer",
      message: `Are you sure you want to delete "${entry.keyword}" (${formatDuration(duration)})?`,
      primaryAction: {
        title: "Delete",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (!confirmed) return;

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Deleting timer...",
      });

      execSync(`/Users/sglavoie/.local/bin/gt delete ${entry.short_id}`, {
        encoding: "utf-8",
      });

      await showToast({
        style: Toast.Style.Success,
        title: "Timer deleted",
        message: `Deleted "${entry.keyword}"`,
      });

      revalidate();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to delete timer",
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
      {!isLoading && (!data || data.length === 0) ? (
        <List.EmptyView
          icon={Icon.Trash}
          title="No Recent Timers"
          description="No timers found in the last 7 days"
        />
      ) : (
        <>
          {data?.map((entry) => {
            const duration =
              currentDurations.get(entry.id) || getCurrentDuration(entry);
            const durationStr = formatDuration(duration);
            const statusStr = entry.active
              ? "Active"
              : `Ended ${formatRelativeTime(entry.end_time)}`;
            const iconColor = entry.active ? Color.Green : Color.SecondaryText;

            return (
              <List.Item
                key={entry.id}
                icon={{ source: Icon.Circle, tintColor: iconColor }}
                title={entry.keyword}
                subtitle={`${durationStr} • ${statusStr}`}
                accessories={[
                  ...(entry.tags ?? []).slice(0, 3).map((tag) => ({
                    tag: { value: tag, color: Color.Blue },
                  })),
                  ...((entry.tags ?? []).length > 3
                    ? [
                        {
                          tag: {
                            value: `+${(entry.tags ?? []).length - 3}`,
                            color: Color.SecondaryText,
                          },
                        },
                      ]
                    : []),
                ]}
                actions={
                  <ActionPanel>
                    <ActionPanel.Section title="Timer Actions">
                      <Action
                        title="Delete Timer"
                        icon={Icon.Trash}
                        style={Action.Style.Destructive}
                        shortcut={{ modifiers: ["cmd"], key: "d" }}
                        onAction={() => handleDeleteTimer(entry)}
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
