import { Action, ActionPanel, Clipboard, Form, Icon, Keyboard, showToast, Toast, useNavigation } from "@raycast/api";
import { useEffect, useState } from "react";
import { Snippet, SnippetFormValues } from "../types";
import { addSnippet, updateSnippet } from "../utils/storage";
import { validateTitle, validateContent, validateTag, getCharacterInfo, VALIDATION_LIMITS } from "../utils/validation";
import { getErrorMessage } from "../utils/errorMessage";
import { PlaceholderSyntaxHelp } from "./PlaceholderSyntaxHelp";

const SYNTAX_HELPERS: { title: string; subtitle: string; content: string; icon: Icon; key: Keyboard.KeyEquivalent }[] = [
  { title: "Basic Placeholder", subtitle: "{{key}}", content: "{{key}}", icon: Icon.CodeBlock, key: "1" },
  { title: "With Default", subtitle: "{{key|default}}", content: "{{key|default}}", icon: Icon.CodeBlock, key: "2" },
  { title: "No-Save Placeholder", subtitle: "{{!key}}", content: "{{!key}}", icon: Icon.EyeDisabled, key: "3" },
  { title: "Wrapper Placeholder", subtitle: "{{prefix:key:suffix}}", content: "{{prefix:key:suffix}}", icon: Icon.ArrowNe, key: "4" },
  { title: "Conditional Block", subtitle: "{{#if key}}...{{/if}}", content: "{{#if key}}\n\n{{/if}}", icon: Icon.Filter, key: "5" },
  { title: "If/Else Block", subtitle: "{{#if key}}...{{#else}}...{{/if}}", content: "{{#if key}}\n\n{{#else}}\n\n{{/if}}", icon: Icon.Switch, key: "6" },
];

