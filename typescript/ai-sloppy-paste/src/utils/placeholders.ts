import { Placeholder } from "../types";

/**
 * System placeholders that auto-resolve without user input.
 * Use ALL-CAPS names to distinguish from user-defined placeholders.
 */
const SYSTEM_PLACEHOLDERS: Record<string, () => string> = {
  DATE: () => {
    const now = new Date();
    return now.toISOString().split("T")[0];
  },
  TIME: () => {
    const now = new Date();
    return now.toLocaleTimeString("en-US", { hour12: false, hour: "2-digit", minute: "2-digit" });
  },
  DATETIME: () => {
    const now = new Date();
    const date = now.toISOString().split("T")[0];
    const time = now.toLocaleTimeString("en-US", { hour12: false, hour: "2-digit", minute: "2-digit" });
    return `${date} ${time}`;
  },
  TODAY: () => {
    const now = new Date();
    return now.toLocaleDateString("en-US", { weekday: "long", year: "numeric", month: "long", day: "numeric" });
  },
  NOW: () => {
    return new Date().toISOString();
  },
  YEAR: () => {
    return new Date().getFullYear().toString();
  },
  MONTH: () => {
    return new Date().toLocaleDateString("en-US", { month: "long" });
  },
  DAY: () => {
    return new Date().toLocaleDateString("en-US", { weekday: "long" });
  },
};

/**
 * Process system placeholders in text.
 * System placeholders use ALL-CAPS names and auto-resolve without user input.
 * Call this BEFORE extractPlaceholders to avoid prompting for system values.
 *
 * Supported placeholders:
 * - {{DATE}} → 2024-01-15
 * - {{TIME}} → 14:30
 * - {{DATETIME}} → 2024-01-15 14:30
 * - {{TODAY}} → Monday, January 15, 2024
 * - {{NOW}} → 2024-01-15T14:30:00.000Z
 * - {{YEAR}} → 2024
 * - {{MONTH}} → January
 * - {{DAY}} → Monday
 */
export function processSystemPlaceholders(text: string): string {
  let result = text;

  for (const [name, getValue] of Object.entries(SYSTEM_PLACEHOLDERS)) {
    const regex = new RegExp(`\\{\\{${name}\\}\\}`, "g");
    if (regex.test(result)) {
      result = result.replace(regex, getValue());
    }
  }

  return result;
}

/**
 * Get list of available system placeholder names for documentation/help
 */
export function getSystemPlaceholderNames(): string[] {
  return Object.keys(SYSTEM_PLACEHOLDERS);
}

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

    // Skip block control tokens — these are handled by processConditionalBlocks
    if (content.startsWith("#if ") || content === "#else" || content === "/if") {
      continue;
    }

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
      // Wrapper syntax implies optionality because replacement logic
      // already handles empty values by omitting the wrapper
      // Exception: {{:key:}} with both wrappers empty is equivalent to {{key}}
      const hasNonEmptyWrappers = prefixWrapper !== undefined || suffixWrapper !== undefined;

      placeholders.push({
        key,
        defaultValue,
        isRequired: !hasDefault && !hasNonEmptyWrappers,
        isSaved,
        prefixWrapper,
        suffixWrapper,
      });
      seen.add(key);
    }
  }

  // Second pass: extract guard-only keys from {{#if key}} patterns
  const ifRegex = /\{\{#if ([^}]+)\}\}/g;
  let ifMatch;
  while ((ifMatch = ifRegex.exec(text)) !== null) {
    const guardKey = ifMatch[1].trim();
    if (guardKey && !seen.has(guardKey)) {
      placeholders.push({
        key: guardKey,
        defaultValue: undefined,
        isRequired: false,
        isSaved: false,
        isGuardOnly: true,
      });
      seen.add(guardKey);
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
    // Determine final value
    const value = values[placeholder.key] ?? placeholder.defaultValue ?? "";

    // Build replacement with conditional wrappers
    let replacement: string;

    if (value.trim()) {
      // Non-empty value: apply wrappers
      replacement = value;
      if (placeholder.prefixWrapper) {
        replacement = placeholder.prefixWrapper + replacement;
      }
      if (placeholder.suffixWrapper) {
        replacement = replacement + placeholder.suffixWrapper;
      }
    } else {
      // Empty or whitespace-only: no wrappers
      replacement = "";
    }

    // Replace all occurrences
    const regex = buildPlaceholderRegex(placeholder);
    result = result.replace(regex, replacement);
  }

  return result;
}

/**
 * Escapes special regex characters in a string
 */
function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Builds regex pattern to match original placeholder syntax including wrappers
 */
function buildPlaceholderRegex(placeholder: Placeholder): RegExp {
  const escapedKey = escapeRegex(placeholder.key);

  // Handle different syntax variations:
  // - {{!prefix:key:suffix|default}}
  // - {{prefix:key:suffix}}
  // - {{!key|default}}
  // - {{key}}

  // Build pattern that matches all variations of this placeholder
  let pattern = "\\{\\{";

  // Optional ! prefix
  pattern += "!?";

  // Optional wrapper syntax
  if (placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined) {
    // Has wrappers: match prefix:key:suffix format
    pattern += "[^:]*:" + escapedKey + ":[^|]*";
  } else {
    // No wrappers: match simple key
    pattern += escapedKey;
  }

  // Optional |default
  pattern += "(?:\\|[^}]*)?";

  pattern += "\\}\\}";

  return new RegExp(pattern, "g");
}

/**
 * Processes conditional block syntax in text.
 * Supports:
 *   {{#if key}}...{{/if}}
 *   {{#if key}}...{{#else}}...{{/if}}
 *
 * A key is truthy when its value is non-empty after trimming.
 * Guard-only keys (checkbox in the form) use "true" for checked, "" for unchecked.
 * Consumes one trailing newline after {{/if}} to avoid blank lines on removal.
 * No nested {{#if}} blocks supported.
 */
export function processConditionalBlocks(text: string, values: Record<string, string>): string {
  const blockRegex = /\{\{#if ([^}]+)\}\}([\s\S]*?)(?:\{\{#else\}\}([\s\S]*?))?\{\{\/if\}\}/g;

  return text.replace(blockRegex, (_match, rawKey, ifBody, elseBody) => {
    const key = rawKey.trim();
    const value = values[key] ?? "";
    const isTruthy = value.trim().length > 0;

    const selectedBody = isTruthy ? ifBody : (elseBody ?? "");

    // Trim exactly one leading and one trailing newline to avoid blank lines
    // when the block tags are on their own lines
    return selectedBody.replace(/^\n/, "").replace(/\n$/, "");
  });
}
