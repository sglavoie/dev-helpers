import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import { useState } from "react";

/**
 * Help view displaying all available search operators with examples
 */
export function SearchOperatorsHelp() {
  const [isShowingDetail, setIsShowingDetail] = useState(false);

  return (
    <List
      navigationTitle="Search Operators Help"
      searchBarPlaceholder="Search help..."
      isShowingDetail={isShowingDetail}
    >
      <List.Section title="Overview">
        <List.Item
          icon={{ source: Icon.Info, tintColor: Color.Blue }}
          title="Operator-based Search"
          subtitle={isShowingDetail ? undefined : "Use operators to precisely filter snippets"}
          detail={
            <List.Item.Detail
              markdown={`
# Operator-based Search

Use operators to precisely filter your snippets. When operators are present in your search query, they **override** the UI filters (tag dropdown, favorites toggle, archived toggle).

## Key Features

- **Precise Filtering**: Find exactly what you need with powerful operators
- **Combinable**: Use multiple operators together for complex queries
- **Override UI**: Operators take precedence over UI filter controls
- **Graceful**: Invalid operators are treated as fuzzy text - search never breaks

## How It Works

1. Type operators in the search bar
2. Operators are parsed and applied as filters
3. UI filters are ignored when operators are present
4. Remaining text is matched using fuzzy search

## All Conditions Use AND Logic

When you use multiple operators, **all conditions must be met**:
- \`tag:work tag:client\` → Must have BOTH work AND client tags
- \`is:favorite not:archived\` → Must be favorite AND not archived
- \`tag:work "api docs" rest\` → Must have work tag AND contain "api docs" exactly AND match "rest" fuzzy

Press Enter on any operator below to see detailed information.
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
            </ActionPanel>
          }
        />
      </List.Section>

      <List.Section title="Tag Operators">
        <List.Item
          icon={{ source: Icon.Tag, tintColor: Color.Green }}
          title="tag:work"
          subtitle={isShowingDetail ? undefined : "Must have the specified tag (supports hierarchy)"}
          accessories={isShowingDetail ? undefined : [{ text: "Example: tag:work api" }]}
          detail={
            <List.Item.Detail
              markdown={`
# tag:work

Filter snippets that have a specific tag. Supports **hierarchical matching**.

## Behavior

- Matches snippets with the exact tag
- **Also matches child tags**: \`tag:work\` matches \`work\`, \`work/projects\`, \`work/client/alpha\`, etc.
- Case-insensitive: \`tag:Work\` and \`tag:work\` are identical
- Can use multiple tag filters (AND logic)

## Examples

\`\`\`
tag:work
\`\`\`
Shows all snippets tagged with "work" or any child of work

\`\`\`
tag:work api
\`\`\`
Shows snippets with work tag that also contain "api" (in title, content, or tags)

\`\`\`
tag:work tag:client
\`\`\`
Shows snippets that have BOTH work and client tags (AND logic)

## Use Cases

- Filter to a specific project or category
- Combine with other operators for precise searches
- Take advantage of tag hierarchy to search broadly or narrowly
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="tag:work api" />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.Tag, tintColor: Color.Green }}
          title="tag:work/projects"
          subtitle={isShowingDetail ? undefined : "Hierarchical tag matching"}
          accessories={isShowingDetail ? undefined : [{ text: "Matches nested tags" }]}
          detail={
            <List.Item.Detail
              markdown={`
# tag:work/projects

Filter using hierarchical (nested) tags with forward slash notation.

## Behavior

- Matches exact hierarchical tag
- Matches all children: \`tag:work/projects\` matches \`work/projects/alpha\`, \`work/projects/beta\`, etc.
- Follows same hierarchy rules as simple tags
- Can be combined with other tag filters

## Examples

\`\`\`
tag:work/projects
\`\`\`
Shows snippets tagged with "work/projects" or any deeper child

\`\`\`
tag:work/projects api
\`\`\`
Shows work/projects snippets containing "api"

\`\`\`
tag:work/projects tag:client
\`\`\`
Must have work/projects AND client tags

## Tag Hierarchy Depth

- Maximum depth: 5 levels
- Format: \`parent/child/grandchild/...\`
- No spaces allowed in tag paths
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="tag:work/projects" />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.Tag, tintColor: Color.Red }}
          title="not:tag:personal"
          subtitle={isShowingDetail ? undefined : "Must NOT have the specified tag"}
          accessories={isShowingDetail ? undefined : [{ text: "Example: not:tag:personal python" }]}
          detail={
            <List.Item.Detail
              markdown={`
# not:tag:personal

**Exclude** snippets that have a specific tag.

## Behavior

- Removes snippets with the specified tag
- Also removes snippets with child tags (hierarchy applies)
- Can use multiple exclusions
- Combine with positive tag filters

## Examples

\`\`\`
not:tag:personal
\`\`\`
Shows all snippets EXCEPT those tagged with "personal" or its children

\`\`\`
not:tag:personal python
\`\`\`
Shows non-personal snippets containing "python"

\`\`\`
tag:work not:tag:client
\`\`\`
Shows work-tagged snippets that do NOT have client tag

\`\`\`
not:tag:archived not:tag:old
\`\`\`
Excludes snippets with "archived" OR "old" tags (either exclusion applies)

## Use Cases

- Filter out specific categories
- Combine with positive filters for precise results
- Clean up search results by removing unwanted categories
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="not:tag:personal python" />
            </ActionPanel>
          }
        />
      </List.Section>

      <List.Section title="Boolean Operators">
        <List.Item
          icon={{ source: Icon.Star, tintColor: Color.Yellow }}
          title="is:favorite"
          subtitle={isShowingDetail ? undefined : "Show only favorite snippets"}
          accessories={isShowingDetail ? undefined : [{ text: "Example: is:favorite api" }]}
          detail={
            <List.Item.Detail
              markdown={`
# is:favorite

Filter to show only snippets marked as **favorites**.

## Behavior

- Shows only snippets where \`isFavorite = true\`
- Overrides the favorites toggle in UI
- Can combine with other operators

## Examples

\`\`\`
is:favorite
\`\`\`
Shows all favorite snippets

\`\`\`
is:favorite api
\`\`\`
Shows favorite snippets containing "api"

\`\`\`
tag:work is:favorite
\`\`\`
Shows favorite snippets that have work tag

\`\`\`
is:favorite not:archived "meeting"
\`\`\`
Shows favorites that aren't archived and contain "meeting" exactly

## Related Operators

- \`not:favorite\` - Show only non-favorite snippets
- Use with \`tag:\` to find favorite snippets in specific categories
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="is:favorite api" />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.Box, tintColor: Color.Purple }}
          title="is:archived"
          subtitle={isShowingDetail ? undefined : "Show only archived snippets"}
          accessories={isShowingDetail ? undefined : [{ text: "Example: is:archived python" }]}
          detail={
            <List.Item.Detail
              markdown={`
# is:archived

Filter to show only **archived** snippets.

## Behavior

- Shows only snippets where \`isArchived = true\`
- Overrides the archived toggle (⌘A) in UI
- Useful for finding old/completed work

## Examples

\`\`\`
is:archived
\`\`\`
Shows all archived snippets

\`\`\`
is:archived python
\`\`\`
Shows archived snippets containing "python"

\`\`\`
tag:work is:archived
\`\`\`
Shows archived work snippets

\`\`\`
is:archived "old project"
\`\`\`
Shows archived snippets with exact phrase "old project"

## Use Cases

- Search through archived content without changing UI toggle
- Find old snippets for reference
- Combine with tags to find archived items in specific categories

## Related

- \`not:archived\` - Explicitly exclude archived snippets (default behavior)
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="is:archived python" />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.Minus, tintColor: Color.SecondaryText }}
          title="is:untagged"
          subtitle={isShowingDetail ? undefined : "Show only snippets without tags"}
          accessories={isShowingDetail ? undefined : [{ text: "Example: is:untagged notes" }]}
          detail={
            <List.Item.Detail
              markdown={`
# is:untagged

Filter to show only snippets that have **no tags**.

## Behavior

- Shows snippets where \`tags.length === 0\`
- Useful for finding snippets that need categorization
- Can combine with text search

## Examples

\`\`\`
is:untagged
\`\`\`
Shows all untagged snippets

\`\`\`
is:untagged meeting
\`\`\`
Shows untagged snippets containing "meeting"

\`\`\`
is:untagged not:archived
\`\`\`
Shows untagged snippets that aren't archived

## Use Cases

- Find snippets that need to be tagged
- Clean up your snippet organization
- Identify uncategorized content

## Related

- \`not:untagged\` - Show only tagged snippets (those with at least one tag)
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="is:untagged notes" />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.XMarkCircle, tintColor: Color.Red }}
          title="not:archived"
          subtitle={isShowingDetail ? undefined : "Exclude archived (also: not:favorite, not:untagged)"}
          accessories={isShowingDetail ? undefined : [{ text: "Example: not:archived meeting" }]}
          detail={
            <List.Item.Detail
              markdown={`
# not:archived

**Exclude** archived snippets from results.

## Available Negations

- \`not:archived\` - Exclude archived snippets
- \`not:favorite\` - Exclude favorite snippets
- \`not:untagged\` - Exclude untagged snippets (show only tagged)

## Behavior

- Opposite of \`is:\` operators
- Can combine multiple \`not:\` operators
- Useful for filtering out unwanted categories

## Examples

\`\`\`
not:archived
\`\`\`
Shows only non-archived snippets

\`\`\`
not:archived meeting
\`\`\`
Shows non-archived snippets containing "meeting"

\`\`\`
not:favorite not:archived
\`\`\`
Shows snippets that are neither favorite nor archived

\`\`\`
tag:work not:archived api
\`\`\`
Shows active (non-archived) work snippets containing "api"

## Use Cases

- Focus on active/current content
- Exclude specific categories from results
- Combine with other filters for precision
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="not:archived meeting" />
            </ActionPanel>
          }
        />
      </List.Section>

      <List.Section title="Text Matching">
        <List.Item
          icon={{ source: Icon.QuotationMarks, tintColor: Color.Magenta }}
          title='"exact phrase"'
          subtitle={isShowingDetail ? undefined : "Match exact phrase in title or content"}
          accessories={isShowingDetail ? undefined : [{ text: 'Example: "meeting notes" urgent' }]}
          detail={
            <List.Item.Detail
              markdown={`
# "exact phrase"

Match an **exact phrase** (substring) in snippet title or content.

## Behavior

- Searches for exact phrase as substring
- **Case-insensitive**: "API" matches "api", "Api", "API"
- Searches in both title AND content
- Can use multiple exact phrases (all must match)
- Wrap phrase in double quotes

## Examples

\`\`\`
"meeting notes"
\`\`\`
Shows snippets containing the exact phrase "meeting notes"

\`\`\`
"api documentation" rest
\`\`\`
Must contain "api documentation" exactly AND "rest" anywhere (fuzzy)

\`\`\`
"rest api" "json"
\`\`\`
Must contain BOTH "rest api" AND "json" as exact phrases

\`\`\`
tag:work "project alpha"
\`\`\`
Work-tagged snippets containing exact phrase "project alpha"

## When to Use

- Finding specific terms or phrases
- When fuzzy search is too broad
- Searching for technical terms that must appear together
- Finding specific error messages or commands

## Notes

- Unmatched quotes are treated as fuzzy text (graceful degradation)
- Empty quotes \`""\` are ignored
- Matches anywhere in title or content
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content='"meeting notes" urgent' />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.MagnifyingGlass, tintColor: Color.Blue }}
          title="fuzzy text"
          subtitle={isShowingDetail ? undefined : "Any text without operators - all words must match"}
          accessories={isShowingDetail ? undefined : [{ text: "Example: api rest endpoint" }]}
          detail={
            <List.Item.Detail
              markdown={`
# Fuzzy Text Search

Any text that isn't an operator or exact phrase is treated as **fuzzy search**.

## Behavior

- **All words must match somewhere** in title, content, or tags
- Case-insensitive matching
- Words can appear in any order
- Partial word matching (substring search)
- Searches across all fields simultaneously

## Examples

\`\`\`
api
\`\`\`
Matches snippets with "api" anywhere (title, content, or tags)

\`\`\`
api rest endpoint
\`\`\`
Must contain ALL three words: "api" AND "rest" AND "endpoint" (in any order)

\`\`\`
python function decorator
\`\`\`
Must contain all three words somewhere in the snippet

## Combined with Operators

\`\`\`
tag:work api rest
\`\`\`
Work tag AND contains "api" AND "rest"

\`\`\`
is:favorite "code snippet" python
\`\`\`
Favorite AND exact phrase "code snippet" AND fuzzy "python"

## Fuzzy vs Exact

| Fuzzy | Exact Phrase |
|-------|--------------|
| \`api rest\` | \`"api rest"\` |
| Words in any order | Exact order required |
| Can be separated | Must be together |
| Matches "rest api" | Only matches "api rest" |

## Use Cases

- Quick exploratory searches
- When you're not sure of exact phrasing
- Finding content across multiple fields
- Natural language queries
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="api rest endpoint" />
            </ActionPanel>
          }
        />
      </List.Section>

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
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
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
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
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
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
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
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
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
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="Price: {{!$:amount: USD|0.00}}" />
            </ActionPanel>
          }
        />
      </List.Section>

      <List.Section title="Complex Examples">
        <List.Item
          icon={{ source: Icon.Layers, tintColor: Color.Green }}
          title='tag:work is:favorite "api docs" rest'
          subtitle={isShowingDetail ? undefined : "Combine multiple operator types"}
          accessories={isShowingDetail ? undefined : [{ text: "Tag + Boolean + Exact + Fuzzy" }]}
          detail={
            <List.Item.Detail
              markdown={`
# Complex Query Example

\`\`\`
tag:work is:favorite "api docs" rest
\`\`\`

## What This Does

This query combines **four different filter types** to find very specific snippets:

1. **tag:work** - Must have "work" tag (or child tags like work/client)
2. **is:favorite** - Must be marked as favorite
3. **"api docs"** - Must contain exact phrase "api docs" in title or content
4. **rest** - Must also contain "rest" anywhere (fuzzy match)

## All Conditions Required (AND Logic)

A snippet must pass **ALL four conditions** to appear in results:
- ✓ Has work tag
- ✓ Is favorite
- ✓ Contains "api docs" exactly
- ✓ Contains "rest" somewhere

## Why This Is Useful

- **Precision**: Finds exactly what you need
- **Combines strengths**: Uses different operator types together
- **Real-world**: This is how you'd find "my favorite work-related REST API documentation"

## Similar Complex Queries

\`\`\`
tag:client not:archived is:favorite proposal
\`\`\`
Active favorite client proposals

\`\`\`
tag:work/projects "meeting notes" action items
\`\`\`
Work project meeting notes with action items
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content='tag:work is:favorite "api docs" rest' />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.Layers, tintColor: Color.Orange }}
          title="tag:work not:tag:client not:archived api"
          subtitle={isShowingDetail ? undefined : "Multiple filters with exclusions"}
          accessories={isShowingDetail ? undefined : [{ text: "Positive + Negative filters" }]}
          detail={
            <List.Item.Detail
              markdown={`
# Exclusion Example

\`\`\`
tag:work not:tag:client not:archived api
\`\`\`

## What This Does

Combines **inclusion and exclusion** filters:

1. **tag:work** - ✓ Must have work tag
2. **not:tag:client** - ✗ Must NOT have client tag
3. **not:archived** - ✗ Must NOT be archived
4. **api** - ✓ Must contain "api"

## Use Case

"Find my active (non-archived) work snippets about APIs, but exclude anything client-related"

Perfect for:
- Internal work documentation
- Non-client work items
- Active projects only

## Understanding Exclusions

- Exclusions **remove** items from results
- Multiple exclusions: removed if it has ANY excluded tag
- Combine with positive filters for precision

## More Exclusion Examples

\`\`\`
not:tag:personal not:tag:draft python
\`\`\`
Python snippets that aren't personal or drafts

\`\`\`
tag:work not:archived not:favorite
\`\`\`
Work snippets that are active and not favorited (need review?)
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
              <Action.CopyToClipboard title="Copy Example" content="tag:work not:tag:client not:archived api" />
            </ActionPanel>
          }
        />
      </List.Section>

      <List.Section title="Tips & Tricks">
        <List.Item
          icon={{ source: Icon.LightBulb, tintColor: Color.Yellow }}
          title="Operators Override UI Filters"
          subtitle={isShowingDetail ? undefined : "Search becomes source of truth when operators present"}
          detail={
            <List.Item.Detail
              markdown={`
# Operators Override UI Filters

When you use search operators, they **take full control** of filtering.

## What This Means

### Without Operators
- Tag dropdown controls visible tags
- ⌘F toggles favorites
- ⌘A toggles archived

### With Operators
- UI filters are **ignored**
- Search operators define all filtering
- UI controls are dimmed (visual feedback)

## Example

If you have:
- Tag dropdown set to "personal"
- Favorites toggle ON

And you search: \`tag:work not:archived\`

Results will show:
- ✓ Work-tagged snippets (not personal)
- ✓ Non-archived only
- ✗ Favorites toggle is ignored

## Why This Design?

- **Consistency**: Search is predictable
- **Power**: Full control via keyboard
- **Clarity**: One source of truth

## Pro Tip

Use operators for temporary filtering without changing UI state. Your UI filter settings remain when you clear the search.
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.LightBulb, tintColor: Color.Yellow }}
          title="Hierarchical Tags"
          subtitle={isShowingDetail ? undefined : "Parent tags match all children automatically"}
          detail={
            <List.Item.Detail
              markdown={`
# Hierarchical Tag Matching

Tag operators **automatically match child tags** in the hierarchy.

## How It Works

If you have snippets tagged:
- \`work\`
- \`work/projects\`
- \`work/projects/alpha\`
- \`work/client\`
- \`personal\`

Searching \`tag:work\` matches:
- ✓ \`work\`
- ✓ \`work/projects\`
- ✓ \`work/projects/alpha\`
- ✓ \`work/client\`
- ✗ \`personal\`

## Benefits

- **Broad searches**: Find everything under a category
- **Flexible**: Organize with deep hierarchies
- **Intuitive**: Parent includes children

## Narrow Your Search

Start broad, then narrow:

1. \`tag:work\` (see everything)
2. \`tag:work/projects\` (narrow to projects)
3. \`tag:work/projects/alpha\` (specific project)

## Exclusions Work the Same Way

\`\`\`
not:tag:work
\`\`\`

Excludes:
- ✗ \`work\`
- ✗ \`work/anything\`
- ✗ All work children

## Max Depth

- Maximum 5 levels deep
- Example: \`level1/level2/level3/level4/level5\`
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
            </ActionPanel>
          }
        />

        <List.Item
          icon={{ source: Icon.LightBulb, tintColor: Color.Yellow }}
          title="Graceful Error Handling"
          subtitle={isShowingDetail ? undefined : "Invalid operators become fuzzy text - search never breaks"}
          detail={
            <List.Item.Detail
              markdown={`
# Graceful Error Handling

Search operators are designed to **never break**. Invalid syntax is handled gracefully.

## What Happens with Errors

Invalid operators are simply treated as fuzzy search text:

| You Type | What Happens |
|----------|--------------|
| \`is:invalidvalue\` | Searches for text "is:invalidvalue" |
| \`tag:\` | Searches for text "tag:" |
| \`unknown:operator\` | Searches for "unknown:operator" |
| \`"unclosed quote\` | Searches for '"unclosed quote' |

## Valid Operator Values

### is: and not:
- ✓ \`favorite\`
- ✓ \`archived\`
- ✓ \`untagged\`
- ✗ Anything else → fuzzy text

### tag: and not:tag:
- ✓ Any tag name
- ✓ Hierarchical tags with \`/\`
- ✗ Empty value → fuzzy text

## Benefits

- **No error messages**: Clean UX
- **Forgiving**: Typos don't break search
- **Discoverable**: Experiment freely

## Example

\`\`\`
is:favorit api
\`\`\`

Since "favorit" isn't valid (missing 'e'), this searches for:
- Fuzzy text: "is:favorit" AND "api"

You can fix by typing:
\`\`\`
is:favorite api
\`\`\`
`}
            />
          }
          actions={
            <ActionPanel>
              <Action
                title="Toggle Detail"
                icon={Icon.AppWindowSidebarLeft}
                onAction={() => setIsShowingDetail(!isShowingDetail)}
              />
            </ActionPanel>
          }
        />
      </List.Section>
    </List>
  );
}
