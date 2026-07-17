import { Placeholder } from "../types";

export interface PlaceholderSourceRange {
  /** Inclusive offset in the source text. */
  start: number;
  /** Exclusive offset in the source text. */
  end: number;
}

export interface ParsedValuePlaceholderOccurrence {
  /** The complete authored expression, including braces. */
  raw: string;
  range: PlaceholderSourceRange;
  key: string;
  prefixWrapper?: string;
  suffixWrapper?: string;
  /** Present when the expression has a trailing `|default`, including an empty default. */
  explicitDefault?: string;
  hasExplicitDefault: boolean;
  isSaved: boolean;
  isRequired: boolean;
  choices?: string[];
  isChoiceDeclaration: boolean;
}

export interface PlaceholderSyntaxDiagnostic {
  range: PlaceholderSourceRange;
  /** The complete offending expression, including braces. */
  expression: string;
  message: string;
}

export interface PlaceholderSyntaxResult {
  occurrences: ParsedValuePlaceholderOccurrence[];
  placeholders: Placeholder[];
  diagnostics: PlaceholderSyntaxDiagnostic[];
}

interface ChoiceParseSuccess {
  kind: "choice";
  key: string;
  prefixWrapper?: string;
  suffixWrapper?: string;
  explicitDefault?: string;
  hasExplicitDefault: boolean;
  isSaved: boolean;
  isRequired: boolean;
  choices: string[];
}

interface ChoiceParseFailure {
  kind: "invalid-choice";
  reason: string;
}

interface NotChoiceSyntax {
  kind: "legacy";
}

type ChoiceParseResult = ChoiceParseSuccess | ChoiceParseFailure | NotChoiceSyntax;

interface ScannedExpression {
  raw: string;
  range: PlaceholderSourceRange;
  content: string;
}

const CONTROL_EXPRESSIONS = new Set(["#else", "/else", "/if"]);
const SUPPORTED_CHOICE_ESCAPES = new Set(["|", "[", "]", "\\"]);

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

function parseChoiceExpression(trimmedContent: string): ChoiceParseResult {
  let body = trimmedContent;
  const isSaved = !body.startsWith("!");
  if (!isSaved) body = body.slice(1).trim();
  if (!hasKeyChoiceIntent(body)) return { kind: "legacy" };

  let opening = -1;
  let closing = -1;
  let insideChoices = false;
  let sawBracket = false;
  const topLevelPipes: number[] = [];
  const topLevelColons: number[] = [];

  for (let index = 0; index < body.length; index++) {
    const character = body[index];

    if (insideChoices) {
      if (character === "\\") {
        if (index + 1 >= body.length) {
          return invalidChoice(
            "the choice list ends with a dangling escape; remove it or escape a supported character",
          );
        }
        const escaped = body[index + 1];
        if (!SUPPORTED_CHOICE_ESCAPES.has(escaped)) {
          return invalidChoice(`unsupported escape \\${escaped}; only \\|, \\[, \\], and \\\\ are supported`);
        }
        index++;
        continue;
      }
      if (character === "[") {
        return invalidChoice("nested choice brackets are not allowed; escape a literal bracket as \\[");
      }
      if (character === "]") {
        closing = index;
        insideChoices = false;
      }
      continue;
    }

    if (character === "[") {
      sawBracket = true;
      if (opening !== -1) {
        const firstColon = body.indexOf(":");
        if (firstColon !== -1 && opening < firstColon && index > firstColon) {
          return invalidChoice(
            "brackets in a prefix wrapper cannot be combined with authored choices; remove the brackets from the prefix wrapper or remove the choice list",
          );
        }
        if (topLevelPipes.some((pipe) => pipe > closing)) {
          return invalidChoice(
            "brackets in a default value cannot be combined with authored choices; remove the brackets from the default value or remove the choice list",
          );
        }
        if (
          topLevelColons.length >= 2 &&
          topLevelColons[0] < opening &&
          closing < topLevelColons[1] &&
          index > topLevelColons[1]
        ) {
          return invalidChoice(
            "brackets in a suffix wrapper cannot be combined with authored choices; remove the brackets from the suffix wrapper or remove the choice list",
          );
        }
        return invalidChoice("only one choice list is allowed per placeholder expression");
      }
      opening = index;
      insideChoices = true;
    } else if (character === "]") {
      sawBracket = true;
      if (opening !== -1 && closing !== -1) {
        if (topLevelPipes.some((pipe) => pipe > closing)) {
          return invalidChoice(
            "brackets in a default value cannot be combined with authored choices; remove the brackets from the default value or remove the choice list",
          );
        }
        if (
          topLevelColons.length >= 2 &&
          topLevelColons[0] < opening &&
          closing < topLevelColons[1] &&
          index > topLevelColons[1]
        ) {
          return invalidChoice(
            "brackets in a suffix wrapper cannot be combined with authored choices; remove the brackets from the suffix wrapper or remove the choice list",
          );
        }
      }
      return invalidChoice(
        "found an unmatched closing bracket; brackets are reserved in choice-capable key positions, so rename the placeholder key to remove `]`",
      );
    } else if (character === "|") {
      topLevelPipes.push(index);
    } else if (character === ":") {
      topLevelColons.push(index);
    }
  }

  if (!sawBracket) return { kind: "legacy" };
  if (insideChoices || closing === -1) {
    return invalidChoice("the choice list is missing a closing bracket `]`");
  }

  // Keep the established rightmost-default behavior, but ignore pipes inside
  // the choice list itself.
  const defaultDelimiter = topLevelPipes.length > 0 ? topLevelPipes[topLevelPipes.length - 1] : undefined;
  const coreEnd = defaultDelimiter ?? body.length;
  if (opening >= coreEnd || closing >= coreEnd) {
    return invalidChoice("the choice list must follow the placeholder key and come before the default value");
  }

  const coreColons = topLevelColons.filter((index) => index < coreEnd);
  let keyStart = 0;
  let keyEnd = coreEnd;
  let prefixWrapper: string | undefined;
  let suffixWrapper: string | undefined;

  if (coreColons.length === 2) {
    const [firstColon, secondColon] = coreColons;
    keyStart = firstColon + 1;
    keyEnd = secondColon;
    prefixWrapper = body.slice(0, firstColon) || undefined;
    suffixWrapper = body.slice(secondColon + 1, coreEnd) || undefined;
  } else if (coreColons.length !== 0) {
    return invalidChoice("wrapper syntax must contain exactly two colons: `prefix:key[one|two]:suffix`");
  }

  if (opening < keyStart || closing >= keyEnd) {
    return invalidChoice("the choice list must be attached to the placeholder key, not its wrappers");
  }

  const key = body.slice(keyStart, opening).trim();
  const trailingKeyText = body.slice(closing + 1, keyEnd).trim();
  if (!key) return invalidChoice("enter a placeholder key before the choice list");
  if (trailingKeyText) {
    return invalidChoice("place the closing choice bracket immediately after the final choice");
  }

  const decoded = decodeChoices(body.slice(opening + 1, closing));
  if (typeof decoded === "string") return invalidChoice(decoded);

  const hasExplicitDefault = defaultDelimiter !== undefined;
  const explicitDefault = hasExplicitDefault ? body.slice(defaultDelimiter + 1).trim() : undefined;
  const hasNonEmptyWrappers = prefixWrapper !== undefined || suffixWrapper !== undefined;

  return {
    kind: "choice",
    key,
    prefixWrapper,
    suffixWrapper,
    explicitDefault,
    hasExplicitDefault,
    isSaved,
    isRequired: !hasExplicitDefault && !hasNonEmptyWrappers,
    choices: decoded,
  };
}

