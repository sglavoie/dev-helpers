'''
Small script that uses `rsync` to make a simple and convenient backup.
Note: requires Python 3.6+.

For each source to backup, a log file is created at the root directory of that
source. In the log file, the text `Command executed:` is inserted at the
beginning with the whole `rsync` command that has been executed as a reference.

By default, the following options are passed to `rsync`:
    '-vaH' → verbose, archive, hard-links (preserve)
    '--delete' → "delete extraneous files from destination dirs"
    '--ignore-errors' → "delete even if there are I/O errors"
    '--force' → "force deletion of directories even if not empty"
    '--prune-empty-dirs' → "prune empty directory chains from the file-list"
    '--delete-excluded' → "also delete excluded files from destination dirs"

How to use:
    1) Set all variables in the SETTINGS section below to suit your needs.
    2) Make sure that the backup destination is available/mounted.
    3) Copy this file somewhere where it will be executed. As an example, I
       put this file in ~/.backup.py and made the following alias in
       ~/.bash_aliases:
       alias backup='/usr/local/bin/python3.7 ~/.backup.py'
    4) In this example, the script can now be executed in a terminal with the
       keyword `backup` along with optional arguments.
'''
import argparse
import datetime
import glob
import os
import subprocess
import sys
import threading
from time import sleep

# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
# SETTINGS
# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=

# Directories to back up, supplied as a dictionary where {key: value}
# corresponds to {'source_to_back_up': ['list', 'of', 'rsync', 'options']}
# If the source directory contains a slash at the end, the CONTENT will be
# copied without recreating the source directory.
# Each source can be specified with its own rsync options.
RSYNC_OPTIONS = ['-vaH', '--delete', '--ignore-errors', '--force',
                 '--prune-empty-dirs', '--delete-excluded']

DATA_SOURCES = {
    '/tmp/seb': RSYNC_OPTIONS,
    '/tmp/yo': RSYNC_OPTIONS
}


# Single destination of the files to back up, supplied as a string
# This can be overridden when passing option '-d' or '--dest' to the script
# DATA_DESTINATION = f'/media/sgdlavoie/Elements'
DATA_DESTINATION = f'/tmp/desti'

# Line length in the terminal, used for printing separators
TERMINAL_WIDTH = 40

# Separator to use along with TERMINAL_WIDTH
SEP = '«»'  # using 2 characters, we have to divide TERMINAL_WIDTH by 2 also

# Sets the prefix of the log filename. If set to None, no log is generated.
# LOG_NAME = None
LOG_NAME = '.backup_log_'

# This goes right after LOG_NAME as a suffix. Reference for modifying format:
# https://docs.python.org/3/library/time.html#time.strftime
LOG_FORMAT = '%y%m%d_%H_%M_%S'

# Default file in each source in DATA_SOURCES where files/directories will be
# ignored. If it doesn't exist, the option "--exclude-from" won't be added.
BACKUP_EXCLUDE = ".backup_exclude"

# How long to wait in seconds for user feedback when --remind option is passed.
# → Frequency at which a sound is played when waiting for user input.
PLAY_WAIT_TIME = 15

# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
# DECORATORS
# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=


def better_separation(the_function):
    '''Decorator used to print separators around `the_function`.'''
    def print_separator(*args, **kwargs):
        '''Surrounds `the_function` with a separator and add a new line.'''
        separator = SEP * TERMINAL_WIDTH
        print(separator)
        the_function(*args, **kwargs)
        print(separator, '\n')
    return print_separator


# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
# FUNCTIONS
# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
REMINDER_IS_SET = False  # Do not modify. Used for background_reminder function


@better_separation  # FIXME: take logfile into account in rsync command
def backing_source(source, backup_cmd):
    '''Print information to STDOUT and to `log_filename` and executes the
    rsync command.'''
    cmd_executed = ' '.join(backup_source)
    msg_executed = f'Command executed:\n{cmd_executed}\n'
    print(msg_executed)
    # with open(log_filename, mode='w') as log_file:
        # log_file.write(f'{msg_executed}\n')
    # subprocess.run(backup_source)

    print(f'\nBackup completed for: {source}')


def background_reminder(wait_time=PLAY_WAIT_TIME):
    """Depends on a function to set `reminder_is_set` to False.
    It will play a sound every `wait_time` in seconds until `reminder_is_set`
    is False."""
    global REMINDER_IS_SET
    while REMINDER_IS_SET:
        os.system(
            "aplay /home/sgdlavoie/Music/.levelup.wav > /dev/null 2>&1 &")
        sleep(wait_time)


