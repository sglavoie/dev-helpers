import { Clipboard } from "@raycast/api";

export async function pasteSnippet(content: string): Promise<void> {
  await Clipboard.paste(content);
}
