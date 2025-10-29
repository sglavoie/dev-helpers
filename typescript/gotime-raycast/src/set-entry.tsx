import {
  Action,
  ActionPanel,
  Alert,
  Color,
  confirmAlert,
  Form,
  Icon,
  List,
  Toast,
  popToRoot,
  closeMainWindow,
  showToast,
} from "@raycast/api";
import { useExec } from "@raycast/utils";
import { useState, useEffect, useMemo } from "react";
import { execSync } from "child_process";
import {
  parseDuration,
  formatDuration,
  formatDurationCompact,
  calculateDuration,
} from "./utils/duration";
import {
  parseTimeInput,
  formatTime,
  applyTimeToDate,
  getTimeValidationError,
} from "./utils/time";

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

function formatDateTime(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  const seconds = String(date.getSeconds()).padStart(2, "0");

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

interface EditFormProps {
  entry: Entry;
  allKeywords: string[];
  allTags: string[];
  onComplete: () => void;
}

function EditEntryForm({
  entry,
  allKeywords,
  allTags,
  onComplete,
}: EditFormProps) {
  const [keywordError, setKeywordError] = useState<string | undefined>();
  const [keywordSearchText, setKeywordSearchText] = useState<string>("");
  const [durationInput, setDurationInput] = useState<string>(
    formatDurationCompact(entry.duration),
  );
  const [durationError, setDurationError] = useState<string | undefined>();
  const [calculatedDuration, setCalculatedDuration] = useState<number>(
    entry.duration,
  );
  const [selectedTags, setSelectedTags] = useState<string[]>(entry.tags ?? []);
  const [startDateTime, setStartDateTime] = useState<Date>(
    new Date(entry.start_time),
  );
  const [endDateTime, setEndDateTime] = useState<Date | undefined>(
    entry.end_time ? new Date(entry.end_time) : undefined,
  );
  const [timestampsChanged, setTimestampsChanged] = useState<boolean>(false);
  const [startTimeInput, setStartTimeInput] = useState<string>(
    formatTime(new Date(entry.start_time)),
  );
  const [endTimeInput, setEndTimeInput] = useState<string>(
    entry.end_time ? formatTime(new Date(entry.end_time)) : "",
  );
  const [startTimeError, setStartTimeError] = useState<string | undefined>();
  const [endTimeError, setEndTimeError] = useState<string | undefined>();

  // Update calculated duration when start or end time changes
  useEffect(() => {
    if (startDateTime && endDateTime) {
      try {
        const duration = calculateDuration(startDateTime, endDateTime);
        setCalculatedDuration(duration);
        setDurationInput(formatDurationCompact(duration));
        setDurationError(undefined);
      } catch (error) {
        setDurationError(
          error instanceof Error ? error.message : "Invalid time range",
        );
      }
    }
  }, [startDateTime, endDateTime]);

  // Detect timestamp changes
  useEffect(() => {
    const originalStart = new Date(entry.start_time);
    const originalEnd = entry.end_time ? new Date(entry.end_time) : null;

    const startChanged = startDateTime.getTime() !== originalStart.getTime();
    const endChanged = endDateTime
      ? originalEnd
        ? endDateTime.getTime() !== originalEnd.getTime()
        : true
      : false;

    setTimestampsChanged(startChanged || endChanged);
  }, [startDateTime, endDateTime, entry]);

  function handleDurationChange(value: string) {
    setDurationInput(value);
    setDurationError(undefined);

    try {
      const seconds = parseDuration(value);
      setCalculatedDuration(seconds);

      // Update end time based on new duration
      if (startDateTime) {
        const newEnd = new Date(startDateTime.getTime() + seconds * 1000);
        setEndDateTime(newEnd);
        setEndTimeInput(formatTime(newEnd));
      }
    } catch (error) {
      setDurationError(
        error instanceof Error ? error.message : "Invalid duration",
      );
    }
  }

  function handleStartTimeChange(value: string) {
    setStartTimeInput(value);
    setStartTimeError(undefined);

    if (!value.trim()) {
      return;
    }

    const parsed = parseTimeInput(value);
    if (parsed) {
      const newDateTime = applyTimeToDate(startDateTime, parsed);
      setStartDateTime(newDateTime);
    } else {
      setStartTimeError(getTimeValidationError(value));
    }
  }

  function handleEndTimeChange(value: string) {
    setEndTimeInput(value);
    setEndTimeError(undefined);

    if (!value.trim()) {
      setEndDateTime(undefined);
      return;
    }

    const parsed = parseTimeInput(value);
    if (parsed) {
      const baseDate = endDateTime || startDateTime;
      const newDateTime = applyTimeToDate(baseDate, parsed);
      setEndDateTime(newDateTime);
    } else {
      setEndTimeError(getTimeValidationError(value));
    }
  }

  async function handleSubmit(values: { keyword: string }) {
    // Use search text if available (for new keywords), otherwise use selected value
    const keyword = (keywordSearchText || values.keyword).trim();

    if (!keyword) {
      setKeywordError("Keyword is required");
      return;
    }

    // Validate keyword format
    if (!/^[a-zA-Z0-9_-]+$/.test(keyword)) {
      setKeywordError(
        "Keyword can only contain letters, numbers, dashes, and underscores",
      );
      return;
    }

    // Validate tags
    for (const tag of selectedTags) {
      if (!/^[a-zA-Z0-9_-]+$/.test(tag)) {
        await showToast({
          style: Toast.Style.Failure,
          title: "Invalid tag format",
          message: `Tag "${tag}" can only contain letters, numbers, dashes, and underscores`,
        });
        return;
      }
    }

    // Validate duration
    if (durationError) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Invalid duration",
        message: durationError,
      });
      return;
    }

    // Validate start time
    if (startTimeError) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Invalid start time",
        message: startTimeError,
      });
      return;
    }

    // Validate end time
    if (endTimeError) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Invalid end time",
        message: endTimeError,
      });
      return;
    }

    try {
      // If timestamps changed, use delete+recreate workflow
      if (timestampsChanged) {
        await handleTimestampEdit(keyword);
      } else {
        await handleSimpleEdit(keyword);
      }

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

  async function handleSimpleEdit(keyword: string) {
    await showToast({
      style: Toast.Style.Animated,
      title: "Updating entry...",
    });

    const commands: string[] = [];

    // Update keyword if changed
    if (keyword !== entry.keyword) {
      commands.push(
        `/Users/sglavoie/.local/bin/gt set ${entry.short_id} keyword ${keyword}`,
      );
    }

    // Update tags if changed
    const currentTags = (entry.tags ?? []).sort().join(",");
    const newTags = selectedTags.sort().join(",");
    if (newTags !== currentTags) {
      const tagsArg = newTags || '""';
      commands.push(
        `/Users/sglavoie/.local/bin/gt set ${entry.short_id} tags ${tagsArg}`,
      );
    }

    // Update duration if changed
    if (calculatedDuration !== entry.duration) {
      commands.push(
        `/Users/sglavoie/.local/bin/gt set ${entry.short_id} duration ${calculatedDuration}`,
      );
    }

    if (commands.length === 0) {
      await showToast({
        style: Toast.Style.Success,
        title: "No changes to apply",
      });
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
  }

  async function handleTimestampEdit(keyword: string) {
    // Confirm with user
    const confirmed = await confirmAlert({
      title: "Recreate Entry?",
      message:
        "Changing timestamps requires recreating the entry with a new ID. The original entry will be deleted. Continue?",
      icon: Icon.ExclamationMark,
      primaryAction: {
        title: "Recreate Entry",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (!confirmed) {
      await showToast({
        style: Toast.Style.Success,
        title: "Update cancelled",
      });
      return;
    }

    await showToast({
      style: Toast.Style.Animated,
      title: "Recreating entry...",
    });

    // Step 1: Calculate duration for backdate (gt expects relative duration, not absolute timestamp)
    const now = new Date();
    const diffMs = now.getTime() - startDateTime.getTime();

    // Validate that start time is not in the future
    if (diffMs < 0) {
      throw new Error("Start time cannot be in the future");
    }

    // Convert to minutes and round to avoid precision issues
    const diffMinutes = Math.round(diffMs / 60000);

    // Step 2: Create new entry with backdated start time
    const tagsArg = selectedTags.length > 0 ? selectedTags.join(" ") : "";
    const createCommand = `/Users/sglavoie/.local/bin/gt start ${keyword} ${tagsArg} --backdate ${diffMinutes}m`;

    execSync(createCommand, { encoding: "utf-8" });

    // Step 3: Get the newly created entry's ID
    const listOutput = execSync(
      `/Users/sglavoie/.local/bin/gt list --active --json`,
      { encoding: "utf-8" },
    );
    const activeEntries = JSON.parse(listOutput.trim()) as Entry[];
    const newEntry = activeEntries.find((e) => e.keyword === keyword);

    if (!newEntry) {
      throw new Error("Failed to find newly created entry");
    }

    // Step 4: Set the duration to match the calculated duration (this also stops the timer)
    execSync(
      `/Users/sglavoie/.local/bin/gt set ${newEntry.short_id} duration ${calculatedDuration}`,
      { encoding: "utf-8" },
    );

    // Step 5: Delete the old entry
    execSync(`/Users/sglavoie/.local/bin/gt delete ${entry.short_id}`, {
      encoding: "utf-8",
    });

    await showToast({
      style: Toast.Style.Success,
      title: "Entry recreated",
      message: `New ID: ${newEntry.short_id}`,
    });
  }

  // Combine existing tags with selected tags for autocomplete
  const availableTags = useMemo(() => {
    return Array.from(new Set([...allTags, ...selectedTags])).sort();
  }, [allTags, selectedTags]);

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
      <Form.Description
        text={
          entry.active
            ? "⚠️ This is an ACTIVE entry. Changes will stop the timer."
            : timestampsChanged
              ? "⚠️ Changing timestamps will RECREATE the entry with a new ID"
              : "Edit entry fields below"
        }
      />

      <Form.Dropdown
        id="keyword"
        title="Keyword"
        error={keywordError}
        defaultValue={entry.keyword}
        info="Type to filter existing keywords or enter a new one"
        onChange={() => setKeywordError(undefined)}
        onSearchTextChange={setKeywordSearchText}
        filtering={true}
        throttle={true}
      >
        {keywordSearchText && !allKeywords.includes(keywordSearchText) && (
          <Form.Dropdown.Item
            value={keywordSearchText}
            title={`Create "${keywordSearchText}"`}
            icon={Icon.Plus}
          />
        )}
        {allKeywords.map((kw) => (
          <Form.Dropdown.Item
            key={kw}
            value={kw}
            title={kw}
            icon={Icon.Clock}
          />
        ))}
      </Form.Dropdown>

      <Form.TagPicker
        id="tags"
        title="Tags"
        value={selectedTags}
        onChange={setSelectedTags}
        placeholder="Select tags for this entry"
      >
        {availableTags.length > 0 ? (
          availableTags.map((tag) => (
            <Form.TagPicker.Item
              key={tag}
              value={tag}
              title={tag}
              icon={Icon.Tag}
            />
          ))
        ) : (
          <Form.TagPicker.Item value="" title="No tags available" />
        )}
      </Form.TagPicker>

      <Form.Separator />

      <Form.TextField
        id="duration"
        title="Duration"
        placeholder="e.g., 1h30m or 90"
        value={durationInput}
        error={durationError}
        info="Enter as: 1h30m, 90m, 2h, or 90 (minutes)"
        onChange={handleDurationChange}
      />

      <Form.Description
        text={`Calculated: ${formatDuration(calculatedDuration)}`}
      />

      <Form.Separator />

      <Form.DatePicker
        id="startDate"
        title="Start Date"
        type={Form.DatePicker.Type.Date}
        value={startDateTime}
        onChange={(date) => {
          if (date) {
            // Preserve the time component when date changes
            const newDate = new Date(date);
            newDate.setHours(startDateTime.getHours());
            newDate.setMinutes(startDateTime.getMinutes());
            newDate.setSeconds(startDateTime.getSeconds());
            setStartDateTime(newDate);
          }
        }}
      />

      <Form.TextField
        id="startTime"
        title="Start Time"
        placeholder="14:30 or 2:30 PM"
        value={startTimeInput}
        error={startTimeError}
        info="Enter time in 24-hour (14:30) or 12-hour (2:30 PM) format"
        onChange={handleStartTimeChange}
      />

      <Form.DatePicker
        id="endDate"
        title="End Date"
        type={Form.DatePicker.Type.Date}
        value={endDateTime}
        onChange={(date) => {
          if (date && endDateTime) {
            // Preserve the time component when date changes
            const newDate = new Date(date);
            newDate.setHours(endDateTime.getHours());
            newDate.setMinutes(endDateTime.getMinutes());
            newDate.setSeconds(endDateTime.getSeconds());
            setEndDateTime(newDate);
          } else if (date) {
            // If no end time was set, use end of day
            const newDate = new Date(date);
            newDate.setHours(23);
            newDate.setMinutes(59);
            newDate.setSeconds(59);
            setEndDateTime(newDate);
            setEndTimeInput(formatTime(newDate));
          }
        }}
      />

      <Form.TextField
        id="endTime"
        title="End Time"
        placeholder="14:30 or 2:30 PM"
        value={endTimeInput}
        error={endTimeError}
        info={
          entry.active
            ? "Setting an end time will stop the active timer. Format: 14:30 or 2:30 PM"
            : "Enter time in 24-hour (14:30) or 12-hour (2:30 PM) format"
        }
        onChange={handleEndTimeChange}
      />

      <Form.Description
        text={`Original: ${formatDuration(entry.duration)} • ${entry.active ? "Active" : `Ended ${formatRelativeTime(entry.end_time)}`}`}
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

  // Extract unique keywords and tags for autocomplete
  const { keywords, tags } = useMemo(() => {
    if (!data || !Array.isArray(data) || data.length === 0) {
      return { keywords: [], tags: [] };
    }

    const keywordSet = new Set<string>();
    const tagSet = new Set<string>();

    data.forEach((entry) => {
      keywordSet.add(entry.keyword);
      const tags = entry.tags ?? [];
      tags.forEach((tag) => tagSet.add(tag));
    });

    return {
      keywords: Array.from(keywordSet).sort(),
      tags: Array.from(tagSet).sort(),
    };
  }, [data]);

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
                      <ActionPanel.Section title="Entry Actions">
                        <Action.Push
                          title="Edit Entry"
                          icon={Icon.Pencil}
                          shortcut={{ modifiers: ["cmd"], key: "e" }}
                          target={
                            <EditEntryForm
                              entry={entry}
                              allKeywords={keywords}
                              allTags={tags}
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
