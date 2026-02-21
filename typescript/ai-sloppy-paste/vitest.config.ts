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
        "/Users/sglavoie/1_dev_projects/sglavoie_dev-helpers/dev-helpers/typescript/ai-sloppy-paste/src/__mocks__/@raycast/api.ts",
    },
  },
});
