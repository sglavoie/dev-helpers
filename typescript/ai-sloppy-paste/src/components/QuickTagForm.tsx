import { Action, ActionPanel, Form, Icon, showToast, Toast, useNavigation } from "@raycast/api";
import { useState } from "react";
import { Snippet } from "../types";
import { updateSnippet } from "../utils/storage";

interface QuickTagFormProps {
  snippet: Snippet;
  availableTags: string[];
  mode: "add" | "remove";
  onUpdated: () => void;
}

export function QuickTagForm(props: QuickTagFormProps) {
  const { pop } = useNavigation();
  const [selectedTag, setSelectedTag] = useState<string>("");

  const tagsToShow =
    props.mode === "add" ? props.availableTags.filter((tag) => !props.snippet.tags.includes(tag)) : props.snippet.tags;

  async function handleSubmit() {
    if (!selectedTag) {
      showToast({
        style: Toast.Style.Failure,
        title: props.mode === "add" ? "Select a tag to add" : "Select a tag to remove",
      });
      return;
    }

    try {
      const newTags =
        props.mode === "add"
          ? [...props.snippet.tags, selectedTag]
          : props.snippet.tags.filter((t) => t !== selectedTag);

      await updateSnippet(props.snippet.id, { tags: newTags });

      showToast({
        style: Toast.Style.Success,
        title: props.mode === "add" ? "Tag added" : "Tag removed",
        message: selectedTag,
      });

      props.onUpdated();
      pop();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: props.mode === "add" ? "Failed to add tag" : "Failed to remove tag",
        message: String(error),
      });
    }
  }

  if (tagsToShow.length === 0) {
    return (
      <Form
        navigationTitle={props.mode === "add" ? "Quick Add Tag" : "Quick Remove Tag"}
        actions={
          <ActionPanel>
            <Action title="Go Back" onAction={pop} />
          </ActionPanel>
        }
      >
        <Form.Description
          title="No Tags Available"
          text={
            props.mode === "add"
              ? "All available tags are already on this snippet, or no tags exist yet."
              : "This snippet has no tags to remove."
          }
        />
      </Form>
    );
  }

  return (
    <Form
      navigationTitle={
        props.mode === "add" ? `Add Tag to "${props.snippet.title}"` : `Remove Tag from "${props.snippet.title}"`
      }
      actions={
        <ActionPanel>
          <Action.SubmitForm
            title={props.mode === "add" ? "Add Tag" : "Remove Tag"}
            icon={props.mode === "add" ? Icon.Plus : Icon.Minus}
            onSubmit={handleSubmit}
          />
        </ActionPanel>
      }
    >
      <Form.Dropdown
        id="tag"
        title={props.mode === "add" ? "Tag to Add" : "Tag to Remove"}
        value={selectedTag}
        onChange={setSelectedTag}
      >
        <Form.Dropdown.Item value="" title="Select a tag..." />
        {tagsToShow.map((tag) => {
          const parts = tag.split("/");
          const depth = parts.length - 1;
          const indent = "  ".repeat(depth);
          return <Form.Dropdown.Item key={tag} value={tag} title={`${indent}${tag}`} icon={Icon.Tag} />;
        })}
      </Form.Dropdown>
    </Form>
  );
}
