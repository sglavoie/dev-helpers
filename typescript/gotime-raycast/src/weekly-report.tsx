import { Action, ActionPanel, Color, Icon, List } from "@raycast/api";
import { useExec } from "@raycast/utils";
import { useMemo, useState } from "react";

interface ReportData {
  title: string;
  time_range: string;
  completed_entries?: KeywordSummary[];
  active_entries?: Entry[];
  weekly_data?: WeeklyData;
  time_series?: TimeSeriesData;
  total_duration: number;
  completed_duration: number;
  active_duration: number;
  filters_applied?: FiltersApplied;
}

interface FiltersApplied {
  keywords?: string[];
  tags?: string[];
  exclude_keywords?: boolean;
  exclude_tags?: boolean;
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

interface TimeSeriesData {
  keywords: TimeSeriesKeyword[];
  period_totals: number[];
  period_labels: string[];
  grand_total: number;
}

interface TimeSeriesKeyword {
  keyword: string;
  period_data: number[];
  keyword_total: number;
}

interface KeywordListItem {
  keyword: string;
  entries: number;
  total_duration: number;
}

interface TagListItem {
  tag: string;
  count: number;
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

function formatDurationCompact(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  return `${hours.toString().padStart(2, "0")}:${minutes.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
}

export default function Command() {
  // Filter state
  const [selectedFilter, setSelectedFilter] = useState<string>("week||");

  // Parse filter components from compound value (timeRange|keyword|tag)
  const { timeRange, keywordFilter, tagFilter } = useMemo(() => {
    const [time, keyword, tag] = selectedFilter.split("|");
    return {
      timeRange: time || "week",
      keywordFilter: keyword || "",
      tagFilter: tag || "",
    };
  }, [selectedFilter]);

  // Fetch available keywords
  const { data: keywordsList } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["keywords", "list", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed || trimmed === "[]") return [];
        return JSON.parse(trimmed) as KeywordListItem[];
      },
    },
  );

  // Fetch available tags
  const { data: tagsList } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--week", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed || trimmed === "[]") return [];
        const entries = JSON.parse(trimmed) as Entry[];
        // Extract unique tags from entries
        const tagSet = new Set<string>();
        entries.forEach((entry) => {
          const tags = entry.tags ?? [];
          tags.forEach((tag) => tagSet.add(tag));
        });
        return Array.from(tagSet)
          .sort()
          .map((tag) => ({ tag, count: 0 }));
      },
    },
  );

  // Build dynamic command arguments based on filters
  const commandArgs = useMemo(() => {
    const args = ["report", "--json"];

    // Add time range flag
    if (timeRange !== "week") {
      if (timeRange === "today") {
        args.push("--today");
      } else if (timeRange === "yesterday") {
        args.push("--yesterday");
      } else if (timeRange === "month") {
        args.push("--month");
      } else if (timeRange === "last-week") {
        // For last week, we'll use --days 7 and adjust
        // Note: This is approximate. A proper implementation might need a --last-week flag
        args.push("--days", "7");
      } else if (timeRange === "last-30-days") {
        args.push("--days", "30");
      }
    }

    // Add keyword filter
    if (keywordFilter) {
      args.push("--keywords", keywordFilter);
    }

    // Add tag filter
    if (tagFilter) {
      args.push("--tags", tagFilter);
    }

    return args;
  }, [timeRange, keywordFilter, tagFilter]);

  // Fetch report data with dynamic filters
  const { isLoading, data, error, revalidate } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    commandArgs,
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

  // Use time_series data (new unified format) or convert weekly_data for backwards compatibility
  const timeSeriesData: TimeSeriesData | undefined =
    data?.time_series ||
    (data?.weekly_data
      ? {
          keywords: data.weekly_data.keywords.map((kw) => ({
            keyword: kw.keyword,
            period_data: kw.daily_data,
            keyword_total: kw.keyword_total,
          })),
          period_totals: data.weekly_data.daily_totals,
          period_labels: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"],
          grand_total: data.weekly_data.grand_total,
        }
      : undefined);

  const totalDuration = data?.total_duration || 0;
  const periodLabels = timeSeriesData?.period_labels || [];

  // Build filter status for display
  const filterStatus = useMemo(() => {
    const parts: string[] = [];
    if (keywordFilter) parts.push(`Keyword: ${keywordFilter}`);
    if (tagFilter) parts.push(`Tag: ${tagFilter}`);
    if (timeRange !== "week") {
      const timeRangeLabels: Record<string, string> = {
        today: "Today",
        yesterday: "Yesterday",
        month: "This Month",
        "last-week": "Last Week",
        "last-30-days": "Last 30 Days",
      };
      parts.push(timeRangeLabels[timeRange] || timeRange);
    }
    return parts.length > 0 ? parts.join(" • ") : undefined;
  }, [timeRange, keywordFilter, tagFilter]);

  return (
    <List
      isLoading={isLoading}
      navigationTitle={data?.title || "Report"}
      searchBarAccessory={
        <List.Dropdown
          tooltip="Filter Report"
          value={selectedFilter}
          onChange={setSelectedFilter}
        >
          {/* Time Range Section */}
          <List.Dropdown.Section title="Time Range">
            <List.Dropdown.Item title="This Week" value="week||" />
            <List.Dropdown.Item title="Today" value="today||" />
            <List.Dropdown.Item title="Yesterday" value="yesterday||" />
            <List.Dropdown.Item title="This Month" value="month||" />
            <List.Dropdown.Item title="Last 7 Days" value="last-week||" />
            <List.Dropdown.Item title="Last 30 Days" value="last-30-days||" />
          </List.Dropdown.Section>

          {/* Keywords Section */}
          {keywordsList && keywordsList.length > 0 && (
            <List.Dropdown.Section title="By Keyword">
              <List.Dropdown.Item
                title="All Keywords"
                value={`${timeRange}||`}
              />
              {keywordsList.map((kw) => (
                <List.Dropdown.Item
                  key={kw.keyword}
                  title={kw.keyword}
                  value={`${timeRange}|${kw.keyword}|`}
                />
              ))}
            </List.Dropdown.Section>
          )}

          {/* Tags Section */}
          {tagsList && tagsList.length > 0 && (
            <List.Dropdown.Section title="By Tag">
              <List.Dropdown.Item title="All Tags" value={`${timeRange}||`} />
              {tagsList.map((tag) => (
                <List.Dropdown.Item
                  key={tag.tag}
                  title={tag.tag}
                  value={`${timeRange}||${tag.tag}`}
                />
              ))}
            </List.Dropdown.Section>
          )}
        </List.Dropdown>
      }
    >
      {!isLoading &&
      (!timeSeriesData || timeSeriesData.keywords.length === 0) ? (
        <List.EmptyView
          icon={Icon.Document}
          title={filterStatus ? "No Matching Data" : "No Data Available"}
          description={
            filterStatus
              ? `No entries match the selected filters. Try adjusting your filters.`
              : "Start tracking time to see your report"
          }
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
          {timeSeriesData && timeSeriesData.keywords.length > 0 && (
            <List.Section title="By Keyword">
              {timeSeriesData.keywords.map((keyword) => {
                const activePeriods = keyword.period_data.filter(
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
                        tooltip: `${activePeriods} period${activePeriods !== 1 ? "s" : ""}`,
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
                            {keyword.period_data.map(
                              (duration, periodIndex) => (
                                <List.Item.Detail.Metadata.Label
                                  key={periodIndex}
                                  title={
                                    periodLabels[periodIndex] ||
                                    `Period ${periodIndex + 1}`
                                  }
                                  text={
                                    duration > 0
                                      ? formatDurationCompact(duration)
                                      : "-"
                                  }
                                />
                              ),
                            )}
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

          {/* Period totals section */}
          {timeSeriesData && timeSeriesData.period_totals.length > 0 && (
            <List.Section title="By Period">
              {timeSeriesData.period_totals.map((duration, periodIndex) => {
                const hasData = duration > 0;
                const label =
                  periodLabels[periodIndex] || `Period ${periodIndex + 1}`;
                return (
                  <List.Item
                    key={periodIndex}
                    title={label}
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
