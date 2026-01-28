import {
  Action,
  ActionPanel,
  Color,
  Form,
  Icon,
  List,
  Toast,
  showToast,
  useNavigation,
  popToRoot,
  closeMainWindow,
} from "@raycast/api";
import { useExec } from "@raycast/utils";
import { useState, useMemo } from "react";
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

interface ConfigFormValues {
  tags: string[];
  backdate: string;
  customBackdate: string;
}

export default function Command() {
  const [selectedKeywords, setSelectedKeywords] = useState<Set<string>>(
    new Set(),
  );
  const { push } = useNavigation();

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

  // Extract unique keywords, excluding those with active timers
  const { availableKeywords, allTags } = useMemo(() => {
    if (!entries || entries.length === 0) {
      return { availableKeywords: [], allTags: [] };
    }

    const activeKeywords = new Set(
      entries.filter((e) => e.active).map((e) => e.keyword),
    );

    const keywordSet = new Set<string>();
    const tagSet = new Set<string>();

    entries.forEach((entry) => {
      if (!activeKeywords.has(entry.keyword)) {
        keywordSet.add(entry.keyword);
      }
      const tags = entry.tags ?? [];
      tags.forEach((tag) => tagSet.add(tag));
    });

    return {
      availableKeywords: Array.from(keywordSet).sort(),
      allTags: Array.from(tagSet).sort(),
    };
  }, [entries]);

  function toggleKeyword(keyword: string) {
    setSelectedKeywords((prev) => {
      const next = new Set(prev);
      if (next.has(keyword)) {
        next.delete(keyword);
      } else {
        next.add(keyword);
      }
      return next;
    });
  }

  function clearSelection() {
    setSelectedKeywords(new Set());
  }

  async function proceedToConfiguration() {
    if (selectedKeywords.size === 0) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Select at least one keyword",
      });
      return;
    }

    push(
      <ConfigurationForm
        selectedKeywords={Array.from(selectedKeywords)}
        allTags={allTags}
      />,
    );
  }

  return (
    <List
      isLoading={isLoading}
      searchBarPlaceholder="Filter keywords..."
      navigationTitle={`Selected: ${selectedKeywords.size}`}
    >
      {!isLoading && availableKeywords.length === 0 ? (
        <List.EmptyView
          icon={Icon.Clock}
          title="No Keywords Available"
          description="No historical keywords found. Start a timer first using 'GoTime: Start Timer'."
        />
      ) : (
        availableKeywords.map((keyword) => {
          const isSelected = selectedKeywords.has(keyword);
          return (
            <List.Item
              key={keyword}
              icon={{
                source: isSelected ? Icon.CheckCircle : Icon.Circle,
                tintColor: isSelected ? Color.Green : Color.SecondaryText,
              }}
              title={keyword}
              accessories={
                isSelected
                  ? [{ tag: { value: "Selected", color: Color.Green } }]
                  : []
              }
              actions={
                <ActionPanel>
                  <ActionPanel.Section title="Selection">
                    <Action
                      title={isSelected ? "Deselect" : "Select"}
                      icon={isSelected ? Icon.Circle : Icon.CheckCircle}
                      shortcut={{ modifiers: ["cmd"], key: "t" }}
                      onAction={() => toggleKeyword(keyword)}
                    />
                    <Action
                      title={`Start Selected (${selectedKeywords.size})`}
                      icon={Icon.Play}
                      shortcut={{ modifiers: ["cmd"], key: "return" }}
                      onAction={proceedToConfiguration}
                    />
                    <Action
                      title="Clear Selection"
                      icon={Icon.XMarkCircle}
                      shortcut={{ modifiers: ["cmd", "shift"], key: "c" }}
                      onAction={clearSelection}
                    />
                  </ActionPanel.Section>
                </ActionPanel>
              }
            />
          );
        })
      )}
    </List>
  );
}

