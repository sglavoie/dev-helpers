import {
  Action,
  ActionPanel,
  Color,
  Icon,
  List,
  Toast,
  showToast,
} from "@raycast/api";
import { useExec } from "@raycast/utils";
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

  async function handleContinueTimer(entry: Entry) {
    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Continuing timer...",
      });

      execSync(`/Users/sglavoie/.local/bin/gt continue ${entry.short_id}`, {
        encoding: "utf-8",
      });

      await showToast({
        style: Toast.Style.Success,
        title: "Timer continued",
        message: `Resumed "${entry.keyword}"`,
      });

      revalidate();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to continue timer",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

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
                      <Action
                        title="Continue Timer"
                        icon={Icon.Play}
                        shortcut={{ modifiers: ["cmd"], key: "c" }}
                        onAction={() => handleContinueTimer(entry)}
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
