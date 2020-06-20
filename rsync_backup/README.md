# rsync backup

## From the script's main docstring

Small script that uses `rsync` to make a simple and convenient backup.

**Note**: requires Python 3.6+.

For each source to backup, a log file is created at the root directory of that
source. In the log file, the text `Command executed:` is inserted at the
beginning with the whole `rsync` command that has been executed as a reference.

By default, the following options are passed to `rsync`:

`-vaAH` → verbose, archive, ACLs, hard-links (preserve)

`--delete` → "delete extraneous files from destination dirs"

`--ignore-errors` → "delete even if there are I/O errors"

`--force` → "force deletion of directories even if not empty"

`--prune-empty-dirs` → "prune empty directory chains from the file-list"

`--delete-excluded` → "also delete excluded files from destination dirs"

How to use:

1. Set all variables in the SETTINGS section of the script to suit your needs.
2. Make sure that the backup destination is available/mounted.
3. Copy this file somewhere where it will be executed. As an example, I
   put this file in ~/.backup.py and made the following alias in
   ~/.bash_aliases:
   alias backup='/usr/local/bin/python3.7 ~/.backup.py'
4. In this example, the script can now be executed in a terminal with the
   keyword `backup` along with optional arguments.
