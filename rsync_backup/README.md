[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/python/black)

# rsync backup

## From the script's main docstring

Small script that uses `rsync` to make a simple and convenient backup.

**Note:** requires Python 3.6+ (otherwise, f-strings need to be converted).

For each source to backup, a log file is created at the root directory of that
source. In the log file, the text _Command executed:_ is inserted at the
beginning with the whole `rsync` command that has been executed as a reference.
By default, the following options are passed to `rsync`:

| Option | Description |
| ------ | ----------- |
| `-vaHh` | verbose, archive, hard-links (preserve), human readable format |
| `--delete` | "_delete extraneous files from destination dirs_" |
| `--ignore-errors` | "_delete even if there are I/O errors_" |
| `--force` | "_force deletion of directories even if not empty_" |
| `--prune-empty-dirs` | "_prune empty directory chains from the file-list_" |
| `--delete-excluded` | "_also delete excluded files from destination dirs_" |

### How to use

1) Set all variables in `settings.py` to suit your needs.
2) Make sure that the backup destination is available/mounted.
3) Copy this file somewhere where it will be executed. As an example, I
   put this file in `~/.backup.py` and made the following alias in
   `~/.bash_aliases`:
   > alias backup='/usr/local/bin/python3.7 ~/.backup.py'
4) In this example, the script can now be executed in a terminal with the
   keyword `backup` along with optional arguments.
