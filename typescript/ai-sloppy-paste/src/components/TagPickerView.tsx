import { Action, ActionPanel, Color, Icon, List, showToast, Toast } from "@raycast/api";
import { useState } from "react";
import { getErrorMessage } from "../utils/errorMessage";
import { removeRedundantParents } from "../utils/tags";
import { buildTagRows, filterTagRows, getCreateCandidate, toggleTag, TagRow } from "../utils/tagPicker";

interface TagPickerViewProps {
  navigationTitle: string;
  initialTags: string[];
  allTags: string[];
  onTagsChange: (tags: string[]) => void | Promise<void>;
}

export function TagPickerView(props: TagPickerViewProps) {
  // Seeded once: a pushed view keeps its own state while the pusher re-renders.
  const [selectedTags, setSelectedTags] = useState(() => removeRedundantParents(props.initialTags));
  const [searchText, setSearchText] = useState("");

  const rows = buildTagRows(props.allTags, selectedTags);
  const visibleRows = filterTagRows(rows, searchText);
  const createCandidate = getCreateCandidate(
    searchText,
    rows.map((row) => row.tag),
  );

  const onSnippetRows = visibleRows.filter((row) => row.state !== "unselected");
  const availableRows = visibleRows.filter((row) => row.state === "unselected");

  async function handleToggle(tag: string) {
    const result = toggleTag(selectedTags, tag);

    if (!result.changed) {
      showToast({
        style: Toast.Style.Failure,
        title: "Already implied",
        message: `"${tag}" is already implied by "${result.impliedBy}"`,
      });
      return;
    }

    const previousTags = selectedTags;
    const wasAdded = result.tags.includes(tag);
    setSelectedTags(result.tags);
    setSearchText("");

    try {
      await props.onTagsChange(result.tags);
      showToast({
        style: Toast.Style.Success,
        title: wasAdded ? "Tag added" : "Tag removed",
        message: tag,
      });
    } catch (error) {
      setSelectedTags(previousTags);
      showToast({
        style: Toast.Style.Failure,
        title: wasAdded ? "Failed to add tag" : "Failed to remove tag",
        message: getErrorMessage(error),
      });
    }
  }

  function renderRow(row: TagRow) {
    const isImplied = row.state === "implied";

    return (
      <List.Item
        key={row.tag}
        icon={
          row.state === "unselected"
            ? Icon.Circle
            : { source: Icon.CheckCircle, tintColor: isImplied ? Color.SecondaryText : Color.Green }
        }
        title={row.tag}
        accessories={
          isImplied
            ? [{ tag: { value: "implied", color: Color.SecondaryText }, tooltip: `Implied by "${row.impliedBy}"` }]
            : undefined
        }
        actions={
          <ActionPanel>
            <Action
              title={row.state === "selected" ? "Remove Tag" : "Add Tag"}
              icon={row.state === "selected" ? Icon.MinusCircle : Icon.PlusCircle}
              onAction={() => handleToggle(row.tag)}
            />
          </ActionPanel>
        }
      />
    );
  }

  return (
    <List
      navigationTitle={props.navigationTitle}
      filtering={false}
      searchText={searchText}
      onSearchTextChange={setSearchText}
      searchBarPlaceholder="Filter tags, or type a new tag name…"
    >
      {createCandidate && "tag" in createCandidate && (
        <List.Section title="Create">
          <List.Item
            icon={{ source: Icon.PlusCircle, tintColor: Color.Green }}
            title={`Create and add "${createCandidate.tag}"`}
            actions={
              <ActionPanel>
                <Action
                  title="Create and Add Tag"
                  icon={Icon.PlusCircle}
                  onAction={() => handleToggle(createCandidate.tag)}
                />
              </ActionPanel>
            }
          />
        </List.Section>
      )}
      {createCandidate && "error" in createCandidate && (
        <List.Section title="Create">
          <List.Item
            icon={{ source: Icon.ExclamationMark, tintColor: Color.Red }}
            title={searchText.trim()}
            subtitle={createCandidate.error}
            actions={
              <ActionPanel>
                <Action
                  title="Show Tag Name Error"
                  icon={Icon.ExclamationMark}
                  onAction={() =>
                    showToast({
                      style: Toast.Style.Failure,
                      title: "Invalid tag name",
                      message: createCandidate.error,
                    })
                  }
                />
              </ActionPanel>
            }
          />
        </List.Section>
      )}
      {onSnippetRows.length > 0 && <List.Section title="On This Snippet">{onSnippetRows.map(renderRow)}</List.Section>}
      {availableRows.length > 0 && <List.Section title="All Tags">{availableRows.map(renderRow)}</List.Section>}
      {visibleRows.length === 0 && !createCandidate && (
        <List.EmptyView
          icon={Icon.Tag}
          title={rows.length === 0 ? "No tags yet" : "No matching tags"}
          description="Type a tag name to create it"
        />
      )}
    </List>
  );
}
