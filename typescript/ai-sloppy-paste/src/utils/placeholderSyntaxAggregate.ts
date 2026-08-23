import { Placeholder } from "../types";
import type { ParsedValuePlaceholderOccurrence, PlaceholderSyntaxDiagnostic } from "./placeholderSyntaxTypes";

export function aggregatePlaceholders(text: string, occurrences: ParsedValuePlaceholderOccurrence[]): Placeholder[] {
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

export function findChoiceConflicts(occurrences: ParsedValuePlaceholderOccurrence[]): PlaceholderSyntaxDiagnostic[] {
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
