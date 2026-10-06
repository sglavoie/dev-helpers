# Utilities for Developers

Small tools built to solve recurring problems: backups, time tracking, snippets,
local monitoring, and everyday terminal work. Each project is independent; build
or install only the tools you use.

## Start here

Commands below run from the linked project directory. Go binaries installed to
`~/.local/bin` need that directory on your `PATH`; create it with
`mkdir -p ~/.local/bin` first. See each tool's README for configuration and runtime
requirements before its first use.

| Tool | Purpose | Environment | Build or install |
| --- | --- | --- | --- |
| [goback](go/goback/README.md) | Incremental backups, mirror previews, and backup history | macOS, Go, rsync | `go build -o ~/.local/bin/goback .` |
| [photos-backup](python/photos_backup/README.md) | Apple Photos archive, SD-card import, SSD/cloud copies | macOS, Python 3.12+, uv; see setup guide | `uv tool install --editable .` |
| [GoTime (`gt`)](go/gotime/README.md) | Concurrent timers, tags, reports, and undo | Go, terminal | `make install` |
| [GoTime for Raycast](typescript/gotime-raycast/README.md) | Start, edit, and review GoTime entries | macOS, Raycast, Node.js, installed `gt` | `npm ci && npm run dev` |
| [Sloppy Paste](swift/sloppy-paste/README.md) | Searchable snippets and placeholder forms | macOS 15+, Swift 6.2+, just | `just setup-signing`, then `just install` |
| [Heartbeat](swift/heartbeat/README.md) | Menu-bar health monitoring for local LaunchAgents | macOS 15+, Swift 6.2+, just | `just setup-signing`, then `just install` |
| [gitbrief](python/gitbrief/README.md) | Summarize git activity across repositories | Python 3.13+, uv, git; Claude or Copilot CLI for AI summaries | `uv tool install --editable .` |
| [ShellShelf (`ss`)](go/shellshelf/README.md) | Save, find, copy, and run shell commands | Go; macOS or Linux for clipboard support | `go build -o ~/.local/bin/ss .` |
| [CCL](go/ccl/README.md) | Expected weekly Claude usage pace | Go, terminal | `go build -o ~/.local/bin/ccl .` |
| [DDC brightness](objective-c/ddc-brightnessd/README.md) | Responsive external-monitor brightness keys | Apple Silicon macOS, Homebrew bash/m1ddc, Xcode for daemon | `make install` |
| [Health Records (`hr`)](go/hr) | Log exercises to CSV and review stats/streaks | Go, terminal | `go build -o ~/.local/bin/hr .` |
| [jsonc2json](go/jsonc2json) | Remove JSONC comments and trailing commas | Go, terminal | `go build -o ~/.local/bin/jsonc2json .` |
| [filter-vscode-issues](python/filter_vscode_issues/README.md) | Trim and deduplicate exported VS Code diagnostics | Python 3.11+, uv | `uv tool install --editable .` |

## Related tools and predecessors

- [goback](go/goback/README.md) is the Go successor to
  [rsync_backup](python/rsync_backup/README.md).
- [Sloppy Paste](swift/sloppy-paste/README.md) replaces the
  [AI Sloppy Paste Raycast extension](typescript/ai-sloppy-paste/README.md).
  The native app's README covers importing existing snippets.
- [GoTime for Raycast](typescript/gotime-raycast/README.md) uses the GoTime CLI
  and its data file; it is a companion interface.
- [photos-backup deployment](python/photos_backup/docs/deployment.md) describes
  running photo exports as a goback daily companion.

## Other focused scripts

These have their own setup and dependencies; follow the linked README or inspect
the script before running it.

- Files: [download directory manager](python/download_directory_manager/README.md),
  [deduplicate files](python/dedupe_files),
  [HAR media downloader](python/har_media_downloader).
- Conversion: [JSONL to CSV](python/jsonl_to_csv_converter/README.md),
  [learning logs to Markdown](python/learning-logs-to-markdown/README.md).
- Shell history: [Bash cleaner](python/bash_history_cleaner/README.md),
  [Zsh cleaner](python/zsh_history_cleaner/README.md).
- Password utilities: [validator](python/password_validator/README.md),
  [Have I Been Pwned checker](python/haveibeenpwnwed_password_checker/README.md).

## Development

There is no repository-wide build or install step. Run checks in the project you
changed: Go tools generally use `go test ./...`; the Swift apps use `just test`;
Python projects and Raycast extensions document their checks in their READMEs.
