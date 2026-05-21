import { List } from "@raycast/api";
import { useState } from "react";

import { placeholderSyntaxHelpSections } from "./PlaceholderSyntaxHelpData";
import { PlaceholderSyntaxHelpItem } from "./PlaceholderSyntaxHelpItem";

/**
 * Help view displaying all placeholder syntax variants with examples.
 * Accessible from the snippet create/edit form.
 */
export function PlaceholderSyntaxHelp() {
  const [isShowingDetail, setIsShowingDetail] = useState(false);

  return (
    <List
      navigationTitle="Placeholder Syntax Help"
      searchBarPlaceholder="Search placeholder syntax..."
      isShowingDetail={isShowingDetail}
    >
      {placeholderSyntaxHelpSections.map((section) => (
        <List.Section key={section.title} title={section.title}>
          {section.items.map((item) => (
            <PlaceholderSyntaxHelpItem
              key={item.title}
              item={item}
              isShowingDetail={isShowingDetail}
              onToggleDetail={() => setIsShowingDetail((current) => !current)}
            />
          ))}
        </List.Section>
      ))}
    </List>
  );
}
