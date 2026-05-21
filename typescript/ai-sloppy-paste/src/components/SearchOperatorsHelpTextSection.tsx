import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpTextSection({ isShowingDetail, onToggleDetail }: SearchOperatorsHelpSectionProps) {
  return (
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
            <Action.CopyToClipboard title="Copy Example" content="api rest endpoint" />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
