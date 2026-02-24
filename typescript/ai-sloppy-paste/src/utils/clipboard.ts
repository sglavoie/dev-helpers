import { Clipboard, getPreferenceValues } from "@raycast/api";

function getClipboardRestoreDelayMs(): number {
  const { clipboardRestoreDelay } = getPreferenceValues<{ clipboardRestoreDelay?: string }>();
  return parseInt(clipboardRestoreDelay ?? "500", 10);
}

export async function pasteWithClipboardRestore(content: string): Promise<void> {
  const delayMs = getClipboardRestoreDelayMs();

  // If delay is 0 ("Never restore"), just paste without saving/restoring
  if (delayMs === 0) {
    await Clipboard.paste(content);
    return;
  }

  // Save original clipboard content
  const originalClipboard = await Clipboard.read();

  // Paste the new content
  await Clipboard.paste(content);

  // Restore original clipboard after a delay
  setTimeout(async () => {
    try {
      if (originalClipboard.text) {
        await Clipboard.copy(originalClipboard.text);
      } else if (originalClipboard.file) {
        await Clipboard.copy({ file: originalClipboard.file });
      }
      // If clipboard was empty, we don't restore anything
    } catch {
      // Silently fail - clipboard restoration is best-effort
    }
  }, delayMs);
}
