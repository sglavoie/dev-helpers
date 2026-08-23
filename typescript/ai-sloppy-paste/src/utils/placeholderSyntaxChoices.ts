import type { ChoiceParseFailure, ChoiceParseResult } from "./placeholderSyntaxTypes";

const SUPPORTED_CHOICE_ESCAPES = new Set(["|", "[", "]", "\\"]);

export function parseChoiceExpression(trimmedContent: string): ChoiceParseResult {
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