function ConfigurationForm(props: {
  selectedKeywords: string[];
  allTags: string[];
}) {
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [newTagInput, setNewTagInput] = useState<string>("");
  const [newTagError, setNewTagError] = useState<string | undefined>();

  const allAvailableTags = useMemo(() => {
    return Array.from(new Set([...props.allTags, ...selectedTags])).sort();
  }, [props.allTags, selectedTags]);

  function handleAddTag() {
    const trimmedTag = newTagInput.trim();

    if (!trimmedTag) {
      return;
    }

    if (!/^[a-zA-Z0-9_-]+$/.test(trimmedTag)) {
      setNewTagError(
        "Tag can only contain letters, numbers, dashes, and underscores",
      );
      return;
    }

    if (selectedTags.includes(trimmedTag)) {
      setNewTagError("Tag already added");
      return;
    }

    setSelectedTags([...selectedTags, trimmedTag]);
    setNewTagInput("");
    setNewTagError(undefined);

    showToast({
      style: Toast.Style.Success,
      title: "Tag added",
      message: trimmedTag,
    });
  }

  async function handleSubmit(values: ConfigFormValues) {
    const backdateValue =
      values.backdate === "custom"
        ? values.customBackdate.trim()
        : values.backdate;

    const successes: string[] = [];
    const failures: { keyword: string; error: string }[] = [];

    await showToast({
      style: Toast.Style.Animated,
      title: "Starting timers...",
      message: `0 of ${props.selectedKeywords.length}`,
    });

    for (const keyword of props.selectedKeywords) {
      try {
        let command = `/Users/sglavoie/.local/bin/gt start ${keyword}`;

        if (values.tags.length > 0) {
          command += ` ${values.tags.join(" ")}`;
        }

        if (backdateValue && backdateValue !== "none") {
          command += ` --backdate ${backdateValue}`;
        }

        execSync(command, { encoding: "utf-8" });
        successes.push(keyword);

        await showToast({
          style: Toast.Style.Animated,
          title: "Starting timers...",
          message: `${successes.length} of ${props.selectedKeywords.length}`,
        });
      } catch (error) {
        const errorMsg = error instanceof Error ? error.message : String(error);
        failures.push({ keyword, error: errorMsg });
      }
    }

    if (failures.length === 0) {
      await showToast({
        style: Toast.Style.Success,
        title: `Started ${successes.length} timer${successes.length !== 1 ? "s" : ""}`,
        message: successes.join(", "),
      });
    } else if (successes.length === 0) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to start timers",
        message: failures.map((f) => f.keyword).join(", "),
      });
    } else {
      await showToast({
        style: Toast.Style.Success,
        title: `Started ${successes.length} timer${successes.length !== 1 ? "s" : ""}`,
        message: `Failed: ${failures.map((f) => f.keyword).join(", ")}`,
      });
    }

    await popToRoot();
    await closeMainWindow();
  }

  return (
    <Form
      navigationTitle="Configure Timers"
      actions={
        <ActionPanel>
          <Action.SubmitForm
            title={`Start ${props.selectedKeywords.length} Timer${props.selectedKeywords.length !== 1 ? "s" : ""}`}
            onSubmit={handleSubmit}
          />
          <Action
            title="Add Tag"
            icon={Icon.Plus}
            shortcut={{ modifiers: ["cmd"], key: "t" }}
            onAction={handleAddTag}
          />
        </ActionPanel>
      }
    >
      <Form.Description
        title="Selected Keywords"
        text={props.selectedKeywords.join(", ")}
      />

      <Form.TagPicker
        id="tags"
        title="Tags"
        value={selectedTags}
        onChange={setSelectedTags}
        placeholder="Select tags to add to all timers"
      >
        {allAvailableTags.length > 0 ? (
          allAvailableTags.map((tag) => (
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
        info="Start all timers as if they began in the past (optional)"
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

      <Form.Description text="💡 Press Cmd+T to add tags. All selected timers will receive the same tags and backdate configuration." />
    </Form>
  );
}
