import {
  Action,
  ActionPanel,
  Form,
  Icon,
  Toast,
  popToRoot,
  closeMainWindow,
  showToast,
} from "@raycast/api";
import { useState, useEffect, useMemo } from "react";
import {
  parseDuration,
  formatDuration,
  formatDurationCompact,
  calculateDuration,
} from "../utils/duration";
import {
  parseTimeInput,
  formatTime,
  applyTimeToDate,
  getTimeValidationError,
} from "../utils/time";
import { Entry, formatRelativeTime } from "../utils/entries";
import {
  applyFieldEdits,
  recreateEntryWithNewTimestamps,
} from "../utils/entry-edit";

export interface EditFormProps {
  entry: Entry;
  allKeywords: string[];
  allTags: string[];
  onComplete: () => void;
}

/** Keywords and tags are restricted to what the gotime CLI accepts unquoted. */
const NAME_PATTERN = /^[a-zA-Z0-9_-]+$/;

export function EditEntryForm({
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
      setStartDateTime(applyTimeToDate(startDateTime, parsed));
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
      setEndDateTime(applyTimeToDate(endDateTime || startDateTime, parsed));
    } else {
      setEndTimeError(getTimeValidationError(value));
    }
  }

  /**
   * Reports the first blocking problem with the form, or undefined when the
   * entry is safe to write back.
   */
  function findSubmitBlocker(): { title: string; message: string } | undefined {
    for (const tag of selectedTags) {
      if (!NAME_PATTERN.test(tag)) {
        return {
          title: "Invalid tag format",
          message: `Tag "${tag}" can only contain letters, numbers, dashes, and underscores`,
        };
      }
    }

    if (durationError) {
      return { title: "Invalid duration", message: durationError };
    }
    if (startTimeError) {
      return { title: "Invalid start time", message: startTimeError };
    }
    if (endTimeError) {
      return { title: "Invalid end time", message: endTimeError };
    }

    return undefined;
  }

  async function handleSubmit(values: { keyword: string }) {
    // Use search text if available (for new keywords), otherwise use selected value
    const keyword = (keywordSearchText || values.keyword).trim();

    if (!keyword) {
      setKeywordError("Keyword is required");
      return;
    }

    if (!NAME_PATTERN.test(keyword)) {
      setKeywordError(
        "Keyword can only contain letters, numbers, dashes, and underscores",
      );
      return;
    }

    const blocker = findSubmitBlocker();
    if (blocker) {
      await showToast({ style: Toast.Style.Failure, ...blocker });
      return;
    }

    try {
      const edit = {
        entry,
        keyword,
        tags: selectedTags,
        duration: calculatedDuration,
      };

      // If timestamps changed, use delete+recreate workflow
      if (timestampsChanged) {
        await recreateEntryWithNewTimestamps({ ...edit, startDateTime });
      } else {
        await applyFieldEdits(edit);
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

  // Combine existing tags with selected tags for autocomplete
  const availableTags = useMemo(() => {
    return Array.from(new Set([...allTags, ...selectedTags])).sort();
  }, [allTags, selectedTags]);

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
            setStartDateTime(withTimeOf(date, startDateTime));
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
            setEndDateTime(withTimeOf(date, endDateTime));
          } else if (date) {
            // If no end time was set, use end of day
            const newDate = new Date(date);
            newDate.setHours(23, 59, 59);
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

/** Returns the given date carrying the time of day of another one. */
function withTimeOf(date: Date, timeSource: Date): Date {
  const combined = new Date(date);
  combined.setHours(
    timeSource.getHours(),
    timeSource.getMinutes(),
    timeSource.getSeconds(),
  );
  return combined;
}
