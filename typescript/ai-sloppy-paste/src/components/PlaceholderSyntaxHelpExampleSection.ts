import { Color, Icon } from "@raycast/api";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

export const placeholderSyntaxHelpExampleSection: PlaceholderSyntaxHelpSectionData = {
  title: "Complete Example",
  items: [
    {
      icon: { source: Icon.Layers, tintColor: Color.Green },
      title: "Hi {{name}}, order {{#:order_id:}} ready",
      subtitle: "All placeholder types in one snippet",
      accessoryText: "Real-world combined usage",
      markdown: `
# Complete Example Snippet

\`\`\`
Hi {{name}}, your order {{#:order_id:}} is ready.
Amount: {{$:price: USD|0.00}}
Notes: {{notes|No notes}}
Ref: {{!ref}}
Generated: {{DATE}}
\`\`\`

## Placeholder Breakdown

| Field | Type | Behaviour |
|-------|------|-----------|
| \`{{name}}\` | Required | Must be filled in; saved to history |
| \`{{#:order_id:}}\` | Optional wrapper | If left empty, "#" prefix is also omitted |
| \`{{$:price: USD\\|0.00}}\` | Optional wrapper + default | Shows "$0.00 USD" if left as default |
| \`{{notes\\|No notes}}\` | Optional with default | Plain text, default "No notes" |
| \`{{!ref}}\` | Required, no-save | Must be filled but not stored in history |
| \`{{DATE}}\` | System | Auto-replaced with today's date |

## Sample Output

Input: name="Alice", order_id="12345", price="99.99", notes="" (cleared), ref="XYZ-001"

\`\`\`
Hi Alice, your order #12345 is ready.
Amount: $99.99 USD
Notes:
Ref: XYZ-001
Generated: 2025-10-30
\`\`\`
`,
      copyActionTitle: "Copy Example Snippet",
      copyContent: `Hi {{name}}, your order {{#:order_id:}} is ready.
Amount: {{$:price: USD|0.00}}
Notes: {{notes|No notes}}
Ref: {{!ref}}
Generated: {{DATE}}`,
    },
  ],
};
