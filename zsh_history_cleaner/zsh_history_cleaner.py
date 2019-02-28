'''
Python script that helps to clean the file containing the Zsh history commands.

Note: Requires Python 3.6+

It will remove any line matching a specified regular expression and can also
remove any line starting with an alias.

Description of available settings in `settings.json`:

    "home_directory":   Absolute path to user's home directory.

    "history_file":     Name of file where the history will be cleaned up.

    "aliases_file":     Name of file where Zsh aliases are set up.

    "ignore_patterns":  List of patterns to ignore in `history_file`.
                        Each line where a pattern is found will be deleted.
                        → Patterns are specified as regular expressions.

    "add_aliases":      Boolean. If set to `true`, aliases from `aliases_file`
                        will be added to `ignore_patterns`.

    "aliases_match_greedily":
                        Boolean. If set to `true`, any line in `history_file`
                        starting with an alias in `aliases_file` will be
                        deleted. If set to `false`, delete line if the alias is
                        the content of the whole line (with optional space at
                        the end): `false` matches "^alias$" or "^alias $" only.

    "backup_history":   Boolean. If set to `true`, `history_file` will be
                        backed up in the same directory with a name ending in
                        .bak based on the current date.

    "delete_logs_without_confirming":
                        Boolean. If set to `true`, script with flag `-c` will
                        automatically delete all the backup files found for
                        `history_file`.

    "remove_all_duplicated_lines":
                        Boolean. If set to `true`, any following line that is
                        found to already be present in the file will be
                        removed. This setting has precedence over
                        `remove_duplicates_within_X_lines` and
                        `remove_consecutive_duplicates` (they won't be
                        executed).

    "remove_duplicates_within_X_lines":
                        Integer. Scan lines one by one. If the current line is
                        found in the next `X` lines defined by this setting, it
                        will be removed. If set to a value greater than `1`,
                        this setting has precedence over
                        `remove_consecutive_duplicates` (it won't be executed).

    "remove_consecutive_duplicates":
                        Boolean. If set to `true`, duplicated lines will be
                        deleted when they are consecutive in order to leave
                        only one match.
'''

from datetime import datetime
from pathlib import Path
import argparse
import fileinput
import glob
import json
import os
import re


def file_length(file_name):
    '''Return the number of lines in a file. Returns 0 if it doesn't exist.'''

    try:
        with open(file_name) as file_to_check:
            for index, _ in enumerate(file_to_check):
                pass
            return index + 1  # Will return UnboundLocalError if file is empty
    except (FileNotFoundError, UnboundLocalError):
        return 0


def generate_date_string() -> str:
    '''Return date formatted string to backup a file.'''

    return datetime.strftime(datetime.today(), '_%Y%m%d_%H%M%S.bak')


def get_current_path():
    '''Returns the current working directory relative to where this script is
    being executed.'''

    return Path(__file__).parents[0]


def user_says_yes(message=""):
    '''Check if user input is either 'y' or 'n'. Returns a boolean.'''

    while True:
        choice = input(message).lower()
        if choice == 'y':
            choice = True
            break
        elif choice == 'n':
            choice = False
            break
        else:
            print("Please enter either 'y' or 'n'.")

    return choice


def delete_logs(settings: dict, history_file: str):
    '''Delete log files in `home_directory` based on `history_file`.'''

    # Retrieve a list of all matching log files
    log_files = glob.glob(f'{history_file}_*.bak')

    if log_files == []:
        print("There is no log file to delete.")
    else:
        print(f'Log files found in {settings["home_directory"]}:')

        for log_file in log_files:
            print(log_file)

        if settings['delete_logs_without_confirming']:
            for log_file in log_files:
                os.remove(log_file)
            print('Log files deleted.')
            return

        message = ("\nDo you want to delete those log files? [y/n] ")

        if user_says_yes(message=message):
            for log_file in log_files:
                os.remove(log_file)
            print('Log files deleted.')
            return

    print('Operation aborted.')
    return


def remove_duplicates_within_range(range_num, history_file):
    '''Scan lines in `history_file` one by one. If the current line is found in
    the next `range_num` lines, it will be removed. The same process is
    repeated on every line so that any line won't have duplicates within
    `range_num`.

    Note: By executing the script various times, duplicates will be searched
    again and within few executions, no line will be repeated within the
    specified range.'''

    file_input = fileinput.FileInput(history_file, inplace=True)
    with open(history_file, 'r') as original_history:
        original_lines = original_history.readlines()
    for index, line in enumerate(file_input):
        next_line = index + 1
        max_line = next_line + range_num
        duplicate = False
        for following_line in range(next_line, max_line):
            try:
                if original_lines[following_line][15:] == line[15:]:
                    duplicate = True
                    break
            except IndexError:  # Got to the end of the file, no more lines
                break
        if duplicate:
            continue
        print(line, end='')


def remove_consecutive_duplicates(history_file):
    '''Go over `history_file` in place and for all consecutive lines that are
    duplicated, skip them (effectively removing them).'''

    file_input = fileinput.FileInput(history_file, inplace=True)
    previous_line = ""
    for line in file_input:
        if previous_line[15:] == line[15:]:
            continue
        previous_line = line
        print(line, end='')