def user_says_yes(message=""):
    """Depends on function `background_reminder`. It creates a thread with
    `background_reminder` and will stop the thread when user input is either
    'y' or 'n'. Returns a boolean."""
    # needs to be set globally for other functions to update correspondingly
    global REMINDER_IS_SET
    if REMINDER_IS_SET:
        # `target` defines a function that will be run as a thread
        reminder_thread = threading.Thread(target=background_reminder)
        reminder_thread.daemon = True
        reminder_thread.start()
    while True:
        choice = input(message)
        if choice == 'y':
            choice = True
            REMINDER_IS_SET = False
            break
        elif choice == 'n':
            choice = False
            break
        else:
            print("Please enter either 'y' or 'n'.")
    return choice


def clear_logs(data_sources=None):
    '''Clear log files for each source specified in DATA_SOURCES.'''
    if data_sources is None:
        data_sources = {}

    if LOG_NAME is None:
        print(f"\nVariable `LOG_NAME` is not defined.")
        sys.exit(0)

    for source in data_sources:
        # Retrieve a list of all matching log files in `source`
        log_files = glob.glob(f'{source}/{LOG_NAME}*')
        if log_files == []:
            print(f"\nThere is no log file to delete in {source}.")
            continue
        else:
            print(f'Log files in {source}:')
            for log_file in log_files:
                print(log_file)
            message = ("\nDo you want to delete log files "
                       "for this source? (y/n) ")
            if user_says_yes(message=message):
                for log_file in log_files:
                    os.remove(log_file)
                print('Log files deleted.')
                continue
            else:
                continue
        print('Exiting script...')
        sys.exit(0)


def check_destination_exists(data_destination):
    """In order to avoid building a list of files with rsync uselessly and
    later realize that rsync fails because destination doesn't exist."""
    while True:
        print(f"The destination doesn't exist.\n({data_destination})\n")
        message = "Do you want to try again? (y/n) "
        if user_says_yes(message=message):
            if not os.path.isdir(data_destination):
                continue
            else:
                break
        else:
            sys.exit(0)


def run_backup(*args, data_destination=DATA_DESTINATION):
    '''This is where all the action happens!'''

    # FIXME: Breakpoint. This is a work in progress now that DATA_SOURCES
    # is a dictionary and new functions have been added.
    sys.exit(0)
    check_destination_exists(data_destination)

    for source in args:
        if LOG_NAME is not None:
            date_now = datetime.datetime.now()
            log_format = datetime.datetime.strftime(date_now, LOG_FORMAT)
            log_filename = f'{source}/{LOG_NAME}{log_format}'
            log_option = f'--log-file={log_filename}'

            backup_source = BACKUP_CMD.copy()
            backup_source.extend([log_option])

        # files to ignore in backup
        exclude_file = f'{source}/{BACKUP_EXCLUDE}'
        if os.path.exists(exclude_file):
            exclude_option = f'--exclude-from={exclude_file}'
            # backup_source.extend([exclude_option, source, data_destination])
        else:  # skips '--exclude-from' option if no file is found
            backup_source.extend([source, data_destination])

        backing_source(source, backup_source, log_filename)


# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
# EXECUTION
# =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=

if __name__ == '__main__':
    # initiate the parser to check all the arguments passed to the script
    PARSER = argparse.ArgumentParser()
    PARSER.add_argument(
        '-a', '--alert',
        help='Play a sound when the backup has completed.',
        action='store_true')
    PARSER.add_argument(
        '-c', '--clear',
        help='Delete all log files for current source in DATA_SOURCES.',
        action='store_true')
    PARSER.add_argument(
        '-d', '--dest', dest='destination', default=None,
        help='Specify an alternative destination for backup as a string.',
        action='store')
    PARSER.add_argument(
        '-p', '--play',
        help='Play a sound in the background when launching the script',
        action='store_true')
    PARSER.add_argument(
        '-r', '--remind', action='store_true',
        help=('Plays a sound every X seconds when waiting for user feedback. '
              'Depends on PLAY_WAIT_TIME.')
        )

    # read arguments from the command line
    ARGUMENTS = PARSER.parse_args()

    if ARGUMENTS.remind:
        REMINDER_IS_SET = True
    if ARGUMENTS.play:
        # Play sound in background and do not output to terminal
        os.system(
            "aplay /home/sgdlavoie/Music/.levelup.wav > /dev/null 2>&1 &")
    if ARGUMENTS.clear:
        clear_logs(DATA_SOURCES)
        sys.exit(0)
    if ARGUMENTS.destination is not None:
        if os.path.isdir(ARGUMENTS.destination):
            DATA_DESTINATION = ARGUMENTS.destination
        print("Please enter a valid destination.")
        sys.exit(0)

    run_backup(DATA_SOURCES)
