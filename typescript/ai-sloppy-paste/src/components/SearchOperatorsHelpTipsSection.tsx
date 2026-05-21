import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpTipsSection({ isShowingDetail, onToggleDetail }: SearchOperatorsHelpSectionProps) {
  return (
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
- ✓ \`favorite\` / \`bookmarked\` (aliases)
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
