# Filter VS Code issues

Trim a VS Code Problems-panel JSON dump down to `resource`, `message`,
`startLineNumber`, `endLineNumber`, deduped and sorted by file + line.

## Usage

```plaintext
pbpaste | filter-vscode-issues
```

Pretty-prints via `jq` if it is on `$PATH`, otherwise falls back to
`json.dumps(indent=2)`.

## Configuration

The tool reads an optional exclusion config from:

```
~/.config/filter-vscode-issues/config.toml
```

`XDG_CONFIG_HOME` is honoured when set. If the file is absent the tool behaves
exactly as before — all issues pass through.

### `[exclude]` table

Each key is an issue field name; each value is a list of Python regex patterns.
An issue is dropped if **any** pattern matches the corresponding field value
(`re.search` — substring match, case-sensitive; use `(?i)` inline flags for
case-insensitive matching).

```toml
[exclude]
message  = [': Unknown word\.$']   # drop cspell "Unknown word" warnings
resource = ['/node_modules/']      # drop issues inside node_modules
```

### Managing the config file

```plaintext
# Open in $VISUAL / $EDITOR / vi (creates the file with a template if absent)
filter-vscode-issues config edit

# Print the resolved config path
filter-vscode-issues config path
```

`EDITOR="code --wait"` works correctly — the editor string is tokenised with
`shlex.split` before being passed to the subprocess.

## Install

```plaintext
cd python/filter_vscode_issues
uv tool install .
```

To pick up local changes later: `uv tool install --reinstall .`.
