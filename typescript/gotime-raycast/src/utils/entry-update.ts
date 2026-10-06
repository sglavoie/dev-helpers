interface EntrySnapshot {
  id: string;
  keyword: string;
  tags: string[] | null;
  start_time: string;
  end_time: string | null;
}

export interface EntryEdit {
  entry: EntrySnapshot;
  keyword: string;
  tags: string[];
  startDateTime: Date;
  endDateTime?: Date;
}

/** A complete timestamp edit is one CLI call, addressed by permanent identity. */
export function entryUpdateArgs({
  entry,
  keyword,
  tags,
  startDateTime,
  endDateTime,
}: EntryEdit): string[] {
  const args = ["update", entry.id];
  if (keyword !== entry.keyword) args.push("--keyword", keyword);
  if ([...(entry.tags ?? [])].sort().join(",") !== [...tags].sort().join(",")) {
    args.push("--tags", tags.join(","));
  }
  const startChanged =
    startDateTime.getTime() !== new Date(entry.start_time).getTime();
  const endChanged =
    (endDateTime?.getTime() ?? null) !==
    (entry.end_time ? new Date(entry.end_time).getTime() : null);
  if (startChanged || endChanged) {
    args.push(
      "--start",
      startDateTime.toISOString(),
      "--end",
      endDateTime?.toISOString() ?? "active",
    );
  }
  return args.length === 2 ? [] : args;
}
