import { Placeholder } from "../types";
import { extractConditionalBlockBodies } from "./placeholders";

const MAX_BODY_PREVIEW = 80;

function normalizeBody(body: string, truncate: boolean): string {
  const collapsed = body.replace(/\s+/g, " ").trim();
  if (!truncate) return collapsed;
  return collapsed.length > MAX_BODY_PREVIEW ? collapsed.slice(0, MAX_BODY_PREVIEW) + "…" : collapsed;
}

export function buildFieldPreview(
  placeholder: Placeholder,
  snippetContent: string,
  currentValue: string,
  options: { truncate?: boolean } = {},
): string | undefined {
  const { truncate = true } = options;
  if (placeholder.isRequired) return undefined;

  if (placeholder.isGuardOnly) {
    const blocks = extractConditionalBlockBodies(snippetContent, placeholder.key);
    if (blocks.length === 0) return undefined;
    const { ifBody, elseBody } = blocks[0];
    const ifText = normalizeBody(ifBody, truncate);
    if (elseBody !== undefined) {
      return `Toggles: "${ifText}" • Else: "${normalizeBody(elseBody, truncate)}"`;
    }
    return `Toggles: "${ifText}"`;
  }

  if (placeholder.prefixWrapper !== undefined || placeholder.suffixWrapper !== undefined) {
    const displayValue = currentValue.trim() || "<value>";
    return `Wraps as: ${placeholder.prefixWrapper ?? ""}${displayValue}${placeholder.suffixWrapper ?? ""}`;
  }

  if (placeholder.defaultValue === undefined) {
    return "Optional — leave blank to omit.";
  }
  if (placeholder.defaultValue === "") {
    return "Empty → renders as empty string.";
  }
  return `Empty → uses default: "${placeholder.defaultValue}"`;
}
