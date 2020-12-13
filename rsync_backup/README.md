# rsync backup

Small script that uses `rsync` to make a simple and convenient backup.
Note: requires Python 3.6+. No other Python third-party libraries required.

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

How to use:

1. Set all variables in the `SETTINGS` section of the script to suit your needs.
2. Make sure that the backup destination is available/mounted. A simple warning will be echoed if the destination can't be found.
3. Copy this file somewhere where it will be executed. As an example, you could put this file in `~/.backup.py` and create the following alias in `~/.bash_aliases`:

       alias backup='python3 ~/.backup.py'

4. In this instance, the script can now be executed in a terminal with the keyword `backup` along with optional arguments.
