import { List } from "@raycast/api";
import { useState } from "react";

import { SearchOperatorsHelpBooleanSection } from "./SearchOperatorsHelpBooleanSection";
import { SearchOperatorsHelpComplexExamplesSection } from "./SearchOperatorsHelpComplexExamplesSection";
import { SearchOperatorsHelpOverviewSection } from "./SearchOperatorsHelpOverviewSection";
import { SearchOperatorsHelpPlaceholderSection } from "./SearchOperatorsHelpPlaceholderSection";
import { SearchOperatorsHelpTagSection } from "./SearchOperatorsHelpTagSection";
import { SearchOperatorsHelpTextSection } from "./SearchOperatorsHelpTextSection";
import { SearchOperatorsHelpTipsSection } from "./SearchOperatorsHelpTipsSection";

/**
 * Help view displaying all available search operators with examples
 */
export function SearchOperatorsHelp() {
  const [isShowingDetail, setIsShowingDetail] = useState(false);
  const toggleDetail = () => setIsShowingDetail((currentValue) => !currentValue);

  return (
    <List
      navigationTitle="Search Operators Help"
      searchBarPlaceholder="Search help..."
      isShowingDetail={isShowingDetail}
    >
      <SearchOperatorsHelpOverviewSection isShowingDetail={isShowingDetail} onToggleDetail={toggleDetail} />
      <SearchOperatorsHelpTagSection isShowingDetail={isShowingDetail} onToggleDetail={toggleDetail} />
      <SearchOperatorsHelpBooleanSection isShowingDetail={isShowingDetail} onToggleDetail={toggleDetail} />
      <SearchOperatorsHelpTextSection isShowingDetail={isShowingDetail} onToggleDetail={toggleDetail} />
      <SearchOperatorsHelpPlaceholderSection isShowingDetail={isShowingDetail} onToggleDetail={toggleDetail} />
      <SearchOperatorsHelpComplexExamplesSection isShowingDetail={isShowingDetail} onToggleDetail={toggleDetail} />
      <SearchOperatorsHelpTipsSection isShowingDetail={isShowingDetail} onToggleDetail={toggleDetail} />
    </List>
  );
}
