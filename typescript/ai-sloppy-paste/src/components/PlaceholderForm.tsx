import {
  Action,
  ActionPanel,
  Clipboard,
  closeMainWindow,
  Form,
  getPreferenceValues,
  showToast,
  Toast,
  useNavigation,
} from "@raycast/api";
import { useState, useEffect, Fragment } from "react";
import { Snippet, Placeholder } from "../types";
import {
  incrementUsage,
  getPlaceholderHistoryForKey,
  addPlaceholderValue,
  getMaxPlaceholderHistoryValues,
} from "../utils/storage";
import { pasteWithClipboardRestore } from "../utils/clipboard";
import { replacePlaceholders } from "../utils/placeholders";
import { getLastUsedValue, getRankedValuesForAutocomplete } from "../utils/placeholderHistory";

const CUSTOM_VALUE_MARKER = "__CUSTOM_VALUE__";

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

  // Sort placeholders: required first, optional after
  const sorted = [...props.placeholders].sort((a, b) => (a.isRequired === b.isRequired ? 0 : a.isRequired ? -1 : 1));
  const requiredPlaceholders = sorted.filter((p) => p.isRequired);
  const optionalPlaceholders = sorted.filter((p) => !p.isRequired);
  const hasBothSections = requiredPlaceholders.length > 0 && optionalPlaceholders.length > 0;

  // Load history and initialize form values on mount
  useEffect(() => {
    async function loadHistory() {
      const initial: Record<string, string> = {};
      const suggestions: Record<string, string[]> = {};
      const dropdownInit: Record<string, string> = {};
      const customInputMode: Record<string, boolean> = {};
      const enabledInit: Record<string, boolean> = {};

      // Get max display values from preferences (storage keeps up to 100)
      const maxDisplayValues = getMaxPlaceholderHistoryValues();

      for (const placeholder of props.placeholders) {
        const history = await getPlaceholderHistoryForKey(placeholder.key);
        // Limit displayed values to preference setting (storage may have up to 100)
        const rankedValues = getRankedValuesForAutocomplete(history, maxDisplayValues);

        suggestions[placeholder.key] = rankedValues;

        // Pre-fill with last-used value from history, or default value
        // Use ?? to preserve empty strings (for {{key|}} syntax)
        const lastUsedValue = getLastUsedValue(history);
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
      }

      setHistorySuggestions(suggestions);
      setFormValues(initial);
      setDropdownSelections(dropdownInit);
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
  const previewContent = replacePlaceholders(props.snippet.content, previewValues, props.placeholders);

  async function handleSubmit(values: Record<string, string>) {
    // Build final values: use formValues which is kept in sync
    const finalValues: Record<string, string> = { ...formValues };

    // Override disabled optional wrapper fields to empty
    for (const [key, enabled] of Object.entries(enabledOptionals)) {
      if (!enabled) finalValues[key] = "";
    }

    // Validate required fields
    const newErrors: Record<string, string> = {};
    for (const placeholder of props.placeholders) {
      if (placeholder.isRequired && !finalValues[placeholder.key]?.trim()) {
        newErrors[placeholder.key] = "This field is required";
      }
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    // Replace placeholders
    const filledContent = replacePlaceholders(props.snippet.content, finalValues, props.placeholders);

    try {
      // Save placeholder values to history (respect isSaved flag)
      for (const placeholder of props.placeholders) {
        const value = finalValues[placeholder.key];
        // Only save if isSaved is true and value is non-empty
        if (placeholder.isSaved && value && value.trim()) {
          await addPlaceholderValue(placeholder.key, value);
        }
      }

      if (props.mode === "paste-direct") {
        // Paste directly to frontmost app
        await pasteWithClipboardRestore(filledContent);
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

  function handleUseDefaults() {
    const newFormValues = { ...formValues };
    const newCustomValues = { ...customValues };
    const newEnabledOptionals = { ...enabledOptionals };

    for (const placeholder of props.placeholders) {
      if (!placeholder.isRequired) {
        const defaultVal = placeholder.defaultValue ?? "";
        newFormValues[placeholder.key] = defaultVal;
        // Keep custom input fields in sync so the text field reflects the default
        newCustomValues[placeholder.key] = defaultVal;
        const isWrapper = placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined;
        if (isWrapper && !placeholder.defaultValue) {
          newEnabledOptionals[placeholder.key] = false;
        }
      }
    }

    setFormValues(newFormValues);
    setCustomValues(newCustomValues);
    setEnabledOptionals(newEnabledOptionals);
  }

  function handleDropdownChange(key: string, value: string) {
    setDropdownSelections((prev) => ({ ...prev, [key]: value }));

    if (value === CUSTOM_VALUE_MARKER) {
      // User selected "Enter new value..." - show custom input
      setUseCustomInput((prev) => ({ ...prev, [key]: true }));
      setFormValues((prev) => ({ ...prev, [key]: customValues[key] || "" }));
    } else {
      // User selected a historical value
      setUseCustomInput((prev) => ({ ...prev, [key]: false }));
      setFormValues((prev) => ({ ...prev, [key]: value }));
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

  function buildInfoText(placeholder: Placeholder): string {
    const parts: string[] = [];
    if (placeholder.isRequired) {
      parts.push("Required field");
    } else {
      const hasWrappers = placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined;
      if (hasWrappers) {
        const example = `${placeholder.prefixWrapper ?? ""}value${placeholder.suffixWrapper ?? ""}`;
        parts.push(`Optional • Output when filled: "${example}" | empty → omitted`);
      } else {
        parts.push(`Optional (default: "${placeholder.defaultValue ?? "none"}")`);
      }
    }
    if (!placeholder.isSaved) {
      parts.push("Won't be saved to history");
    }
    return parts.join(" • ");
  }

  function renderPlaceholderField(placeholder: Placeholder) {
    const suggestions = historySuggestions[placeholder.key] || [];
    const hasHistory = suggestions.length > 0;
    const showCustomInput = useCustomInput[placeholder.key];
    const isWrapperField =
      !placeholder.isRequired && (placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined);
    const isEnabled = isWrapperField ? (enabledOptionals[placeholder.key] ?? false) : true;

    // Build title with indicators
    let title = placeholder.key;
    if (placeholder.isRequired) title += " *";
    if (!placeholder.isSaved) title = `🚫 ${title}`;

    return (
      <Fragment key={placeholder.key}>
        {isWrapperField && (
          <Form.Checkbox
            id={`${placeholder.key}-enabled`}
            title={`Include ${placeholder.key}`}
            label="Include in output"
            value={isEnabled}
            onChange={(checked) => setEnabledOptionals((prev) => ({ ...prev, [placeholder.key]: checked }))}
          />
        )}
        {(!isWrapperField || isEnabled) &&
          (hasHistory ? (
            <>
              <Form.Dropdown
                id={`${placeholder.key}-dropdown`}
                title={title}
                value={dropdownSelections[placeholder.key] || CUSTOM_VALUE_MARKER}
                onChange={(value) => handleDropdownChange(placeholder.key, value)}
                error={!showCustomInput ? errors[placeholder.key] : undefined}
                info={!placeholder.isSaved ? "Won't be saved to history" : undefined}
              >
                {suggestions.map((value) => (
                  <Form.Dropdown.Item key={value} value={value} title={value} />
                ))}
                <Form.Dropdown.Item value={CUSTOM_VALUE_MARKER} title="✏️ Enter new value..." />
              </Form.Dropdown>
              {showCustomInput && (
                <Form.TextField
                  id={`${placeholder.key}-custom`}
                  title={`${title} (Custom)`}
                  placeholder={placeholder.defaultValue || "Enter custom value..."}
                  value={customValues[placeholder.key] ?? ""}
                  error={errors[placeholder.key]}
                  onChange={(value) => handleCustomInputChange(placeholder.key, value)}
                  info={buildInfoText(placeholder)}
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
              info={buildInfoText(placeholder)}
            />
          ))}
      </Fragment>
    );
  }

  if (isLoadingHistory) {
    return (
      <Form navigationTitle={`Fill Placeholders: ${props.snippet.title}`} isLoading={true}>
        <Form.Description text="Loading history..." />
      </Form>
    );
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
          <Action
            title="Use Defaults for All Optional"
            shortcut={{ modifiers: ["cmd"], key: "d" }}
            onAction={handleUseDefaults}
          />
        </ActionPanel>
      }
    >
      <Form.Description text="Fill in the placeholder values below. Required fields (*) must be filled. Wrapper fields (checkbox) are omitted from output when unchecked." />
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
