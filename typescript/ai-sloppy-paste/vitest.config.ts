import { defineConfig } from "vitest/config";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const projectRoot = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    setupFiles: ["./src/test-setup.ts"],
  },
  resolve: {
    alias: {
      "@raycast/api": resolve(projectRoot, "src/__mocks__/@raycast/api.ts"),
    },
  },
});
