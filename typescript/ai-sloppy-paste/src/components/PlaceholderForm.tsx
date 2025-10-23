import { Action, ActionPanel, Clipboard, closeMainWindow, Form, showToast, Toast, useNavigation } from "@raycast/api";
import { useState } from "react";
import { Snippet, Placeholder } from "../types";
import { incrementUsage } from "../utils/storage";
import { replacePlaceholders } from "../utils/placeholders";

export function PlaceholderForm(props: {
  snippet: Snippet;
  placeholders: Placeholder[];
  mode: "copy" | "paste" | "paste-direct";
  onComplete: () => void;
}) {
  const { pop } = useNavigation();
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formValues, setFormValues] = useState<Record<string, string>>(() => {
    // Initialize with default values
    const initial: Record<string, string> = {};
    props.placeholders.forEach((p) => {
      initial[p.key] = p.defaultValue || "";
    });
    return initial;
  });

  // Compute live preview
  const previewContent = replacePlaceholders(props.snippet.content, formValues, props.placeholders);

  async function handleSubmit(values: Record<string, string>) {
    // Validate required fields
    const newErrors: Record<string, string> = {};
    for (const placeholder of props.placeholders) {
      if (placeholder.isRequired && !values[placeholder.key]?.trim()) {
        newErrors[placeholder.key] = "This field is required";
      }
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    // Replace placeholders
    const filledContent = replacePlaceholders(props.snippet.content, values, props.placeholders);

    try {
      if (props.mode === "paste-direct") {
        // Paste directly to frontmost app
        await Clipboard.paste(filledContent);
        await incrementUsage(props.snippet.id);
        props.onComplete();
        pop();
        await closeMainWindow();
        showToast({
          style: Toast.Style.Success,
          title: "Pasted to frontmost app",
          message: "Snippet with filled placeholders",
        });
      } else {
        // Copy to clipboard
        await Clipboard.copy(filledContent);
        await incrementUsage(props.snippet.id);

        if (props.mode === "copy") {
          // Navigate back to main view, then close window
          props.onComplete();
          pop();
          await closeMainWindow();
          showToast({
            style: Toast.Style.Success,
            title: "Copied to clipboard",
            message: "Snippet with filled placeholders",
          });
        } else {
          // Copy without closing: keep window open for multiple operations
          showToast({
            style: Toast.Style.Success,
            title: "Copied to clipboard",
            message: "Window stays open for multiple copies",
          });
          props.onComplete();
          pop();
        }
      }
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: props.mode === "paste-direct" ? "Failed to paste" : "Failed to copy",
        message: String(error),
      });
    }
  }

  return (
    <Form
      navigationTitle={`Fill Placeholders: ${props.snippet.title}`}
      actions={
        <ActionPanel>
          <Action.SubmitForm
            title={
              props.mode === "paste-direct"
                ? "Paste & Close"
                : props.mode === "copy"
                  ? "Copy & Close"
                  : "Copy & Stay Open"
            }
            onSubmit={handleSubmit}
          />
        </ActionPanel>
      }
    >
      <Form.Description text="Fill in the placeholder values below. Required fields are marked with *." />
      {props.placeholders.map((placeholder) => (
        <Form.TextField
          key={placeholder.key}
          id={placeholder.key}
          title={placeholder.isRequired ? `${placeholder.key} *` : placeholder.key}
          placeholder={placeholder.defaultValue || "Enter value..."}
          defaultValue={placeholder.defaultValue || ""}
          error={errors[placeholder.key]}
          onChange={(value) => {
            // Update form values for live preview
            setFormValues((prev) => ({ ...prev, [placeholder.key]: value }));

            // Clear error if exists
            if (errors[placeholder.key]) {
              setErrors((prev) => {
                const newErrors = { ...prev };
                delete newErrors[placeholder.key];
                return newErrors;
              });
            }
          }}
          info={
            placeholder.isRequired ? "Required field" : `Optional (default: "${placeholder.defaultValue || "empty"}")`
          }
        />
      ))}
      <Form.Separator />
      <Form.Description title="Preview" text={previewContent} />
    </Form>
  );
}
