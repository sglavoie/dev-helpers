# Archive safety and cleanup

[Back to the user guide](../README.md)

## Archive layout and safety

Everything the archive owns lives under one hidden directory inside
`apple_photos.archive`:

| Path | Contents |
|------|----------|
| `.photos-backup/export.db` | osxphotos export database |
| `.photos-backup/state.json` | versioned run state |
| `.photos-backup/last-export-attempt.json` | latest export attempt, separate from successful baseline state |
| `.photos-backup/archive.lock` | `flock` held for the duration of a run |
| `.photos-backup/reports/` | per-host export and late-additions reports |
| `.photos-backup/migrations/` | last-known-good export-database backup |
| `.photos-backup/cleanup/` | pending cleanup manifests |

Before any write, `apple_photos.volume` must exist, be a real mount point, not be
a symlink, and resolve to itself; the archive must descend from it through real
directories only. A missing volume is never created, and only the archive subtree
is. Paths containing `..`, symlinked components, or a component that is not a
directory fail closed with an error naming the offending path.

A volume named with `--volume` waives the mount-point requirement only, so an
ordinary directory such as `~/Pictures/archive` is accepted. Every other check
still applies, including having to exist already: a mistyped `--volume` fails
instead of quietly exporting into a new tree.

One run at a time holds an exclusive `flock` on `archive.lock`, released when the
run ends or the process dies, so runs cannot overlap. State is replaced
atomically and rejects an unrecognized version, an unknown key, a naive
timestamp, or unparseable JSON rather than guessing.

State version 2 stores report paths beneath the archive relative to its root,
so `status` and `verify` find them after a mount name or archive location changes.
Version 1 remains readable: absolute paths to managed `.photos-backup/reports/`
files are interpreted beneath the current archive. Other historical absolute
report references retain their original location. Read-only commands do not
rewrite state; the next state write saves version 2. Update photos-backup on
every Mac sharing the archive before a newer writer saves state: older builds
reject version 2. This change concerns report references, not relocation of
other archive metadata such as pending cleanup manifests.

A `--dry-run` persists nothing: it creates no directory, no lock file, and no
state, and takes only a read lock when a lock file already exists.

## Mirroring deletions

Before approving a pending run, preview its current effect:

```sh
photos-backup approve-cleanup RUN_ID --dry-run
```

The preview applies the same owner, manifest, and library revalidation checks as
approval. It lists the paths and total bytes that would be deleted, plus changed,
restored, or unreviewed candidates that would be kept. It creates no lock file,
changes no state, and leaves the cleanup pending. Approval revalidates again,
so a preview does not authorize later changes. `--dry-run` cannot be combined
with `--discard`.

Once an export is done, `daily` reconciles the archive against the library so a
photo deleted in Photos eventually leaves the archive too. Nothing is ever
deleted by osxphotos itself: `--cleanup` is never passed, and every deletion goes
through the check below.

Reconciliation is skipped outright, leaving the archive untouched, when
`apple_photos.mirror` is `false`, the run was incremental, the export was not
clean, or assets are missing from iCloud — an export that does not describe the
whole library may not decide what the library no longer has.

Otherwise every export-database record absent from the library is mapped back to
the files it wrote, and each file in the archive is classified:

| Class | Meaning |
|-------|---------|
| candidate | Exactly one absent asset claims the file, and it still matches the recorded size and modification time |
| changed | An absent asset claims it, but the file on disk no longer matches what osxphotos recorded |
| ambiguous | More than one export-database asset claims the same path |
| unknown | No export-database record claims the file at all |

`.DS_Store`, `.localized`, and AppleDouble `._` files are ignored, so browsing
the archive in the Finder never blocks a cleanup. Deletion happens automatically
only when there is nothing changed, ambiguous, or unknown, and the absent assets
stay within both `cleanup_max_assets` (default 10) and `cleanup_max_fraction`
(default 0.1%). Directories those deletions emptied are then pruned (other
empty directories are left alone), and
`last_mirror_completed_at` advances only when the mirror is actually complete.

Anything else is written to `.photos-backup/cleanup/RUN_ID.json`, recorded in
`state.pending_cleanup_run_id`, and the run exits 3 naming the manifest. That
manifest lists the exact candidates with their signatures alongside the changed,
ambiguous, and unknown paths, and is never rewritten, so the set being approved
is the set that was reviewed. While a run is pending, later `daily` runs still
export normally but neither recompute nor replace it; they exit 3 until it is
resolved.

