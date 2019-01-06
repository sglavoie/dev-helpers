# Bash History Cleaner

This is a Python 3.6+ script that helps to clean the file containing the Bash history commands. It will remove any line matching a specified regular expression and can also remove any line starting with an alias.

The idea behind this small utility was simple:

- The Bash history file (usually located in `~/.bash_history`) contains much of the work one ends up doing in the terminal.
- The history can grow large over time and it becomes more cumbersome to find interesting information in all that clutter, such as a rarely used command with specific flags.
- By removing all superfluous commands that are repeated often and which give no real benefit in certain contexts (such as `ls`, `cd`, `cat`, etc.), the history is much cleaner and easier to navigate and actually becomes much more useful in my opinion.
- True, it will be harder to follow the bread crumbs for everything you did, but I haven't come across a situation where having access to yet another empty `ls` or `cd` has proven necessary and reading `.bash_history` doesn't make for a great narrative story either.

It comes in two files that need to be in the same directory:

- One is a Python file that needs to be launched from the terminal with Python 3.
- The other file, `settings.json`, is a JSON file used to store the settings of the script, which will be detailed below.

----

## Make history in a big way

I took advantage of the fact that the history can be cleaned with the script you are about to see and set up what is known as an _eternal history_ which, as it sounds like, can grow infinitely big! All you have to do is append the following lines to the file `~/.bashrc`:

```bash
# Eternal bash history.
# ---------------------
# Undocumented feature which sets the size to "unlimited".
# http://stackoverflow.com/questions/9457233/unlimited-bash-history
export HISTFILESIZE=-1
export HISTSIZE=-1
export HISTTIMEFORMAT="[%F %T] "
# Change the file location because certain bash sessions truncate .bash_history file upon close.
# http://superuser.com/questions/575479/bash-history-truncated-to-500-lines-on-each-login
export HISTFILE=~/.bash_eternal_history
# Force prompt to write history after every command.
# http://superuser.com/questions/20900/bash-history-loss
PROMPT_COMMAND="history -a; $PROMPT_COMMAND"
```
----

## Description of available settings in `settings.json`

| Name of setting | Description |
| --------------- | ----------- |
| `home_directory` | Absolute path to user's home directory. |
| `history_file`  | Name of file where the history will be cleaned up. |
| `aliases_file`  | Name of file where Bash aliases are set up. |
| `ignore_patterns` | List of patterns to ignore in `history_file`. Each line where a pattern is found will be deleted. Patterns are specified as regular expressions. |
| `add_aliases` | Boolean. If set to `True`, aliases from `aliases_file` will be added to `ignore_patterns`. (Default: `True`) |
| `aliases_match_greedily` | Boolean. If set to `True`, any line in `history_file` starting with an alias in `aliases_file` will be deleted. If set to `False`, delete line if the alias is the content of the whole line (with optional space at the end): `False` matches "^alias$" or "^alias $" only. |
| `backup_history` | Boolean. If set to `True`, `history_file` will be backed up in the same directory with a name ending in .bak based on the current date. (Default: `True`) |
| `delete_logs_without_confirming` | Boolean. If set to `True`, script with flag `-c` will automatically delete all the backup files found for `history_file`. (Default: `False`) |
| `remove_all_duplicated_lines` | Boolean. If set to `true`, any following line that is found to already be present in the file will be removed. This setting has precedence over `remove_duplicates_within_X_lines` and `remove_consecutive_duplicates` (they won't be executed). |
| `remove_duplicates_within_X_lines` | Integer. Scan lines one by one. If the current line is found in the next `X` lines defined by this setting, it will be removed. If set to a value greater than `1`, this setting has precedence over `remove_consecutive_duplicates` (it won't be executed). |
| `remove_consecutive_duplicates` | Boolean. If set to `true`, duplicated lines will be deleted when they are consecutive in order to leave only one match. |


## Anecdotal evidence of satisfying performances

Performance-wise, this scans ~8,300 lines per second on my modest Intel Core i5 laptop with files of over 200,000 lines long. Not that I type so much stuff in the terminal: I just duplicated many lines :).


## Your opinion is welcome!

I haven't noticed any bug up to now for my personal use, but please feel free to let me know if you are aware of any unwanted behavior. To stay on the side of caution, setting `backup_history` to `True` has proven to be useful.


## Conclusion

This is a simple solution to an nonexistent problem, but it was in the end very instructive to me nonetheless. You may even find a use for it! Otherwise, you might use the same functions for other files such as logs!
