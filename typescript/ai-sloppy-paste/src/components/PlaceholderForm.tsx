import {
  Action,
  ActionPanel,
  Clipboard,
  closeMainWindow,
  Form,
  Icon,
  showToast,
  Toast,
  useNavigation,
} from "@raycast/api";
import { useState, useEffect, Fragment, useRef } from "react";
import { Snippet, Placeholder } from "../types";
import { getPlaceholderHistoryForKey, getMaxPlaceholderHistoryValues } from "../utils/storage";
import { pasteSnippet } from "../utils/clipboard";
import { replacePlaceholders, processConditionalBlocks } from "../utils/placeholders";
import { getLastUsedValue, getRankedValuesForAutocomplete } from "../utils/placeholderHistory";
import { getErrorMessage } from "../utils/errorMessage";
import { buildFieldPreview } from "../utils/fieldPreview";
import { PlaceholderValueToRecord } from "../utils/storage";
import { runBestEffort, runSnippetAction } from "../utils/snippet-use";

export const CUSTOM_VALUE_MARKER = "__CUSTOM_VALUE__";
const AUTHORED_CHOICE_OPTION_PREFIX = "__AUTHORED_CHOICE__";

export interface AuthoredChoiceFieldState {
  formValue: string;
  dropdownSelection: string;
  customValue: string;
  useCustomInput: boolean;
  enabledOptional?: boolean;
}

function hasAuthoredChoices(placeholder: Placeholder): placeholder is Placeholder & { choices: string[] } {
  return (placeholder.choices?.length ?? 0) > 0;
}

export function getAuthoredChoiceOptionId(index: number): string {
  return `${AUTHORED_CHOICE_OPTION_PREFIX}${index}`;
}

export function getAuthoredChoiceValue(choices: readonly string[], optionId: string): string | undefined {
  if (!optionId.startsWith(AUTHORED_CHOICE_OPTION_PREFIX)) return undefined;
  const rawIndex = optionId.slice(AUTHORED_CHOICE_OPTION_PREFIX.length);
  if (!/^\d+$/.test(rawIndex)) return undefined;
  return choices[Number(rawIndex)];
}

export function initializeAuthoredChoiceState(
  placeholder: Placeholder & { choices: string[] },
): AuthoredChoiceFieldState {
  const { choices, defaultValue } = placeholder;
  const hasExplicitDefault = defaultValue !== undefined;
  const matchingDefaultIndex = hasExplicitDefault ? choices.indexOf(defaultValue) : -1;
  const isWrapper = placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined;
  const enabledOptional = !placeholder.isRequired && isWrapper ? !!defaultValue : undefined;

  if (matchingDefaultIndex >= 0) {
    return {
      formValue: choices[matchingDefaultIndex],
      dropdownSelection: getAuthoredChoiceOptionId(matchingDefaultIndex),
      customValue: "",
      useCustomInput: false,
      enabledOptional,
    };
  }

  if (hasExplicitDefault) {
    return {
      formValue: defaultValue,
      dropdownSelection: CUSTOM_VALUE_MARKER,
      customValue: defaultValue,
      useCustomInput: true,
      enabledOptional,
    };
  }

  return {
    formValue: choices[0] ?? "",
    dropdownSelection: choices.length > 0 ? getAuthoredChoiceOptionId(0) : CUSTOM_VALUE_MARKER,
    customValue: "",
    useCustomInput: choices.length === 0,
    enabledOptional,
  };
}

export function resolveAuthoredChoiceSelection(
  choices: readonly string[],
  optionId: string,
  customValue: string,
): AuthoredChoiceFieldState | undefined {
  if (optionId === CUSTOM_VALUE_MARKER) {
    return {
      formValue: customValue,
      dropdownSelection: CUSTOM_VALUE_MARKER,
      customValue,
      useCustomInput: true,
    };
  }

  const authoredValue = getAuthoredChoiceValue(choices, optionId);
  if (authoredValue === undefined) return undefined;
  return {
    formValue: authoredValue,
    dropdownSelection: optionId,
    customValue,
    useCustomInput: false,
  };
}

