#!/usr/local/bin/python3.7
"""
Small script that uses `rsync` to make a simple and convenient backup.
Note: requires Python 3.6+ (otherwise, f-strings need to be converted).

For each source to backup, a log file is created at the root directory of that
source. In the log file, the text `Command executed:` is inserted at the
beginning with the whole `rsync` command that has been executed as a reference.

By default, the following options are passed to `rsync`:
    '-vaHh' → verbose, archive, hard-links (preserve), human readable format
    '--delete' → "delete extraneous files from destination dirs"
    '--ignore-errors' → "delete even if there are I/O errors"
    '--force' → "force deletion of directories even if not empty"
    '--prune-empty-dirs' → "prune empty directory chains from the file-list"
    '--delete-excluded' → "also delete excluded files from destination dirs"

How to use:
    1) Set all variables in settings.py to suit your needs.
    2) Make sure that the backup destination is available/mounted.
    3) Copy this file somewhere where it will be executed. As an example, I
       put this file in ~/.backup.py and made the following alias in
       ~/.bash_aliases:
       alias backup='/usr/local/bin/python3.7 ~/.backup.py'
    4) In this example, the script can now be executed in a terminal with the
       keyword `backup` along with optional arguments.
"""
# Standard library imports
from time import sleep
import argparse
import datetime
import glob
import os
import subprocess
import sys
import threading

# Local imports
from settings import (
    BACKUP_EXCLUDE,
    DATA_DESTINATION,
    DATA_SOURCES,
    LOG_FORMAT,
    LOG_NAME,
    PLAY_WAIT_TIME,
    SEP,
    SOUND_PATH,
    TERMINAL_WIDTH,
)

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


def play_sound(sound_path=SOUND_PATH):
    """Play sound in background and do not output to terminal."""
    return os.system(f"aplay {sound_path} > /dev/null 2>&1 &")


@better_separation
def backing_source(source, main_options, custom_options):
    '''Print information to STDOUT and to `log_filename` and executes the
    rsync command.'''
    cmd_executed = main_options.copy()
    cmd_executed.insert(0, 'rsync')

    if custom_options['logfile'] is not None:
        cmd_executed.append(custom_options['logfile'])
    if custom_options['exclude_option'] is not None:
        cmd_executed.append(custom_options['exclude_option'])
    cmd_executed.append(source)
    cmd_executed.append(DATA_DESTINATION)
    cmd_to_run = ' '.join(cmd_executed)
    msg_executed = f'Command being executed:\n{cmd_to_run}\n'
    print(msg_executed)

    if custom_options['logfilename'] is not None:
        with open(custom_options['logfilename'], mode='w') as log_file:
            log_file.write(f'{msg_executed}\n')

    subprocess.run(cmd_to_run, shell=True)

    print(f'\nBackup completed for: {source}')


def background_reminder(wait_time=PLAY_WAIT_TIME):
    """Depends on a function to set `reminder_is_set` to False.
    It will play a sound every `wait_time` in seconds until `REMINDER_IS_SET`
    is False."""
    global REMINDER_IS_SET
    while REMINDER_IS_SET:
        play_sound()
        sleep(wait_time)


# FIXME: This need further attention.
# Use alternative to threading for background processes?
def user_says_yes(message=""):
    """Depends on function `background_reminder`. It creates a thread with
    `background_reminder` and will stop the thread when user input is either
    'y' or 'n'. Returns a boolean."""
    # needs to be set globally for other functions to update correspondingly
    # Needs to reset for cases when feedback is required various times in a row
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
            break
        elif choice == 'n':
            choice = False
            break
        else:
            print("Please enter either 'y' or 'n'.")
    REMINDER_IS_SET = False
    return choice


def clear_logs(data_sources=None):
    '''Clear log files for each source specified in `DATA_SOURCES`.'''
    global REMINDER_IS_SET
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
                if ARGUMENTS.remind:
                    REMINDER_IS_SET = True
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
    later realize that rsync fails because the destination doesn't exist."""
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


def run_backup(data_sources=None):
    '''This is where all the action happens!'''

    options = {}  # Initially empty. Rsync options that will adjust on the fly.

    if data_sources is None:
        data_sources = {}

    for source, source_options in data_sources.items():
        print(source)
        source_options.append(options)
        main_options = source_options[0]  # Everything defined in settings.py
        custom_options = source_options[1]  # Set up on the fly for each source

        if LOG_NAME is not None:
            date_now = datetime.datetime.now()
            log_format = datetime.datetime.strftime(date_now, LOG_FORMAT)
            log_filename = f'{source}/{LOG_NAME}{log_format}'
            custom_options['logfilename'] = log_filename
            log_option = f'--log-file={log_filename}'
            custom_options['logfile'] = log_option
        else:
            custom_options['logfile'] = None

        # files to ignore in backup
        exclude_file = f'{source}/{BACKUP_EXCLUDE}'
        if os.path.exists(exclude_file):
            exclude_option = f'--exclude-from={exclude_file}'
            custom_options['exclude_option'] = exclude_option
        else:
            custom_options['exclude_option'] = None

        backing_source(source, main_options, custom_options)

        if PLAY_ON_EXIT:
            play_sound()


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
        help='Delete all log files for current source in `DATA_SOURCES`.',
        action='store_true')
    PARSER.add_argument(
        '-d', '--dest', dest='destination', default=None,
        help='Specify an alternative destination for backup as a string.',
        action='store')
    PARSER.add_argument(
        '-p', '--play',
        help='Play a sound in the background when launching the script.',
        action='store_true')
    PARSER.add_argument(
        '-r', '--remind', action='store_true',
        help=('Play a sound every X seconds when waiting for user feedback. '
              'Depends on `PLAY_WAIT_TIME`.')
        )

    # read arguments from the command line
    ARGUMENTS = PARSER.parse_args()

    PLAY_ON_EXIT = bool(ARGUMENTS.alert)
    REMINDER_IS_SET = bool(ARGUMENTS.remind)

    if ARGUMENTS.play:
        play_sound()
    if ARGUMENTS.clear:
        clear_logs(DATA_SOURCES)
        sys.exit(0)
    if ARGUMENTS.destination is not None:
        if os.path.isdir(ARGUMENTS.destination):
            DATA_DESTINATION = ARGUMENTS.destination
        print("Please enter a valid destination.")
        sys.exit(0)

    run_backup(data_sources=DATA_SOURCES)
