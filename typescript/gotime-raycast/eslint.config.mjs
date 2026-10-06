import raycast from "@raycast/eslint-config";
import { defineConfig } from "eslint/config";

export default defineConfig(raycast, {
  ignores: ["dist/**", "raycast-env.d.ts"],
});