/**
 * Brackets only opt into the new grammar when they occur where a key's choice
 * list can begin. Brackets in a legacy default or suffix remain ordinary text.
 * A balanced bracket in a prefix is also unambiguous because two wrapper
 * colons follow it, unless the wrapped key itself contains choice intent.
 */
function hasKeyChoiceIntent(body: string): boolean {
  const firstOpening = body.indexOf("[");
  const firstClosing = body.indexOf("]");
  const candidates = [firstOpening, firstClosing].filter((index) => index !== -1);
  if (candidates.length === 0) return false;

  const firstBracket = Math.min(...candidates);
  const beforeBracket = body.slice(0, firstBracket);
  if (beforeBracket.includes("|") || countCharacters(beforeBracket, ":") >= 2) return false;

  // `{{pre[fix]:key:suffix}}` is a legacy wrapper whose prefix happens to
  // contain balanced brackets, not a choice list attached to `pre`.
  if (firstOpening !== -1 && firstOpening === firstBracket) {
    const closing = findUnescapedClosingBracket(body, firstOpening + 1);
    if (closing !== -1) {
      const afterBracket = body.slice(closing + 1).trimStart();
      if (
        beforeBracket.indexOf(":") === -1 &&
        afterBracket.startsWith(":") &&
        countCharacters(afterBracket, ":") >= 2
      ) {
        const nextOpening = afterBracket.indexOf("[", 1);
        const nextClosing = afterBracket.indexOf("]", 1);
        const nextBrackets = [nextOpening, nextClosing].filter((index) => index !== -1);
        const nextColon = afterBracket.indexOf(":", 1);
        const keyBracket = nextBrackets.length > 0 ? Math.min(...nextBrackets) : -1;

        // A bracket before the second wrapper delimiter belongs to the key
        // segment. Let the choice parser diagnose the unsupported combination
        // instead of silently accepting a mangled legacy key.
        if (keyBracket !== -1 && (nextColon === -1 || keyBracket < nextColon)) return true;
        return false;
      }
    }
  }

  return true;
}

function findUnescapedClosingBracket(source: string, from: number): number {
  for (let index = from; index < source.length; index++) {
    if (source[index] === "\\") {
      index++;
    } else if (source[index] === "]") {
      return index;
    }
  }
  return -1;
}

