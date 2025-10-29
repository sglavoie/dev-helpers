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
    ["list", "--active", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          return [];
        }
        try {
          const parsed = JSON.parse(trimmed);
          return Array.isArray(parsed) ? parsed : [];
        } catch (e) {
          console.error("Failed to parse JSON:", e);
          return [];
        }
      },
    },
  );

  // Update durations every second for active timers
  useEffect(() => {
    if (!data || !Array.isArray(data) || data.length === 0) return;

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

  async function handleStopTimer(entry: Entry) {
    const confirmed = await confirmAlert({
      title: "Stop Timer",
      message: `Stop timer for "${entry.keyword}"?`,
      primaryAction: {
        title: "Stop",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (!confirmed) return;

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Stopping timer...",
      });

      execSync(`/Users/sglavoie/.local/bin/gt stop ${entry.short_id}`, {
        encoding: "utf-8",
      });

      await showToast({
        style: Toast.Style.Success,
        title: "Timer stopped",
        message: `Stopped "${entry.keyword}"`,
      });

      revalidate();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to stop timer",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async function handleStopAll() {
    if (!data || data.length === 0) return;

    const confirmed = await confirmAlert({
      title: "Stop All Timers",
      message: `Stop all ${data.length} active timer(s)?`,
      primaryAction: {
        title: "Stop All",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (!confirmed) return;

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Stopping all timers...",
      });

      execSync(`/Users/sglavoie/.local/bin/gt stop --all`, {
        encoding: "utf-8",
      });

      await showToast({
        style: Toast.Style.Success,
        title: "All timers stopped",
      });

      revalidate();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to stop all timers",
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
          icon={Icon.Clock}
          title="No Active Timers"
          description="Start a timer with 'Start Timer' command or gt CLI"
        />
      ) : (
        <>
          {Array.isArray(data) &&
            data.map((entry) => {
              const duration =
                currentDurations.get(entry.id) || getCurrentDuration(entry);
              const durationStr = formatDuration(duration);

              return (
                <List.Item
                  key={entry.id}
                  icon={{ source: Icon.Circle, tintColor: Color.Green }}
                  title={entry.keyword}
                  subtitle={durationStr}
                  accessories={[
                    ...(entry.tags ?? []).slice(0, 3).map((tag: string) => ({
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
                          title="Stop Timer"
                          icon={Icon.Stop}
                          style={Action.Style.Destructive}
                          shortcut={{ modifiers: ["cmd"], key: "s" }}
                          onAction={() => handleStopTimer(entry)}
                        />
                        {data && data.length > 1 && (
                          <Action
                            title="Stop All Timers"
                            icon={Icon.XMarkCircle}
                            style={Action.Style.Destructive}
                            shortcut={{ modifiers: ["cmd", "shift"], key: "s" }}
                            onAction={handleStopAll}
                          />
                        )}
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
