import { Alert, Icon, Toast, confirmAlert, showToast } from "@raycast/api";
import { runGT } from "./gt";
import { entryUpdateArgs, EntryEdit } from "./entry-update";
import { Entry } from "./entries";
import { formatDuration } from "./duration";

/** Validate and apply all changed fields in one operation, keeping the UUID. */
export async function updateEntry(edit: EntryEdit) {
  const args = entryUpdateArgs(edit);
  if (args.length === 0) {
    await showToast({
      style: Toast.Style.Success,
      title: "No changes to apply",
    });
    return;
  }
  await showToast({ style: Toast.Style.Animated, title: "Updating entry..." });
  await runGT(args);
  await showToast({ style: Toast.Style.Success, title: "Entry updated" });
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

    await runGT(["delete", entry.id]);

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
