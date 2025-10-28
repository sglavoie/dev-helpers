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
  if (!dateString) return "Unknown";

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

export default function Command() {
  const { isLoading, data, error, revalidate } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--days", "7", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          return [];
        }
        const entries = JSON.parse(trimmed) as Entry[];

        // Build set of keywords that have active timers
        const activeKeywords = new Set(
          entries.filter((e) => e.active).map((e) => e.keyword),
        );

        // Filter to stopped timers whose keywords aren't currently active
        const stoppedFiltered = entries
          .filter((e) => !e.active && !activeKeywords.has(e.keyword))
          .sort((a, b) => {
            if (!a.end_time) return 1;
            if (!b.end_time) return -1;
            return (
              new Date(b.end_time).getTime() - new Date(a.end_time).getTime()
            );
          });

        // Filter to keep only unique keyword + tag combinations (most recent first)
        const seen = new Set<string>();
        return stoppedFiltered.filter((entry) => {
          const sortedTags = [...entry.tags].sort().join(",");
          const key = `${entry.keyword}:${sortedTags}`;
          if (seen.has(key)) {
            return false;
          }
          seen.add(key);
          return true;
        });
      },
    },
  );

  if (error) {
    return (
      <List>
        <List.Item title="Error" subtitle={error.message} />
      </List>
    );
  }

  return (
    <List isLoading={isLoading}>
      {!isLoading && (!data || data.length === 0) ? (
        <List.EmptyView
          icon={Icon.Clock}
          title="No Recent Stopped Timers"
          description="No stopped timers found in the last 7 days"
        />
      ) : (
        <>
          {data?.map((entry) => {
            const durationStr = formatDuration(entry.duration);
            const endedStr = formatRelativeTime(entry.end_time);

            return (
              <List.Item
                key={entry.id}
                icon={{ source: Icon.Circle, tintColor: Color.SecondaryText }}
                title={entry.keyword}
                subtitle={`${durationStr} • Ended ${endedStr}`}
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
                    <ActionPanel.Section title="Timer Actions">
                      <ContinueTimerAction
                        entry={entry}
                        onComplete={revalidate}
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

interface BackdateFormValues {
  backdate: string;
  customBackdate: string;
}

function ContinueTimerAction(props: { entry: Entry; onComplete: () => void }) {
  const { push } = useNavigation();

  return (
    <Action
      title="Continue Timer"
      icon={Icon.Play}
      shortcut={{ modifiers: ["cmd"], key: "c" }}
      onAction={() => {
        push(
          <BackdateForm entry={props.entry} onComplete={props.onComplete} />,
        );
      }}
    />
  );
}

function BackdateForm(props: { entry: Entry; onComplete: () => void }) {
  const { pop } = useNavigation();

  async function handleSubmit(values: BackdateFormValues) {
    const backdateValue =
      values.backdate === "custom"
        ? values.customBackdate.trim()
        : values.backdate;

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Continuing timer...",
      });

      // Build command
      let command = `/Users/sglavoie/.local/bin/gt continue ${props.entry.short_id}`;

      // Add backdate flag if present
      if (backdateValue && backdateValue !== "none") {
        command += ` --backdate ${backdateValue}`;
      }

      execSync(command, { encoding: "utf-8" });

      await showToast({
        style: Toast.Style.Success,
        title: "Timer continued",
        message: `Resumed "${props.entry.keyword}"`,
      });

      props.onComplete();
      pop();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to continue timer",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Continue Timer" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.Description
        title="Continue Timer"
        text={`Continuing "${props.entry.keyword}" with tags: ${props.entry.tags.length > 0 ? props.entry.tags.join(", ") : "none"}`}
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

      <Form.Description text="💡 Backdate allows you to start the timer as if it began in the past." />
    </Form>
  );
}
