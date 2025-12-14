import { Action, ActionPanel, Clipboard, Form, Icon, popToRoot, showToast, Toast } from "@raycast/api";
import { useState, useEffect } from "react";
import { addSnippet, getTags } from "./utils/storage";
import { validateTitle, validateTag, getCharacterInfo, VALIDATION_LIMITS } from "./utils/validation";

export default function SaveClipboardCommand() {
  const [clipboardContent, setClipboardContent] = useState<string>("");
  const [isLoading, setIsLoading] = useState(true);
  const [availableTags, setAvailableTags] = useState<string[]>([]);
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [titleError, setTitleError] = useState<string | undefined>();
  const [titleCharInfo, setTitleCharInfo] = useState("");
  const [newTagInput, setNewTagInput] = useState<string>("");
  const [newTagError, setNewTagError] = useState<string | undefined>();

  useEffect(() => {
    async function loadData() {
      try {
        const [text, tags] = await Promise.all([Clipboard.readText(), getTags()]);

        if (!text || text.trim() === "") {
          showToast({
            style: Toast.Style.Failure,
            title: "Clipboard is empty",
            message: "Copy some text first, then try again",
          });
          popToRoot();
          return;
        }

        setClipboardContent(text);
        setAvailableTags(tags);

        const suggestedTitle = generateSuggestedTitle(text);
        const charInfo = getCharacterInfo(suggestedTitle, VALIDATION_LIMITS.TITLE_MAX_LENGTH);
        setTitleCharInfo(charInfo.info);
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to read clipboard",
          message: String(error),
        });
        popToRoot();
      } finally {
        setIsLoading(false);
      }
    }

    loadData();
  }, []);

  function generateSuggestedTitle(content: string): string {
    const firstLine = content.split("\n")[0].trim();
    const maxLength = 50;

    if (firstLine.length <= maxLength) {
      return firstLine;
    }

    return firstLine.substring(0, maxLength - 3) + "...";
  }

  function handleAddTag() {
    const trimmedTag = newTagInput.trim();

    if (!trimmedTag) {
      return;
    }

    const tagValidation = validateTag(trimmedTag);
    if (!tagValidation.isValid) {
      setNewTagError(tagValidation.error);
      return;
    }

    if (selectedTags.includes(trimmedTag)) {
      setNewTagError("Tag already added");
      return;
    }

    setSelectedTags([...selectedTags, trimmedTag]);
    setNewTagInput("");
    setNewTagError(undefined);

    showToast({
      style: Toast.Style.Success,
      title: "Tag added",
      message: trimmedTag,
    });
  }

  async function handleSubmit(values: { title: string }) {
    const titleValidation = validateTitle(values.title);
    if (!titleValidation.isValid) {
      setTitleError(titleValidation.error);
      return;
    }

    try {
      await addSnippet({
        title: values.title.trim(),
        content: clipboardContent,
        description: "",
        tags: selectedTags,
      });

      showToast({
        style: Toast.Style.Success,
        title: "Snippet saved",
        message: values.title.trim(),
      });

      popToRoot();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to save snippet",
        message: String(error),
      });
    }
  }

  if (isLoading) {
    return <Form isLoading={true} />;
  }

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Save Snippet" icon={Icon.Plus} onSubmit={handleSubmit} />
          <Action title="Add Tag" icon={Icon.Tag} shortcut={{ modifiers: ["cmd"], key: "t" }} onAction={handleAddTag} />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="title"
        title="Title"
        placeholder="Enter snippet title"
        defaultValue={generateSuggestedTitle(clipboardContent)}
        error={titleError}
        info={titleCharInfo}
        onChange={(value) => {
          setTitleError(undefined);
          const charInfo = getCharacterInfo(value, VALIDATION_LIMITS.TITLE_MAX_LENGTH);
          setTitleCharInfo(charInfo.info);
        }}
        autoFocus
      />
      <Form.Description
        title="Content Preview"
        text={clipboardContent.substring(0, 200) + (clipboardContent.length > 200 ? "..." : "")}
      />
      <Form.TagPicker
        id="tags"
        title="Tags"
        value={selectedTags}
        onChange={setSelectedTags}
        placeholder="Select tags (optional)"
      >
        {(() => {
          const allTags = Array.from(new Set([...availableTags, ...selectedTags])).sort();

          return allTags.length > 0
            ? allTags.map((tag) => {
                const parts = tag.split("/");
                const depth = parts.length - 1;
                const indent = "  ".repeat(depth);
                return <Form.TagPicker.Item key={tag} value={tag} title={`${indent}${tag}`} icon={Icon.Tag} />;
              })
            : null;
        })()}
      </Form.TagPicker>
      <Form.TextField
        id="newTag"
        title="Add New Tag"
        placeholder="e.g., work/projects or personal"
        value={newTagInput}
        error={newTagError}
        info="Type a tag name and press Cmd+T to add it"
        onChange={(value) => {
          setNewTagInput(value);
          setNewTagError(undefined);
        }}
      />
    </Form>
  );
}
