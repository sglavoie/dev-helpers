import { Action, ActionPanel, Clipboard, Form, Icon, popToRoot, showToast, Toast } from "@raycast/api";
import { useState, useEffect } from "react";
import { addSnippet, getTags } from "./utils/storage";
import { validateTitle, getCharacterInfo, VALIDATION_LIMITS } from "./utils/validation";
import { getErrorMessage } from "./utils/errorMessage";
import { TagPickerView } from "./components/TagPickerView";

export default function SaveClipboardCommand() {
  const [clipboardContent, setClipboardContent] = useState<string>("");
  const [isLoading, setIsLoading] = useState(true);
  const [availableTags, setAvailableTags] = useState<string[]>([]);
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [titleError, setTitleError] = useState<string | undefined>();
  const [titleCharInfo, setTitleCharInfo] = useState("");

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
          message: getErrorMessage(error),
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
        message: getErrorMessage(error),
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
          <Action.Push
            title="Edit Tags"
            icon={Icon.Tag}
            shortcut={{ modifiers: ["cmd"], key: "t" }}
            target={
              <TagPickerView
                navigationTitle="Edit Tags"
                initialTags={selectedTags}
                allTags={availableTags}
                onTagsChange={setSelectedTags}
              />
            }
          />
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
      <Form.Description title="Tags" text={selectedTags.join(", ") || "None"} />
      <Form.Description text="Press Cmd+T from any field to filter, toggle, and create tags. Use slashes for hierarchy (e.g., work/projects)." />
    </Form>
  );
}
