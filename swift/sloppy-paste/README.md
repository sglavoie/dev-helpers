# Sloppy Paste

A native macOS app for storing, searching and pasting text snippets. It replaces
the [AI Sloppy Paste](../../typescript/ai-sloppy-paste) Raycast extension and
keeps its data format, tags, `{{…}}` placeholder grammar and keyboard shortcuts.

A global hotkey (⌃⌥⌘V by default) opens a Spotlight-style picker. Choosing a
snippet pastes it into the app that was frontmost. The app lives in the menu bar
and has no Dock icon.

## Requirements

- macOS 15 or later.
- The Xcode Command Line Tools with Swift 6.2 or later. Xcode itself is not
  needed. Tests use Swift Testing, because XCTest ships only with Xcode.
- [`just`](https://github.com/casey/just).

## Build

See the available recipes:

```bash
just
```

| Recipe | What it does |
| --- | --- |
| `build` | Debug build of every target. |
| `test` | Run the SloppyCore test suite (`swift test`). |
| `test-paste-target-safety` | Run only the paste-back target safety tests. |
| `bundle` | Assemble an unsigned `SloppyPaste.app` in `.build/bundle`. |
| `sign` | Bundle, then sign it with the local signing identity. |
| `install` | Sign, replace `~/Applications/SloppyPaste.app` and launch it. |
| `run` | Sign and launch the app straight from `.build/bundle`. |
| `uninstall` | Quit the app and remove it from `~/Applications`. |
| `show-dr` | Print the designated requirement of the built app. |
| `setup-signing` | One-off: create the local code-signing identity. |
| `clean` | Remove build artifacts. |

The package has three targets:

- `SloppyCore`: Foundation-only models, placeholder engine, search, tags and
  storage. All the logic lives here and is covered by the tests.
- `SloppyPaste`: the AppKit and SwiftUI app.
- `sloppyctl`: a small command-line tool on top of SloppyCore (see
  [Migration](#migration-from-raycast)).

The `.app` bundle is put together by hand from `Resources/Info.plist.in` and
`Resources/icon.png`, since there is no Xcode project. The app target uses no
SwiftPM resources for the same reason.

## Signing

Accessibility permission is tied to the app's designated requirement. With
ad-hoc signing that requirement changes on every build, so macOS would forget
the permission after each rebuild. To avoid that, the app is signed with a
stable self-signed identity:

```bash
just setup-signing   # once per machine
just install         # every time you want the latest build
```

`setup-signing` creates the "Sloppy Paste Local Signing" certificate in its own
keychain, `~/Library/Keychains/sloppy-paste-signing.keychain-db`, and trusts it
for code signing. It never replaces an existing keychain, because a new
certificate would change the designated requirement. `just sign` unlocks that
keychain on its own, including after a reboot.

To check that a rebuild kept the same identity, compare `just show-dr` before and
after. The output should be identical, naming `dev.sglavoie.SloppyPaste` and the
same certificate leaf hash.

## Permissions

### Accessibility

Pasting posts a ⌘V key press to the frontmost app, and that needs Accessibility
permission. On first launch the app asks for it. You can also grant it later from
the menu bar item, from Settings → Pasting, or directly in System Settings →
Privacy & Security → Accessibility.

Until the permission is granted, the app runs in copy-only mode. Choosing a
snippet copies it to the clipboard, a HUD says so, and the picker footer shows a
warning. The app notices the grant on its own, with no restart needed. It also
falls back to copy-only while Secure Input is on, for example in a password
field.

If macOS stops recognising the app, for example after the signing keychain was
recreated, reset the entry and grant it again:

```bash
tccutil reset Accessibility dev.sglavoie.SloppyPaste
```

### Global hotkey

The hotkey uses Carbon's `RegisterEventHotKey`, which needs no permission. You
can change it in Settings → General. If another app already uses the chord,
Settings says it is not available and the previous hotkey stays active.

### Launch at login

Turn it on in Settings → General. It uses `SMAppService`, so it only works from
the signed bundle, ideally the copy in `~/Applications`. If macOS wants approval,
Settings links to System Settings → General → Login Items.

## Data

Snippets are stored in `~/Library/Application Support/SloppyPaste/data.json`.
It uses the same `StorageData` JSON shape as the Raycast extension's export.

- Writes are atomic and pretty-printed.
- A replace-import first copies the current file to `data.json.bak`.
- Editing the file by hand is fine: the app reloads it when it changes.
- A file that fails to decode is never overwritten. It is moved aside as
  `data.corrupt-<timestamp>.json`, the app starts empty, and an alert explains
  how to restore it.

Settings → Data shows the path, with a Reveal button, and the storage size.

## Migration from Raycast

1. In Raycast, open **Manage Snippets** and run **Export All Snippets** (⇧⌘E).
   This writes a JSON file.
2. Optionally, check the export before importing it:

   ```bash
   swift run sloppyctl stats --data ~/Downloads/<export>.json
   ```

3. Import it, with either:
   - the app: menu bar item → **Import…**, or ⇧⌘I in the picker. Merge adds
     snippets that aren't there yet; Replace overwrites everything after writing
     `data.json.bak`.
   - the command line, with the app not running:

     ```bash
     swift run sloppyctl import ~/Downloads/<export>.json            # merge
     swift run sloppyctl import ~/Downloads/<export>.json --replace  # replace
     ```

4. Compare the counts with the export:

   ```bash
   swift run sloppyctl stats
   jq '.snippets | length' ~/Downloads/<export>.json
   ```

5. Turn off the Raycast hotkey: Raycast Settings → Extensions → AI Sloppy Paste
   → Manage Snippets → clear the hotkey. Then disable or uninstall the extension.
   If you want the new app on the same chord, set it in Sloppy Paste's Settings
   after clearing it in Raycast.

Legacy exports that use a single `category` instead of tags are migrated on
import, and the category becomes a tag. Importing the same file twice with Merge
adds no duplicates, because snippet IDs keep the extension's
`snippet-<ms>-<base36>` format.

## sloppyctl

```bash
swift run sloppyctl --help
```

- `import`: merge or replace from an export.
- `stats`: counts that match simple `jq` queries; `--json` is available.
- `search`: the picker's query syntax (`tag:`, `not:tag:`, `ctx:`, `not:ctx:`,
  `is:`, `not:`, quoted phrases, fuzzy words).
- `render`: expand a snippet's placeholders with `key=value` pairs.

Every command takes `--data <path>` to work on a file other than the live one.

## Troubleshooting

- **Paste lands nowhere, or only copies.** Check Accessibility in Settings →
  Pasting. If the target app is slow to take focus, raise the paste delay (60 ms
  by default) in the same section. The app also only copies when the app you
  opened the picker from quit or lost focus before ⌘V, so it never pastes into
  a different app.
- **Accessibility is granted but ignored after a rebuild.** Compare
  `just show-dr` with an earlier build. Then run the `tccutil reset` command
  above and grant the permission again.
- **`swift test` can't find `TestingMacros`.** `Package.swift` already passes
  the Command Line Tools plugin path. Make sure
  `xcode-select -p` points at `/Library/Developer/CommandLineTools`.
