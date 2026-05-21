import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpBooleanSection({
  isShowingDetail,
  onToggleDetail,
}: SearchOperatorsHelpSectionProps) {
  return (
    <List.Section title="Boolean Operators">
      <List.Item
        icon={{ source: Icon.Star, tintColor: Color.Yellow }}
        title="is:favorite / is:bookmarked"
        subtitle={isShowingDetail ? undefined : "Show only bookmarked snippets"}
        accessories={isShowingDetail ? undefined : [{ text: "Example: is:bookmarked api" }]}
        detail={
          <List.Item.Detail
            markdown={`
# is:favorite / is:bookmarked

Filter to show only snippets marked as **bookmarks**.

\`is:bookmarked\` and \`is:favorite\` are aliases — both work identically.

## Behavior

- Shows only snippets where \`isFavorite = true\`
- Overrides the bookmarks toggle in UI
- Can combine with other operators

## Examples

\`\`\`
is:bookmarked
\`\`\`
Shows all bookmarked snippets

\`\`\`
is:bookmarked api
\`\`\`
Shows bookmarked snippets containing "api"

\`\`\`
tag:work is:bookmarked
\`\`\`
Shows bookmarked snippets that have work tag

\`\`\`
is:bookmarked not:archived "meeting"
\`\`\`
Shows bookmarks that aren't archived and contain "meeting" exactly

## Related Operators

- \`not:bookmarked\` / \`not:favorite\` - Show only non-bookmarked snippets
- Use with \`tag:\` to find bookmarked snippets in specific categories
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="is:bookmarked api" />
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
- \`not:bookmarked\` / \`not:favorite\` - Exclude bookmarked snippets
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="not:archived meeting" />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
