import { Action, ActionPanel, Color, Icon, List } from "@raycast/api";
import { useExec } from "@raycast/utils";

interface ReportData {
  title: string;
  time_range: string;
  completed_entries?: KeywordSummary[];
  active_entries?: Entry[];
  weekly_data?: WeeklyData;
  total_duration: number;
  completed_duration: number;
  active_duration: number;
}

interface KeywordSummary {
  keyword: string;
  duration: number;
  entries: number;
  tags: string[];
}

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

interface WeeklyData {
  keywords: WeeklyKeyword[];
  daily_totals: number[];
  grand_total: number;
}

interface WeeklyKeyword {
  keyword: string;
  daily_data: number[];
  keyword_total: number;
}

const DAY_NAMES = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

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

function formatDurationCompact(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  return `${hours.toString().padStart(2, "0")}:${minutes.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
}

export default function Command() {
  const { isLoading, data, error, revalidate } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["report", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          throw new Error("Empty output from report command");
        }
        return JSON.parse(trimmed) as ReportData;
      },
    },
  );

  if (error) {
    return (
      <List>
        <List.EmptyView
          icon={Icon.XMarkCircle}
          title="Failed to load report"
          description={error.message}
          actions={
            <ActionPanel>
              <Action
                title="Retry"
                icon={Icon.ArrowClockwise}
                onAction={() => revalidate()}
              />
            </ActionPanel>
          }
        />
      </List>
    );
  }

  const weeklyData = data?.weekly_data;
  const totalDuration = data?.total_duration || 0;

  return (
    <List
      isLoading={isLoading}
      navigationTitle={data?.title || "Weekly Report"}
    >
      {!isLoading && (!weeklyData || weeklyData.keywords.length === 0) ? (
        <List.EmptyView
          icon={Icon.Document}
          title="No Data for This Week"
          description="Start tracking time to see your weekly report"
        />
      ) : (
        <>
          {/* Summary section */}
          <List.Section title="Summary">
            <List.Item
              title="Total Time This Week"
              icon={Icon.Clock}
              accessories={[
                {
                  text: formatDuration(totalDuration),
                  tooltip: formatDurationCompact(totalDuration),
                },
              ]}
            />
          </List.Section>

          {/* Keywords section */}
          {weeklyData && weeklyData.keywords.length > 0 && (
            <List.Section title="By Keyword">
              {weeklyData.keywords.map((keyword) => {
                const activeDays = keyword.daily_data.filter(
                  (d) => d > 0,
                ).length;

                return (
                  <List.Item
                    key={keyword.keyword}
                    title={keyword.keyword}
                    icon={{ source: Icon.Tag, tintColor: Color.Blue }}
                    accessories={[
                      {
                        text: formatDuration(keyword.keyword_total),
                        tooltip: `${activeDays} day${activeDays !== 1 ? "s" : ""}`,
                      },
                    ]}
                    detail={
                      <List.Item.Detail
                        metadata={
                          <List.Item.Detail.Metadata>
                            <List.Item.Detail.Metadata.Label
                              title="Total"
                              text={formatDuration(keyword.keyword_total)}
                            />
                            <List.Item.Detail.Metadata.Separator />
                            {keyword.daily_data.map((duration, dayIndex) => (
                              <List.Item.Detail.Metadata.Label
                                key={dayIndex}
                                title={DAY_NAMES[dayIndex]}
                                text={
                                  duration > 0
                                    ? formatDurationCompact(duration)
                                    : "-"
                                }
                              />
                            ))}
                          </List.Item.Detail.Metadata>
                        }
                      />
                    }
                    actions={
                      <ActionPanel>
                        <Action.CopyToClipboard
                          title="Copy Keyword"
                          content={keyword.keyword}
                          shortcut={{ modifiers: ["cmd"], key: "c" }}
                        />
                        <Action
                          title="Refresh"
                          icon={Icon.ArrowClockwise}
                          shortcut={{ modifiers: ["cmd"], key: "r" }}
                          onAction={() => revalidate()}
                        />
                      </ActionPanel>
                    }
                  />
                );
              })}
            </List.Section>
          )}

          {/* Daily totals section */}
          {weeklyData && weeklyData.daily_totals.length > 0 && (
            <List.Section title="By Day">
              {weeklyData.daily_totals.map((duration, dayIndex) => {
                const hasData = duration > 0;
                return (
                  <List.Item
                    key={dayIndex}
                    title={DAY_NAMES[dayIndex]}
                    icon={{
                      source: Icon.Calendar,
                      tintColor: hasData ? Color.Blue : Color.SecondaryText,
                    }}
                    accessories={[
                      {
                        text: hasData ? formatDuration(duration) : "No data",
                        tooltip: hasData
                          ? formatDurationCompact(duration)
                          : undefined,
                      },
                    ]}
                  />
                );
              })}
            </List.Section>
          )}
        </>
      )}
    </List>
  );
}
