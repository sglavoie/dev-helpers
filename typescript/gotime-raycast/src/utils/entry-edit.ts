import { Alert, Icon, Toast, confirmAlert, showToast } from "@raycast/api";
import { execSync } from "child_process";
import { Entry, GT_BIN } from "./entries";
import { formatDuration } from "./duration";

interface EntryEdit {
  entry: Entry;
  keyword: string;
  tags: string[];
  duration: number;
}

/**
 * Applies the changed fields of an entry in place with `gt set`, which is all
 * that is needed as long as the timestamps are untouched.
 */
export async function applyFieldEdits({
  entry,
  keyword,
  tags,
  duration,
}: EntryEdit) {
  await showToast({
    style: Toast.Style.Animated,
    title: "Updating entry...",
  });

  const commands: string[] = [];

  // Update keyword if changed
  if (keyword !== entry.keyword) {
    commands.push(`${GT_BIN} set ${entry.short_id} keyword ${keyword}`);
  }

  // Update tags if changed
  const currentTags = [...(entry.tags ?? [])].sort().join(",");
  const newTags = [...tags].sort().join(",");
  if (newTags !== currentTags) {
    const tagsArg = newTags || '""';
    commands.push(`${GT_BIN} set ${entry.short_id} tags ${tagsArg}`);
  }

  // Update duration if changed
  if (duration !== entry.duration) {
    commands.push(`${GT_BIN} set ${entry.short_id} duration ${duration}`);
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

/**
 * Rewrites an entry whose timestamps changed. `gt` cannot move an existing
 * entry in time, so the entry is recreated with a backdated start and the
 * original is deleted.
 */
export async function recreateEntryWithNewTimestamps({
  entry,
  keyword,
  tags,
  duration,
  startDateTime,
}: EntryEdit & { startDateTime: Date }) {
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
  const tagsArg = tags.length > 0 ? tags.join(" ") : "";
  execSync(`${GT_BIN} start ${keyword} ${tagsArg} --backdate ${diffMinutes}m`, {
    encoding: "utf-8",
  });

  // Step 3: Get the newly created entry's ID
  const listOutput = execSync(`${GT_BIN} list --active --json`, {
    encoding: "utf-8",
  });
  const activeEntries = JSON.parse(listOutput.trim()) as Entry[];
  const newEntry = activeEntries.find((e) => e.keyword === keyword);

  if (!newEntry) {
    throw new Error("Failed to find newly created entry");
  }

  // Step 4: Set the duration to match the calculated duration (this also stops the timer)
  execSync(`${GT_BIN} set ${newEntry.short_id} duration ${duration}`, {
    encoding: "utf-8",
  });

  // Step 5: Delete the old entry
  execSync(`${GT_BIN} delete ${entry.short_id}`, { encoding: "utf-8" });

  await showToast({
    style: Toast.Style.Success,
    title: "Entry recreated",
    message: `New ID: ${newEntry.short_id}`,
  });
}

/** Deletes an entry after asking for confirmation, reporting via toasts. */
export async function deleteEntryWithConfirmation(
  entry: Entry,
): Promise<boolean> {
  const confirmed = await confirmAlert({
    title: "Delete Entry",
    message: `Delete "${entry.keyword}" (${formatDuration(entry.duration)})?`,
    icon: Icon.Trash,
    primaryAction: {
      title: "Delete",
      style: Alert.ActionStyle.Destructive,
    },
  });

  if (!confirmed) return false;

  try {
    await showToast({
      style: Toast.Style.Animated,
      title: "Deleting entry...",
    });

    execSync(`${GT_BIN} delete ${entry.short_id}`, { encoding: "utf-8" });

    await showToast({
      style: Toast.Style.Success,
      title: "Entry deleted",
      message: `Deleted "${entry.keyword}"`,
    });

    return true;
  } catch (error) {
    await showToast({
      style: Toast.Style.Failure,
      title: "Failed to delete entry",
      message: error instanceof Error ? error.message : String(error),
    });
    return false;
  }
}
