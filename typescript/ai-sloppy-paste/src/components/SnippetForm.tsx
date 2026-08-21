import { Action, ActionPanel, Clipboard, Form, Icon, Keyboard, showToast, Toast, useNavigation } from "@raycast/api";
import { useEffect, useMemo, useState } from "react";
import { Snippet, SnippetFormValues } from "../types";
import { addSnippet, updateSnippet } from "../utils/storage";
import { validateTitle, validateContent, getCharacterInfo, VALIDATION_LIMITS } from "../utils/validation";
import { getErrorMessage } from "../utils/errorMessage";
import { PlaceholderSyntaxHelp } from "./PlaceholderSyntaxHelp";
import { TagPickerView } from "./TagPickerView";
import { buildSnippetPreview } from "../utils/snippetPreview";

const SYNTAX_HELPERS: { title: string; subtitle: string; content: string; icon: Icon; key: Keyboard.KeyEquivalent }[] =
  [
    { title: "Basic Placeholder", subtitle: "{{key}}", content: "{{key}}", icon: Icon.CodeBlock, key: "1" },
    { title: "With Default", subtitle: "{{key|default}}", content: "{{key|default}}", icon: Icon.CodeBlock, key: "2" },
    { title: "No-Save Placeholder", subtitle: "{{!key}}", content: "{{!key}}", icon: Icon.EyeDisabled, key: "3" },
    {
      title: "Wrapper Placeholder",
      subtitle: "{{prefix:key:suffix}}",
      content: "{{prefix:key:suffix}}",
      icon: Icon.ArrowNe,
      key: "4",
    },
    {
      title: "Conditional Block",
      subtitle: "{{#if key}}...{{/if}}",
      content: "{{#if key}}\n\n{{/if}}",
      icon: Icon.Filter,
      key: "5",
    },
    {
      title: "If/Else Block",
      subtitle: "{{#if key}}...{{#else}}...{{/if}}",
      content: "{{#if key}}\n\n{{#else}}\n\n{{/if}}",
      icon: Icon.Switch,
      key: "6",
    },
    {
      title: "Choice Placeholder",
      subtitle: "{{tone[Formal|Casual|Technical]|Casual}}",
      content: "{{tone[Formal|Casual|Technical]|Casual}}",
      icon: Icon.CodeBlock,
      key: "7",
    },
  ];

export function SnippetForm(props: { snippet?: Snippet; onSubmit: () => void; tags: string[] }) {
  const { pop } = useNavigation();
  const [titleError, setTitleError] = useState<string | undefined>();
  const [contentError, setContentError] = useState<string | undefined>();
  const [titleCharInfo, setTitleCharInfo] = useState("");
  const [contentCharInfo, setContentCharInfo] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>(props.snippet?.tags || []);
  const [contentValue, setContentValue] = useState(props.snippet?.content || "");

  const previewString = useMemo(() => buildSnippetPreview(contentValue), [contentValue]);

  // Initialize character counts for edit mode
  useEffect(() => {
    if (props.snippet) {
      const titleInfo = getCharacterInfo(props.snippet.title, VALIDATION_LIMITS.TITLE_MAX_LENGTH);
      setTitleCharInfo(titleInfo.info);
      const contentInfo = getCharacterInfo(props.snippet.content, VALIDATION_LIMITS.CONTENT_MAX_LENGTH);
      setContentCharInfo(contentInfo.info);
    }
  }, [props.snippet]);

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
          <Action.Push
            title="Edit Tags"
            icon={Icon.Tag}
            shortcut={{ modifiers: ["cmd"], key: "t" }}
            target={
              <TagPickerView
                navigationTitle="Edit Tags"
                initialTags={selectedTags}
                allTags={props.tags}
                onTagsChange={setSelectedTags}
              />
            }
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
                  await showToast({
                    style: Toast.Style.Success,
                    title: "Copied",
                    message: `${h.subtitle} — Paste with ⌘V`,
                  });
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
        value={contentValue}
        error={contentError}
        info={contentCharInfo}
        enableMarkdown={true}
        onChange={(value) => {
          setContentError(undefined);
          setContentValue(value);
          const charInfo = getCharacterInfo(value, VALIDATION_LIMITS.CONTENT_MAX_LENGTH);
          setContentCharInfo(charInfo.info);
        }}
      />
      <Form.Description
        title="Placeholders"
        text="Required: {{name}}   Optional: {{name|default}}   No-save: {{!name}}"
      />
      <Form.Description
        title="Choices"
        text="{{tone[Formal|Casual|Technical]}} — dropdown plus Custom; the first choice starts selected. Default: {{tone[Formal|Casual]|Casual}}"
      />
      <Form.Description
        title="Choice + Wrappers"
        text="{{!$:amount[10|20]: USD|10}} — combine no-save (!), wrappers, authored choices, and a default."
      />
      <Form.Description
        title="Wrappers"
        text="{{prefix:key:suffix}} — wrapping text only appears when value is non-empty. Example: {{$:price: USD}}"
      />
      <Form.Description
        title="Conditionals"
        text="{{#if key}}...{{/if}} — toggle section.  Else: ...{{#else}}...  Press Cmd+Shift+P to insert syntax."
      />
      <Form.Description
        title="System (auto)"
        text="{{DATE}}  {{TIME}}  {{DATETIME}}  {{TODAY}}  {{NOW}}  {{YEAR}}  {{MONTH}}  {{DAY}}"
      />
      {previewString && <Form.Description title="Preview" text={previewString} />}
      <Form.Separator />
      <Form.TextArea
        id="description"
        title="Description"
        placeholder="Optional description for this snippet..."
        defaultValue={props.snippet?.description || ""}
        enableMarkdown={true}
      />
      <Form.Separator />
      <Form.Description title="Tags" text={selectedTags.join(", ") || "None"} />
      <Form.Description text="Press Cmd+T from any field to filter, toggle, and create tags. Use slashes for hierarchy (e.g., work/projects). No spaces - use dashes (e.g., my-project)." />
    </Form>
  );
}
