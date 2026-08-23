import { Form, Icon } from "@raycast/api";
import { Fragment } from "react";
import { Placeholder } from "../types";
import { buildFieldPreview } from "../utils/fieldPreview";
import {
  CUSTOM_VALUE_MARKER,
  getAuthoredChoiceOptionId,
  hasAuthoredChoices,
  isWrapperPlaceholder,
} from "../utils/placeholderFormChoices";
import { PlaceholderFormState } from "../hooks/usePlaceholderFormState";

function truncateTitle(text: string, max = 25): string {
  return text.length <= max ? text : text.slice(0, max - 1) + "…";
}

/** Summarises a placeholder's rules for the field's info tooltip. */
export function buildInfoText(placeholder: Placeholder): string {
  if (placeholder.isGuardOnly) {
    return "Conditional — checked = block shown, unchecked = block omitted";
  }
  const parts: string[] = [];
  if (placeholder.isRequired) {
    parts.push("Required field");
  } else if (isWrapperPlaceholder(placeholder)) {
    const example = `${placeholder.prefixWrapper ?? ""}value${placeholder.suffixWrapper ?? ""}`;
    parts.push(`Optional wrapper • Output when included: "${example}" • Uncheck to omit it`);
  } else {
    parts.push(`Optional (default: "${placeholder.defaultValue ?? "none"}")`);
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

/**
 * Renders one placeholder as the form control that matches its kind: a checkbox
 * for guard-only keys, a dropdown when authored choices or history exist, and a
 * plain text field otherwise. Optional wrapper fields gain a leading checkbox
 * that hides the value control when unchecked.
 */
export function renderPlaceholderField(placeholder: Placeholder, snippetContent: string, state: PlaceholderFormState) {
  const suggestions = state.historySuggestions[placeholder.key] || [];
  const authoredChoices = hasAuthoredChoices(placeholder) ? placeholder.choices : undefined;
  const hasAuthoredChoiceDropdown = authoredChoices !== undefined;
  const hasHistory = suggestions.length > 0;
  const showCustomInput = state.useCustomInput[placeholder.key];
  const currentValue = state.formValues[placeholder.key] ?? "";
  const fieldHint = buildFieldPreview(placeholder, snippetContent, currentValue);
  const fieldHintFull = buildFieldPreview(placeholder, snippetContent, currentValue, {
    truncate: false,
  });
  const fieldHintTooltip = fieldHintFull && fieldHintFull !== fieldHint ? fieldHintFull : undefined;
  const withFieldHint = (base: string | undefined): string | undefined => {
    const parts = [base, fieldHintTooltip].filter(Boolean) as string[];
    return parts.length > 0 ? parts.join("\n\n") : undefined;
  };
  if (placeholder.isGuardOnly) {
    const isChecked = state.enabledOptionals[placeholder.key] ?? false;
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
          onChange={(checked) => state.setEnabled(placeholder.key, checked)}
        />
        {fieldHint && <Form.Description text={fieldHint} />}
      </Fragment>
    );
  }

  const isWrapperField = !placeholder.isRequired && isWrapperPlaceholder(placeholder);
  const isEnabled = isWrapperField ? (state.enabledOptionals[placeholder.key] ?? false) : true;

  // Build title with indicators
  let title = placeholder.key;
  if (placeholder.isRequired) title += " *";
  if (state.prefilledKeys.has(placeholder.key)) title += " (↻ last used)";
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
          onChange={(checked) => state.setEnabled(placeholder.key, checked)}
        />
      )}
      {(!isWrapperField || isEnabled) &&
        (hasAuthoredChoiceDropdown || hasHistory ? (
          <>
            <Form.Dropdown
              id={`${placeholder.key}-dropdown`}
              title={title}
              value={state.dropdownSelections[placeholder.key] || CUSTOM_VALUE_MARKER}
              onChange={(value) => state.handleDropdownChange(placeholder, value)}
              error={!showCustomInput ? state.errors[placeholder.key] : undefined}
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
                value={state.customValues[placeholder.key] ?? ""}
                error={state.errors[placeholder.key]}
                onChange={(value) => state.handleCustomInputChange(placeholder.key, value)}
                info={withFieldHint(buildInfoText(placeholder))}
              />
            )}
          </>
        ) : (
          <Form.TextField
            id={placeholder.key}
            title={title}
            placeholder={placeholder.defaultValue || "Enter value..."}
            value={currentValue}
            error={state.errors[placeholder.key]}
            onChange={(value) => state.handleCustomInputChange(placeholder.key, value)}
            info={withFieldHint(buildInfoText(placeholder))}
          />
        ))}
      {fieldHint && <Form.Description text={fieldHint} />}
    </Fragment>
  );
}