def remove_all_duplicates(history_file):
    '''Go over `history_file` in place and for every line that has been seen
    before, do not add it back to the file (effectively removing it).'''

    seen = set()
    file_input = fileinput.FileInput(history_file, inplace=True)
    for line in file_input:
        if line[15:] in seen:
            continue  # Skip duplicate

        seen.add(line[15:])
        print(line, end='')


def load_settings(settings_file: str) -> dict:
    '''Load settings in the script. Return them as a dictionary.'''

    with open(settings_file, "r") as read_file:
        settings = json.load(read_file)
    return settings


def get_list_aliases(zsh_aliases_file: str, settings: dict) -> list:
    '''Retrieve the name of all the aliases specified in `zsh_aliases_file`.

    Return aliases as a list of strings formatted as regular expressions.'''

    match_whole_line = bool(settings['aliases_match_greedily'])

    with open(zsh_aliases_file) as file:
        content = file.read().splitlines()  # one alias per line
        aliases_list = []
        for line in content:
            try:
                # Get the actual alias in each line
                # Use negative lookbehind to remove 'alias ' at the beginning.
                # Matches anything after that is a dot, digit, underscore
                # or letter (will stop at the equal sign: alias blah='...')
                alias = re.search(r'(?<!not )alias ([\.\w]*)', line).group(1)

            # If for some reason the alias cannot be extracted, skip it
            # If search doesn't match, it's of type None and won't work
            except AttributeError:
                continue

            if match_whole_line:
                # Match the whole line if it starts with the alias.
                alias = f'^{alias}( )?$|^{alias} .*'
            else:
                # Will match only when alias is the whole content of the line,
                # followed by optional space.
                alias = f'^{alias}( )?$'

            # Escape dots in alias
            alias = alias.translate(str.maketrans({".":  r"\."}))

            aliases_list.append(alias)

    return aliases_list


def clean_zsh_history(settings: dict, history_file: str):
    '''Modify in place `history_file` by removing every line where
    `ignore_patterns` is found.

    Optionally, add a list of aliases to `ignore_patterns` with `aliases` based
    on the value of `add_aliases` in settings.json.'''

    original_num_lines = file_length(history_file)

    if settings['backup_history']:
        backup_str = generate_date_string()
        file_input = fileinput.FileInput(history_file,
                                         inplace=True,
                                         backup=backup_str)
    else:
        file_input = fileinput.FileInput(history_file,
                                         inplace=True)

    with file_input as file:
        for line in file:
            has_match = False
            for pattern in settings['ignore_patterns']:
                matched = re.compile(pattern).search
                if matched(line[15:]):
                    has_match = True
                    break
            # If no match is found (nothing to ignore), print the line
            # back into the file. Otherwise, it will be empty.
            if not has_match:
                print(line, end='')  # Line already has carriage return

    num_lines = settings['remove_duplicates_within_X_lines']

    if settings['remove_all_duplicated_lines']:
        remove_all_duplicates(history_file)
        print("All duplicated lines were removed.")

    # if num_lines <= 1 → no need to check (use consecutive instead, if set)
    elif num_lines > 1:
        remove_duplicates_within_range(num_lines, history_file)
        print(f"All duplicates within {num_lines} lines were removed.")

    elif settings['remove_consecutive_duplicates']:
        remove_consecutive_duplicates(history_file)
        print("All consecutive duplicated lines were removed.")

    final_num_lines = file_length(history_file)
    num_lines_deleted = original_num_lines - final_num_lines
    lines = "line" if num_lines_deleted < 2 else "lines"

    print(f"{num_lines_deleted} {lines} deleted.")


def launch_cleanup(settings: dict, history_file: str, aliases_file: str):
    '''Main function that launches the cleanup process.'''

    zsh_aliases = None
    if settings['add_aliases']:
        try:
            zsh_aliases = get_list_aliases(aliases_file, settings)

            # add aliases to list of patterns to ignore
            settings['ignore_patterns'].extend(zsh_aliases)
        except FileNotFoundError:
            print(f"File not found: {aliases_file}")
            quit()
    try:
        clean_zsh_history(settings, history_file)
    except FileNotFoundError:
        print(f"File not found: {history_file}")


if __name__ == '__main__':
    SETTINGS_FILE_PATH = get_current_path() / 'settings.json'
    SETTINGS = load_settings(SETTINGS_FILE_PATH)
    ALIASES_FILE = SETTINGS['home_directory'] + '/' + SETTINGS['aliases_file']
    HISTORY_FILE = SETTINGS['home_directory'] + '/' + SETTINGS['history_file']

    # initiate the parser to check all the arguments passed to the script
    PARSER = argparse.ArgumentParser()
    PARSER.add_argument(
        '-c', '--clear', help='Delete all log files', action='store_true')

    # read arguments from the command line
    ARGUMENTS = PARSER.parse_args()

    if ARGUMENTS.clear:
        delete_logs(SETTINGS, HISTORY_FILE)
        quit()

    launch_cleanup(SETTINGS, HISTORY_FILE, ALIASES_FILE)
