import { Color, Icon } from "@raycast/api";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

export const placeholderSyntaxHelpAdvancedSection: PlaceholderSyntaxHelpSectionData = {
  title: "Advanced Placeholders",
  items: [
    {
      icon: { source: Icon.CodeBlock, tintColor: Color.Purple },
      title: "{{prefix:key:suffix}}",
      subtitle: "Wrapper — surrounding text only appears when value is non-empty",
      accessoryText: "Example: {{$:price: USD}}",
      markdown: `
# {{prefix:key:suffix}}

Wrapper text that only appears when the placeholder value is non-empty. Leaving the field empty omits both the value and its surrounding text entirely.

## Syntax

Three colon-separated parts: \`prefix:key:suffix\`

- **prefix** — text placed immediately before the value (can be empty)
- **key** — the placeholder name
- **suffix** — text placed immediately after the value (can be empty)

Use \`{{:key:}}\` for a plain optional field with no wrapping text.

## Behaviour

- Wrappers only render when value is non-empty
- Empty or whitespace-only value → no output at all (value and wrappers are omitted)
- Avoids awkward partial phrases like "Order #" when no ID was given

## Examples

\`\`\`
Order {{#:id:}}
\`\`\`
Enter "12345" → "Order #12345"
Leave empty → "Order "

\`\`\`
Price {{$:amount: USD}}
\`\`\`
Enter "25.50" → "Price $25.50 USD"
Leave empty → "Price "

\`\`\`
Saved{{to :location:}}
\`\`\`
Enter "/docs" → "Saved to /docs"
Leave empty → "Saved"  ← no trailing space!

## Combining with a Default

\`\`\`
File saved{{to :location:|current directory}}
\`\`\`
Enter "/docs" → "File saved to /docs"
Leave default → "File saved to current directory"

## When to Use

- Formatting symbols ($, #, %) that should only appear with a value
- Units or qualifiers (USD, px, kg) that depend on a non-empty value
- Natural language phrases that are grammatically correct only when the value is present
`,
      copyContent: "Price {{$:amount: USD}}",
    },
    {
      icon: { source: Icon.XMarkCircle, tintColor: Color.Orange },
      title: "{{!key}}",
      subtitle: "No-save — value is not stored in placeholder history",
      accessoryText: "Example: {{!date}}",
      markdown: `
# {{!key}}

The \`!\` flag prevents the entered value from being saved to placeholder history. The placeholder works exactly like a normal required field — it just won't appear in autocomplete suggestions.

## Behaviour

- Value is NOT saved to placeholder history
- No autocomplete suggestions appear for this field
- The snippet still works normally; only persistence is affected
- Can be combined with wrappers and defaults (see Combined Example)

## Examples

\`\`\`
Event on {{!date}}
\`\`\`
Date is required but one-off dates won't pollute autocomplete history.

\`\`\`
Timestamp: {{!timestamp|now}}
\`\`\`
Optional ephemeral value with a default.

\`\`\`
Reference {{#:!temp_id:}}
\`\`\`
Temporary ID with a prefix wrapper, not saved.

## When to Use

- Dates, timestamps, and temporary IDs that change every time
- Sensitive information that should never persist
- Any value you'll never want as an autocomplete suggestion
`,
      copyContent: "Event on {{!date}}",
    },
    {
      icon: { source: Icon.Layers, tintColor: Color.Blue },
      title: "{{!$:price: USD|0.00}}",
      subtitle: "Combined: no-save + wrappers + default",
      accessoryText: "All features together",
      markdown: `
# Combined Features Example

\`\`\`
{{!$:price: USD|0.00}}
\`\`\`

This single placeholder uses all four features together:

1. **!** — No-save: value is not stored in history
2. **$:** — Prefix wrapper: "$" appears before the value
3. **price** — The placeholder key (field label in the form)
4. **: USD** — Suffix wrapper: " USD" appears after the value
5. **|0.00** — Default value: "0.00" if left unchanged

## Behaviour

**User enters "25.50":** → "$25.50 USD" (not saved to history)

**User leaves default:** → "$0.00 USD" (uses default, not saved)

**User clears the field:** → "" (empty — no wrappers applied)

## Syntax Order

Always: \`{{!prefix:key:suffix|default}}\`

- \`!\` must come first (if present)
- \`prefix:key:suffix\` in the middle (colon-separated)
- \`|default\` at the end (if present)

## Real-World Usage

\`\`\`
Report for {{!:date:}} — Total: {{$:amount: USD}}
\`\`\`

Input: date="2025-10-30", amount="1500"
Output: "Report for 2025-10-30 — Total: $1500 USD"

- \`date\`: ephemeral one-off value, not saved to history
- \`amount\`: saved to history for reuse, shown with currency wrappers
`,
      copyContent: "Price: {{!$:amount: USD|0.00}}",
    },
  ],
};
