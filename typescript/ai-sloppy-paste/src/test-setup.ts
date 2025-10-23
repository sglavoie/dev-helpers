import { beforeEach } from "vitest";
import { LocalStorage } from "@raycast/api";

beforeEach(async () => {
  // Clear storage before each test
  await LocalStorage.clear();
});
