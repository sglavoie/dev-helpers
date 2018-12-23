'''
Python script that helps to clean the file containing the Bash history
commands.

It will remove any line matching a specified regular expression and can also
remove any line starting with an alias.

Description of available settings in `SETTINGS` (below `PATTERNS_TO_IGNORE`):

    'history_file':     Path to the file where the history will be cleaned up,
                        relative to `HOME_DIRECTORY`, set before `SETTINGS`.
    'aliases_file':     Path to the file where aliases are set up, relative to
                        `HOME_DIRECTORY`.
    'ignore_patterns':  List of patterns to ignore in `history_file`.
                        Each line where a pattern is found will be deleted.
                            → Patterns are specified as regular expressions.
    'add_aliases':      Boolean. If set to True, aliases from `aliases_file`
                        will be added to `ignore_patterns`.
'''
import fileinput
import re

PATTERNS_TO_IGNORE = [
    r'^\#\d+',
    r'^$',  # empty lines
    r'^(\.\/)?pip$',
    r'^(\.\/)?python.*$',
    r'^\.\.$',
    r'^alias',
    r'^cd ',
    r'^cd$',
    r'^cd..$',
    r'^fg$',
    r'^df( )?',
    r'^du( )?',
    r'^exit$',
    r'^git checkout master',
    r'^git log',
    r'^git push',
    r'^git status$',
    r'^git stauts$',  # when trying to type faster than what's realistic
    r'^kill \d+.*',  # killing processes, e.g.: kill 123 3229
    r'^ls -a$',
    r'^ls$',
    r'^make|make install$',
    r'^man ',
    r'^pelican( )?$',
    r'^pip install -r requirements.txt',
    r'^pip.* list|pip.* show$',
    r'^source ',
    r'^sudo apt-get autoclean|sudo apt-get autoremove$',
    r'^sudo apt-get dist-upgrade|sudo apt-get update$',
    r'^which ',
]

# Absolute path to user's home directory.
HOME_DIRECTORY = '/home/sglavoie/'

SETTINGS = {
    'history_file': HOME_DIRECTORY + '.bash_eternal_history',
    'aliases_file': HOME_DIRECTORY + '.bash_aliases',
    'ignore_patterns': PATTERNS_TO_IGNORE,
    'add_aliases': True
}


def get_list_aliases(bash_aliases_file: str) -> list:
    '''Retrieve the name of all the aliases specified in `bash_aliases_file`
    and return them as a list of strings formatted as regular expressions.'''

    with open(bash_aliases_file) as file:
        content = file.read().splitlines()  # one alias per line
        aliases_list = []
        for line in content:
            # Get the actual alias in each line
            try:
                alias = re.search(r'(?<!not )alias (([\.]?[\w]?[\.]?)+)',
                                  line).group(1)
            except AttributeError:
                continue
            # Reformat as a regular expression
            alias = f'^{alias}( )?'

            # Escape dots in alias
            alias = alias.translate(str.maketrans({".":  r"\."}))

            aliases_list.append(alias)

    return aliases_list


def clean_bash_history(file_to_clean: str,
                       patterns_to_ignore: list,
                       aliases: list = None):
    '''Modify in place `file_to_clean` by removing every line where
    `patterns_to_ignore` is found.

    Optionally, add a list of aliases to `patterns_to_ignore` with
    `aliases`.'''

    if aliases is not None:
        # add aliases to list of patterns to ignore
        patterns_to_ignore.extend(aliases)

    for pattern in patterns_to_ignore:
        with fileinput.FileInput(file_to_clean,
                                 inplace=True,
                                 backup='.bak') as file:
            matched = re.compile(pattern).search
            for line in file:
                if not matched(line):
                    # If no match is found (nothing to ignore), print the line
                    # back into the file. Otherwise, it will be empty.
                    print(line, end='')  # Line already has carriage return


def launch_cleanup(settings: dict):
    '''Main function that launches the cleanup process.'''

    bash_aliases = None
    if settings['add_aliases']:
        try:
            bash_aliases = get_list_aliases(settings['aliases_file'])
        except FileNotFoundError:
            print(f"File not found: {settings['aliases_file']}")
            quit()
    try:
        clean_bash_history(settings['history_file'],
                           settings['ignore_patterns'],
                           aliases=bash_aliases)
    except FileNotFoundError:
        print(f"File not found: {settings['history_file']}")


if __name__ == '__main__':
    launch_cleanup(SETTINGS)
