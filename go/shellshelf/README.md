# ShellShelf

Save shell commands with names, descriptions, and tags, then find, copy, edit,
or run them from the terminal. Commands and settings live in `~/.shellshelf.json`.

## Install and use

From this directory, with Go installed:

```bash
mkdir -p ~/.local/bin
go build -o ~/.local/bin/ss .
ss --help
ss add --name status --command 'git status --short'
ss find
```

Add `~/.local/bin` to your `PATH`. `ss find` opens the interactive command picker
and offers Run, Copy, Edit, and Print. Clipboard copying uses `pbcopy` on macOS,
or `xclip`/`xsel` on Linux.

## Development

```bash
go test ./...
just --list
```

The justfile includes build, test, format, and lint recipes. Linting requires
`golangci-lint`. There is no npm or web-server setup for the CLI.

### Debugging in GoLand

- Create a new "Go Build" configuration.
- Configuration section:
  - Set the path to `Files` to point to `main.go`.
  - Tick `Run after build`.
  - Set the working directory to the root of the project (`./go/shellshelf`).
  - For the program arguments, pass exactly what would be needed on the command line, e.g. `edit 1 -e` or `add -n 'command name' -c 'echo hello'`.
- Close and apply changes.
- Set a breakpoint in the code.
- Run the configuration.

### Debugging with Delve and GoLand

- Create a new "Go Remote" configuration, with default settings.
- Build the project with `go build main.go`.
- Set a breakpoint in the code.
- Run the debugger by passing the necessary program arguments, e.g. `dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./main -- edit something -e`.
- GoLand will connect to the debugger and stop at the breakpoint when a debugging session starts.
