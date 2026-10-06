# Build the resident DDC brightness applier.
#
# Uses the Xcode toolchain explicitly: the Command Line Tools linker cannot
# read the MacOSX26+ SDK stubs ("tapi error: malformed file").

BIN     := ddc-brightnessd
PREFIX  ?= $(HOME)/.local/bin
CC      := xcrun --sdk macosx clang
# ARC is required, not cosmetic: dispatch_main() exits the main thread, which
# drains main's autorelease pool while the main queue lives on. Without ARC the
# cached path globals are autoreleased and go stale the moment the first event
# is handled.
CFLAGS  := -fmodules -fobjc-arc -Wall -Wextra -O2
LDLIBS  := -framework Foundation -framework IOKit \
           -framework CoreGraphics -framework CoreDisplay

.PHONY: all install clean

all: $(BIN)

$(BIN): $(BIN).m
	$(CC) $(CFLAGS) $< -o $@ $(LDLIBS)

# rm first: the target may be a symlink (e.g. a stale stow link), and cp would
# write through it instead of replacing it.
install: $(BIN)
	mkdir -p $(PREFIX)
	rm -f $(PREFIX)/$(BIN)
	cp $(BIN) $(PREFIX)/$(BIN)

clean:
	rm -f $(BIN)
