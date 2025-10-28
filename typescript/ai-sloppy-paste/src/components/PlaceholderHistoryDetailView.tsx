import { Action, ActionPanel, Alert, confirmAlert, Icon, List, showToast, Toast, useNavigation } from "@raycast/api";
import { useEffect, useState } from "react";
import { EditPlaceholderValueForm } from "./EditPlaceholderValueForm";
import { getPlaceholderHistoryForKey, deletePlaceholderValue, getMaxPlaceholderHistoryValues } from "../utils/storage";
import { rankPlaceholderValues, formatRelativeTime, formatAbsoluteDate } from "../utils/placeholderHistory";
import { PlaceholderHistoryValue } from "../types";

export function PlaceholderHistoryDetailView(props: { placeholderKey: string; onUpdated: () => void }) {
  const { push, pop } = useNavigation();
  const [values, setValues] = useState<PlaceholderHistoryValue[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadValues();
  }, [props.placeholderKey]);

  async function loadValues() {
    setIsLoading(true);
    try {
      const history = await getPlaceholderHistoryForKey(props.placeholderKey);
      const ranked = rankPlaceholderValues(history);

      // Limit displayed values to preference setting (storage may have up to 100)
      const maxDisplayValues = getMaxPlaceholderHistoryValues();
      const limitedValues = ranked.slice(0, maxDisplayValues);

      setValues(limitedValues);
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to load values",
        message: String(error),
      });
    } finally {
      setIsLoading(false);
    }
  }

  async function handleDeleteValue(value: string) {
    const confirmed = await confirmAlert({
      title: "Delete Value",
      message: `Delete "${value}" from history for "${props.placeholderKey}"?`,
      primaryAction: {
        title: "Delete",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (confirmed) {
      try {
        await deletePlaceholderValue(props.placeholderKey, value);
        await loadValues();
        props.onUpdated();

        // If no values left, go back
        if (values.length === 1) {
          pop();
        }

        showToast({
          style: Toast.Style.Success,
          title: "Value deleted",
        });
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to delete value",
          message: String(error),
        });
      }
    }
  }

  async function handleValueUpdated() {
    await loadValues();
    props.onUpdated();
  }

  function getAccessories(historyValue: PlaceholderHistoryValue): List.Item.Accessory[] {
    const accessories: List.Item.Accessory[] = [];

    // Use count
    accessories.push({
      tag: { value: `${historyValue.useCount} use${historyValue.useCount !== 1 ? "s" : ""}`, color: "#00aa00" },
    });

    // Last used
    accessories.push({
      text: formatRelativeTime(historyValue.lastUsed),
      tooltip: `Last used: ${formatAbsoluteDate(historyValue.lastUsed)}\nCreated: ${formatAbsoluteDate(historyValue.createdAt)}`,
    });

    return accessories;
  }

  const displayLimit = getMaxPlaceholderHistoryValues();

  return (
    <List
      isLoading={isLoading}
      navigationTitle={`Values for "${props.placeholderKey}"`}
      searchBarPlaceholder="Search values..."
    >
      {values.length === 0 ? (
        <List.EmptyView icon={Icon.Text} title="No values" description="This placeholder key has no saved values" />
      ) : (
        <>
          <List.Section
            title={`Showing ${values.length} of up to 100 stored values (filtered by ${displayLimit} preference)`}
          >
            {values.map((historyValue, index) => (
              <List.Item
                key={`${historyValue.value}-${index}`}
                icon={Icon.Text}
                title={historyValue.value}
                accessories={getAccessories(historyValue)}
                actions={
                  <ActionPanel>
                    <ActionPanel.Section title="Value Actions">
                      <Action
                        title="Edit Value"
                        icon={Icon.Pencil}
                        onAction={() => {
                          push(
                            <EditPlaceholderValueForm
                              placeholderKey={props.placeholderKey}
                              oldValue={historyValue.value}
                              onUpdated={handleValueUpdated}
                            />,
                          );
                        }}
                      />
                      <Action
                        title="Delete Value"
                        icon={Icon.Trash}
                        style={Action.Style.Destructive}
                        shortcut={{ modifiers: ["cmd"], key: "delete" }}
                        onAction={() => handleDeleteValue(historyValue.value)}
                      />
                    </ActionPanel.Section>
                  </ActionPanel>
                }
              />
            ))}
          </List.Section>
        </>
      )}
    </List>
  );
}
