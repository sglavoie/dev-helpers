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

export interface ChoiceParseSuccess {
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

export interface ChoiceParseFailure {
  kind: "invalid-choice";
  reason: string;
}

export interface NotChoiceSyntax {
  kind: "legacy";
}

export type ChoiceParseResult = ChoiceParseSuccess | ChoiceParseFailure | NotChoiceSyntax;

export interface ScannedExpression {
  raw: string;
  range: PlaceholderSourceRange;
  content: string;
}
