import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpComplexExamplesSection({
  isShowingDetail,
  onToggleDetail,
}: SearchOperatorsHelpSectionProps) {
  return (
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="tag:work not:tag:client not:archived api" />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
