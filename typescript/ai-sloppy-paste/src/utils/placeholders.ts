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
    let content = match[1].trim();

    // 1. Check for no-save flag (!)
    const isSaved = !content.startsWith("!");
    if (!isSaved) {
      content = content.slice(1).trim();
    }

    // 2. Extract default value (split on rightmost |)
    const pipeIndex = content.lastIndexOf("|");
    const hasDefault = pipeIndex !== -1;
    const defaultValue = hasDefault ? content.slice(pipeIndex + 1).trim() : undefined;
    const coreContent = hasDefault ? content.slice(0, pipeIndex).trim() : content;

    // 3. Parse key and wrappers
    const parts = coreContent.split(":");

    let key: string;
    let prefixWrapper: string | undefined;
    let suffixWrapper: string | undefined;

    if (parts.length === 1) {
      // Simple format: {{key}}
      key = parts[0].trim();
    } else if (parts.length === 3) {
      // Wrapper format: {{prefix:key:suffix}}
      const prefix = parts[0];
      key = parts[1].trim();
      const suffix = parts[2];

      // Convert empty strings to undefined
      prefixWrapper = prefix || undefined;
      suffixWrapper = suffix || undefined;
    } else {
      // Invalid format (1, 2, or 4+ colons) - treat entire content as key
      key = coreContent;
    }

    // Only add unique placeholders (by key)
    if (key && !seen.has(key)) {
      placeholders.push({
        key,
        defaultValue,
        isRequired: !hasDefault,
        isSaved,
        prefixWrapper,
        suffixWrapper,
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
    // Use user value, or fall back to default value (which may be empty string), or empty string
    const value = values[placeholder.key] ?? placeholder.defaultValue ?? "";

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
