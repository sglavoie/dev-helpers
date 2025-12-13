import { Clipboard } from "@raycast/api";

const CLIPBOARD_RESTORE_DELAY_MS = 50;

export async function pasteWithClipboardRestore(content: string): Promise<void> {
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
  }, CLIPBOARD_RESTORE_DELAY_MS);
}
