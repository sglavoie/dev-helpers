# ShellShelf

## Debugging in GoLand

* Create a new "Go Build" configuration.
* Configuration section:
  * Set the path to `Files` to point to `main.go`.
  * Tick `Run after build`.
  * Set the working directory to the root of the project (`./go/shellshelf`).
  * For the program arguments, pass exactly what would be needed on the command line, e.g. `edit 1 -e` or `add -n 'command name' -c 'echo hello'`.
* Close and apply changes.
* Set a breakpoint in the code.
* Run the configuration.

## Debugging with Delve and GoLand

* Create a new "Go Remote" configuration, with default settings.
* Build the project with `go build main.go`.
* Set a breakpoint in the code.
* Run the debugger by passing the necessary program arguments, e.g. `dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./main -- edit something -e`.
* GoLand will connect to the debugger and stop at the breakpoint when a debugging session starts.