function countCharacters(source: string, character: string): number {
  let count = 0;
  for (const current of source) {
    if (current === character) count++;
  }
  return count;
}

function invalidChoice(reason: string): ChoiceParseFailure {
  return { kind: "invalid-choice", reason };
}

function decodeChoices(source: string): string[] | string {
  const choices: string[] = [];
  let current = "";

  for (let index = 0; index < source.length; index++) {
    const character = source[index];
    if (character === "\\") {
      if (index + 1 >= source.length) {
        return "the choice list ends with a dangling escape; remove it or escape a supported character";
      }
      current += source[++index];
    } else if (character === "|") {
      choices.push(current.trim());
      current = "";
    } else {
      current += character;
    }
  }
  choices.push(current.trim());

  if (choices.some((choice) => choice.length === 0)) {
    return "choice values cannot be empty; remove the empty entry or enter a value";
  }
  if (choices.length < 2) {
    return "add at least two unique choices separated by `|`";
  }

  const seen = new Set<string>();
  for (const choice of choices) {
    if (seen.has(choice)) return `choice ${JSON.stringify(choice)} is duplicated; keep each choice unique`;
    seen.add(choice);
  }

  return choices;
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

function aggregatePlaceholders(text: string, occurrences: ParsedValuePlaceholderOccurrence[]): Placeholder[] {
  const placeholders: Placeholder[] = [];
  const indexByKey = new Map<string, number>();

  for (const occurrence of occurrences) {
    const existingIndex = indexByKey.get(occurrence.key);
    const placeholder = occurrenceToPlaceholder(occurrence);

    if (existingIndex === undefined) {
      indexByKey.set(occurrence.key, placeholders.length);
      placeholders.push(placeholder);
    } else if (occurrence.isChoiceDeclaration && placeholders[existingIndex].choices === undefined) {
      // A declaration owns field-level metadata even if a plain reference was
      // authored first. Keep the field's original ordering in the form.
      placeholders[existingIndex] = placeholder;
    }
  }

  // Preserve the established guard-only second pass and ordering.
  const ifRegex = /\{\{#if\s+(\+?)(\S+?)(?:\s+"([^"]*)")?\s*\}\}/g;
  let match: RegExpExecArray | null;
  while ((match = ifRegex.exec(text)) !== null) {
    const key = match[2].trim();
    if (!key || indexByKey.has(key)) continue;
    indexByKey.set(key, placeholders.length);
    placeholders.push({
      key,
      defaultValue: undefined,
      isRequired: false,
      isSaved: false,
      isGuardOnly: true,
      label: match[3],
      defaultOn: match[1] === "+",
    });
  }

  return placeholders;
}

function occurrenceToPlaceholder(occurrence: ParsedValuePlaceholderOccurrence): Placeholder {
  const placeholder: Placeholder = {
    key: occurrence.key,
    defaultValue: occurrence.explicitDefault,
    isRequired: occurrence.isRequired,
    isSaved: occurrence.isSaved,
    prefixWrapper: occurrence.prefixWrapper,
    suffixWrapper: occurrence.suffixWrapper,
  };
  if (occurrence.choices) placeholder.choices = [...occurrence.choices];
  return placeholder;
}

function findChoiceConflicts(occurrences: ParsedValuePlaceholderOccurrence[]): PlaceholderSyntaxDiagnostic[] {
  const byKey = new Map<string, ParsedValuePlaceholderOccurrence[]>();
  for (const occurrence of occurrences) {
    if (!occurrence.isChoiceDeclaration) continue;
    const declarations = byKey.get(occurrence.key) ?? [];
    declarations.push(occurrence);
    byKey.set(occurrence.key, declarations);
  }

  const diagnostics: PlaceholderSyntaxDiagnostic[] = [];
  for (const [key, declarations] of byKey) {
    if (declarations.length < 2) continue;
    const differingFields = getDifferingDeclarationFields(declarations);
    if (differingFields.length === 0) continue;

    for (const declaration of declarations) {
      diagnostics.push({
        range: declaration.range,
        expression: declaration.raw,
        message: `Conflicting authored choices for ${JSON.stringify(key)} in ${declaration.raw}: all declarations must use the same ${differingFields.join(", ")}.`,
      });
    }
  }
  return diagnostics;
}

function getDifferingDeclarationFields(declarations: ParsedValuePlaceholderOccurrence[]): string[] {
  const fields: string[] = [];
  if (!allEqual(declarations.map((item) => JSON.stringify(item.choices)))) fields.push("choices");
  if (!allEqual(declarations.map((item) => `${item.hasExplicitDefault}:${item.explicitDefault ?? ""}`))) {
    fields.push("default");
  }
  if (!allEqual(declarations.map((item) => item.isSaved))) fields.push("save policy");
  if (!allEqual(declarations.map((item) => item.isRequired))) fields.push("required/optional status");
  return fields;
}

function allEqual<T>(values: T[]): boolean {
  return values.every((value) => value === values[0]);
}
