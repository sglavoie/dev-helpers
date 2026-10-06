import { getPreferenceValues } from "@raycast/api";
import { resolveGTBinary, runGTProcess } from "./gt-process";

export const GT_BIN = resolveGTBinary(
  getPreferenceValues<{ gtBinary?: string }>().gtBinary,
);

export function runGT(args: string[]): Promise<string> {
  return runGTProcess(GT_BIN, args);
}
