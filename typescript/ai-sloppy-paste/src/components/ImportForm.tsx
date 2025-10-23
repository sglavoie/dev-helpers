import { Action, ActionPanel, Form, showToast, Toast, useNavigation } from "@raycast/api";
import * as fs from "fs";
import { ExportData } from "../types";
import { importData } from "../utils/storage";

export function ImportForm(props: { onImported: () => void }) {
  const { pop } = useNavigation();

  async function handleSubmit(values: { filepath: string[]; merge: boolean }) {
    if (!values.filepath || values.filepath.length === 0) {
      showToast({
        style: Toast.Style.Failure,
        title: "Please select a file",
      });
      return;
    }

    const filepath = values.filepath[0];

    try {
      const fileContent = fs.readFileSync(filepath, "utf-8");
      const data: ExportData = JSON.parse(fileContent);

      if (!data.snippets) {
        throw new Error("Invalid file format");
      }

      await importData(data, values.merge);
      props.onImported();
      pop();

      showToast({
        style: Toast.Style.Success,
        title: "Import successful",
        message: `Imported ${data.snippets.length} snippets`,
      });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Import failed",
        message: String(error),
      });
    }
  }

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Import" onSubmit={handleSubmit} />
        </ActionPanel>
      }
    >
      <Form.FilePicker
        id="filepath"
        title="Select File"
        allowMultipleSelection={false}
        canChooseDirectories={false}
        canChooseFiles={true}
      />
      <Form.Checkbox
        id="merge"
        label="Merge with existing data"
        defaultValue={true}
        info="If unchecked, existing data will be replaced"
      />
    </Form>
  );
}
