# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
just build        # compile the binary
just install      # build and copy to ~/.local/bin/goback
just clean        # remove the compiled binary
just uninstall    # remove from ~/.local/bin
```

Tests live beside the code they cover and run with `go test ./...`.

## Architecture

goback is a CLI backup tool that wraps `rsync` for incremental daily, weekly, and monthly backups. It targets macOS (uses `diskutil eject` for volume unmounting) and uses Cobra for CLI structure, Viper for configuration, SQLite for backup history, and Bubbletea for interactive prompts and paged output.

### Command layer (`cmd/`)

All commands are Cobra subcommands registered under `RootCmd`. The `PersistentPreRun` hook on the root command calls `config.MustInitConfig` before every subcommand, but skips config validation for `config` subcommands to avoid a chicken-and-egg problem.

The main commands are `run daily|weekly|monthly|all` (execute backups), `preview daily|weekly|monthly` (print the rsync command without running it), `mirror` (mirror one configured directory onto another), `config edit|print|reset`, `clean db|logs|backup`, `usage last|view|reset`, and `eject [--all|--volume NAME|--list]`. Preview supports `--test-pattern`, `--excluded`, `--subdir`, and `--depth` to try exclude patterns against the source.

Commands declare whether they need an active profile through the `profileResolution` cobra annotation in `profileresolution.go`. The default is `profileRequired`; `mirror` is `profileNotRequired` because it reads a global configuration block, and `eject` is `profileUnlessAll` because `--all` acts on everything the configuration knows about and `--volume` and `--list` name what they act on without one. This is what keeps those two usable on a machine whose hostname matches no profile.

### Backup flow

Initialization and profile iteration share `config.SelectProfiles`, including the sole-profile fallback and the error for an empty selection. `config.ValidateCompanionPlacement` rejects top-level `dailyCompanions` on operational commands; config editing and printing skip this validation so the user can repair the file.

The backup pipeline flows through three stages. First, `pkg/buildcmd` constructs the rsync command: `BuildDaily/Weekly/Monthly` factory functions read source/destination from Viper config, validate paths, and assemble the command string from per-type boolean flags and exclude patterns. Daily backups copy from the configured source to `<dest>/daily/`. Weekly and monthly copy from `<dest>/daily/` to `<dest>/weekly/` or `<dest>/monthly/`, treating the daily snapshot as input. Second, `pkg/run` orchestrates execution by calling the builder, checking that rsync exists, showing a confirmation prompt (if `confirmExec` is true), and running the command via `bash -c` under a signal context created once per `run` invocation. Third, the result is recorded in SQLite.

Ejection is not part of `pkg/run`. `cmd/profiles.go` drives every profile of a snapshot command through `forEachProfile`, collects the destination of each profile that succeeded, and calls `eject.EjectPaths` once at the end when `ejectOnExit` is true.

`pkg/run` classifies the main rsync as skipped, declined, interrupted, failed, or succeeded, and the daily companions run only when rsync was actually attempted, whether it succeeded or failed. A build or path failure, a missing rsync, a declined confirmation, and an interruption all skip the companions. A companion never changes the exit status: `run daily` fails only when its own rsync failed, while every companion outcome is recorded in history and shown in the combined table each profile prints once its companions have finished (replacing the history summary that used to be printed there). `run all` calls the same daily entry point, so companions behave identically there, and `preview daily` lists the companions with the exact argument vector of a real run and of a dry run. A companion runs with its `dryRunArgs` when the backup is a dry run from either `--dry-run` or `rsync.daily.dryRun`.

### Mirror (`pkg/mirror`)

Mirror `Command` carries `Argv` and `Dir` through both process adapters and is embedded in `Plan` and `Result`. For the final preflight and transfer, `Dir` is the bound destination and the destination argument is `.`; the source argument is its bound name. This avoids rsync 3.5's component-wise walk of the non-traversable `/.vol/<dev>` intermediate path. The adapters resolve executables before setting the child directory, and command display/history includes the working directory.

`goback mirror` makes a destination an exact copy of a source. It shares nothing with the snapshot flow: no profile, no `pkg/buildcmd`, no `bash -c`, and no ejection.

The rsync policy is fixed in `argv.go` and not configurable: `--archive --hard-links --acls --xattrs --crtimes --delete --delete-delay --partial-dir=.goback-partial`, plus a machine-readable `--out-format` and `--stats`. `--checksum`, `--ignore-errors`, `--inplace`, `--delete-during`, and `--delete-before` are deliberately absent. `capability.go` parses `rsync --version` and refuses an rsync that lacks hard links, ACLs, xattrs, or crtimes rather than degrading silently, so the rsync shipped with macOS does not qualify.

A mirror runs in two phases. `Validate` (`validate.go`) checks every precondition without writing: absolute, distinct, non-nested paths; a source that exists, is a readable directory, and holds something the mirror would actually transfer, which is not the same as being non-empty because rsync excludes `.goback-partial` itself; a destination that already exists and is a writable directory, since the mirror never creates one; neither endpoint a stale `/Volumes/<name>` directory on the system disk; neither endpoint resolving through a symlink out of its `/Volumes/<name>` mount point, since `Stat`, the device check, and rsync itself all follow symlinks; and the two endpoints on different filesystems. `DryRun` (`plan.go`) then produces a `Plan` by running rsync with `--dry-run` and parsing its itemized output at fixed offsets (`parse.go`), which is what both `--dry-run` and a real run print. `Mirror` (`execute.go`) takes the destination's exclusive lock, then asks the `Approver` for a confirmation that defaults to No, always, because `confirmExec` is not read here. After approval it binds both endpoints and computes a second plan *through those bindings*, and `stillApproved` refuses to write when a device identity changed, when the destination resolves to another directory than the one the lock was taken on, when the destination stopped being the directory it was even though its path and its device are unchanged, or when the destination gained deletions the user never reviewed. The transfer is a third rsync process that scans the destination on its own, so `destinationUnchanged` lists the bound destination before the revalidation and again just before the transfer and refuses when anything appeared in between, `.goback-partial` excepted. `endpointsStillResolve` also resolves the configured endpoints once more and refuses unless they still resolve where `Validate` accepted them, which is what reports a source leaf or a destination leaf swapped for an escaping symlink after the revalidation.

The mirror is bound rather than checked. `Validate` records what each endpoint *is* and not only where it is: `Devices.Identity` names the source and the destination as `<dev>/<ino>`, stored on `Endpoints` as `SourceIdentity` and `DestinationIdentity`. `Binder.Bind` takes a path *and* the identity the preflight recorded, opens the path with `O_DIRECTORY|O_NOFOLLOW`, and refuses unless the directory it opened is that exact identity, so a directory substituted at the path for the instant of the binding alone is refused instead of bound and then restored. The fd stays open until the transfer is over so the inode cannot be reused underneath it, and `osBinder` also refuses a volume whose `/.vol/<dev>/<ino>` name does not lead back to the directory it opened. Both endpoints are bound immediately after the approval, before anything is observed again.

The mirror creates nothing, and that is what makes every binding a comparison against a recorded identity. `validateDestination` refuses an absent destination outright and tells the user to create it: macOS has no atomic create-and-open for directories, so a creation would have to look its own result up by name, and nothing the kernel offers distinguishes the directory it made from one another process running as this user put at that name in the meantime. `ATTR_CMN_GEN_COUNT` cannot close that gap either, since `getattrlist(2)` permits comparing a generation value with an earlier one for equality only and defines no meaning for the number itself, so an arithmetic delta is not proof that exactly one modification occurred. There is therefore no `DirectoryMaker`, no staging-and-publish, and no generation read anywhere in the package; `TestOSDirectoryMakerDoesNotInferModificationCountFromOpaqueGenerationValues` in `deps_test.go` keeps it that way by failing when a non-test file mentions `Mkdir`, `Mkdirat`, `getattrlist`, or a generation count.

`dryRunBound` runs the final preflight on the two bound names, and the destination walks use them too, which is why a directory that stands in for an endpoint only while rsync scans it can no longer make the approved plan describe one directory while the transfer acts on another: the bound plan reports the validated directory, and its deletions are then compared with the reviewed ones. The transfer uses those same bindings through its source argument and working directory, which is why replacing an endpoint after the binding, including at the instant `Streamer.Stream` starts rsync, redirects nothing: rsync opens its own arguments later than any check the mirror can make, and only a name that survives the replacement closes that window. Root remains outside the mirror's trust boundary: it can act on the volume and on the process in ways no check the mirror makes could survive.

`destinationStillLocked` runs between the approval and the binding, since from there on the mirror revalidates whatever was bound: it re-reads the identity of the resolved destination and refuses a path that came to hold another ordinary directory while the prompt was on screen, before that directory can be bound and transferred into under a lock taken for a different one.

The last listing `destinationUnchanged` performed is also what bounds the transfer's deletions. `DeletionRules` turns it into rsync filter rules, one `R /<path>` per entry the mirror walked followed by a terminating `P /**`, written by a `RuleWriter` to a throwaway file rsync merges through `--filter=merge <path>`. `--from0` makes that file null-delimited, so an entry whose name holds a newline stays one rule rather than splitting into two, and `filterPattern` backslash-escapes `*`, `?`, and `[` in the names that contain them, since rsync honors an escape only in a pattern that already looks like a wildcard. The effect is that rsync's own destination scan, which happens after every listing the mirror made, may delete exactly what the mirror saw and nothing else: content another writer adds in that last window survives instead of being deleted unreviewed. Filter rules affect `--delete` only, never what is transferred.

The lock is `flock` on a file named after the resolved destination in the temporary directory, never inside either endpoint, since anything inside the destination is content the mirror deletes. Hashing the resolved path rather than the configured one is what makes two symlink aliases of one directory take the same lock, and `stillApproved`'s resolved-destination comparison is what keeps the lock held equal to the destination the transfer uses when an alias is retargeted during the prompt. It is taken before the prompt and released when `Mirror` returns, and it never waits: a second mirror of the same destination is refused rather than queued behind an approval nobody may answer.

Dependencies split by capability: `Deps` (`FS`, `Capacity`, `Devices`, `Clock`, `Runner`) is read-only and is all a preflight ever receives, while `ExecDeps` (`Approver`, `Locker`, `Rules`, `Binder`, `Streamer`) carries the write-capable seams. Nothing in either set creates a directory, so no preflight and no transfer can materialize an endpoint. Cancellation sends SIGTERM and then kills after `killGrace`; the partial data stays in `.goback-partial` for the next run.

### Ejection (`pkg/eject`)

`Eject` unmounts the volume holding the active profile's destination, `EjectPaths` unmounts the destinations a snapshot run collected, and `All` unmounts every path it is given, which is what `eject --all` hands `config.ConfiguredEndpoints` to. All three funnel through `ejectVolume`, which resolves a path to its `/Volumes/<name>` mount point (`volumes.go`), skips anything outside `/Volumes`, and unmounts each volume once in mount-point order. A volume that is not mounted is an absent outcome rather than a failure, one refusal never stops the others, and only a mounted volume that could not be ejected makes the command exit nonzero.

`eject --list` never reaches `ejectVolume`: `MountedVolumes` (`list.go`) groups `config.ConfiguredEndpointRefs`, which keeps each path's owning profile or mirror and its source/destination role, by mount point and keeps the mounted ones; `cmd/eject.go` renders them and suggests `--profile NAME` for profile destinations and `--volume NAME` for volumes no profile destination is on. `eject --volume` resolves its argument through `ConfiguredVolume` (`volumes.go`), which accepts only a bare name or exact mount point of a volume `config.ConfiguredEndpoints` references, then hands that one volume to `EjectPaths`. `osMounts` (`deps.go`) is shared by listing and ejecting and treats a volume as mounted only when its root is a non-symlink directory on a different device than its parent, which excludes stale `/Volumes/<name>` directories.

### Backup type system (`pkg/models`)

`BackupTypes` is an interface implemented by four empty structs (`Daily`, `Weekly`, `Monthly`, `Mirror`) plus `NoBackupType`. These act as a Go-idiomatic enum with `String()` methods and flow through the entire stack to parameterize config lookups, DB queries, and path construction.

### Configuration (`pkg/config`)

Config lives at `~/.goback.json` and is read via Viper; `--config <path>` points any command at a different file, which is read as-is and never rewritten when it already exists. The top-level keys are `confirmExec`, `ejectOnExit`, `showProgress`, `editor`, a `mirror` object, and a `profiles` object. Defaults are set in `defaults.go`, which writes `profiles.default.rsync.daily.*`, `profiles.default.rsync.weekly.*`, and `mirror.*`. Monthly has no dedicated defaults and must be added manually by the user.

Backups are organized under `profiles`: each profile owns its `source`, `destination`, `hostname`, `rsync` settings, and optional `dailyCompanions`. The active profile is the one whose `hostname` matches, or the one named by `--profile`; `--all` covers every profile. `config.ConfiguredEndpoints` returns every path the configuration points at, each profile's source and destination in sorted profile order followed by the mirror's, deduplicated and never touched on disk.

The mirror is configured separately as a single top-level block with `source` and `destination`, both full paths and both required. It belongs to no profile, and nothing falls back to a compiled default: `config.DefaultMirrorSource` and `config.DefaultMirrorDestination` only seed a generated configuration.

A profile may also declare `dailyCompanions`, a list of programs run alongside its daily backup. Each entry has `id` (letters, digits, dots, dashes and underscores, unique within the profile), an optional `name` for display, `command` (the program followed by each argument as a separate list item), and optional `dryRunArgs` appended when the run is a dry run. `companions.go` parses and validates them; a command written as one shell string is rejected because companions are executed without a shell.

The Apple Photos companion this repository ships alongside is configured as:

```json
"dailyCompanions": [
  {
    "id": "apple-photos",
    "name": "Apple Photos",
    "command": ["photos-backup", "daily"],
    "dryRunArgs": ["--dry-run"]
  }
]
```

`photos-backup` is the console script of `python/photos_backup`, resolved through `PATH`, and `pkg/run/installed_cli_test.go` runs a real executable of that name from a temporary `PATH` entry to prove the resolution and the argument vector across process boundaries.

### Database (`pkg/db`)

SQLite database at `~/.goback.db` with a single `backups` table storing id, created_at, backup_type, execution_time, command, profile, and exit_code. `backup_type` is `daily`, `weekly`, `monthly`, `mirror`, or `companion/<id>` for a companion program. A mirror belongs to no profile, so it is recorded under the `global` profile (`db.MirrorProfile`) and only when rsync was actually started; `usage view --mirror` and `usage reset --mirror` are what read and trim those rows, and filtering history by profile never shows them. Every write goes through `db.RecordBackup`, which logs a warning rather than failing the backup. The `WithDb` callback pattern opens, passes, and closes the connection. `WithQuery` closes the connection before rows are consumed, so rows must be read within a `WithRows` callback.

### Interactive UI

Bubbletea is used for yes/no confirmation prompts (`pkg/inputs`), a full-screen scrollable pager (`pkg/printer`), and config display. The `go-pretty` library renders tabular backup history.
