import {
  Action,
  ActionPanel,
  Form,
  Icon,
  Toast,
  closeMainWindow,
  popToRoot,
  showToast,
} from "@raycast/api";
import { useExec } from "@raycast/utils";
import { useState, useEffect, useMemo } from "react";
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
  keyword: string;
  tags: string[];
  backdate: string;
  customBackdate: string;
}

export default function Command() {
  const [keywordError, setKeywordError] = useState<string | undefined>();
  const [keywordSearchText, setKeywordSearchText] = useState<string>("");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [newTagInput, setNewTagInput] = useState<string>("");
  const [newTagError, setNewTagError] = useState<string | undefined>();

  // Fetch existing entries to extract keywords and tags
  const { isLoading, data: entries } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--days", "30", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) return [];
        return JSON.parse(trimmed) as Entry[];
      },
    },
  );

  // Extract unique keywords and tags from entries
  const { keywords, tags } = useMemo(() => {
    if (!entries || entries.length === 0) {
      return { keywords: [], tags: [] };
    }

    const keywordSet = new Set<string>();
    const tagSet = new Set<string>();

    entries.forEach((entry) => {
      keywordSet.add(entry.keyword);
      entry.tags.forEach((tag) => tagSet.add(tag));
    });

    return {
      keywords: Array.from(keywordSet).sort(),
      tags: Array.from(tagSet).sort(),
    };
  }, [entries]);

  // Combine existing tags with selected tags for the picker
  const allTags = useMemo(() => {
    return Array.from(new Set([...tags, ...selectedTags])).sort();
  }, [tags, selectedTags]);

  // Handle adding a new tag
  function handleAddTag() {
    const trimmedTag = newTagInput.trim();

    if (!trimmedTag) {
      return;
    }

    // Validate tag format
    if (!/^[a-zA-Z0-9_-]+$/.test(trimmedTag)) {
      setNewTagError(
        "Tag can only contain letters, numbers, dashes, and underscores",
      );
      return;
    }

    // Check if tag already exists
    if (selectedTags.includes(trimmedTag)) {
      setNewTagError("Tag already added");
      return;
    }

    // Add the tag
    setSelectedTags([...selectedTags, trimmedTag]);
    setNewTagInput("");
    setNewTagError(undefined);

    showToast({
      style: Toast.Style.Success,
      title: "Tag added",
      message: trimmedTag,
    });
  }

  async function handleSubmit(values: FormValues) {
    // Use the search text if available (for new keywords), otherwise use selected value
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

    // Get backdate value
    const backdateValue =
      values.backdate === "custom"
        ? values.customBackdate.trim()
        : values.backdate;

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Starting timer...",
      });

      // Build command
      let command = `/Users/sglavoie/.local/bin/gt start ${keyword}`;

      // Add tags if present
      if (values.tags.length > 0) {
        command += ` ${values.tags.join(" ")}`;
      }

      // Add backdate flag if present
      if (backdateValue && backdateValue !== "none") {
        command += ` --backdate ${backdateValue}`;
      }

      execSync(command, { encoding: "utf-8" });

      await showToast({
        style: Toast.Style.Success,
        title: "Timer started",
        message: `Started "${keyword}"`,
      });

      await popToRoot();
      await closeMainWindow();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to start timer",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  return (
    <Form
      isLoading={isLoading}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Start Timer" onSubmit={handleSubmit} />
          <Action
            title="Add Tag"
            icon={Icon.Plus}
            shortcut={{ modifiers: ["cmd"], key: "t" }}
            onAction={handleAddTag}
          />
        </ActionPanel>
      }
    >
      <Form.Dropdown
        id="keyword"
        title="Keyword"
        error={keywordError}
        info="Type to filter existing keywords or enter a new one"
        onChange={() => setKeywordError(undefined)}
        onSearchTextChange={setKeywordSearchText}
        filtering={true}
        throttle={true}
      >
        {keywordSearchText && !keywords.includes(keywordSearchText) && (
          <Form.Dropdown.Item
            value={keywordSearchText}
            title={`Create "${keywordSearchText}"`}
            icon={Icon.Plus}
          />
        )}
        {keywords.map((kw) => (
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
        placeholder="Select tags to add to this timer"
      >
        {allTags.length > 0 ? (
          allTags.map((tag) => (
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

      <Form.TextField
        id="newTag"
        title="Add New Tag"
        placeholder="e.g., golang, cli, project-name"
        value={newTagInput}
        error={newTagError}
        info="Type a tag name and press Cmd+T to add it"
        onChange={(value) => {
          setNewTagInput(value);
          setNewTagError(undefined);
        }}
      />

      <Form.Dropdown
        id="backdate"
        title="Backdate"
        info="Start the timer as if it began in the past (optional)"
        defaultValue="none"
      >
        <Form.Dropdown.Item
          value="none"
          title="No backdate (start now)"
          icon={Icon.Clock}
        />
        <Form.Dropdown.Section title="Quick Options">
          <Form.Dropdown.Item
            value="5m"
            title="5 minutes ago"
            icon={Icon.ChevronDown}
          />
          <Form.Dropdown.Item
            value="10m"
            title="10 minutes ago"
            icon={Icon.ChevronDown}
          />
          <Form.Dropdown.Item
            value="15m"
            title="15 minutes ago"
            icon={Icon.ChevronDown}
          />
          <Form.Dropdown.Item
            value="20m"
            title="20 minutes ago"
            icon={Icon.ChevronDown}
          />
          <Form.Dropdown.Item
            value="25m"
            title="25 minutes ago"
            icon={Icon.ChevronDown}
          />
          <Form.Dropdown.Item
            value="30m"
            title="30 minutes ago"
            icon={Icon.ChevronDown}
          />
        </Form.Dropdown.Section>
        <Form.Dropdown.Section title="Custom">
          <Form.Dropdown.Item
            value="custom"
            title="Custom value..."
            icon={Icon.Pencil}
          />
        </Form.Dropdown.Section>
      </Form.Dropdown>

      <Form.TextField
        id="customBackdate"
        title="Custom Backdate"
        placeholder="e.g., 45m, 1h, 1h30m"
        info="Enter custom backdate value (e.g., 45m, 1h, 1h30m)"
      />

      <Form.Description text="💡 Press Cmd+T to add tags. Tags appear as badges above - click to remove. Keywords and tags can only contain letters, numbers, dashes, and underscores (no spaces)." />
    </Form>
  );
}
