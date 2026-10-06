import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, writeFile, rm } from "node:fs/promises";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";
import { resolveGTBinary, runGTProcess } from "../src/utils/gt-process.ts";

test("default and custom binary paths", () => {
  assert.equal(resolveGTBinary(), join(homedir(), ".local/bin/gt"));
  assert.equal(
    resolveGTBinary(" ~/My Tools/gt "),
    join(homedir(), "My Tools/gt"),
  );
  assert.equal(resolveGTBinary("/opt/bin/gt"), "/opt/bin/gt");
});

test("passes spaces, empty values, and shell syntax literally to a binary with spaces", async () => {
  const directory = await mkdtemp(join(tmpdir(), "gt runner "));
  try {
    const binary = join(directory, "fake gt");
    await writeFile(
      binary,
      `#!${process.execPath}\nconsole.log(JSON.stringify(process.argv.slice(2)));\n`,
      { mode: 0o700 },
    );
    const args = [
      "start",
      "client's work",
      "",
      "$(echo unintended)",
      "; echo unintended",
      "--backdate",
      "1h",
    ];
    assert.deepEqual(JSON.parse(await runGTProcess(binary, args)), args);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("reports CLI errors and missing installations", async () => {
  await assert.rejects(
    runGTProcess(process.execPath, [
      "-e",
      'console.error("invalid backdate"); process.exit(2)',
    ]),
    /invalid backdate/,
  );
  await assert.rejects(
    runGTProcess("/does-not-exist/gt", []),
    /GoTime Binary.*preferences/,
  );
});
