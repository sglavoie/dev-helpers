import { Icon, List } from "@raycast/api";
import { useExec } from "@raycast/utils";
import { EntryListItem } from "./components/EntryListItem";
import {
  GT_BIN,
  getCurrentDuration,
  parseEntries,
  useKeywordsAndTags,
  useLiveDurations,
} from "./utils/entries";

export default function Command() {
  const { isLoading, data, error, revalidate } = useExec(
    GT_BIN,
    ["list", "--days", "30", "--json"],
    { parseOutput: ({ stdout }) => parseEntries(stdout) },
  );

  const { keywords, tags } = useKeywordsAndTags(data);
  const currentDurations = useLiveDurations(data);

  if (error) {
    return (
      <List>
        <List.Item title="Error" subtitle={error.message} />
      </List>
    );
  }

  return (
    <List isLoading={isLoading}>
      {!isLoading && (!data || !Array.isArray(data) || data.length === 0) ? (
        <List.EmptyView
          icon={Icon.Pencil}
          title="No Recent Entries"
          description="No entries found in the last 30 days"
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
              />
            ))}
        </>
      )}
    </List>
  );
}
