import { Color, Icon } from "@raycast/api";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

export const placeholderSyntaxHelpConditionalSection: PlaceholderSyntaxHelpSectionData = {
  title: "Conditional Blocks",
  items: [
    {
      icon: { source: Icon.CodeBlock, tintColor: Color.Blue },
      title: "{{#if key}}...{{/if}}",
      subtitle: "Block shown when key is non-empty",
      accessoryText: "Example: CC line only when cc is filled",
      markdown: `
# {{#if key}}...{{/if}}

Conditionally includes an entire block of content based on whether a placeholder value is non-empty. When the key's value is empty (or the key is absent), the entire block is omitted cleanly — no blank lines left behind.

## Behaviour

- Block is shown when \`key\` has a non-empty (non-whitespace) value
- Block is entirely omitted when \`key\` is empty or whitespace-only
- One leading and one trailing newline are consumed to avoid blank lines on removal
- The \`key\` can be any placeholder that also appears as \`{{key}}\` elsewhere in the snippet

## Example

\`\`\`
Hi {{name}}!
{{#if cc}}
CC: {{cc}}
{{/if}}
Sent {{DATE}}
\`\`\`

Fill \`cc="boss@co.com"\` → includes the CC line.
Leave \`cc\` empty → CC line is omitted entirely.

## When to Use

- Lines or paragraphs that only make sense when a field has a value
- Avoid the old pattern of needing two separate snippets for "with X" vs "without X"
`,
      copyContent: `Hi {{name}}!
{{#if cc}}
CC: {{cc}}
{{/if}}
Sent {{DATE}}`,
    },
    {
      icon: { source: Icon.CodeBlock, tintColor: Color.Green },
      title: "{{#if key}}...{{#else}}...{{/if}}",
      subtitle: "If/else block — different content based on key value",
      accessoryText: "Example: HIGH vs NORMAL priority",
      markdown: `
# {{#if key}}...{{#else}}...{{/if}}

Shows one block of content when a key is non-empty, and a different block when the key is empty. The \`{{#else}}\` branch is optional — omit it for an if-only block.

## Behaviour

- \`{{#if key}}\` branch shown when \`key\` is non-empty
- \`{{#else}}\` branch shown when \`key\` is empty or absent
- Both branches consume surrounding newlines to keep output clean
- \`{{/else}}\` closing tag is optional — use it for readability if you prefer explicit block endings
- The same key can appear in multiple \`{{#if}}\` blocks — they all reference the same value

## Syntax Variants

\`\`\`
{{#if key}}yes{{#else}}no{{/if}}
{{#if key}}yes{{#else}}no{{/else}}{{/if}}
\`\`\`

Both forms are equivalent. Use \`{{/else}}\` when you want each block to have a clear closing tag.

## Example

\`\`\`
{{#if priority}}
Priority: {{priority}}
{{#else}}
Priority: NORMAL
{{/if}}
\`\`\`

Fill \`priority="HIGH"\` → "Priority: HIGH"
Leave \`priority\` empty → "Priority: NORMAL"

## Repeated Variable Example

The same guard key can control multiple blocks:

\`\`\`
{{#if +loop}}/loop {{!duration|5}}m {{/if}}Commit {{#if loop}}each round{{#else}}once{{/else}}{{/if}}
\`\`\`

Check \`loop\` → "/loop 5m Commit each round"
Uncheck \`loop\` → "Commit once"

## When to Use

- Alternate phrasing depending on whether a value is provided
- Conditional greetings, subject lines, or closing paragraphs
- Any situation requiring a fallback block of text
`,
      copyContent: `{{#if priority}}
Priority: {{priority}}
{{#else}}
Priority: NORMAL
{{/if}}`,
    },
    {
      icon: { source: Icon.Checkmark, tintColor: Color.Purple },
      title: "Guard-only {{#if key}} (checkbox)",
      subtitle: "Key only in condition — renders as a checkbox in the form",
      accessoryText: "Example: {{#if include_signature}}",
      markdown: `
# Guard-only Conditional Keys

When a key appears only inside \`{{#if key}}\` and never as \`{{key}}\` elsewhere in the snippet, it becomes a **guard-only** key. Guard-only keys render as a checkbox in the fill-in form rather than a text field.

## Behaviour

- Checkbox unchecked (default) → block is omitted
- Checkbox checked → block is included in output
- Use \`+\` prefix (\`{{#if +key}}\`) to default the checkbox to **checked**
- No text value is substituted — the key is purely a visibility toggle
- Guard-only keys are not saved to placeholder history

## Example

\`\`\`
Dear {{name}},
{{#if include_signature}}
Best regards,
The Team
{{/if}}
\`\`\`

The form shows a text field for \`name\` and a checkbox for \`include_signature\`.
Check the box → signature appears. Uncheck → no blank line, clean output.

## Labeled Checkbox

Add a quoted label after the key to customise the checkbox text:

\`\`\`
{{#if include_signature "Include signature block"}}
Best regards,
The Team
{{/if}}
\`\`\`

The form shows the checkbox labeled "Include signature block" instead of the default "Include include_signature?".

## Default-On Checkbox

Add a \`+\` before the key to default the checkbox to checked:

\`\`\`
{{#if +include_signature}}
Best regards,
The Team
{{/if}}
\`\`\`

The checkbox starts checked — the block is included by default. Uncheck to remove it.

This can be combined with a label:

\`\`\`
{{#if +include_signature "Include signature block"}}
Best regards,
The Team
{{/if}}
\`\`\`

## When to Use

- Optional sections like signatures, disclaimers, or boilerplate
- Any block you want to toggle on/off without typing a value
- Cleaner than creating two separate snippets for "with/without" variants
`,
      copyContent: `Dear {{name}},
{{#if include_signature}}
Best regards,
The Team
{{/if}}`,
    },
    {
      icon: { source: Icon.CodeBlock, tintColor: Color.Orange },
      title: "Nested {{#if}} blocks",
      subtitle: "Conditional blocks can be nested inside each other",
      accessoryText: "Example: formal + first_contact",
      markdown: `
# Nested {{#if}} Blocks

Conditional blocks can be nested inside each other to express compound logic — for example, "if formal, include greeting; and if first contact, also include introduction."

Blocks are resolved inside-out (innermost first), up to 10 levels deep.

## Example

\`\`\`
{{#if formal}}
Dear {{name}},
{{#if first_contact}}
Allow me to introduce myself.
{{/if}}
{{#else}}
Hey {{name}}!
{{/if}}
\`\`\`

- \`formal=yes\`, \`first_contact=yes\` → formal greeting + introduction
- \`formal=yes\`, \`first_contact=""\` → formal greeting only
- \`formal=""\` → casual greeting, inner block never evaluated

## When to Use

- Multi-condition logic that would otherwise require separate snippets
- Layered optional sections (e.g. salutation + opener + closing)
`,
      copyContent: `{{#if formal}}
Dear {{name}},
{{#if first_contact}}
Allow me to introduce myself.
{{/if}}
{{#else}}
Hey {{name}}!
{{/if}}`,
    },
  ],
};