export function buildRequiredPlaceholderErrors(
  placeholders: Placeholder[],
  finalValues: Record<string, string>,
): Record<string, string> {
  const errors: Record<string, string> = {};
  for (const placeholder of placeholders) {
    if (placeholder.isRequired && !finalValues[placeholder.key]?.trim()) {
      errors[placeholder.key] = "This field is required";
    }
  }
  return errors;
}

interface PlaceholderFormSubmission {
  snippet: Snippet;
  placeholders: Placeholder[];
  finalValues: Record<string, string>;
  mode: "copy" | "paste" | "paste-direct";
  onPreparationFailure: (error: unknown) => unknown | Promise<unknown>;
  onPrimaryFailure: (error: unknown) => unknown | Promise<unknown>;
}

/**
 * Retains the submitted value and parsed placeholder metadata for the atomic
 * storage operation. recordSnippetUse owns the no-save and blank exclusions.
 */
export function buildTrackedPlaceholderValues(
  placeholders: Placeholder[],
  finalValues: Record<string, string>,
): PlaceholderValueToRecord[] {
  return placeholders.map((placeholder) => ({
    key: placeholder.key,
    value: finalValues[placeholder.key] ?? "",
    isSaved: placeholder.isSaved,
  }));
}

export async function submitPlaceholderForm({
  snippet,
  placeholders,
  finalValues,
  mode,
  onPreparationFailure,
  onPrimaryFailure,
}: PlaceholderFormSubmission): Promise<boolean> {
  return runSnippetAction({
    prepare: () => {
      const afterBlocks = processConditionalBlocks(snippet.content, finalValues);
      return replacePlaceholders(afterBlocks, finalValues, placeholders);
    },
    primaryOperation: (content) => (mode === "paste-direct" ? pasteSnippet(content) : Clipboard.copy(content)),
    snippetId: snippet.id,
    placeholderValues: buildTrackedPlaceholderValues(placeholders, finalValues),
    onPreparationFailure,
    onPrimaryFailure,
  });
}

