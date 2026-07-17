import { Color, Icon, type ColorLike } from "@raycast/api";

type PlaceholderSyntaxHelpIcon = {
  source: Icon;
  tintColor: ColorLike;
};

export type PlaceholderSyntaxHelpItemData = {
  icon: PlaceholderSyntaxHelpIcon;
  title: string;
  subtitle: string;
  accessoryText?: string;
  markdown: string;
  copyActionTitle?: string;
  copyContent?: string;
};

export type PlaceholderSyntaxHelpSectionData = {
  title: string;
  items: PlaceholderSyntaxHelpItemData[];
};

export const placeholderSyntaxHelpSections: PlaceholderSyntaxHelpSectionData[] = [
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
];
