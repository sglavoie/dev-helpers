import { Action, ActionPanel, closeMainWindow, Form, showToast, Toast, useNavigation } from "@raycast/api";
import { Snippet, Placeholder } from "../types";
import { replacePlaceholders, processConditionalBlocks } from "../utils/placeholders";
import { getErrorMessage } from "../utils/errorMessage";
import { buildRequiredPlaceholderErrors } from "../utils/placeholderFormChoices";
import { submitPlaceholderForm } from "../utils/placeholderFormSubmit";
import { runBestEffort } from "../utils/snippet-use";
import { usePlaceholderFormState } from "../hooks/usePlaceholderFormState";
import { renderPlaceholderField } from "./PlaceholderFormField";

export function PlaceholderForm(props: {
  snippet: Snippet;
  placeholders: Placeholder[];
  mode: "copy" | "paste" | "paste-direct";
  onComplete: () => void;
}) {
  const { pop } = useNavigation();
  const state = usePlaceholderFormState(props.placeholders);
  const { enabledOptionals, formValues } = state;

  // Sort placeholders: required first, optional after
  const sorted = [...props.placeholders].sort((a, b) => (a.isRequired === b.isRequired ? 0 : a.isRequired ? -1 : 1));
  const requiredPlaceholders = sorted.filter((p) => p.isRequired);
  const optionalPlaceholders = sorted.filter((p) => !p.isRequired);
  const hasBothSections = requiredPlaceholders.length > 0 && optionalPlaceholders.length > 0;
  const totalRequired = requiredPlaceholders.length;
  const filledCount = requiredPlaceholders.filter((p) => (formValues[p.key] ?? "").trim() !== "").length;

  // Compute live preview — treat disabled optional wrapper fields as empty
  const previewValues = { ...formValues };
  for (const [key, enabled] of Object.entries(enabledOptionals)) {
    if (!enabled) previewValues[key] = "";
  }
  // Guard-only keys: map boolean to "true" / ""
  for (const p of props.placeholders) {
    if (p.isGuardOnly) {
      previewValues[p.key] = enabledOptionals[p.key] ? "true" : "";
    }
  }
  const afterBlocks = processConditionalBlocks(props.snippet.content, previewValues);
  const previewContent = replacePlaceholders(afterBlocks, previewValues, props.placeholders);

  async function handleSubmit() {
    // Build final values: use formValues which is kept in sync
    const finalValues: Record<string, string> = { ...formValues };

    // Override disabled optional wrapper fields to empty
    for (const [key, enabled] of Object.entries(enabledOptionals)) {
      if (!enabled) finalValues[key] = "";
    }

    // Validate required fields
    const newErrors = buildRequiredPlaceholderErrors(props.placeholders, finalValues);

    if (Object.keys(newErrors).length > 0) {
      state.setErrors(newErrors);
      return;
    }

    // Guard-only keys: map boolean to "true" / ""
    for (const p of props.placeholders) {
      if (p.isGuardOnly) {
        finalValues[p.key] = enabledOptionals[p.key] ? "true" : "";
      }
    }

    const isPaste = props.mode === "paste-direct";
    const verb = isPaste ? "paste" : "copy";

    const didComplete = await submitPlaceholderForm({
      snippet: props.snippet,
      placeholders: props.placeholders,
      finalValues,
      mode: props.mode,
      onPreparationFailure: (error) =>
        showToast({
          style: Toast.Style.Failure,
          title: isPaste ? "Failed to prepare paste" : "Failed to prepare copy",
          message: getErrorMessage(error),
        }),
      onPrimaryFailure: (error) =>
        showToast({
          style: Toast.Style.Failure,
          title: isPaste ? "Failed to paste" : "Failed to copy",
          message: getErrorMessage(error),
        }),
    });
    if (!didComplete) return;

    // "paste" (copy & stay open) is the only mode that keeps the window around,
    // so it reports success first and never closes Raycast.
    if (props.mode === "paste") {
      await runBestEffort(
        () =>
          showToast({
            style: Toast.Style.Success,
            title: "Copied to clipboard",
            message: "Window stays open for multiple copies",
          }),
        "Unable to show placeholder copy success",
      );
      await runBestEffort(() => props.onComplete(), "Unable to refresh after placeholder copy");
      await runBestEffort(() => pop(), "Unable to navigate after placeholder copy");
      return;
    }

    await runBestEffort(() => props.onComplete(), `Unable to refresh after placeholder ${verb}`);
    await runBestEffort(() => pop(), `Unable to navigate after placeholder ${verb}`);
    await runBestEffort(() => closeMainWindow(), `Unable to close Raycast after placeholder ${verb}`);
    await runBestEffort(
      () =>
        showToast({
          style: Toast.Style.Success,
          title: isPaste ? "Pasted to frontmost app" : "Copied to clipboard",
          message: "Snippet with filled placeholders",
        }),
      `Unable to show placeholder ${verb} success`,
    );
  }

  const navigationTitle = `Step 2 of 2 — Fill Placeholders (${filledCount}/${totalRequired}): ${props.snippet.title}`;

  if (state.isLoadingHistory) {
    return (
      <Form navigationTitle={navigationTitle} isLoading={true}>
        <Form.Description text="Loading history..." />
      </Form>
    );
  }

  const prefilledCount = state.prefilledKeys.size;
  const allFieldsPrefilled = requiredPlaceholders.length > 0 && prefilledCount === requiredPlaceholders.length;
  const fieldWord = `field${requiredPlaceholders.length !== 1 ? "s" : ""}`;
  const formSummary = allFieldsPrefilled
    ? `All ${requiredPlaceholders.length} ${fieldWord} pre-filled from history — submit to ${props.mode === "paste-direct" ? "paste" : "copy"}.`
    : prefilledCount > 0
      ? `${prefilledCount} of ${requiredPlaceholders.length} ${fieldWord} pre-filled from history — review and submit.`
      : undefined;

  const renderField = (placeholder: Placeholder) => renderPlaceholderField(placeholder, props.snippet.content, state);

  return (
    <Form
      navigationTitle={navigationTitle}
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
          <Action
            title="Use Defaults for All Optional"
            shortcut={{ modifiers: ["cmd"], key: "d" }}
            onAction={state.handleUseDefaults}
          />
        </ActionPanel>
      }
    >
      <Form.Description text="Fill in the placeholder values below. Required fields (*) must be filled. Wrapper fields (checkbox) are omitted from output when unchecked. Conditional fields (checkbox) control whether entire blocks appear." />
      {formSummary && <Form.Description text={formSummary} />}
      {requiredPlaceholders.map(renderField)}
      {hasBothSections && (
        <>
          <Form.Separator />
          <Form.Description text="Optional fields" />
        </>
      )}
      {optionalPlaceholders.map(renderField)}
      <Form.Separator />
      <Form.Description title="Preview" text={previewContent} />
    </Form>
  );
}
