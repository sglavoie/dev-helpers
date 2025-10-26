import {
  Action,
  ActionPanel,
  Form,
  Toast,
  closeMainWindow,
  popToRoot,
  showToast,
} from "@raycast/api";
import { useState } from "react";
import { execSync } from "child_process";

interface FormValues {
  keyword: string;
  tags: string;
}

export default function Command() {
  const [keywordError, setKeywordError] = useState<string | undefined>();

  async function handleSubmit(values: FormValues) {
    // Validate keyword
    const keyword = values.keyword.trim();
    if (!keyword) {
      setKeywordError("Keyword is required");
      return;
    }

    // Validate keyword format (no spaces, alphanumeric and dashes/underscores)
    if (!/^[a-zA-Z0-9_-]+$/.test(keyword)) {
      setKeywordError(
        "Keyword can only contain letters, numbers, dashes, and underscores",
      );
      return;
    }

    // Parse tags (comma-separated)
    const tags = values.tags
      .split(",")
      .map((t) => t.trim())
      .filter((t) => t.length > 0);

    // Validate tags format
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
        title: "Starting timer...",
      });

      // Build command
      const command =
        tags.length > 0
          ? `/Users/sglavoie/.local/bin/gt start ${keyword} ${tags.join(" ")}`
          : `/Users/sglavoie/.local/bin/gt start ${keyword}`;

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
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Start Timer" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="keyword"
        title="Keyword"
        placeholder="e.g., coding, meeting, research"
        error={keywordError}
        info="A short identifier for this activity"
        onChange={() => setKeywordError(undefined)}
      />
      <Form.TextField
        id="tags"
        title="Tags"
        placeholder="e.g., golang, cli, project-name"
        info="Optional tags (comma-separated)"
      />
      <Form.Description text="💡 Keywords and tags can only contain letters, numbers, dashes, and underscores (no spaces)." />
    </Form>
  );
}
