import { Action, ActionPanel, Form, showToast, Toast, useNavigation } from "@raycast/api";
import { useState } from "react";
import { updatePlaceholderValue } from "../utils/storage";

export function EditPlaceholderValueForm(props: { placeholderKey: string; oldValue: string; onUpdated: () => void }) {
  const { pop } = useNavigation();
  const [error, setError] = useState<string | undefined>();

  async function handleSubmit(values: { newValue: string }) {
    const newValue = values.newValue.trim();

    // Validate new value
    if (!newValue) {
      setError("Value cannot be empty");
      return;
    }

    // Check if value is unchanged
    if (newValue === props.oldValue) {
      setError("New value must be different");
      return;
    }

    try {
      await updatePlaceholderValue(props.placeholderKey, props.oldValue, newValue);
      props.onUpdated();
      pop();
      showToast({
        style: Toast.Style.Success,
        title: "Value updated",
        message: `Updated value for "${props.placeholderKey}"`,
      });
    } catch (error) {
      const errorMessage = String(error);
      if (errorMessage.includes("already exists")) {
        setError("A value with this name already exists");
      } else {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to update value",
          message: errorMessage,
        });
      }
    }
  }

  return (
    <Form
      navigationTitle={`Edit Value for "${props.placeholderKey}"`}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Update Value" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="newValue"
        title="Value"
        placeholder="Enter new value"
        defaultValue={props.oldValue}
        error={error}
        onChange={() => setError(undefined)}
        info={`Editing the value for placeholder key "${props.placeholderKey}"`}
      />
    </Form>
  );
}
