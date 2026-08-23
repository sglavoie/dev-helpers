import { Action, Icon, List } from "@raycast/api";
import { useExec } from "@raycast/utils";
import { useState, useMemo } from "react";
import { EntryListItem } from "./components/EntryListItem";
import {
  Entry,
  GT_BIN,
  getCurrentDuration,
  parseEntries,
  useKeywordsAndTags,
  useLiveDurations,
} from "./utils/entries";
import { deleteEntryWithConfirmation } from "./utils/entry-edit";

export default function Command() {
  const [timeRange, setTimeRange] = useState<string>("7");
  const [activityFilter, setActivityFilter] = useState<string>("all");
  const [keywordFilter, setKeywordFilter] = useState<string>("");
  const [tagFilter] = useState<string>("");

  // Build command arguments based on filters
  const commandArgs = useMemo(() => {
    const args = ["list", "--json"];

    // Time range filter
    if (timeRange === "today") {
      // Default is today, no flag needed
    } else if (timeRange === "week") {
      args.push("--week");
    } else if (timeRange === "month") {
      args.push("--month");
    } else if (timeRange === "yesterday") {
      args.push("--yesterday");
    } else {
      // Custom days (7, 30, etc.)
      args.push("--days", timeRange);
    }

    // Activity filter
    if (activityFilter === "active") {
      args.push("--active");
    } else if (activityFilter === "stopped") {
      args.push("--no-active");
    }

    // Keyword filter
    if (keywordFilter.trim()) {
      args.push("--keywords", keywordFilter.trim());
    }

    // Tag filter
    if (tagFilter.trim()) {
      args.push("--tags", tagFilter.trim());
    }

    return args;
  }, [timeRange, activityFilter, keywordFilter, tagFilter]);

  const { isLoading, data, error, revalidate } = useExec(GT_BIN, commandArgs, {
    parseOutput: ({ stdout }) => parseEntries(stdout),
  });

  const { keywords, tags } = useKeywordsAndTags(data);
  const currentDurations = useLiveDurations(data);

  async function handleDeleteEntry(entry: Entry) {
    if (await deleteEntryWithConfirmation(entry)) {
      revalidate();
    }
  }

  if (error) {
    return (
      <List
        searchBarAccessory={
          <List.Dropdown
            tooltip="Time Range"
            value={timeRange}
            onChange={setTimeRange}
          >
            <List.Dropdown.Item title="Today" value="today" />
            <List.Dropdown.Item title="Yesterday" value="yesterday" />
            <List.Dropdown.Item title="Last 7 days" value="7" />
            <List.Dropdown.Item title="Last 30 days" value="30" />
            <List.Dropdown.Item title="This week" value="week" />
            <List.Dropdown.Item title="This month" value="month" />
          </List.Dropdown>
        }
      >
        <List.Item title="Error" subtitle={error.message} />
      </List>
    );
  }

  return (
    <List
      isLoading={isLoading}
      searchBarPlaceholder="Search entries by keyword..."
      onSearchTextChange={setKeywordFilter}
      searchBarAccessory={
        <List.Dropdown
          tooltip="Filter Options"
          value={`${timeRange}|${activityFilter}`}
          onChange={(value) => {
            const [newTimeRange, newActivity] = value.split("|");
            setTimeRange(newTimeRange);
            setActivityFilter(newActivity);
          }}
        >
          <List.Dropdown.Section title="Time Range">
            <List.Dropdown.Item title="Today" value="today|all" />
            <List.Dropdown.Item title="Yesterday" value="yesterday|all" />
            <List.Dropdown.Item title="Last 7 days" value="7|all" />
            <List.Dropdown.Item title="Last 30 days" value="30|all" />
            <List.Dropdown.Item title="This week" value="week|all" />
            <List.Dropdown.Item title="This month" value="month|all" />
          </List.Dropdown.Section>
          <List.Dropdown.Section title="Activity Status">
            <List.Dropdown.Item title="All entries" value="7|all" />
            <List.Dropdown.Item title="Active only" value="7|active" />
            <List.Dropdown.Item title="Stopped only" value="7|stopped" />
          </List.Dropdown.Section>
        </List.Dropdown>
      }
    >
      {!isLoading && (!data || !Array.isArray(data) || data.length === 0) ? (
        <List.EmptyView
          icon={Icon.List}
          title="No Entries Found"
          description="No entries match the selected filters"
        />
      ) : (
        <>
          {Array.isArray(data) &&
            data.map((entry) => (
              <EntryListItem
                key={entry.id}
                entry={entry}
                duration={
                  currentDurations.get(entry.id) || getCurrentDuration(entry)
                }
                allKeywords={keywords}
                allTags={tags}
                onRefresh={() => revalidate()}
                extraActions={
                  <Action
                    title="Delete Entry"
                    icon={Icon.Trash}
                    style={Action.Style.Destructive}
                    shortcut={{ modifiers: ["cmd"], key: "d" }}
                    onAction={() => handleDeleteEntry(entry)}
                  />
                }
              />
            ))}
        </>
      )}
    </List>
  );
}
