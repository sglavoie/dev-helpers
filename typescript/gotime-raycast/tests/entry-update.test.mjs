import { test } from "node:test";
import assert from "node:assert/strict";
import { entryUpdateArgs } from "../src/utils/entry-update.ts";

const entry = {
  id: "0d2e9e58-7943-4391-b2b3-353d36101e87",
  keyword: "coding",
  tags: ["work"],
  start_time: "2026-01-01T09:00:00-06:00",
  end_time: "2026-01-01T10:00:00-06:00",
};
const edit = {
  entry,
  keyword: entry.keyword,
  tags: entry.tags,
  startDateTime: new Date(entry.start_time),
  endDateTime: new Date(entry.end_time),
};

test("timestamp changes update the original UUID once, preserving exact instants", () => {
  const startDateTime = new Date("2026-01-01T09:15:23.123-06:00");
  assert.deepEqual(entryUpdateArgs({ ...edit, startDateTime, tags: [] }), [
    "update",
    entry.id,
    "--tags",
    "",
    "--start",
    "2026-01-01T15:15:23.123Z",
    "--end",
    "2026-01-01T16:00:00.000Z",
  ]);
});

test("a metadata edit never resets a running timer's duration", () => {
  assert.deepEqual(
    entryUpdateArgs({
      ...edit,
      entry: { ...entry, end_time: null },
      endDateTime: undefined,
      keyword: "review",
    }),
    ["update", entry.id, "--keyword", "review"],
  );
});

test("clearing an end time explicitly resumes the entry; unchanged forms do nothing", () => {
  assert.deepEqual(entryUpdateArgs(edit), []);
  assert.deepEqual(entryUpdateArgs({ ...edit, endDateTime: undefined }), [
    "update",
    entry.id,
    "--start",
    "2026-01-01T15:00:00.000Z",
    "--end",
    "active",
  ]);
});
