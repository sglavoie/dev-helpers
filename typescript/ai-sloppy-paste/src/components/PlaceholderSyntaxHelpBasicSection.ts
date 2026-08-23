import { Color, Icon } from "@raycast/api";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

export const placeholderSyntaxHelpBasicSection: PlaceholderSyntaxHelpSectionData = {
  title: "Basic Placeholders",
  items: [
    {
      icon: { source: Icon.CodeBlock, tintColor: Color.Red },
      title: "{{key}}",
      subtitle: "Required — must be filled in before pasting",
      accessoryText: "Example: Hello {{name}}",
      markdown: `
# {{key}}

A required placeholder that must be filled in. The snippet cannot be pasted until this field has a value.

## Behaviour

- Shown as a required text field in the fill-in form
- Value is saved to history for autocomplete on future uses
- Snippet cannot be copied without filling this placeholder

## Examples

\`\`\`
Hello {{name}}!
\`\`\`
User enters "Alice" → "Hello Alice!"

\`\`\`
Your order {{order_id}} is ready.
\`\`\`
User enters "12345" → "Your order 12345 is ready."

## When to Use

- Any value that must always be provided (names, IDs, required parameters)
- Values you want saved for quick reuse via autocomplete
`,
      copyContent: "Hello {{name}}",
    },
    {
      icon: { source: Icon.CodeBlock, tintColor: Color.Green },
      title: "{{key|default}}",
      subtitle: "Optional — pre-filled with a default value",
      accessoryText: "Example: {{name|Guest}}",
      markdown: `
# {{key|default}}

An optional placeholder that comes pre-filled with a default value. The user can accept the default or type a new value.

## Behaviour

- Shown as an optional text field, pre-filled with the default
- User can override or leave as-is
- Value is saved to history for autocomplete
- Snippet can be pasted without changing the default

## Examples

\`\`\`
Hello {{name|Guest}}!
\`\`\`
Leave default → "Hello Guest!"
Enter "Alice" → "Hello Alice!"

\`\`\`
Amount: {{amount|0.00}}
\`\`\`
Leave default → "Amount: 0.00"
Enter "25.50" → "Amount: 25.50"

## Empty-string Default

\`\`\`
Note{{context|}}
\`\`\`
The \`|}\` syntax makes the field truly optional with no default text — leaving it empty simply omits any value.

## When to Use

- Optional parameters that have a sensible fallback
- Fields the user will often leave unchanged
`,
      copyContent: "Hello {{name|Guest}}",
    },
  ],
};
