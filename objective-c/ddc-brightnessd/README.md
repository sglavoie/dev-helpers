# ddc-brightness - external monitor brightness over DDC/CI

Two pieces that set an external monitor's brightness over DDC/CI on Apple
Silicon Macs, meant to be bound to keyboard keys (e.g. with Karabiner):

- `ddc-brightness`: the command you run or bind to keys. Each call drops a
  request into a queue directory and returns within a few milliseconds.
- `ddc-brightnessd`: an optional background daemon that picks up queued
  requests, adds them together and sends the result to the monitor. Without
  it, `ddc-brightness` applies the changes itself with m1ddc.

It exists to make brightness keys feel instant. Running
[m1ddc](https://github.com/waydabber/m1ddc) for every keypress costs about
74 ms, of which only about 30 ms is the DDC write itself. The rest is process
startup and searching the IORegistry for the display again. The daemon keeps
the display handle open between keypresses, so that overhead goes away.

The DDC and IORegistry code is adapted from m1ddc (MIT, waydabber).

## Requirements

- An Apple Silicon Mac
- [m1ddc](https://github.com/waydabber/m1ddc) and Homebrew's bash
  (`brew install m1ddc bash`). The script is pinned to `/opt/homebrew/bin/bash`
  because Karabiner runs commands with a minimal PATH, where `bash` is macOS's
  3.2, which lacks `EPOCHREALTIME`.
- Xcode, for the daemon. The Command Line Tools linker cannot read the MacOSX26+ SDK stubs, so
  the Makefile builds with `xcrun --sdk macosx clang`.

## Installation

```bash
make                    # build ./ddc-brightnessd
make install            # install ddc-brightnessd and ddc-brightness to ~/.local/bin
make install PREFIX=/usr/local/bin
make clean
```

`make install` deletes the installed files before copying, so it replaces
symlinks at those paths instead of writing through them.

## Usage

```bash
ddc-brightness up [step]      # raise brightness
ddc-brightness down [step]    # lower brightness
ddc-brightness set <0-100>    # set an absolute level
ddc-brightness get            # print the cached level
ddc-brightness sync           # re-read the level from the monitor
```

Example Karabiner action:

```json
"to": [{ "shell_command": "/Users/you/.local/bin/ddc-brightness up 8" }]
```

The current level is kept in a cache file instead of being read from the
monitor each time. Some panels fail a large share of DDC reads, and
`m1ddc chg` jumps to a garbage level when its read fails, so only absolute
writes are sent. Run `sync` if the cache drifts after the monitor's own
buttons are used.

## Configuration

Settings are read from the environment:

| Variable         | Used by | Purpose                                                        |
| ---------------- | ------- | -------------------------------------------------------------- |
| `DDC_DISPLAY`    | both    | Text that must appear in the display's name (default `PA27JCV`); for the daemon, a first command-line argument overrides it |
| `XDG_CACHE_HOME` | both    | Base of the state directory (default `~/.cache`)               |
| `DDC_STEP`       | script  | Default step for `up` and `down` (default 20)                  |
| `DDC_IDLE_MS`    | script  | How long the fallback applier waits for more input before releasing its lock (default 50) |
| `DDC_NOTIFY`     | script  | Set to `1` to show a macOS notification after each change      |
| `M1DDC`          | script  | Path to m1ddc (default `/opt/homebrew/bin/m1ddc`)              |
| `DDC_DEBUG`      | daemon  | When set to any value, log more detail                         |

State is kept in `$XDG_CACHE_HOME/ddc-brightness/<DDC_DISPLAY>/`:

- `queue/`: pending requests
- `level`: the last brightness level that was applied
- `daemon.pid`: the daemon's PID, which clients use to check that it is running

## Queue protocol

Each request is an empty file in `queue/`. The request is encoded in the end
of the file name, after the last `.`:

| Tag   | Meaning               |
| ----- | --------------------- |
| `pNN` | raise brightness by NN |
| `mNN` | lower brightness by NN |
| `aNN` | set brightness to NN   |

Because the request is in the name, the daemon can never read a file that is
only half written. Requests are applied in name order (a plain string sort),
so name them in the order they were created, for example with a timestamp
prefix:
`1759750000.123456.4242.17.p10`. The result is clamped to 0–100.

`ddc-brightness` names its entries `${EPOCHREALTIME}.$$.${RANDOM}.<tag>`.

## Write behaviour

Each level is sent twice, 10 ms apart, which is what m1ddc does. Some panels
drop writes that arrive closer together, and their DDC reads are too unreliable
to confirm that a write worked. Writes are spaced at least 40 ms apart. Once
the bus has been quiet for about 200 ms, the final level is sent once more in
case a write was dropped. A failed write is retried once with a freshly opened
display handle.

## Running as a LaunchAgent

```xml
<key>ProgramArguments</key>
<array>
    <string>/Users/you/.local/bin/ddc-brightnessd</string>
</array>
<key>EnvironmentVariables</key>
<dict>
    <key>DDC_DISPLAY</key>
    <string>PA27JCV</string>
</dict>
<key>RunAtLoad</key>
<true/>
<key>KeepAlive</key>
<dict>
    <!-- Keep it running while the binary exists; launchd starts it once installed -->
    <key>PathState</key>
    <dict>
        <key>/Users/you/.local/bin/ddc-brightnessd</key>
        <true/>
    </dict>
</dict>
<key>ProcessType</key>
<string>Interactive</string>
```

## Without the daemon

When `daemon.pid` doesn't name a live process, the `ddc-brightness` call that
takes an `mkdir` lock becomes the applier. It empties the queue, writes the
total with m1ddc, and keeps emptying it while new requests arrive. Requests
that come in during a write are picked up by the next pass, so none are lost.
Each keypress then costs about 74 ms of m1ddc time instead of a
few milliseconds with the daemon.
