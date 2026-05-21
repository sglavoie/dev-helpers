import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpTagSection({ isShowingDetail, onToggleDetail }: SearchOperatorsHelpSectionProps) {
  return (
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="not:tag:personal python" />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
