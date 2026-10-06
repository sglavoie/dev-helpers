import { execFile } from "node:child_process";
import { homedir } from "node:os";
import { join } from "node:path";

export function resolveGTBinary(preference?: string): string {
  const binary = preference?.trim() || "~/.local/bin/gt";
  return binary.startsWith("~/") ? join(homedir(), binary.slice(2)) : binary;
}

/** Execute arguments literally; never pass form input through a shell. */
export function runGTProcess(binary: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = execFile(
      binary,
      args,
      { encoding: "utf8", timeout: 30_000, maxBuffer: 10 * 1024 * 1024 },
      (error, stdout, stderr) => {
        if (!error) {
          resolve(stdout);
        } else if (error.code === "ENOENT" || error.code === "EACCES") {
          reject(
            new Error(
              `Cannot run GoTime at ${binary}. Install gt or set GoTime Binary in the extension preferences.`,
            ),
          );
        } else {
          reject(new Error(stderr.trim() || error.message));
        }
      },
    );
    // Extension actions are noninteractive; a CLI prompt must not wait for input.
    child.stdin?.end();
  });
}
