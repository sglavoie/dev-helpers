import { Color, Icon } from "@raycast/api";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

export const placeholderSyntaxHelpSystemSection: PlaceholderSyntaxHelpSectionData = {
  title: "System Placeholders",
  items: [
    {
      icon: { source: Icon.Clock, tintColor: Color.Yellow },
      title: "{{DATE}}, {{TIME}}, {{DATETIME}}, …",
      subtitle: "Auto-filled — no user input required",
      accessoryText: "Replaced at paste time",
      markdown: `
# System Placeholders

System placeholders are replaced automatically when you paste the snippet. No form field is shown for them — they just work.

## Available System Placeholders

| Placeholder | Example output |
|-------------|----------------|
| \`{{DATE}}\` | 2025-10-30 |
| \`{{TIME}}\` | 14:35:22 |
| \`{{DATETIME}}\` | 2025-10-30 14:35:22 |
| \`{{TODAY}}\` | 2025-10-30 |
| \`{{NOW}}\` | 2025-10-30 14:35:22 |
| \`{{YEAR}}\` | 2025 |
| \`{{MONTH}}\` | 10 |
| \`{{DAY}}\` | 30 |

## Examples

\`\`\`
Meeting notes — {{DATE}}
\`\`\`
→ "Meeting notes — 2025-10-30"

\`\`\`
Generated at {{TIME}} on {{DATE}}
\`\`\`
→ "Generated at 14:35:22 on 2025-10-30"

\`\`\`
Invoice #INV-{{YEAR}}{{MONTH}}{{DAY}}-{{id}}
\`\`\`
→ "Invoice #INV-20251030-42" (with user filling in \`id\`)

## When to Use

- Any snippet that benefits from an automatic timestamp
- Log entries, reports, meeting notes, invoices
- Combine freely with regular placeholders in the same snippet
`,
      copyContent: "Meeting notes — {{DATE}}",
    },
  ],
};
