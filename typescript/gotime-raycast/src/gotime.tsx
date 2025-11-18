import {
  Action,
  ActionPanel,
  Alert,
  Color,
  Icon,
  List,
  Toast,
  confirmAlert,
  showToast,
  Keyboard,
} from "@raycast/api";
import { useExec } from "@raycast/utils";
import { execSync } from "child_process";
import ActiveTimers from "./active-timers";
import ContinueTimer from "./continue-timer";
import DeleteTimer from "./delete-timer";
import StartTimer from "./start-timer";
import WeeklyReport from "./weekly-report";
import TagsList from "./tags-list";
import TagsRename from "./tags-rename";
import TagsRemove from "./tags-remove";
import SetEntry from "./set-entry";
import ListEntries from "./list-entries";

interface CommandItem {
  id: string;
  title: string;
  description: string;
  icon: Icon;
  iconColor: Color;
  component: React.ComponentType;
  shortcut?: Keyboard.Shortcut;
}

export default function Command() {
  const { data: activeTimers, revalidate } = useExec(
    "/Users/sglavoie/.local/bin/gt",
    ["list", "--active", "--json"],
    {
      parseOutput: ({ stdout }) => {
        const trimmed = stdout.trim();
        if (!trimmed) {
          return [];
        }
        try {
          const parsed = JSON.parse(trimmed);
          return Array.isArray(parsed) ? parsed : [];
        } catch (e) {
          console.error("Failed to parse JSON:", e);
          return [];
        }
      },
    },
  );

  async function handleStopAll() {
    if (!activeTimers || activeTimers.length === 0) return;

    const confirmed = await confirmAlert({
      title: "Stop All Timers",
      message: `Stop all ${activeTimers.length} active timer(s)?`,
      primaryAction: {
        title: "Stop All",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (!confirmed) return;

    try {
      await showToast({
        style: Toast.Style.Animated,
        title: "Stopping all timers...",
      });

      execSync(`/Users/sglavoie/.local/bin/gt stop --all`, {
        encoding: "utf-8",
      });

      await showToast({
        style: Toast.Style.Success,
        title: "All timers stopped",
      });

      revalidate();
    } catch (error) {
      await showToast({
        style: Toast.Style.Failure,
        title: "Failed to stop all timers",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  const timerCommands: CommandItem[] = [
    {
      id: "start-timer",
      title: "Start Timer",
      description: "Start a new timer with keyword and tags",
      icon: Icon.Play,
      iconColor: Color.Green,
      component: StartTimer,
      shortcut: { modifiers: ["cmd"], key: "s" },
    },
    {
      id: "active-timers",
      title: "Active Timers",
      description: "View and stop active timers",
      icon: Icon.Clock,
      iconColor: Color.Green,
      component: ActiveTimers,
      shortcut: { modifiers: ["cmd"], key: "a" },
    },
    {
      id: "continue-timer",
      title: "Continue Timer",
      description: "Continue a previous timer",
      icon: Icon.RotateClockwise,
      iconColor: Color.Blue,
      component: ContinueTimer,
      shortcut: { modifiers: ["cmd"], key: "c" },
    },
  ];

  const reportCommands: CommandItem[] = [
    {
      id: "list-entries",
      title: "List Entries",
      description: "List and filter time tracking entries",
      icon: Icon.List,
      iconColor: Color.Blue,
      component: ListEntries,
      shortcut: { modifiers: ["cmd", "shift"], key: "l" },
    },
    {
      id: "reports",
      title: "Reports",
      description: "View time tracking reports",
      icon: Icon.BarChart,
      iconColor: Color.Purple,
      component: WeeklyReport,
      shortcut: { modifiers: ["cmd"], key: "w" },
    },
    {
      id: "set-entry",
      title: "Edit Entry",
      description: "Edit an existing timer entry",
      icon: Icon.Pencil,
      iconColor: Color.Orange,
      component: SetEntry,
      shortcut: { modifiers: ["cmd"], key: "e" },
    },
    {
      id: "delete-timer",
      title: "Delete Timer",
      description: "Delete a timer entry",
      icon: Icon.Trash,
      iconColor: Color.Red,
      component: DeleteTimer,
      shortcut: { modifiers: ["cmd"], key: "d" },
    },
  ];

  const tagCommands: CommandItem[] = [
    {
      id: "tags-list",
      title: "List Tags",
      description: "View all tags with usage statistics",
      icon: Icon.List,
      iconColor: Color.Blue,
      component: TagsList,
      shortcut: { modifiers: ["cmd"], key: "l" },
    },
    {
      id: "tags-rename",
      title: "Rename Tag",
      description: "Rename a tag across all entries",
      icon: Icon.Pencil,
      iconColor: Color.Orange,
      component: TagsRename,
      shortcut: { modifiers: ["cmd"], key: "r" },
    },
    {
      id: "tags-remove",
      title: "Remove Tag",
      description: "Remove a tag from all entries",
      icon: Icon.XMarkCircle,
      iconColor: Color.Red,
      component: TagsRemove,
      shortcut: { modifiers: ["cmd"], key: "x" },
    },
  ];

  const hasActiveTimers =
    activeTimers && Array.isArray(activeTimers) && activeTimers.length > 0;

  return (
    <List>
      <List.Section title="Timer Management">
        {timerCommands.map((cmd) => (
          <List.Item
            key={cmd.id}
            icon={{ source: cmd.icon, tintColor: cmd.iconColor }}
            title={cmd.title}
            subtitle={cmd.description}
            actions={
              <ActionPanel>
                <Action.Push
                  title={`Open ${cmd.title}`}
                  target={<cmd.component />}
                  icon={cmd.icon}
                  shortcut={cmd.shortcut}
                />
              </ActionPanel>
            }
          />
        ))}
        {hasActiveTimers && (
          <List.Item
            key="stop-all"
            icon={{ source: Icon.Stop, tintColor: Color.Red }}
            title="Stop All Active Timers"
            subtitle={`${activeTimers.length} timer${activeTimers.length > 1 ? "s" : ""} running`}
            actions={
              <ActionPanel>
                <Action
                  title="Stop All Timers"
                  icon={Icon.Stop}
                  style={Action.Style.Destructive}
                  shortcut={{ modifiers: ["cmd", "shift"], key: "s" }}
                  onAction={handleStopAll}
                />
              </ActionPanel>
            }
          />
        )}
      </List.Section>

      <List.Section title="Tag Management">
        {tagCommands.map((cmd) => (
          <List.Item
            key={cmd.id}
            icon={{ source: cmd.icon, tintColor: cmd.iconColor }}
            title={cmd.title}
            subtitle={cmd.description}
            actions={
              <ActionPanel>
                <Action.Push
                  title={`Open ${cmd.title}`}
                  target={<cmd.component />}
                  icon={cmd.icon}
                  shortcut={cmd.shortcut}
                />
              </ActionPanel>
            }
          />
        ))}
      </List.Section>

      <List.Section title="Reports & Data">
        {reportCommands.map((cmd) => (
          <List.Item
            key={cmd.id}
            icon={{ source: cmd.icon, tintColor: cmd.iconColor }}
            title={cmd.title}
            subtitle={cmd.description}
            actions={
              <ActionPanel>
                <Action.Push
                  title={`Open ${cmd.title}`}
                  target={<cmd.component />}
                  icon={cmd.icon}
                  shortcut={cmd.shortcut}
                />
              </ActionPanel>
            }
          />
        ))}
      </List.Section>
    </List>
  );
}