export function SnippetForm(props: { snippet?: Snippet; onSubmit: () => void; tags: string[] }) {
  const { pop } = useNavigation();
  const [titleError, setTitleError] = useState<string | undefined>();
  const [contentError, setContentError] = useState<string | undefined>();
  const [tagsError, setTagsError] = useState<string | undefined>();
  const [titleCharInfo, setTitleCharInfo] = useState("");
  const [contentCharInfo, setContentCharInfo] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>(props.snippet?.tags || []);
  const [newTagInput, setNewTagInput] = useState<string>("");
  const [newTagError, setNewTagError] = useState<string | undefined>();

  // Initialize character counts for edit mode
  useEffect(() => {
    if (props.snippet) {
      const titleInfo = getCharacterInfo(props.snippet.title, VALIDATION_LIMITS.TITLE_MAX_LENGTH);
      setTitleCharInfo(titleInfo.info);
      const contentInfo = getCharacterInfo(props.snippet.content, VALIDATION_LIMITS.CONTENT_MAX_LENGTH);
      setContentCharInfo(contentInfo.info);
    }
  }, [props.snippet]);

  // Handle adding a new tag
  function handleAddTag() {
    const trimmedTag = newTagInput.trim();

    if (!trimmedTag) {
      return;
    }

    // Validate the tag
    const tagValidation = validateTag(trimmedTag);
    if (!tagValidation.isValid) {
      setNewTagError(tagValidation.error);
      return;
    }

    // Check if tag already exists
    if (selectedTags.includes(trimmedTag)) {
      setNewTagError("Tag already added");
      return;
    }

    // Add the tag
    setSelectedTags([...selectedTags, trimmedTag]);
    setNewTagInput("");
    setNewTagError(undefined);

    showToast({
      style: Toast.Style.Success,
      title: "Tag added",
      message: trimmedTag,
    });
  }

  async function handleSubmit(values: SnippetFormValues) {
    // Validation
    const titleValidation = validateTitle(values.title);
    if (!titleValidation.isValid) {
      setTitleError(titleValidation.error);
      return;
    }

    const contentValidation = validateContent(values.content);
    if (!contentValidation.isValid) {
      setContentError(contentValidation.error);
      return;
    }

    // Use selectedTags for final submission
    const finalTags = selectedTags;

    try {
      if (props.snippet) {
        // Update existing snippet
        await updateSnippet(props.snippet.id, {
          title: values.title.trim(),
          content: values.content.trim(),
          description: values.description?.trim() || "",
          tags: finalTags,
        });
        showToast({
          style: Toast.Style.Success,
          title: "Snippet updated",
        });
      } else {
        // Create new snippet
        await addSnippet({
          title: values.title.trim(),
          content: values.content.trim(),
          description: values.description?.trim() || "",
          tags: finalTags,
        });
        showToast({
          style: Toast.Style.Success,
          title: "Snippet created",
        });
      }
      props.onSubmit();
      pop();
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: props.snippet ? "Failed to update snippet" : "Failed to create snippet",
        message: getErrorMessage(error),
      });
    }
  }

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm title={props.snippet ? "Update Snippet" : "Create Snippet"} onSubmit={handleSubmit} />
          <Action
            title="Add Tag"
            icon={Icon.Plus}
            shortcut={{ modifiers: ["cmd"], key: "t" }}
            onAction={handleAddTag}
          />
          <ActionPanel.Submenu
            title="Insert Placeholder Syntax"
            icon={Icon.CodeBlock}
            shortcut={{ modifiers: ["cmd", "shift"], key: "p" }}
          >
            {SYNTAX_HELPERS.map((h) => (
              <Action
                key={h.title}
                title={`${h.title}  —  ${h.subtitle}`}
                icon={h.icon}
                shortcut={{ modifiers: ["cmd"], key: h.key }}
                onAction={async () => {
                  await Clipboard.copy(h.content);
                  await showToast({ style: Toast.Style.Success, title: "Copied", message: `${h.subtitle} — Paste with ⌘V` });
                }}
              />
            ))}
          </ActionPanel.Submenu>
          <Action.Push
            title="View Placeholder Syntax"
            icon={Icon.QuestionMarkCircle}
            shortcut={{ modifiers: ["cmd", "shift"], key: "h" }}
            target={<PlaceholderSyntaxHelp />}
          />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="title"
        title="Title"
        placeholder="Enter snippet title"
        defaultValue={props.snippet?.title || ""}
        error={titleError}
        info={titleCharInfo}
        onChange={(value) => {
          setTitleError(undefined);
          const charInfo = getCharacterInfo(value, VALIDATION_LIMITS.TITLE_MAX_LENGTH);
          setTitleCharInfo(charInfo.info);
        }}
      />
      <Form.TextArea
        id="content"
        title="Content"
        placeholder="Enter snippet content"
        defaultValue={props.snippet?.content || ""}
        error={contentError}
        info={contentCharInfo}
        enableMarkdown={true}
        onChange={(value) => {
          setContentError(undefined);
          const charInfo = getCharacterInfo(value, VALIDATION_LIMITS.CONTENT_MAX_LENGTH);
          setContentCharInfo(charInfo.info);
        }}
      />
      <Form.Description
        title="Placeholders"
        text="Required: {{name}}   Optional: {{name|default}}   No-save: {{!name}}"
      />
      <Form.Description
        title="Wrappers"
        text="{{prefix:key:suffix}} — wrapping text only appears when value is non-empty. Example: {{$:price: USD}}"
      />
      <Form.Description
        title="Conditionals"
        text='{{#if key}}...{{/if}} — toggle section.  Else: ...{{#else}}...  Press Cmd+Shift+P to insert syntax.'
      />
      <Form.Description
        title="System (auto)"
        text="{{DATE}}  {{TIME}}  {{DATETIME}}  {{TODAY}}  {{NOW}}  {{YEAR}}  {{MONTH}}  {{DAY}}"
      />
      <Form.TextArea
        id="description"
        title="Description"
        placeholder="Optional description for this snippet..."
        defaultValue={props.snippet?.description || ""}
        enableMarkdown={true}
      />
      <Form.TagPicker
        id="tags"
        title="Tags"
        value={selectedTags}
        onChange={setSelectedTags}
        placeholder="Select tags to add to this snippet"
        error={tagsError}
      >
        {(() => {
          // Combine existing tags with any newly created tags that aren't in the list yet
          const allTags = Array.from(new Set([...props.tags, ...selectedTags])).sort();

          return allTags.length > 0 ? (
            allTags.map((tag) => {
              // Calculate display: show hierarchy with visual indentation
              const parts = tag.split("/");
              const depth = parts.length - 1;
              const name = parts[parts.length - 1];
              const indent = "  ".repeat(depth); // 2 spaces per level

              return <Form.TagPicker.Item key={tag} value={tag} title={`${indent}${tag}`} icon={Icon.Tag} />;
            })
          ) : (
            <Form.TagPicker.Item value="" title="No tags available" />
          );
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
      <Form.Description text="💡 Press Cmd+T to add tags. Tags appear as badges above - click to remove. Use slashes for hierarchy (e.g., work/projects). No spaces - use dashes (e.g., my-project)." />
    </Form>
  );
}
