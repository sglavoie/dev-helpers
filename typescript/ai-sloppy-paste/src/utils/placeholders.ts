import { Placeholder } from "../types";
import { parsePlaceholderSyntax } from "./placeholderSyntax";

/**
 * System placeholders that auto-resolve without user input.
 * Use ALL-CAPS names to distinguish from user-defined placeholders.
 */
function localDateISO(now: Date): string {
  // Use the local-timezone YYYY-MM-DD; toISOString() converts to UTC and would
  // return tomorrow's date when the user runs the snippet late in the evening
  // in a timezone west of UTC.
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

const SYSTEM_PLACEHOLDERS: Record<string, () => string> = {
  DATE: () => {
    return localDateISO(new Date());
  },
  TIME: () => {
    const now = new Date();
    return now.toLocaleTimeString("en-US", { hour12: false, hour: "2-digit", minute: "2-digit" });
  },
  DATETIME: () => {
    const now = new Date();
    const date = localDateISO(now);
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
    // Allow optional whitespace around the name to mirror extractPlaceholders
    // (which trims keys), so e.g. `{{ DATE }}` is resolved.
    const regex = new RegExp(`\\{\\{\\s*${name}\\s*\\}\\}`, "g");
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
  return parsePlaceholderSyntax(text).placeholders;
}

/**
 * Replaces placeholders in text with provided values
 * Falls back to default values if no value provided
 */
export function replacePlaceholders(text: string, values: Record<string, string>, placeholders: Placeholder[]): string {
  const canonicalByKey = new Map(placeholders.map((placeholder) => [placeholder.key, placeholder]));
  const occurrences = parsePlaceholderSyntax(text).occurrences;
  let result = "";
  let cursor = 0;

  for (const occurrence of occurrences) {
    result += text.slice(cursor, occurrence.range.start);
    const canonical = canonicalByKey.get(occurrence.key);
    if (!canonical) {
      result += occurrence.raw;
      cursor = occurrence.range.end;
      continue;
    }

    const value = values[occurrence.key] ?? canonical.defaultValue ?? "";
    result += value.trim() ? (occurrence.prefixWrapper ?? "") + value + (occurrence.suffixWrapper ?? "") : "";
    cursor = occurrence.range.end;
  }

  return result + text.slice(cursor);
}

/**
 * Processes conditional block syntax in text.
 * Supports:
 *   {{#if key}}...{{/if}}
 *   {{#if key}}...{{#else}}...{{/if}}
 *   Nested {{#if}} blocks (resolved inside-out, up to 10 levels deep)
 *
 * A key is truthy when its value is non-empty after trimming.
 * Guard-only keys (checkbox in the form) use "true" for checked, "" for unchecked.
 * Consumes one trailing newline after {{/if}} to avoid blank lines on removal.
 */
export function processConditionalBlocks(text: string, values: Record<string, string>): string {
  // Leaf block: body and else-body use a negative lookahead to exclude any nested
  // {{#if, so this regex only matches the innermost (leaf) blocks directly.
  const leafRegex =
    /\{\{#if ([^}]+)\}\}((?:(?!\{\{#if\s)[\s\S])*?)(?:\{\{#else\}\}((?:(?!\{\{#if\s)[\s\S])*?)(?:\{\{\/else\}\})?)?\{\{\/if\}\}/g;

  const MAX_ITERATIONS = 10;
  let result = text;
  let iteration = 0;

  while (iteration < MAX_ITERATIONS) {
    let foundLeaf = false;
    result = result.replace(leafRegex, (_match, rawKey, ifBody, elseBody) => {
      foundLeaf = true;
      const key = rawKey
        .trim()
        .replace(/\s+"[^"]*"$/, "")
        .replace(/^\+/, "");
      const value = values[key] ?? "";
      const isTruthy = value.trim().length > 0;
      const selectedBody = isTruthy ? ifBody : (elseBody ?? "");
      return selectedBody.replace(/^\n/, "").replace(/\n$/, "");
    });

    if (!foundLeaf) break;
    iteration++;
  }

  return result;
}

export interface ConditionalBlockBody {
  ifBody: string;
  elseBody?: string;
}

// Nested {{#if}} blocks require a depth-aware scan — a regex alone cannot balance them.
export function extractConditionalBlockBodies(text: string, key: string): ConditionalBlockBody[] {
  // Captures the key while ignoring the optional `+` prefix and `"Label"`; both are stripped before comparison.
  const openRegex = /\{\{#if\s+\+?([^\s}"]+)(?:\s+"[^"]*")?\s*\}\}/g;
  const results: ConditionalBlockBody[] = [];

  let openMatch: RegExpExecArray | null;
  while ((openMatch = openRegex.exec(text)) !== null) {
    if (openMatch[1].trim() !== key) continue;

    const openEnd = openRegex.lastIndex;
    const close = findMatchingClose(text, openEnd);
    if (close === null) continue;

    results.push(splitTopLevelElse(text.slice(openEnd, close.start)));
    openRegex.lastIndex = close.end;
  }

  return results;
}

function findMatchingClose(text: string, from: number): { start: number; end: number } | null {
  const tokenRegex = /\{\{#if\s|\{\{\/if\}\}/g;
  tokenRegex.lastIndex = from;
  let depth = 1;
  let m: RegExpExecArray | null;
  while ((m = tokenRegex.exec(text)) !== null) {
    if (m[0] === "{{/if}}") {
      depth--;
      if (depth === 0) return { start: m.index, end: tokenRegex.lastIndex };
    } else {
      depth++;
    }
  }
  return null;
}

function splitTopLevelElse(body: string): ConditionalBlockBody {
  const tokenRegex = /\{\{#if\s|\{\{\/if\}\}|\{\{#else\}\}/g;
  let depth = 0;
  let m: RegExpExecArray | null;
  while ((m = tokenRegex.exec(body)) !== null) {
    const tok = m[0];
    if (tok === "{{#else}}") {
      if (depth === 0) {
        const ifBody = body.slice(0, m.index);
        let elseBody = body.slice(tokenRegex.lastIndex);
        if (elseBody.endsWith("{{/else}}")) elseBody = elseBody.slice(0, -9);
        return { ifBody, elseBody };
      }
    } else if (tok === "{{/if}}") {
      depth--;
    } else {
      depth++;
    }
  }
  return { ifBody: body };
}
