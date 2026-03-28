import { Alert, Toast, confirmAlert, open, showToast } from "@raycast/api";
import { writeFile } from "fs/promises";
import * as os from "os";
import * as path from "path";
import { useEffect, useState } from "react";
import { Snippet } from "../types";
import { deleteSnippet, exportData, getSnippets, getStorageSize } from "../utils/storage";
import { getErrorMessage } from "../utils/errorMessage";

export function useSnippets() {
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    setIsLoading(true);
    try {
      const loadedSnippets = await getSnippets();
      setSnippets(loadedSnippets);
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to load data",
        message: getErrorMessage(error),
      });
    } finally {
      setIsLoading(false);
    }
  }

  async function handleDelete(snippet: Snippet) {
    const confirmed = await confirmAlert({
      title: "Delete Snippet",
      message: `Are you sure you want to delete "${snippet.title}"?`,
      primaryAction: {
        title: "Delete",
        style: Alert.ActionStyle.Destructive,
      },
    });

    if (confirmed) {
      try {
        await deleteSnippet(snippet.id);
        await loadData();
        showToast({
          style: Toast.Style.Success,
          title: "Snippet deleted",
        });
      } catch (error) {
        showToast({
          style: Toast.Style.Failure,
          title: "Failed to delete snippet",
          message: getErrorMessage(error),
        });
      }
    }
  }

  async function handleExport() {
    try {
      const data = await exportData();
      const downloadsPath = path.join(os.homedir(), "Downloads");
      const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
      const filename = `ai-sloppy-paste-${timestamp}.json`;
      const filepath = path.join(downloadsPath, filename);

      await writeFile(filepath, JSON.stringify(data, null, 2));

      showToast({
        style: Toast.Style.Success,
        title: "Export successful",
        message: `Saved to ${filename}`,
        primaryAction: {
          title: "Open Folder",
          onAction: () => {
            open(downloadsPath);
          },
        },
      });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Export failed",
        message: getErrorMessage(error),
      });
    }
  }

  async function handleShowStorageInfo() {
    try {
      const storage = await getStorageSize();
      let message = `Using ${storage.formatted} (${storage.percentage.toFixed(1)}% of estimated 5MB limit)`;

      if (storage.percentage > 90) {
        message += "\n⚠️ Approaching storage limit!";
      } else if (storage.percentage > 75) {
        message += "\n⚠️ Storage usage is high";
      }

      await confirmAlert({
        title: "Storage Information",
        message: message,
        primaryAction: {
          title: "OK",
          style: Alert.ActionStyle.Default,
        },
      });
    } catch (error) {
      showToast({
        style: Toast.Style.Failure,
        title: "Failed to get storage info",
        message: getErrorMessage(error),
      });
    }
  }

  return { snippets, isLoading, loadData, handleDelete, handleExport, handleShowStorageInfo };
}
