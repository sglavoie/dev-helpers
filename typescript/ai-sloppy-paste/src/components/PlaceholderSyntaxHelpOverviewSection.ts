import { Color, Icon } from "@raycast/api";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

export const placeholderSyntaxHelpOverviewSection: PlaceholderSyntaxHelpSectionData = {
  title: "Overview",
  items: [
    {
      icon: { source: Icon.Info, tintColor: Color.Blue },
      title: "Placeholder Syntax",
      subtitle: "Add dynamic fill-in fields to your snippets",
      markdown: `
# Placeholder Syntax

Placeholders let you add dynamic, fill-in fields to your snippets. When you paste a snippet containing placeholders, a form appears so you can fill in each value before the final text is inserted.

## Syntax Variants

| Syntax | Behaviour |
|--------|-----------|
| \`{{key}}\` | Required — must be filled in |
| \`{{key\\|default}}\` | Optional — pre-filled with default |
| \`{{prefix:key:suffix}}\` | Wrapper — prefix/suffix only appear when non-empty |
| \`{{!key}}\` | No-save — value not stored in history |
| \`{{#if key}}...{{/if}}\` | Block shown when key is non-empty |
| \`{{#if key}}...{{#else}}...{{/if}}\` | If/else block based on key value (\`{{/else}}\` closing tag optional) |
| \`{{#if key "label"}}...{{/if}}\` | Labeled checkbox — custom label instead of key name |
| \`{{#if +key}}...{{/if}}\` | Guard checkbox defaults to **checked** |
| Guard-only \`{{#if key}}\` | Key only in condition → renders as checkbox |

## System Placeholders (auto-filled)

\`{{DATE}}\`  \`{{TIME}}\`  \`{{DATETIME}}\`  \`{{TODAY}}\`  \`{{NOW}}\`  \`{{YEAR}}\`  \`{{MONTH}}\`  \`{{DAY}}\`

These are replaced automatically — no form field is shown for them.

Press Enter on any item below to see detailed information.
`,
    },
  ],
};
