import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    setupFiles: ["./src/test-setup.ts"],
  },
  resolve: {
    alias: {
      "@raycast/api":
        "/Users/sglavoie/dev/sglavoie/dev-helpers/typescript/ai-sloppy-paste/src/__mocks__/@raycast/api.ts",
    },
  },
});
