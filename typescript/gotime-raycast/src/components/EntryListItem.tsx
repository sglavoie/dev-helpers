import { Action, ActionPanel, Color, Icon, List } from "@raycast/api";
import { ComponentProps } from "react";
import { formatDuration } from "../utils/duration";
import { Entry, formatRelativeTime } from "../utils/entries";
import { EditEntryForm } from "./EditEntryForm";

interface EntryListItemProps {
  entry: Entry;
  /** Elapsed seconds to display, which ticks for active entries. */
  duration: number;
  allKeywords: string[];
  allTags: string[];
  onRefresh: () => void;
  /** Extra actions shown alongside "Edit Entry". */
  extraActions?: ComponentProps<typeof ActionPanel.Section>["children"];
}

/** How many tags are shown before the rest are collapsed into a "+n" chip. */
const VISIBLE_TAGS = 3;

export function EntryListItem({
  entry,
  duration,
  allKeywords,
  allTags,
  onRefresh,
  extraActions,
}: EntryListItemProps) {
  const statusStr = entry.active
    ? "Active"
    : `Ended ${formatRelativeTime(entry.end_time)}`;
  const tags = entry.tags ?? [];
  const hiddenTags = tags.length - VISIBLE_TAGS;

  return (
    <List.Item
      icon={{
        source: Icon.Circle,
        tintColor: entry.active ? Color.Green : Color.SecondaryText,
      }}
      title={entry.keyword}
      subtitle={`${formatDuration(duration)} • ${statusStr}`}
      accessories={[
        ...tags.slice(0, VISIBLE_TAGS).map((tag) => ({
          tag: { value: tag, color: Color.Blue },
        })),
        ...(hiddenTags > 0
          ? [{ tag: { value: `+${hiddenTags}`, color: Color.SecondaryText } }]
          : []),
      ]}
      actions={
        <ActionPanel>
          <ActionPanel.Section title="Entry Actions">
            <Action.Push
              title="Edit Entry"
              icon={Icon.Pencil}
              shortcut={{ modifiers: ["cmd"], key: "e" }}
              target={
                <EditEntryForm
                  entry={entry}
                  allKeywords={allKeywords}
                  allTags={allTags}
                  onComplete={onRefresh}
                />
              }
            />
            {extraActions}
          </ActionPanel.Section>
          <ActionPanel.Section title="Actions">
            <Action
              title="Refresh"
              icon={Icon.ArrowClockwise}
              shortcut={{ modifiers: ["cmd"], key: "r" }}
              onAction={onRefresh}
            />
          </ActionPanel.Section>
        </ActionPanel>
      }
    />
  );
}
