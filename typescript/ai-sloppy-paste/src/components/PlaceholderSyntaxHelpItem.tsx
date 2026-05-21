import { Action, ActionPanel, Icon, List } from "@raycast/api";

import type { PlaceholderSyntaxHelpItemData } from "./PlaceholderSyntaxHelpData";

type Props = {
  item: PlaceholderSyntaxHelpItemData;
  isShowingDetail: boolean;
  onToggleDetail: () => void;
};

export function PlaceholderSyntaxHelpItem({ item, isShowingDetail, onToggleDetail }: Props) {
  return (
    <List.Item
      icon={item.icon}
      title={item.title}
      subtitle={isShowingDetail ? undefined : item.subtitle}
      accessories={isShowingDetail || !item.accessoryText ? undefined : [{ text: item.accessoryText }]}
      detail={<List.Item.Detail markdown={item.markdown} />}
      actions={
        <ActionPanel>
          <Action title="Toggle Detail" icon={Icon.AppWindowSidebarLeft} onAction={onToggleDetail} />
          {item.copyContent ? (
            <Action.CopyToClipboard title={item.copyActionTitle ?? "Copy Example"} content={item.copyContent} />
          ) : null}
        </ActionPanel>
      }
    />
  );
}
