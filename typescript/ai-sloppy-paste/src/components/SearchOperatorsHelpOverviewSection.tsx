import { List, ActionPanel, Action, Icon, Color } from "@raycast/api";
import type { SearchOperatorsHelpSectionProps } from "./SearchOperatorsHelpTypes";

export function SearchOperatorsHelpOverviewSection({
  isShowingDetail,
  onToggleDetail,
}: SearchOperatorsHelpSectionProps) {
  return (
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
            <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
          </ActionPanel>
        }
      />
    </List.Section>
  );
}
