# rsync backup

### Table of contents

- [Introduction](#introduction)
- [Description of available settings in `settings.json`](#description-of-available-settings-in-settingsjson)
- [How to use](#how-to-use)
- [Help menu](#help-menu)

## Introduction

Script that uses `rsync` to make a simple and convenient backup.

**Note:** requires Python 3.6+. No other Python third-party libraries are required.

For each source to backup, a log file is created at the root directory of that
source. In the log file, the text `Command executed:` is inserted at the
beginning with the whole `rsync` command that has been executed as a reference.

Example (long line broken for extra readability):

    Command executed:
    rsync -vaAHh --delete --ignore-errors --force --prune-empty-dirs \
    --delete-excluded --log-file=/home/sglavoie/.backup_log_201213_11_11_30 \
    --exclude-from=/home/sglavoie/.backup_exclude /home/sglavoie \
    /media/sglavoie/Elements

By default, the following options are passed to `rsync`:

| Option               | Description                                          |
| -------------------- | ---------------------------------------------------- |
| `-vaAH`              | verbose, archive, ACLs, hard-links (preserve)        |
| `--delete`           | _"delete extraneous files from destination dirs"_    |
| `--ignore-errors`    | _"delete even if there are I/O errors"_              |
| `--force`            | _"force deletion of directories even if not empty"_  |
| `--prune-empty-dirs` | _"prune empty directory chains from the file-list"_  |
| `--delete-excluded`  | _"also delete excluded files from destination dirs"_ |

## Description of available settings in `settings.json`

| Name of setting    | Description                                                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `data_sources`     | Directories to backup, supplied as a list of strings (no slash at the end).                                                                |
| `data_destination` | Single destination of the files to backup, supplied as a string. This can be overridden when passing option `-d` or `--dest` to the script |
| `terminal_width`   | Line length in the terminal, used for printing separators.                                                                                 |
| `sep`              | Separator to use along with `terminal_width`.                                                                                              |
| `log_name`         | Sets the prefix of the log filename.                                                                                                       |
| `log_format`       | This goes right after `log_name` as a suffix.                                                                                              |
| `rsync_options`    | Options to use with rsync as a list of strings.                                                                                            |
| `backup_exclude`   | Default file in each source in `data_sources` where files/directories will be ignored.                                                     |

## How to use

1. Set all values in `settings.json` to suit your needs.
2. Make sure that the backup destination is available/mounted. A simple warning will be echoed if the destination can't be found.
3. Call this file. As an example, if this repository is located at `/home/user/backup/`, then you could put the following alias in `~/.bash_aliases` (or equivalent)\*:

       alias backup='python3 /home/user/backup/rsync_backup.py'

4. In this instance, the script can now be executed in a terminal with the keyword `backup` along with optional arguments.

\* Python may be called differently on your system, e.g. simply `python` instead of `python3`.

## Help menu

    usage: backup [-h] [-c] [-s SOURCE] [-d DESTINATION]

    optional arguments:
    -h, --help            show this help message and exit
    -c, --clear           Delete all log files for current source in DATA_SOURCES.
    -s SOURCE, --src SOURCE
                            Specify an alternative source to backup as a string.
    -d DESTINATION, --dest DESTINATION
                            Specify an alternative destination for backup as a string.
