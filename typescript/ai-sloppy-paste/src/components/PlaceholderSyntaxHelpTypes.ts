import type { ColorLike, Icon } from "@raycast/api";

type PlaceholderSyntaxHelpIcon = {
  source: Icon;
  tintColor: ColorLike;
};

export type PlaceholderSyntaxHelpItemData = {
  icon: PlaceholderSyntaxHelpIcon;
  title: string;
  subtitle: string;
  accessoryText?: string;
  markdown: string;
  copyActionTitle?: string;
  copyContent?: string;
};

export type PlaceholderSyntaxHelpSectionData = {
  title: string;
  items: PlaceholderSyntaxHelpItemData[];
};
