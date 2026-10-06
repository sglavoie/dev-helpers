# ddc-brightnessd - resident DDC/CI brightness applier

A small background daemon for Apple Silicon Macs that sets an external
monitor's brightness over DDC/CI. Brightness requests are dropped into a queue
directory; the daemon picks them up, adds them together and sends the result to
the monitor.

It exists to make brightness keys feel instant. Running
[m1ddc](https://github.com/waydabber/m1ddc) for every keypress costs about
74 ms, of which only about 30 ms is the DDC write itself. The rest is process
startup and searching the IORegistry for the display again. The daemon keeps
the display handle open between keypresses, so that overhead goes away.

The DDC and IORegistry code is adapted from m1ddc (MIT, waydabber).

## Requirements

- An Apple Silicon Mac
- Xcode. The Command Line Tools linker cannot read the MacOSX26+ SDK stubs, so
  the Makefile builds with `xcrun --sdk macosx clang`.

## Installation

```bash
make                    # build ./ddc-brightnessd
make install            # install to ~/.local/bin
make install PREFIX=/usr/local/bin
make clean
```

`make install` deletes the installed file before copying, so it replaces a
symlink at that path instead of writing through it.

## Configuration

Settings are read from the environment:

| Variable         | Purpose                                                        |
| ---------------- | -------------------------------------------------------------- |
| `DDC_DISPLAY`    | Text that must appear in the display's name (default `PA27JCV`); a first command-line argument overrides it |
| `XDG_CACHE_HOME` | Base of the state directory (default `~/.cache`)               |
| `DDC_DEBUG`      | When set to any value, log more detail                         |

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

Example client:

```bash
q=~/.cache/ddc-brightness/PA27JCV/queue
mkdir -p "$q"
: > "$q/${EPOCHREALTIME}.$$.p10"    # +10
: > "$q/${EPOCHREALTIME}.$$.a40"    # set 40
```

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

Clients should still work when the daemon is not running, for example by
calling m1ddc directly when no live process is listed in `daemon.pid`.
