'''
Python script that helps to clean the file containing the Bash history
commands.

Note: Requires Python 3.6+

It will remove any line matching a specified regular expression and can also
remove any line starting with an alias.

Description of available settings in `settings.json`:

    "home_directory":   Absolute path to user's home directory.

    "history_file":     Name of file where the history will be cleaned up.

    "aliases_file":     Name of file where Bash aliases are set up.

    "ignore_patterns":  List of patterns to ignore in `history_file`.
                        Each line where a pattern is found will be deleted.
                            → Patterns are specified as regular expressions.

    "add_aliases":      Boolean. If set to True, aliases from `aliases_file`
                        will be added to `ignore_patterns`.

    "aliases_match_greedily":
                        Boolean. If set to True, any line in `history_file`
                        starting with an alias in `aliases_file` will be
                        deleted. If set to False, delete line if the alias is
                        the content of the whole line (with optional space at
                        the end): False matches "^alias$" or "^alias $" only.

    "backup_history":   Boolean. If set to True, `history_file` will be backed
                        up in the same directory with a name ending in .bak
                        based on the current date.

    "delete_logs_without_confirming":
                        Boolean. If set to True, script with flag `-c` will
                        automatically delete all the backup files found for
                        `history_file`.
'''
from datetime import datetime
from pathlib import Path
import argparse
import fileinput
import glob
import json
import os
import re


def get_current_path():
    '''Returns the current working directory relative to where this script
    is being executed.'''
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


def generate_date_string() -> str:
    '''Return date formatted string to backup a file.'''
    return datetime.strftime(datetime.today(), '_%Y%m%d_%H%M%S.bak')


def load_settings(settings_file: str) -> dict:
    '''Load settings in the script. Return them as a dictionary.'''
    with open(settings_file, "r") as read_file:
        settings = json.load(read_file)
    return settings


def get_list_aliases(bash_aliases_file: str, settings: dict) -> list:
    '''Retrieve the name of all the aliases specified in `bash_aliases_file`.

    Return aliases as a list of strings formatted as regular expressions.'''

    match_whole_line = bool(settings['aliases_match_greedily'])

    with open(bash_aliases_file) as file:
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
                # Will match only when line starts with alias, followed by
                # optional space.
                alias = f'^{alias}( )?$'

            # Escape dots in alias
            alias = alias.translate(str.maketrans({".":  r"\."}))

            aliases_list.append(alias)

    return aliases_list


def clean_bash_history(settings: dict, history_file: str):
    '''Modify in place `history_file` by removing every line where
    `ignore_patterns` is found.

    Optionally, add a list of aliases to `ignore_patterns` with
    `aliases` based on the value of `add_aliases` in settings.json.'''

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
                if matched(line):
                    has_match = True
                    break
            # If no match is found (nothing to ignore), print the line
            # back into the file. Otherwise, it will be empty.
            if not has_match:
                print(line, end='')  # Line already has carriage return


def launch_cleanup(settings: dict, history_file: str, aliases_file: str):
    '''Main function that launches the cleanup process.'''

    bash_aliases = None
    if settings['add_aliases']:
        try:
            bash_aliases = get_list_aliases(aliases_file, settings)

            # add aliases to list of patterns to ignore
            settings['ignore_patterns'].extend(bash_aliases)
        except FileNotFoundError:
            print(f"File not found: {aliases_file}")
            quit()
    try:
        clean_bash_history(settings, history_file)
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
