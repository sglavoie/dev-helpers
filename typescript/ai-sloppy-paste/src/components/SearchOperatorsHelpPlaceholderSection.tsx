import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpPlaceholderSection({
  isShowingDetail,
  onToggleDetail,
}: SearchOperatorsHelpSectionProps) {
  return (
    <List.Section title="Placeholder Syntax">
      <List.Item
        icon={{ source: Icon.CodeBlock, tintColor: Color.Blue }}
        title="{{key}}"
        subtitle={isShowingDetail ? undefined : "Required placeholder"}
        accessories={isShowingDetail ? undefined : [{ text: "Example: Hello {{name}}" }]}
        detail={
          <List.Item.Detail
            markdown={`
# {{key}}

Basic required placeholder that must be filled in.

## Behavior

- User is prompted to enter a value
- Value is saved to history for autocomplete
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

## Use Cases

- User names, IDs, required parameters
- Values that must always be provided
- Values you want saved for quick reuse
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="Hello {{name}}" />
          </ActionPanel>
        }
      />

      <List.Item
        icon={{ source: Icon.CodeBlock, tintColor: Color.Green }}
        title="{{key|default}}"
        subtitle={isShowingDetail ? undefined : "Optional with default value"}
        accessories={isShowingDetail ? undefined : [{ text: "Example: {{name|Guest}}" }]}
        detail={
          <List.Item.Detail
            markdown={`
# {{key|default}}

Optional placeholder with a default value that can be overridden.

## Behavior

- Pre-filled with default value in form
- User can override or leave as-is
- Value is saved to history for autocomplete
- Snippet can be copied without changing the default

## Examples

\`\`\`
Hello {{name|Guest}}!
\`\`\`
User leaves default → "Hello Guest!"
User enters "Alice" → "Hello Alice!"

\`\`\`
Price: {{amount|0.00}}
\`\`\`
User enters "25.50" → "Price: 25.50"
User leaves default → "Price: 0.00"

## Empty String Default

\`\`\`
Message{{context|}}
\`\`\`
Empty default with \`|}\` syntax makes placeholder optional with no default text.

## Use Cases

- Optional parameters with sensible defaults
- Fallback values
- Context that may or may not be needed
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="Hello {{name|Guest}}" />
          </ActionPanel>
        }
      />

      <List.Item
        icon={{ source: Icon.CodeBlock, tintColor: Color.Purple }}
        title="{{prefix:key:suffix}}"
        subtitle={isShowingDetail ? undefined : "Conditional wrapper text"}
        accessories={isShowingDetail ? undefined : [{ text: "Example: {{$:price: USD}}" }]}
        detail={
          <List.Item.Detail
            markdown={`
# {{prefix:key:suffix}}

Wrapper text that only appears when placeholder value is non-empty.

## Syntax

- **prefix:key:suffix** - Three parts separated by colons
- Middle part is always the key
- First part is prefix (shown before value)
- Third part is suffix (shown after value)
- Use empty parts to skip: \`{{:key:}}\` = no wrappers

## Behavior

- Wrappers only render when value is non-empty
- Empty or whitespace-only values → no wrappers
- Enables natural language flow without awkward text

## Examples

\`\`\`
Order {{#:id:}}
\`\`\`
User enters "12345" → "Order #12345"
User leaves empty → "Order "

\`\`\`
Price {{$:amount: USD}}
\`\`\`
User enters "25.50" → "Price $25.50 USD"
User leaves empty → "Price "

\`\`\`
Message{{with :context:}}
\`\`\`
User enters "urgent" → "Message with urgent"
User leaves empty → "Message"

This avoids "Message with " when context is empty.

\`\`\`
File saved{{to :location:|current directory}}
\`\`\`
User enters "/docs" → "File saved to /docs"
User leaves default → "File saved to current directory"

## Use Cases

- Natural language where text depends on context
- Formatting symbols (# $ % etc.)
- Units or qualifiers that only apply to non-empty values
- Avoiding awkward partial phrases
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="Price {{$:amount: USD}}" />
          </ActionPanel>
        }
      />

      <List.Item
        icon={{ source: Icon.XMarkCircle, tintColor: Color.Red }}
        title="{{!key}}"
        subtitle={isShowingDetail ? undefined : "No-save flag (value not saved to history)"}
        accessories={isShowingDetail ? undefined : [{ text: "Example: {{!date}}" }]}
        detail={
          <List.Item.Detail
            markdown={`
# {{!key}}

No-save flag prevents placeholder value from being saved to history.

## Behavior

- Value is NOT saved to placeholder history
- No autocomplete suggestions for this placeholder
- Useful for ephemeral or one-off values
- Can be combined with wrappers and defaults

## Examples

\`\`\`
Event on {{!date}}
\`\`\`
Date is required but won't clutter history with one-time values.

\`\`\`
Timestamp: {{!timestamp|now}}
\`\`\`
Optional ephemeral value with default.

\`\`\`
Reference {{#:!temp_id:}}
\`\`\`
Temporary ID with prefix wrapper, not saved.

## Use Cases

- Dates, timestamps, temporary IDs
- One-off values that won't be reused
- Sensitive information that shouldn't persist
- Preventing history pollution from unique values

## Note

The \`!\` flag ONLY affects history saving. The placeholder still functions normally - it's just not remembered for autocomplete.
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="Event on {{!date}}" />
          </ActionPanel>
        }
      />

      <List.Item
        icon={{ source: Icon.Layers, tintColor: Color.Orange }}
        title="{{!$:price: USD|0.00}}"
        subtitle={isShowingDetail ? undefined : "Combined: no-save + wrappers + default"}
        accessories={isShowingDetail ? undefined : [{ text: "All features together" }]}
        detail={
          <List.Item.Detail
            markdown={`
# Combined Features Example

\`\`\`
{{!$:price: USD|0.00}}
\`\`\`

## What This Does

Combines all enhanced placeholder features:

1. **!** - No-save flag: value NOT saved to history
2. **$:** - Prefix wrapper: "$" appears before value
3. **price** - The placeholder key
4. **: USD** - Suffix wrapper: " USD" appears after value
5. **|0.00** - Default value: "0.00" if user doesn't provide value

## Behavior Examples

**User enters "25.50":**
→ "$25.50 USD"
→ NOT saved to history

**User leaves default:**
→ "$0.00 USD"
→ Uses default, NOT saved to history

**User enters empty string:**
→ "" (empty)
→ No wrappers applied ($ and USD don't appear)

## Real-World Usage

\`\`\`
Report for {{!:date:}} - Total: {{$:amount: USD}}
\`\`\`

**User input:** date="2025-10-30", amount="1500"
**Output:** "Report for 2025-10-30 - Total: $1500 USD"

- date: ephemeral, not saved
- amount: saved to history, has wrappers

## Syntax Order

Always: \`{{!prefix:key:suffix|default}}\`

- \`!\` must be first (if present)
- \`prefix:key:suffix\` in middle (colon-separated)
- \`|default\` at end (if present)

## When to Use

- Temporary values with formatting needs
- One-off prices, quantities, IDs
- Sensitive data that needs structure
- Combining multiple features for complex use cases
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="Price: {{!$:amount: USD|0.00}}" />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
