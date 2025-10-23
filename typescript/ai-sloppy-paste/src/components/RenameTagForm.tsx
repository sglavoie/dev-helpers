import { Action, ActionPanel, Form, showToast, Toast, useNavigation } from "@raycast/api";
import { useState } from "react";
import { renameTag } from "../utils/storage";
import { validateTag } from "../utils/validation";

export function RenameTagForm(props: { tag: string; onRenamed: () => void }) {
  const { pop } = useNavigation();
  const [error, setError] = useState<string | undefined>();

  async function handleSubmit(values: { newTag: string }) {
    const newTag = values.newTag.trim();

    // Validate new tag
    const validation = validateTag(newTag);
    if (!validation.isValid) {
      setError(validation.error);
      return;
    }

    // Check if tag name is unchanged
    if (newTag === props.tag) {
      setError("New tag name must be different");
      return;
    }

    try {
      const affectedCount = await renameTag(props.tag, newTag);
      props.onRenamed();
      pop();
      showToast({
        style: Toast.Style.Success,
        title: "Tag renamed",
        message: `Updated ${affectedCount} snippet${affectedCount !== 1 ? "s" : ""}`,
      });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to rename tag",
        message: String(error),
      });
    }
  }

  return (
    <Form
      navigationTitle={`Rename Tag: ${props.tag}`}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Rename Tag" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.TextField
        id="newTag"
        title="New Tag Name"
        placeholder="Enter new tag name"
        defaultValue={props.tag}
        error={error}
        onChange={() => setError(undefined)}
        info={`Renaming "${props.tag}" will also update any child tags (e.g., "${props.tag}/..." becomes "new-name/...")`}
      />
    </Form>
  );
}
