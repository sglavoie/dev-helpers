import { Color, Icon } from "@raycast/api";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

export const placeholderSyntaxHelpChoicesSection: PlaceholderSyntaxHelpSectionData = {
  title: "Authored Choices",
  items: [
    {
      icon: { source: Icon.CodeBlock, tintColor: Color.Blue },
      title: "{{key[choice1|choice2]|default}}",
      subtitle: "Single-select dropdown with an always-available Custom option",
      accessoryText: "Example: {{tone[Formal|Casual]|Casual}}",
      markdown: `
# Authored Choice Dropdowns

Add a literal bracketed list immediately after the key to define a stable single-select dropdown.

## Exact Grammar

\`\`\`
{{[!]prefix:key[choice1|choice2]:suffix[|default]}}
\`\`\`

In that grammar, \`[!]\` and \`[|default]\` denote optional parts. The brackets around \`choice1|choice2\` are literal. Without wrappers, use:

\`\`\`
{{tone[Formal|Casual|Technical]}}
{{tone[Formal|Casual|Technical]|Casual}}
\`\`\`

Choice boundaries are trimmed, authored order is preserved, and at least two unique, non-empty choices are required.

## Initialization Precedence

1. An explicit trailing default wins. If it matches a choice, that choice starts selected.
2. A non-empty default outside the list starts in **Custom** with that value.
3. An empty explicit default starts in **Custom** with an empty value.
4. Without an explicit default, a normal choice placeholder starts on the first authored choice.

The dropdown always contains the authored values in their configured order plus **Custom**.

## Example

\`\`\`
Write a {{tone[Formal|Casual|Technical]|Casual}} reply.
\`\`\`

The field starts on \`Casual\`. Choose **Custom** to type any other value.
`,
      copyContent: "{{tone[Formal|Casual|Technical]|Casual}}",
    },
    {
      icon: { source: Icon.Layers, tintColor: Color.Green },
      title: "Choices, Custom, and repeated keys",
      subtitle: "One declaration controls every occurrence of the same key",
      accessoryText: "{{tone[Formal|Casual]}} … {{tone}}",
      markdown: `
# Choices, Custom Values, and Repeated Keys

A choice-bearing declaration defines the field-level choices, default, save policy, and required/optional behavior for its key. Plain occurrences of the same key share the selected value, even if they appear before the declaration.

## End-to-End Example

\`\`\`
{{tone[Formal|Casual|Technical]|Casual}} opening — finish in {{tone}} tone.
\`\`\`

- Starts on \`Casual\`
- Selecting **Custom** and entering \`Direct\` replaces both occurrences with \`Direct\`
- Source order does not matter: a plain \`{{tone}}\` may appear before the declaration

## History Policy

- Normal insertion ignores saved history when initializing or populating an authored-choice dropdown
- The dropdown contains exactly the authored choices plus **Custom**
- The explicit **Paste with Last Values** action may restore a previous value
- Submitted authored and custom values are recorded unless \`!\` disables saving
- Recorded custom values never become authored dropdown options

When the same key has multiple choice-bearing declarations, their choices, default, save policy, and required/optional status must agree. Their occurrence-specific prefix and suffix wrappers may differ.
`,
      copyContent: "{{tone[Formal|Casual|Technical]|Casual}} … {{tone}}",
    },
    {
      icon: { source: Icon.ArrowNe, tintColor: Color.Orange },
      title: "Wrappers, escaping, and validation",
      subtitle: "Compose choices safely with wrappers, no-save, and escaped literals",
      accessoryText: "{{!$:amount[10|20]: USD|10}}",
      markdown: `
# Wrappers, Escaping, and Validation

## Wrapper Enablement

\`\`\`
{{!$:amount[10|20]: USD|10}}
\`\`\`

This starts included as \`$10 USD\`; \`!\` prevents the submitted value from being saved.

\`\`\`
{{$:amount[10|20]: USD}}
\`\`\`

A wrapper choice without a default starts unchecked. If enabled, it uses the first authored choice (\`10\`). Only a non-empty wrapper default starts included.

## Escaping Inside Choice Lists

Use these four supported escapes inside the brackets:

- \`\\|\` for a literal pipe
- \`\\[\` and \`\\]\` for literal brackets
- \`\\\\\` for a literal backslash

\`\`\`
{{value[A\\|B|\\[bracket\\]|path\\\\name]}}
\`\`\`

The decoded choices are \`A|B\`, \`[bracket]\`, and \`path\\name\`.

## Validation and Conflicts

The create/edit form rejects empty entries, duplicate choices, fewer than two choices, unmatched or nested brackets, unsupported or dangling escapes, and conflicting declarations.

\`\`\`
{{tone[A||B]}}                 invalid: empty entry
{{tone[A|A]}}                  invalid: duplicate
{{tone[A|B]}} {{tone[A|C]}}    invalid: conflicting choices
\`\`\`

Malformed imported or clipboard-saved content remains safe to run through the legacy fallback, but must be corrected before the editor can save it.
`,
      copyContent: "{{!$:amount[10|20]: USD|10}}",
    },
  ],
};
