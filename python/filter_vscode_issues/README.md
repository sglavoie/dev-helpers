# Filter VS Code issues

Trim a VS Code Problems-panel JSON dump down to `resource`, `message`,
`startLineNumber`, `endLineNumber`, deduped and sorted by file + line.

## Usage

```plaintext
pbpaste | filter-vscode-issues
```

Pretty-prints via `jq` if it is on `$PATH`, otherwise falls back to
`json.dumps(indent=2)`.

## Install

```plaintext
cd python/filter_vscode_issues
uv tool install .
```

To pick up local changes later: `uv tool install --reinstall .`.
