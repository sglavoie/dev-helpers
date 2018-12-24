# Bash History Cleaner

Python script that helps to clean the file containing the Bash history
commands.

**It will remove any line matching a specified regular expression and can also
remove any line starting with an alias.**

## Description of available settings in `settings.json`

| Name of setting | Description |
| --------------- | ----------- |
| `home_directory` | Absolute path to user's home directory. |
| `history_file`  | Name of file where the history will be cleaned up. |
| `aliases_file`  | Name of file where Bash aliases are set up. |
| `ignore_patterns` | List of patterns to ignore in `history_file`. Each line where a pattern is found will be deleted. Patterns are specified as regular expressions. |
| `add_aliases` | Boolean. If set to `True`, aliases from `aliases_file` will be added to `ignore_patterns`. (Default: `True`) |
| `backup_history` | Boolean. If set to `True`, `history_file` will be backed up in the same directory with a name ending in .bak based on the current date. (Default: `True`) |
| `delete_logs_without_confirming` | Boolean. If set to `True`, script with flag `-c` will automatically delete all the backup files found for `history_file`. (Default: `False`) |


## Anecdotal evidence of satisfying performances

Performance-wise, this scans ~8,300 lines per second on my modest Intel Core i5 laptop with files of over 200,000 lines long. Not that I type so much stuff in the terminal: I just duplicated many lines :).


## Your opinion is welcome!

I haven't noticed any bug up to now for my personal use, but please feel free to let me know if you are aware of any unwanted behavior. To stay on the side of caution, setting `backup_history` to `True` has proven useful.

----

Note: Requires Python 3.6+
