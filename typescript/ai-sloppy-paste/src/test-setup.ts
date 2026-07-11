import { beforeEach, vi } from "vitest";
import {
  Clipboard,
  LocalStorage,
  closeMainWindow,
  getPreferenceValues,
  popToRoot,
  showHUD,
  showToast,
} from "@raycast/api";
import { invalidateCache } from "./utils/storage";

beforeEach(async () => {
  invalidateCache();
  await LocalStorage.clear();

  vi.mocked(Clipboard.clear).mockReset();
  vi.mocked(Clipboard.copy).mockReset();
  vi.mocked(Clipboard.paste).mockReset();
  vi.mocked(Clipboard.read).mockReset();
  vi.mocked(Clipboard.readText).mockReset();

  vi.mocked(getPreferenceValues).mockReset();
  vi.mocked(getPreferenceValues).mockReturnValue({});
  vi.mocked(closeMainWindow).mockReset();
  vi.mocked(popToRoot).mockReset();
  vi.mocked(showHUD).mockReset();
  vi.mocked(showToast).mockReset();
});
