import {
  Action,
  ActionPanel,
  Color,
  Form,
  Icon,
  List,
  Toast,
  popToRoot,
  closeMainWindow,
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

interface EditFormProps {
  entry: Entry;
  onComplete: () => void;
}

function EditEntryForm({ entry, onComplete }: EditFormProps) {
  const [keywordError, setKeywordError] = useState<string | undefined>();

  async function handleSubmit(values: {
    keyword: string;
    tags: string;
    startDate: Date;
    startTime: Date;
    endDate?: Date;
    endTime?: Date;
  }) {
    const keyword = values.keyword.trim();
    if (!keyword) {
      setKeywordError("Keyword is required");
      return;
    }

    if (!/^[a-zA-Z0-9_-]+$/.test(keyword)) {
      setKeywordError(
        "Keyword can only contain letters, numbers, dashes, and underscores",
      );
      return;
    }

    const tags = values.tags
      .split(",")
      .map((t) => t.trim())
      .filter((t) => t.length > 0);

    for (const tag of tags) {
      if (!/^[a-zA-Z0-9_-]+$/.test(tag)) {
        await showToast({
          style: Toast.Style.Failure,
          title: "Invalid tag format",
          message: `Tag "${tag}" can only contain letters, numbers, dashes, and underscores`,
        });
        return;
      }
    }

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Updating entry...",
      });

      // Combine date and time for start
      const startDateTime = new Date(values.startDate);
      startDateTime.setHours(values.startTime.getHours());
      startDateTime.setMinutes(values.startTime.getMinutes());
      startDateTime.setSeconds(values.startTime.getSeconds());

      const commands: string[] = [];

      // Update keyword if changed
      if (keyword !== entry.keyword) {
        commands.push(
          `/Users/sglavoie/.local/bin/gt set ${entry.short_id} keyword ${keyword}`,
        );
      }

      // Update tags if changed
      const currentTags = entry.tags.sort().join(",");
      const newTags = tags.sort().join(",");
      if (newTags !== currentTags) {
        commands.push(
          `/Users/sglavoie/.local/bin/gt set ${entry.short_id} tags ${newTags || '""'}`,
        );
      }

      // Update start time if changed
      const newStartTime = startDateTime.toISOString();
      if (newStartTime !== entry.start_time) {
        commands.push(
          `/Users/sglavoie/.local/bin/gt set ${entry.short_id} start_time "${newStartTime}"`,
        );
      }

      // Update end time if provided and changed
      if (values.endDate && values.endTime) {
        const endDateTime = new Date(values.endDate);
        endDateTime.setHours(values.endTime.getHours());
        endDateTime.setMinutes(values.endTime.getMinutes());
        endDateTime.setSeconds(values.endTime.getSeconds());
        const newEndTime = endDateTime.toISOString();

        if (newEndTime !== entry.end_time) {
          commands.push(
            `/Users/sglavoie/.local/bin/gt set ${entry.short_id} end_time "${newEndTime}"`,
          );
        }
      }

      if (commands.length === 0) {
        await showToast({
          style: Toast.Style.Success,
          title: "No changes to apply",
        });
        await popToRoot();
        await closeMainWindow();
        return;
      }

      // Execute all commands
      for (const command of commands) {
        execSync(command, { encoding: "utf-8" });
      }

      await showToast({
        style: Toast.Style.Success,
        title: "Entry updated",
        message: `Updated ${commands.length} field${commands.length > 1 ? "s" : ""}`,
      });

      onComplete();
      await popToRoot();
      await closeMainWindow();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to update entry",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  const startTime = new Date(entry.start_time);
  const endTime = entry.end_time ? new Date(entry.end_time) : null;

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Update Entry" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="keyword"
        title="Keyword"
        placeholder="e.g., coding, meeting"
        defaultValue={entry.keyword}
        error={keywordError}
        onChange={() => setKeywordError(undefined)}
      />
      <Form.TextField
        id="tags"
        title="Tags"
        placeholder="e.g., golang, cli, project-name"
        defaultValue={entry.tags.join(", ")}
        info="Comma-separated tags"
      />
      <Form.Separator />
      <Form.DatePicker
        id="startDate"
        title="Start Date"
        defaultValue={startTime}
      />
      <Form.DatePicker
        id="startTime"
        title="Start Time"
        type={Form.DatePicker.Type.DateTime}
        defaultValue={startTime}
      />
      <Form.Separator />
      <Form.DatePicker
        id="endDate"
        title="End Date"
        defaultValue={endTime || undefined}
        info="Leave empty for active entry"
      />
      <Form.DatePicker
        id="endTime"
        title="End Time"
        type={Form.DatePicker.Type.DateTime}
        defaultValue={endTime || undefined}
        info="Leave empty for active entry"
      />
      <Form.Description
        text={`💡 Current duration: ${formatDuration(entry.duration)} • ${entry.active ? "Active" : "Stopped"}`}
      />
    </Form>
  );
}

export default function Command() {
  const [currentDurations, setCurrentDurations] = useState<Map<string, number>>(
    new Map(),
  );

  const { isLoading, data, error, revalidate } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--days", "30", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          return [];
        }
        const entries = JSON.parse(trimmed) as Entry[];
        // Sort: active first, then by start time descending
        return entries.sort((a, b) => {
          if (a.active === b.active) {
            return (
              new Date(b.start_time).getTime() -
              new Date(a.start_time).getTime()
            );
          }
          return a.active ? -1 : 1;
        });
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
          icon={Icon.Pencil}
          title="No Recent Entries"
          description="No entries found in the last 30 days"
        />
      ) : (
        <>
          {Array.isArray(data) &&
            data.map((entry) => {
              const duration =
                currentDurations.get(entry.id) || getCurrentDuration(entry);
              const durationStr = formatDuration(duration);
              const statusStr = entry.active
                ? "Active"
                : `Ended ${formatRelativeTime(entry.end_time)}`;
              const iconColor = entry.active
                ? Color.Green
                : Color.SecondaryText;

              return (
                <List.Item
                  key={entry.id}
                  icon={{ source: Icon.Circle, tintColor: iconColor }}
                  title={entry.keyword}
                  subtitle={`${durationStr} • ${statusStr}`}
                  accessories={[
                    ...entry.tags.slice(0, 3).map((tag) => ({
                      tag: { value: tag, color: Color.Blue },
                    })),
                    ...(entry.tags.length > 3
                      ? [
                          {
                            tag: {
                              value: `+${entry.tags.length - 3}`,
                              color: Color.SecondaryText,
                            },
                          },
                        ]
                      : []),
                  ]}
                  actions={
                    <ActionPanel>
                      <ActionPanel.Section title="Entry Actions">
                        <Action.Push
                          title="Edit Entry"
                          icon={Icon.Pencil}
                          shortcut={{ modifiers: ["cmd"], key: "e" }}
                          target={
                            <EditEntryForm
                              entry={entry}
                              onComplete={() => revalidate()}
                            />
                          }
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
