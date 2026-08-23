import { Placeholder } from "../types";

export const CUSTOM_VALUE_MARKER = "__CUSTOM_VALUE__";
const AUTHORED_CHOICE_OPTION_PREFIX = "__AUTHORED_CHOICE__";

export interface AuthoredChoiceFieldState {
  formValue: string;
  dropdownSelection: string;
  customValue: string;
  useCustomInput: boolean;
  enabledOptional?: boolean;
}

export function hasAuthoredChoices(placeholder: Placeholder): placeholder is Placeholder & { choices: string[] } {
  return (placeholder.choices?.length ?? 0) > 0;
}

export function isWrapperPlaceholder(placeholder: Placeholder): boolean {
  return placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined;
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
  const enabledOptional = !placeholder.isRequired && isWrapperPlaceholder(placeholder) ? !!defaultValue : undefined;

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
