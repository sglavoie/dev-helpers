import { Placeholder } from "../types";

/**
 * Extracts placeholders from text in the format {{key}} or {{key|default}}
 * Returns an array of unique placeholders with their keys and default values
 */
export function extractPlaceholders(text: string): Placeholder[] {
  const regex = /\{\{([^}]+)\}\}/g;
  const placeholders: Placeholder[] = [];
  const seen = new Set<string>();

  let match;
  while ((match = regex.exec(text)) !== null) {
    const content = match[1].trim();
    const parts = content.split("|");
    const key = parts[0].trim();
    const defaultValue = parts[1]?.trim();

    // Only add unique placeholders (by key)
    if (!seen.has(key)) {
      placeholders.push({
        key,
        defaultValue: defaultValue || undefined,
        isRequired: !defaultValue,
      });
      seen.add(key);
    }
  }

  return placeholders;
}

/**
 * Replaces placeholders in text with provided values
 * Falls back to default values if no value provided
 */
export function replacePlaceholders(text: string, values: Record<string, string>, placeholders: Placeholder[]): string {
  let result = text;

  for (const placeholder of placeholders) {
    // Use user value, or fall back to default value, or empty string
    const value = values[placeholder.key] || placeholder.defaultValue || "";

    // Match both {{key}} and {{key|default}} formats
    const regex = new RegExp(`\\{\\{${escapeRegex(placeholder.key)}(?:\\|[^}]*)?\\}\\}`, "g");
    result = result.replace(regex, value);
  }

  return result;
}

/**
 * Escapes special regex characters in a string
 */
function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
