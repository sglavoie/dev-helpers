import { placeholderSyntaxHelpOverviewSection } from "./PlaceholderSyntaxHelpOverviewSection";
import { placeholderSyntaxHelpBasicSection } from "./PlaceholderSyntaxHelpBasicSection";
import { placeholderSyntaxHelpChoicesSection } from "./PlaceholderSyntaxHelpChoicesSection";
import { placeholderSyntaxHelpAdvancedSection } from "./PlaceholderSyntaxHelpAdvancedSection";
import { placeholderSyntaxHelpConditionalSection } from "./PlaceholderSyntaxHelpConditionalSection";
import { placeholderSyntaxHelpSystemSection } from "./PlaceholderSyntaxHelpSystemSection";
import { placeholderSyntaxHelpExampleSection } from "./PlaceholderSyntaxHelpExampleSection";
import type { PlaceholderSyntaxHelpSectionData } from "./PlaceholderSyntaxHelpTypes";

/** Ordered sections rendered by {@link PlaceholderSyntaxHelp}. */
export const placeholderSyntaxHelpSections: PlaceholderSyntaxHelpSectionData[] = [
  placeholderSyntaxHelpOverviewSection,
  placeholderSyntaxHelpBasicSection,
  placeholderSyntaxHelpChoicesSection,
  placeholderSyntaxHelpAdvancedSection,
  placeholderSyntaxHelpConditionalSection,
  placeholderSyntaxHelpSystemSection,
  placeholderSyntaxHelpExampleSection,
];
