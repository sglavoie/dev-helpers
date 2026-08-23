import type {
  ParsedValuePlaceholderOccurrence,
  PlaceholderSyntaxDiagnostic,
  PlaceholderSyntaxResult,
  ScannedExpression,
} from "./placeholderSyntaxTypes";
import { parseChoiceExpression } from "./placeholderSyntaxChoices";
import { aggregatePlaceholders, findChoiceConflicts } from "./placeholderSyntaxAggregate";

export type {
  ParsedValuePlaceholderOccurrence,
  PlaceholderSourceRange,
  PlaceholderSyntaxDiagnostic,
  PlaceholderSyntaxResult,
} from "./placeholderSyntaxTypes";

const CONTROL_EXPRESSIONS = new Set(["#else", "/else", "/if"]);

/**
 * Parses all user-value placeholder syntax in one pass.
 *
 * Invalid authored-choice expressions receive diagnostics but deliberately use
 * the legacy parser for their runtime occurrence. This keeps imported or
 * clipboard-saved malformed snippets safe to run while editor validation can
 * reject the same content.
 */
export function parsePlaceholderSyntax(text: string): PlaceholderSyntaxResult {
  const occurrences: ParsedValuePlaceholderOccurrence[] = [];
  const diagnostics: PlaceholderSyntaxDiagnostic[] = [];

  for (const expression of scanExpressions(text)) {
    const trimmed = expression.content.trim();
    if (trimmed.startsWith("#if ") || CONTROL_EXPRESSIONS.has(trimmed)) continue;

    const choice = parseChoiceExpression(trimmed);
    let occurrence: ParsedValuePlaceholderOccurrence;

    if (choice.kind === "choice") {
      occurrence = {
        raw: expression.raw,
        range: expression.range,
        key: choice.key,
        prefixWrapper: choice.prefixWrapper,
        suffixWrapper: choice.suffixWrapper,
        explicitDefault: choice.explicitDefault,
        hasExplicitDefault: choice.hasExplicitDefault,
        isSaved: choice.isSaved,
        isRequired: choice.isRequired,
        choices: choice.choices,
        isChoiceDeclaration: true,
      };
    } else {
      if (choice.kind === "invalid-choice") {
        diagnostics.push({
          range: expression.range,
          expression: expression.raw,
          message: `Invalid authored choices in ${expression.raw}: ${choice.reason}`,
        });
      }
      occurrence = parseLegacyOccurrence(expression);
    }

    if (occurrence.key) occurrences.push(occurrence);
  }

  diagnostics.push(...findChoiceConflicts(occurrences));
  diagnostics.sort((a, b) => a.range.start - b.range.start || a.range.end - b.range.end);

  return {
    occurrences,
    placeholders: aggregatePlaceholders(text, occurrences),
    diagnostics,
  };
}

function scanExpressions(text: string): ScannedExpression[] {
  const expressions: ScannedExpression[] = [];
  const regex = /\{\{([^}]+)\}\}/g;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(text)) !== null) {
    expressions.push({
      raw: match[0],
      range: { start: match.index, end: regex.lastIndex },
      content: match[1],
    });
  }

  return expressions;
}

function parseLegacyOccurrence(expression: ScannedExpression): ParsedValuePlaceholderOccurrence {
  let content = expression.content.trim();
  const isSaved = !content.startsWith("!");
  if (!isSaved) content = content.slice(1).trim();

  const pipeIndex = content.lastIndexOf("|");
  const hasExplicitDefault = pipeIndex !== -1;
  const explicitDefault = hasExplicitDefault ? content.slice(pipeIndex + 1).trim() : undefined;
  const coreContent = hasExplicitDefault ? content.slice(0, pipeIndex).trim() : content;
  const parts = coreContent.split(":");

  let key: string;
  let prefixWrapper: string | undefined;
  let suffixWrapper: string | undefined;

  if (parts.length === 1) {
    key = parts[0].trim();
  } else if (parts.length === 3) {
    prefixWrapper = parts[0] || undefined;
    key = parts[1].trim();
    suffixWrapper = parts[2] || undefined;
  } else {
    key = coreContent;
  }

  const hasNonEmptyWrappers = prefixWrapper !== undefined || suffixWrapper !== undefined;
  return {
    raw: expression.raw,
    range: expression.range,
    key,
    prefixWrapper,
    suffixWrapper,
    explicitDefault,
    hasExplicitDefault,
    isSaved,
    isRequired: !hasExplicitDefault && !hasNonEmptyWrappers,
    isChoiceDeclaration: false,
  };
}
