# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

- Create a todo list when working on complex tasks to track progress and remain on track.
- Please avoid reading whole files when not necessary. Instead, use `cat` and pipe into `grep`, for instance, to reduce the amount of text read.
- There is no backward compatibility to maintain: make breaking changes as you wish.
- Please ignore any TODOs or FIXMEs in the code, as they are not relevant to the current context.
- If some code is surrounded by a comment, please take the comment into account before offering changes.
- Please make sure you update any related documentation to changes you are making.
- Always review the current state of the project before creating new functions. Try to reuse code as much as possible to avoid code duplication, and refactor to make this possible when it makes sense.
- Please report on the changes you make to the system as if you were addressing a System Architect.

To build the project, be sure to run from the directory `/Users/sglavoie/1_dev_projects/sglavoie_dev-helpers/dev-helpers/go/gotime` the command `go build -o gt` and run the tool with `./gt`.

Whenever you have to work with a command requiring interactivity (TUI), you will face the following error:

> Error: failed to run selector TUI: could not open a new TTY: open /dev/tty: device not configured

You should make sure to plan for this and work from the code in those cases so as to not depend on the TUI.

Please use `--config gotime.json` to use the configuration file from the current working directory, `/Users/sglavoie/1_dev_projects/sglavoie_dev-helpers/dev-helpers/go/gotime`.