export function PlaceholderForm(props: {
  snippet: Snippet;
  placeholders: Placeholder[];
  mode: "copy" | "paste" | "paste-direct";
  onComplete: () => void;
}) {
  const { pop } = useNavigation();
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [historySuggestions, setHistorySuggestions] = useState<Record<string, string[]>>({});
  const [isLoadingHistory, setIsLoadingHistory] = useState(true);
  const [formValues, setFormValues] = useState<Record<string, string>>({});
  const [dropdownSelections, setDropdownSelections] = useState<Record<string, string>>({});
  const [customValues, setCustomValues] = useState<Record<string, string>>({});
  const [useCustomInput, setUseCustomInput] = useState<Record<string, boolean>>({});
  const [enabledOptionals, setEnabledOptionals] = useState<Record<string, boolean>>({});
  const prefilledRef = useRef(new Set<string>());

  // Sort placeholders: required first, optional after
  const sorted = [...props.placeholders].sort((a, b) => (a.isRequired === b.isRequired ? 0 : a.isRequired ? -1 : 1));
  const requiredPlaceholders = sorted.filter((p) => p.isRequired);
  const optionalPlaceholders = sorted.filter((p) => !p.isRequired);
  const hasBothSections = requiredPlaceholders.length > 0 && optionalPlaceholders.length > 0;
  const totalRequired = requiredPlaceholders.length;
  const filledCount = requiredPlaceholders.filter((p) => (formValues[p.key] ?? "").trim() !== "").length;

  // Load history and initialize form values on mount
  useEffect(() => {
    async function loadHistory() {
      const initial: Record<string, string> = {};
      const suggestions: Record<string, string[]> = {};
      const dropdownInit: Record<string, string> = {};
      const customInit: Record<string, string> = {};
      const customInputMode: Record<string, boolean> = {};
      const enabledInit: Record<string, boolean> = {};

      // Get max display values from preferences (storage keeps up to 100)
      const maxDisplayValues = getMaxPlaceholderHistoryValues();
      prefilledRef.current.clear();

      for (const placeholder of props.placeholders) {
        if (hasAuthoredChoices(placeholder)) {
          const choiceState = initializeAuthoredChoiceState(placeholder);
          suggestions[placeholder.key] = [];
          initial[placeholder.key] = choiceState.formValue;
          dropdownInit[placeholder.key] = choiceState.dropdownSelection;
          customInit[placeholder.key] = choiceState.customValue;
          customInputMode[placeholder.key] = choiceState.useCustomInput;
          if (choiceState.enabledOptional !== undefined) {
            enabledInit[placeholder.key] = choiceState.enabledOptional;
          }
          continue;
        }

        const history = await getPlaceholderHistoryForKey(placeholder.key);
        // Limit displayed values to preference setting (storage may have up to 100)
        const rankedValues = getRankedValuesForAutocomplete(history, maxDisplayValues);

        suggestions[placeholder.key] = rankedValues;

        // Pre-fill with last-used value from history, or default value
        // Use ?? to preserve empty strings (for {{key|}} syntax)
        const lastUsedValue = getLastUsedValue(history);
        if (lastUsedValue) {
          prefilledRef.current.add(placeholder.key);
        }
        const defaultValue = lastUsedValue ?? placeholder.defaultValue ?? "";

        initial[placeholder.key] = defaultValue;

        if (rankedValues.length > 0) {
          // Has history - use dropdown with pre-selected value
          dropdownInit[placeholder.key] = defaultValue || CUSTOM_VALUE_MARKER;
          customInputMode[placeholder.key] = !defaultValue; // Show custom input if no default
        } else {
          // No history - always show custom input
          customInputMode[placeholder.key] = true;
        }

        // Initialize enabledOptionals for optional wrapper fields:
        // enabled if there's existing history or a non-empty default, disabled otherwise
        const isWrapper = placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined;
        if (!placeholder.isRequired && isWrapper) {
          enabledInit[placeholder.key] = rankedValues.length > 0 || !!placeholder.defaultValue;
        }

        // Guard-only keys default to unchecked (or checked if defaultOn)
        if (placeholder.isGuardOnly) {
          enabledInit[placeholder.key] = placeholder.defaultOn ?? false;
        }
      }

      setHistorySuggestions(suggestions);
      setFormValues(initial);
      setDropdownSelections(dropdownInit);
      setCustomValues(customInit);
      setUseCustomInput(customInputMode);
      setEnabledOptionals(enabledInit);
      setIsLoadingHistory(false);
    }

    loadHistory();
  }, [props.placeholders]);

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
      setErrors(newErrors);
      return;
    }

    // Guard-only keys: map boolean to "true" / ""
    for (const p of props.placeholders) {
      if (p.isGuardOnly) {
        finalValues[p.key] = enabledOptionals[p.key] ? "true" : "";
      }
    }

    const didComplete = await submitPlaceholderForm({
      snippet: props.snippet,
      placeholders: props.placeholders,
      finalValues,
      mode: props.mode,
      onPreparationFailure: (error) =>
        showToast({
          style: Toast.Style.Failure,
          title: props.mode === "paste-direct" ? "Failed to prepare paste" : "Failed to prepare copy",
          message: getErrorMessage(error),
        }),
      onPrimaryFailure: (error) =>
        showToast({
          style: Toast.Style.Failure,
          title: props.mode === "paste-direct" ? "Failed to paste" : "Failed to copy",
          message: getErrorMessage(error),
        }),
    });
    if (!didComplete) return;

    if (props.mode === "paste-direct") {
      await runBestEffort(() => props.onComplete(), "Unable to refresh after placeholder paste");
      await runBestEffort(() => pop(), "Unable to navigate after placeholder paste");
      await runBestEffort(() => closeMainWindow(), "Unable to close Raycast after placeholder paste");
      await runBestEffort(
        () =>
          showToast({
            style: Toast.Style.Success,
            title: "Pasted to frontmost app",
            message: "Snippet with filled placeholders",
          }),
        "Unable to show placeholder paste success",
      );
      return;
    }

    if (props.mode === "copy") {
      await runBestEffort(() => props.onComplete(), "Unable to refresh after placeholder copy");
      await runBestEffort(() => pop(), "Unable to navigate after placeholder copy");
      await runBestEffort(() => closeMainWindow(), "Unable to close Raycast after placeholder copy");
      await runBestEffort(
        () =>
          showToast({
            style: Toast.Style.Success,
            title: "Copied to clipboard",
            message: "Snippet with filled placeholders",
          }),
        "Unable to show placeholder copy success",
      );
      return;
    }

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
  }

  function handleUseDefaults() {
    const newFormValues = { ...formValues };
    const newDropdownSelections = { ...dropdownSelections };
    const newCustomValues = { ...customValues };
    const newUseCustomInput = { ...useCustomInput };
    const newEnabledOptionals = { ...enabledOptionals };

    for (const placeholder of props.placeholders) {
      if (placeholder.isRequired || placeholder.isGuardOnly) continue;

      if (hasAuthoredChoices(placeholder)) {
        const choiceState = initializeAuthoredChoiceState(placeholder);
        newFormValues[placeholder.key] = choiceState.formValue;
        newDropdownSelections[placeholder.key] = choiceState.dropdownSelection;
        newUseCustomInput[placeholder.key] = choiceState.useCustomInput;
        if (choiceState.useCustomInput) {
          newCustomValues[placeholder.key] = choiceState.customValue;
        }
        if (choiceState.enabledOptional !== undefined) {
          newEnabledOptionals[placeholder.key] = choiceState.enabledOptional;
        }
        continue;
      }

      const defaultVal = placeholder.defaultValue ?? "";
      const suggestions = historySuggestions[placeholder.key] ?? [];
      newFormValues[placeholder.key] = defaultVal;
      newCustomValues[placeholder.key] = defaultVal;
      if (suggestions.length > 0) {
        const useDefaultSuggestion = defaultVal !== "" && suggestions.includes(defaultVal);
        newDropdownSelections[placeholder.key] = useDefaultSuggestion ? defaultVal : CUSTOM_VALUE_MARKER;
        newUseCustomInput[placeholder.key] = !useDefaultSuggestion;
      }
      const isWrapper = placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined;
      if (isWrapper) {
        newEnabledOptionals[placeholder.key] = !!placeholder.defaultValue;
      }
    }

    setFormValues(newFormValues);
    setDropdownSelections(newDropdownSelections);
    setCustomValues(newCustomValues);
    setUseCustomInput(newUseCustomInput);
    setEnabledOptionals(newEnabledOptionals);
  }

  function handleDropdownChange(placeholder: Placeholder, optionId: string) {
    const { key } = placeholder;

    if (hasAuthoredChoices(placeholder)) {
      const choiceState = resolveAuthoredChoiceSelection(placeholder.choices, optionId, customValues[key] ?? "");
      if (!choiceState) return;
      setDropdownSelections((prev) => ({ ...prev, [key]: choiceState.dropdownSelection }));
      setUseCustomInput((prev) => ({ ...prev, [key]: choiceState.useCustomInput }));
      setFormValues((prev) => ({ ...prev, [key]: choiceState.formValue }));
    } else {
      setDropdownSelections((prev) => ({ ...prev, [key]: optionId }));

      if (optionId === CUSTOM_VALUE_MARKER) {
        // User selected "Enter new value..." - show custom input
        setUseCustomInput((prev) => ({ ...prev, [key]: true }));
        setFormValues((prev) => ({ ...prev, [key]: customValues[key] ?? "" }));
      } else {
        // User selected a historical value
        setUseCustomInput((prev) => ({ ...prev, [key]: false }));
        setFormValues((prev) => ({ ...prev, [key]: optionId }));
      }
    }

    // Clear error if exists
    if (errors[key]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[key];
        return newErrors;
      });
    }
  }

  function handleCustomInputChange(key: string, value: string) {
    setCustomValues((prev) => ({ ...prev, [key]: value }));
    setFormValues((prev) => ({ ...prev, [key]: value }));

    // Clear error if exists
    if (errors[key]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[key];
        return newErrors;
      });
    }
  }

  function truncateTitle(text: string, max = 25): string {
    return text.length <= max ? text : text.slice(0, max - 1) + "…";
  }

  function buildInfoText(placeholder: Placeholder): string {
    if (placeholder.isGuardOnly) {
      return "Conditional — checked = block shown, unchecked = block omitted";
    }
    const parts: string[] = [];
    if (placeholder.isRequired) {
      parts.push("Required field");
    } else {
      const hasWrappers = placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined;
      if (hasWrappers) {
        const example = `${placeholder.prefixWrapper ?? ""}value${placeholder.suffixWrapper ?? ""}`;
        parts.push(`Optional wrapper • Output when included: "${example}" • Uncheck to omit it`);
      } else {
        parts.push(`Optional (default: "${placeholder.defaultValue ?? "none"}")`);
      }
    }
    if (hasAuthoredChoices(placeholder)) {
      parts.push(`Configured choices: ${placeholder.choices.map((choice) => JSON.stringify(choice)).join(", ")}`);
      parts.push("Choose Enter custom value… to type another value");
    }
    if (!placeholder.isSaved) {
      parts.push("Won't be saved to history");
    }
    return parts.join(" • ");
  }

  function renderPlaceholderField(placeholder: Placeholder) {
    const suggestions = historySuggestions[placeholder.key] || [];
    const authoredChoices = hasAuthoredChoices(placeholder) ? placeholder.choices : undefined;
    const hasAuthoredChoiceDropdown = authoredChoices !== undefined;
    const hasHistory = suggestions.length > 0;
    const showCustomInput = useCustomInput[placeholder.key];
    const fieldHint = buildFieldPreview(placeholder, props.snippet.content, formValues[placeholder.key] ?? "");
    const fieldHintFull = buildFieldPreview(placeholder, props.snippet.content, formValues[placeholder.key] ?? "", {
      truncate: false,
    });
    const fieldHintTooltip = fieldHintFull && fieldHintFull !== fieldHint ? fieldHintFull : undefined;
    const withFieldHint = (base: string | undefined): string | undefined => {
      const parts = [base, fieldHintTooltip].filter(Boolean) as string[];
      return parts.length > 0 ? parts.join("\n\n") : undefined;
    };
    if (placeholder.isGuardOnly) {
      const isChecked = enabledOptionals[placeholder.key] ?? false;
      const label = placeholder.label || `Include ${placeholder.key}?`;
      const truncated = truncateTitle(label);
      const infoBase = buildInfoText(placeholder);
      const baseInfo = label !== truncated ? [label, infoBase].filter(Boolean).join(" • ") : infoBase;
      const info = withFieldHint(baseInfo || undefined);
      return (
        <Fragment key={placeholder.key}>
          <Form.Checkbox
            id={placeholder.key}
            title={truncated}
            label="Include in output"
            value={isChecked}
            info={info}
            onChange={(checked) => setEnabledOptionals((prev) => ({ ...prev, [placeholder.key]: checked }))}
          />
          {fieldHint && <Form.Description text={fieldHint} />}
        </Fragment>
      );
    }

    const isWrapperField =
      !placeholder.isRequired && (placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined);
    const isEnabled = isWrapperField ? (enabledOptionals[placeholder.key] ?? false) : true;

    // Build title with indicators
    let title = placeholder.key;
    if (placeholder.isRequired) title += " *";
    if (prefilledRef.current.has(placeholder.key)) title += " (↻ last used)";
    // isSaved=false is indicated via buildInfoText ("Won't be saved to history")

    return (
      <Fragment key={placeholder.key}>
        {isWrapperField && (
          <Form.Checkbox
            id={`${placeholder.key}-enabled`}
            title={`Include ${placeholder.key}`}
            label="Include in output"
            value={isEnabled}
            info={withFieldHint(buildInfoText(placeholder))}
            onChange={(checked) => setEnabledOptionals((prev) => ({ ...prev, [placeholder.key]: checked }))}
          />
        )}
        {(!isWrapperField || isEnabled) &&
          (hasAuthoredChoiceDropdown || hasHistory ? (
            <>
              <Form.Dropdown
                id={`${placeholder.key}-dropdown`}
                title={title}
                value={dropdownSelections[placeholder.key] || CUSTOM_VALUE_MARKER}
                onChange={(value) => handleDropdownChange(placeholder, value)}
                error={!showCustomInput ? errors[placeholder.key] : undefined}
                info={withFieldHint(buildInfoText(placeholder))}
              >
                {authoredChoices
                  ? authoredChoices.map((value, index) => {
                      const optionId = getAuthoredChoiceOptionId(index);
                      return <Form.Dropdown.Item key={optionId} value={optionId} title={value} />;
                    })
                  : suggestions.map((value) => <Form.Dropdown.Item key={value} value={value} title={value} />)}
                <Form.Dropdown.Item
                  value={CUSTOM_VALUE_MARKER}
                  title={authoredChoices ? "Enter custom value…" : "Enter new value..."}
                  icon={Icon.Pencil}
                />
              </Form.Dropdown>
              {showCustomInput && (
                <Form.TextField
                  id={`${placeholder.key}-custom`}
                  title={`${title} (Custom)`}
                  placeholder={placeholder.defaultValue || "Enter custom value..."}
                  value={customValues[placeholder.key] ?? ""}
                  error={errors[placeholder.key]}
                  onChange={(value) => handleCustomInputChange(placeholder.key, value)}
                  info={withFieldHint(buildInfoText(placeholder))}
                />
              )}
            </>
          ) : (
            <Form.TextField
              id={placeholder.key}
              title={title}
              placeholder={placeholder.defaultValue || "Enter value..."}
              value={formValues[placeholder.key] ?? ""}
              error={errors[placeholder.key]}
              onChange={(value) => handleCustomInputChange(placeholder.key, value)}
              info={withFieldHint(buildInfoText(placeholder))}
            />
          ))}
        {fieldHint && <Form.Description text={fieldHint} />}
      </Fragment>
    );
  }

  if (isLoadingHistory) {
    return (
      <Form
        navigationTitle={`Step 2 of 2 — Fill Placeholders (${filledCount}/${totalRequired}): ${props.snippet.title}`}
        isLoading={true}
      >
        <Form.Description text="Loading history..." />
      </Form>
    );
  }

  const prefilledCount = prefilledRef.current.size;
  const allFieldsPrefilled = requiredPlaceholders.length > 0 && prefilledCount === requiredPlaceholders.length;
  const formSummary = allFieldsPrefilled
    ? `All ${requiredPlaceholders.length} field${requiredPlaceholders.length !== 1 ? "s" : ""} pre-filled from history — submit to ${props.mode === "paste-direct" ? "paste" : "copy"}.`
    : prefilledCount > 0
      ? `${prefilledCount} of ${requiredPlaceholders.length} field${requiredPlaceholders.length !== 1 ? "s" : ""} pre-filled from history — review and submit.`
      : undefined;

  return (
    <Form
      navigationTitle={`Step 2 of 2 — Fill Placeholders (${filledCount}/${totalRequired}): ${props.snippet.title}`}
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
            onAction={handleUseDefaults}
          />
        </ActionPanel>
      }
    >
      <Form.Description text="Fill in the placeholder values below. Required fields (*) must be filled. Wrapper fields (checkbox) are omitted from output when unchecked. Conditional fields (checkbox) control whether entire blocks appear." />
      {formSummary && <Form.Description text={formSummary} />}
      {requiredPlaceholders.map(renderPlaceholderField)}
      {hasBothSections && (
        <>
          <Form.Separator />
          <Form.Description text="Optional fields" />
        </>
      )}
      {optionalPlaceholders.map(renderPlaceholderField)}
      <Form.Separator />
      <Form.Description title="Preview" text={previewContent} />
    </Form>
  );
}
