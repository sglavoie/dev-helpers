import { useState, useEffect, useRef } from "react";
import { Placeholder } from "../types";
import { getPlaceholderHistoryForKey, getMaxPlaceholderHistoryValues } from "../utils/storage";
import { getLastUsedValue, getRankedValuesForAutocomplete } from "../utils/placeholderHistory";
import {
  CUSTOM_VALUE_MARKER,
  hasAuthoredChoices,
  initializeAuthoredChoiceState,
  isWrapperPlaceholder,
  resolveAuthoredChoiceSelection,
} from "../utils/placeholderFormChoices";

/**
 * Owns every mutable field of the placeholder fill-in form: the current values,
 * which fields show a dropdown versus a free-text input, and which optional
 * fields are enabled. Values are seeded from placeholder history on mount.
 */
export function usePlaceholderFormState(placeholders: Placeholder[]) {
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [historySuggestions, setHistorySuggestions] = useState<Record<string, string[]>>({});
  const [isLoadingHistory, setIsLoadingHistory] = useState(true);
  const [formValues, setFormValues] = useState<Record<string, string>>({});
  const [dropdownSelections, setDropdownSelections] = useState<Record<string, string>>({});
  const [customValues, setCustomValues] = useState<Record<string, string>>({});
  const [useCustomInput, setUseCustomInput] = useState<Record<string, boolean>>({});
  const [enabledOptionals, setEnabledOptionals] = useState<Record<string, boolean>>({});
  const prefilledRef = useRef(new Set<string>());

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

      for (const placeholder of placeholders) {
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
        if (!placeholder.isRequired && isWrapperPlaceholder(placeholder)) {
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
  }, [placeholders]);

  function clearError(key: string) {
    if (!errors[key]) return;
    setErrors((prev) => {
      const newErrors = { ...prev };
      delete newErrors[key];
      return newErrors;
    });
  }

  /** Resets every optional field back to its authored default. */
  function handleUseDefaults() {
    const newFormValues = { ...formValues };
    const newDropdownSelections = { ...dropdownSelections };
    const newCustomValues = { ...customValues };
    const newUseCustomInput = { ...useCustomInput };
    const newEnabledOptionals = { ...enabledOptionals };

    for (const placeholder of placeholders) {
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
      if (isWrapperPlaceholder(placeholder)) {
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

    clearError(key);
  }

  function handleCustomInputChange(key: string, value: string) {
    setCustomValues((prev) => ({ ...prev, [key]: value }));
    setFormValues((prev) => ({ ...prev, [key]: value }));
    clearError(key);
  }

  function setEnabled(key: string, enabled: boolean) {
    setEnabledOptionals((prev) => ({ ...prev, [key]: enabled }));
  }

  return {
    errors,
    setErrors,
    historySuggestions,
    isLoadingHistory,
    formValues,
    dropdownSelections,
    customValues,
    useCustomInput,
    enabledOptionals,
    setEnabled,
    /** Keys that were pre-filled from history on the last load. */
    prefilledKeys: prefilledRef.current,
    handleUseDefaults,
    handleDropdownChange,
    handleCustomInputChange,
  };
}

export type PlaceholderFormState = ReturnType<typeof usePlaceholderFormState>;