`photos-backup approve-cleanup RUN_ID` deletes those files, but only after
recomputing the reconciliation from scratch. Invoking it with the run ID is the
approval: it does not prompt or require a terminal, so review the manifest first.
It refuses a run that is not the pending one, a manifest that is gone or
malformed, and a Mac that does not own
the archive. A file is deleted only when its path, UUID, size, and modification
time still match the manifest exactly, so a replacement whose export-database
signature was refreshed too is reported as kept rather than deleted. Any other
candidate, including one that appeared after the manifest was written, is left
for the next full export to propose, which keeps the mirror open until a person
has seen every deletion.

`photos-backup approve-cleanup RUN_ID --discard` is the other way out: it clears
the pending run without deleting anything and leaves the manifest on disk as the
record of what was rejected. Because it touches no file, any Mac that can read
the archive may discard, while only the owning Mac may approve. A later full
export proposes whatever is still deletable under a new run id.

## Deleting the legacy local export

`~/Pictures/export` predates the archive and is not imported into it. Once the
archive holds everything, `photos-backup cleanup-local-export` empties that
directory — and nothing else. It has no path argument: the target is always
`apple_photos.legacy_export`, so no invocation can aim it elsewhere.

Every one of these must hold before a single file is deleted:

| Gate | Refused when |
|------|--------------|
| terminal | stdin is not a terminal, so a script, a scheduled job, or a test can never reach the deletion |
| mount | `apple_photos.volume` is not a real, resolved mount point |
| bootstrap | the archive has no `initialized_at` |
| verification | `verify` fails any check in this same run |
| target | the directory is the filesystem root, the home directory, the archive, the archive volume, a Photos library, or a parent of any of them |
| contents | anything under it is a symlink, a Photos library, or a file an Apple Photos export does not write |
| confirmation | the full directory path is not typed back at the prompt |

Recognized contents are media and sidecar files (`.jpg`, `.heic`, `.dng`, `.mov`,
`.aae`, `.json`, `.xmp`, and the rest), the `.osxphotos_export.db` files, and the
`.DS_Store`, `.localized`, and AppleDouble `._` files macOS leaves behind. One
unrecognized file refuses the whole run rather than being skipped, because a
directory holding something unexplained is not the directory this command was
built to delete.

The prompt names the directory, the file count, and the total size before asking.
Typing anything but the exact path cancels and exits 0 — a refused confirmation
is not a failure. The directory itself is kept; only its contents and the
directories they emptied are removed. `--dry-run` prints the same report,
prompts for nothing, and deletes nothing.

## Cross-Mac takeover

The archive records the hostname that last wrote it in `state.writer_hostname`.
On macOS this is the configured `LocalHostName` (read with
`scutil --get LocalHostName`) plus `.local`, not the network-dependent hostname.
Tailscale, DNS, and DHCP changes therefore do not trigger a takeover. If the
local name cannot be read, the command stops instead of using a network name.
When that name still matches, `daily` costs one state read and never opens the
Photos library for a takeover check. When it differs — a second Mac, or a Mac
whose configured local name changed —
the run first proves that this Mac holds the same photos.

Every export-database record is matched to a library asset on
`original_filename` plus iCloud cloud GUID, which survives the per-library UUIDs
that Photos assigns. The run stops with exit 3, having written nothing, when:

| Situation | Reading |
|-----------|---------|
| No record carries a cloud GUID | the library cannot be identified |
| Not one record exists in the library | this is a different library |
| Absence exceeds `cleanup_max_assets` or `cleanup_max_fraction` | the library is incomplete |

Below those limits the absent records become deletion candidates rather than a
reason to stop. If matched assets carry new UUIDs, the export database is
repointed at this library with `osxphotos exportdb --migrate-photos-library`.
That runs only when a UUID actually changed, so an unchanged writer and a
returning Mac both skip it.

Before migrating, the live database is copied to
`.photos-backup/migrations/export.db.last-known-good` through the SQLite backup
API, so an uncheckpointed write-ahead log travels with it. The migration must
leave no record still pointing at the previous library; a failure or a stale
record restores that backup and exits 3. `state.writer_hostname` is only written
after the migration succeeds, under the same archive lock, so an interrupted
takeover is simply retried.

A `--dry-run` reports the comparison and the migration it would run without
copying, migrating, or claiming the archive.

