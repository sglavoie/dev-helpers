import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpContextSection({
  isShowingDetail,
  onToggleDetail,
}: SearchOperatorsHelpSectionProps) {
  return (
    <List.Section title="Context Operators">
      <List.Item
        icon={{ source: Icon.Bookmark, tintColor: Color.Purple }}
        title="ctx:asl"
        subtitle={isShowingDetail ? undefined : "Must carry the given title prefix context"}
        accessories={isShowingDetail ? undefined : [{ text: "Example: ctx:asl run" }]}
        detail={
          <List.Item.Detail
            markdown={`
# ctx:asl

Filter snippets by their **context** — the \`Prefix: \` namespace at the start of a title.

## What Is a Context?

A title like \`asl: run\` has the context \`asl\` and displays as **run** behind a coloured
badge. Contexts are derived from the title as you type it — there is nothing extra to
store or maintain.

A prefix only counts as a context when it:

- is followed by a colon **and at least one space** (so \`https://example.com\` is not a context)
- has a non-empty remainder (so a title that is only \`asl:\` stays intact)
- is at most 24 characters and 3 words (so \`Remember to do this: now\` is not a context)

## Behavior

- Exact match, no hierarchy — unlike \`tag:\`, contexts are flat
- Case-insensitive: \`ctx:ASL\` and \`ctx:asl\` are identical
- Multiple context filters use AND logic, so they can only ever match one context

## Examples

\`\`\`
ctx:asl
\`\`\`
Shows every snippet titled \`asl: …\`

\`\`\`
ctx:asl run
\`\`\`
Shows \`asl:\` snippets that also contain "run"

\`\`\`
ctx:refactor tag:work
\`\`\`
Combines the context with a tag filter

## Tips

- Type \`ctx:\` on its own to autocomplete from the contexts you already use
- Press Enter on a snippet's **Filter by Context** action to jump straight to its family
- The stored title is never rewritten — searching \`asl\` still finds \`asl: run\`
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="ctx:asl run" />
          </ActionPanel>
        }
      />

      <List.Item
        icon={{ source: Icon.Bookmark, tintColor: Color.Red }}
        title="not:ctx:asl"
        subtitle={isShowingDetail ? undefined : "Must NOT carry the given context"}
        accessories={isShowingDetail ? undefined : [{ text: "Example: not:ctx:asl deploy" }]}
        detail={
          <List.Item.Detail
            markdown={`
# not:ctx:asl

**Exclude** snippets that carry a specific context.

## Behavior

- Removes snippets whose title starts with \`asl: \`
- Snippets with no context at all are kept
- Case-insensitive, and multiple exclusions can be combined

## Examples

\`\`\`
not:ctx:asl
\`\`\`
Shows everything except the \`asl:\` family

\`\`\`
not:ctx:asl deploy
\`\`\`
Shows non-\`asl\` snippets containing "deploy"

\`\`\`
ctx:refactor not:tag:archived
\`\`\`
Mixes context and tag operators freely

## Use Cases

- Hide a noisy context while browsing everything else
- Narrow a broad fuzzy search that keeps surfacing one family
`}
          />
        }
        actions={
          <ActionPanel>
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="not:ctx:asl deploy" />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
