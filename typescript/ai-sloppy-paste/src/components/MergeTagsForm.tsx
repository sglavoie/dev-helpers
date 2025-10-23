import { Action, ActionPanel, Form, showToast, Toast, useNavigation } from "@raycast/api";
import { useState } from "react";
import { mergeTags } from "../utils/storage";

export function MergeTagsForm(props: { tags: string[]; onMerged: () => void }) {
  const { pop } = useNavigation();
  const [sourceTagError, setSourceTagError] = useState<string | undefined>();
  const [targetTagError, setTargetTagError] = useState<string | undefined>();

  async function handleSubmit(values: { sourceTag: string; targetTag: string }) {
    // Validation
    if (!values.sourceTag) {
      setSourceTagError("Please select a source tag");
      return;
    }

    if (!values.targetTag) {
      setTargetTagError("Please select a target tag");
      return;
    }

    if (values.sourceTag === values.targetTag) {
      setSourceTagError("Source and target tags must be different");
      setTargetTagError("Source and target tags must be different");
      return;
    }

    try {
      const affectedCount = await mergeTags(values.sourceTag, values.targetTag);
      props.onMerged();
      pop();
      showToast({
        style: Toast.Style.Success,
        title: "Tags merged",
        message: `Merged "${values.sourceTag}" into "${values.targetTag}" (${affectedCount} snippets affected)`,
      });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to merge tags",
        message: String(error),
      });
    }
  }

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Merge Tags" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.Description text="Merge the source tag into the target tag. All snippets with the source tag will be updated to use the target tag instead. The source tag will be removed." />
      <Form.Dropdown
        id="sourceTag"
        title="Source Tag"
        error={sourceTagError}
        onChange={() => setSourceTagError(undefined)}
      >
        <Form.Dropdown.Item value="" title="Select a tag to merge from..." />
        {props.tags.map((tag) => (
          <Form.Dropdown.Item key={tag} value={tag} title={tag} />
        ))}
      </Form.Dropdown>
      <Form.Dropdown
        id="targetTag"
        title="Target Tag"
        error={targetTagError}
        onChange={() => setTargetTagError(undefined)}
      >
        <Form.Dropdown.Item value="" title="Select a tag to merge into..." />
        {props.tags.map((tag) => (
          <Form.Dropdown.Item key={tag} value={tag} title={tag} />
        ))}
      </Form.Dropdown>
    </Form>
  );
}
