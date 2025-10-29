import {
  Action,
  ActionPanel,
  Form,
  Toast,
  popToRoot,
  closeMainWindow,
  showToast,
} from "@raycast/api";
import { useExec } from "@raycast/utils";
import { useState } from "react";
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

interface FormValues {
  oldTag: string;
  newTag: string;
}

export default function Command() {
  const [newTagError, setNewTagError] = useState<string | undefined>();

  const { isLoading, data: tags } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--days", "3650", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          return [];
        }
        const entries = JSON.parse(trimmed) as Entry[];

        // Extract unique tags
        const tagSet = new Set<string>();
        entries.forEach((entry) => {
          const tags = entry.tags ?? [];
          tags.forEach((tag) => tagSet.add(tag));
        });

        // Convert to sorted array
        return Array.from(tagSet).sort();
      },
    },
  );

  async function handleSubmit(values: FormValues) {
    // Validate old tag
    const oldTag = values.oldTag?.trim();
    if (!oldTag) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Please select a tag to rename",
      });
      return;
    }

    // Validate new tag
    const newTag = values.newTag.trim();
    if (!newTag) {
      setNewTagError("New tag name is required");
      return;
    }

    // Validate new tag format (alphanumeric, dashes, underscores)
    if (!/^[a-zA-Z0-9_-]+$/.test(newTag)) {
      setNewTagError(
        "Tag can only contain letters, numbers, dashes, and underscores",
      );
      return;
    }

    if (oldTag === newTag) {
      setNewTagError("New tag must be different from old tag");
      return;
    }

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Renaming tag...",
      });

      execSync(
        `/Users/sglavoie/.local/bin/gt tags rename ${oldTag} ${newTag}`,
        { encoding: "utf-8" },
      );

      await showToast({
        style: Toast.Style.Success,
        title: "Tag renamed",
        message: `Renamed "${oldTag}" to "${newTag}"`,
      });

      await popToRoot();
      await closeMainWindow();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to rename tag",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  return (
    <Form
      isLoading={isLoading}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Rename Tag" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.Dropdown
        id="oldTag"
        title="Old Tag Name"
        info="Select the tag you want to rename"
      >
        <Form.Dropdown.Item value="" title="Select a tag..." />
        {tags && tags.length > 0 ? (
          tags.map((tag) => (
            <Form.Dropdown.Item key={tag} value={tag} title={tag} />
          ))
        ) : (
          <Form.Dropdown.Item value="" title="No tags found" />
        )}
      </Form.Dropdown>
      <Form.TextField
        id="newTag"
        title="New Tag Name"
        placeholder="e.g., office, project-new"
        error={newTagError}
        info="The new name for the tag"
        onChange={() => setNewTagError(undefined)}
      />
      <Form.Description text="💡 This will rename the tag across ALL entries that use it." />
    </Form>
  );
}
