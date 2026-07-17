import { ParsedValuePlaceholderOccurrence, parsePlaceholderSyntax } from "./placeholderSyntax";
import { getSystemPlaceholderNames, processSystemPlaceholders } from "./placeholders";

const MAX_PREVIEW_LENGTH = 500;

function previewSystemPlaceholders(content: string): string {
  let result = content;

  for (const name of getSystemPlaceholderNames()) {
    const resolved = processSystemPlaceholders(`{{${name}}}`);
    result = result.replace(new RegExp(`\\{\\{${name}\\}\\}`, "g"), `⟨${name} → ${resolved}⟩`);
  }

  return result;
}

function previewConditionalBlocks(content: string): string {
  return content.replace(/\{\{#if ([^}]+)\}\}([\s\S]*?)\{\{\/if\}\}/g, (_match, key: string, body: string) => {
    const displayKey = key
      .trim()
      .replace(/\s+"[^"]*"$/, "")
      .replace(/^\+/, "");
    const cleanBody = body.replace(/\{\{#else\}\}/g, " | else: ").replace(/\{\{\/else\}\}/g, "");
    return `⟨if ${displayKey}: ${cleanBody.trim()}⟩`;
  });
}

function displayKeyFor(occurrence: ParsedValuePlaceholderOccurrence): string {
  const prefix = occurrence.prefixWrapper?.trim() ?? "";
  const suffix = occurrence.suffixWrapper?.trim() ?? "";
  const main = prefix ? `${prefix}${occurrence.key}` : occurrence.key;
  const wrapped = suffix ? `${main} ${suffix}` : main;
  return occurrence.isSaved ? wrapped : `!${wrapped}`;
}

function previewOccurrence(occurrence: ParsedValuePlaceholderOccurrence): string {
  const label = displayKeyFor(occurrence);

  if (occurrence.choices) {
    const choices = occurrence.choices.map((choice) => JSON.stringify(choice)).join(", ");
    const defaultLabel = occurrence.hasExplicitDefault
      ? `; default: ${JSON.stringify(occurrence.explicitDefault ?? "")}`
      : "";
    return `⟨${label} — choices: ${choices}${defaultLabel}⟩`;
  }

  return occurrence.hasExplicitDefault && occurrence.explicitDefault
    ? `⟨${label} = ${JSON.stringify(occurrence.explicitDefault)}⟩`
    : `⟨${label}⟩`;
}

function previewValuePlaceholders(content: string): string {
  const occurrences = parsePlaceholderSyntax(content).occurrences;
  let result = "";
  let cursor = 0;

  for (const occurrence of occurrences) {
    result += content.slice(cursor, occurrence.range.start);
    result += previewOccurrence(occurrence);
    cursor = occurrence.range.end;
  }

  return result + content.slice(cursor);
}

/** Builds the authoring preview without duplicating placeholder grammar in the form component. */
export function buildSnippetPreview(content: string): string {
  if (!content.includes("{{")) return "";

  const withSystems = previewSystemPlaceholders(content);
  const withConditionals = previewConditionalBlocks(withSystems);
  return previewValuePlaceholders(withConditionals).slice(0, MAX_PREVIEW_LENGTH);
}
