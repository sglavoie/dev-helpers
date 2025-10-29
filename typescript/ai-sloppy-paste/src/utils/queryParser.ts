import { normalizeTag } from "./tags";

/**
 * Represents a parsed search query with extracted operators and filters
 */
export interface ParsedQuery {
  /** Tag filters - snippet must have all these tags (hierarchical matching) */
  tags: string[];
  /** Negative tag filters - snippet must NOT have these tags */
  notTags: string[];
  /** Boolean "is" filters - favorite, archived, untagged */
  is: string[];
  /** Boolean "not" filters - negated favorite, archived, untagged */
  not: string[];
  /** Exact phrase matches - must appear as substring in title or content */
  exactPhrases: string[];
  /** Remaining fuzzy text - all words must appear somewhere */
  fuzzyText: string;
  /** Quick flag to check if any operators are present */
  hasOperators: boolean;
}

/**
 * Valid values for is:/not: operators
 */
const VALID_BOOLEAN_OPERATORS = ["favorite", "archived", "untagged"] as const;

/**
 * Parses a search query string into structured operators and filters.
 *
 * Supported operators:
 * - tag:work          - Must have tag (supports hierarchy)
 * - not:tag:personal  - Must NOT have tag
 * - is:favorite       - Must be favorite/archived/untagged
 * - not:archived      - Must NOT be archived/favorite/untagged
 * - "exact phrase"    - Must contain exact phrase (case-insensitive)
 * - remaining text    - Fuzzy search (all words must match somewhere)
 *
 * Examples:
 * - "tag:work api"                    → Has work tag AND contains "api"
 * - "not:archived python"             → Not archived AND contains "python"
 * - "tag:work is:favorite rest"       → Work tag AND favorite AND contains "rest"
 * - '"meeting notes" urgent'          → Contains "meeting notes" exactly AND "urgent"
 *
 * Error handling: Invalid or malformed operators are silently treated as fuzzy text.
 *
 * @param query - The search query string from the user
 * @returns ParsedQuery object with extracted operators and filters
 */
export function parseSearchQuery(query: string): ParsedQuery {
  const result: ParsedQuery = {
    tags: [],
    notTags: [],
    is: [],
    not: [],
    exactPhrases: [],
    fuzzyText: "",
    hasOperators: false,
  };

  // Empty query - return empty result
  if (!query.trim()) {
    return result;
  }

  // Step 1: Extract quoted phrases for exact matching
  // Match content between double quotes
  const quoteRegex = /"([^"]+)"/g;
  let match;
  while ((match = quoteRegex.exec(query)) !== null) {
    const phrase = match[1].trim();
    if (phrase) {
      result.exactPhrases.push(phrase);
    }
  }

  // Remove quoted phrases from query for operator parsing
  let remainingQuery = query.replace(quoteRegex, "").trim();

  // Step 2: Parse operators from remaining text
  // Split by whitespace and process each token
  const tokens = remainingQuery.split(/\s+/);
  const fuzzyTokens: string[] = [];

  for (const token of tokens) {
    if (!token) continue;

    let consumed = false;

    // Check for not:tag:value (must check before tag: to avoid false match)
    if (token.startsWith("not:tag:")) {
      const tagValue = token.substring("not:tag:".length);
      if (tagValue) {
        result.notTags.push(normalizeTag(tagValue));
        consumed = true;
      }
    }
    // Check for tag:value
    else if (token.startsWith("tag:")) {
      const tagValue = token.substring("tag:".length);
      if (tagValue) {
        result.tags.push(normalizeTag(tagValue));
        consumed = true;
      }
    }
    // Check for not:value (boolean operators)
    else if (token.startsWith("not:")) {
      const notValue = token.substring("not:".length);
      if (VALID_BOOLEAN_OPERATORS.includes(notValue as any)) {
        result.not.push(notValue);
        consumed = true;
      }
    }
    // Check for is:value (boolean operators)
    else if (token.startsWith("is:")) {
      const isValue = token.substring("is:".length);
      if (VALID_BOOLEAN_OPERATORS.includes(isValue as any)) {
        result.is.push(isValue);
        consumed = true;
      }
    }

    // If token wasn't consumed by an operator, add to fuzzy search
    if (!consumed) {
      fuzzyTokens.push(token);
    }
  }

  // Step 3: Build fuzzy text from remaining tokens
  result.fuzzyText = fuzzyTokens.join(" ").trim();

  // Step 4: Set hasOperators flag
  // Include fuzzyText so that simple searches also trigger applySearchFilters
  result.hasOperators =
    result.tags.length > 0 ||
    result.notTags.length > 0 ||
    result.is.length > 0 ||
    result.not.length > 0 ||
    result.exactPhrases.length > 0 ||
    result.fuzzyText.length > 0;

  return result;
}

/**
 * Helper to check if a parsed query is empty (no operators and no text)
 */
export function isEmptyQuery(query: ParsedQuery): boolean {
  return !query.hasOperators && !query.fuzzyText;
}
