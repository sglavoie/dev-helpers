"""
Small script that uses `rsync` to make a simple and convenient backup.
Note: requires Python 3.6+. No other Python third-party libraries required.
"""
import argparse
import datetime
import glob
import os
import subprocess
import sys


BACKUP_CMD = ["rsync"]  # This will be used to add options to `rsync` command

###############################################################################
# SETTINGS
###############################################################################

# Directories to backup, supplied as a list of strings (no slash at the end)
DATA_SOURCES = [
    "/home/sglavoie",
    # '/usr/bin',
    # '/etc'
]

# Single destination of the files to backup, supplied as a string
# This can be overridden when passing option '-d' or '--dest' to the script
DATA_DESTINATION = "/media/sglavoie/Elements"

# Line length in the terminal, used for printing separators
TERMINAL_WIDTH = 40

# Separator to use along with TERMINAL_WIDTH
SEP = "=-"  # using 2 characters, we have to divide TERMINAL_WIDTH by 2 also

# Sets the prefix of the log filename
LOG_NAME = ".backup_log_"

# This goes right after LOG_NAME as a suffix
LOG_FORMAT = "%y%m%d_%H_%M_%S"

# Options to use with rsync as a list of strings
RSYNC_OPTIONS = [
    "-vaAHh",
    "--delete",
    "--ignore-errors",
    "--force",
    "--prune-empty-dirs",
    "--delete-excluded",
]

BACKUP_CMD.extend(RSYNC_OPTIONS)  # adds options set above

# Default file in each source in DATA_SOURCES where files/directories
# will be ignored
BACKUP_EXCLUDE = ".backup_exclude"

###############################################################################
# FUNCTIONS
###############################################################################


def better_separation(the_function):
    """Decorator used to print separators around `the_function`."""

    def print_separator(*args, **kwargs):
        """Surrounds `the_function` with a separator and add a new line."""
        separator = SEP * TERMINAL_WIDTH
        print(separator)
        the_function(*args, **kwargs)
        print(separator, "\n")

    return print_separator


@better_separation
def backing_source(source, backup_source, log_filename):
    """Print information to STDOUT and to `log_filename` and executes the
    rsync command."""
    cmd_executed = " ".join(backup_source)
    msg_executed = f"Command executed:\n{cmd_executed}\n"
    print(msg_executed)
    with open(log_filename, mode="w") as log_file:
        log_file.write(f"{msg_executed}\n")
    subprocess.run(backup_source)

    print(f"\nBackup completed for: {source}")


def user_says_yes():
    """Asks the user to enter either "y" or "n" to confirm. Returns boolean."""
    choice = None
    while choice is None:
        user_input = input("\nDo you want to delete log files for this source? (y/n) ")
        if user_input.lower() == "y":
            choice = True
        elif user_input.lower() == "n":
            choice = False
        else:
            print('Please enter either "y" or "n".')
    return choice


def clear_logs(*args):
    """Clears log files for each source specified in SETTINGS."""
    for source in args:
        # Retrieve a list of all matching log files in `source`
        log_files = glob.glob(f"{source}/{LOG_NAME}*")
        if log_files == []:
            print(f"\nThere is no log file to delete in {source}.")
            sys.exit(0)
        else:
            print(f"Log files in {source}:")
            for log_file in log_files:
                print(log_file)
            if user_says_yes():
                for log_file in log_files:
                    os.remove(log_file)
                print("Log files deleted.")
        print("Exiting script...")
        sys.exit(0)


def run_backup(*args, data_destination=DATA_DESTINATION):
    """This is where all the action happens!"""
    # initiate the parser
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "-c",
        "--clear",
        help="Delete all log files for current source in DATA_SOURCES.",
        action="store_true",
    )
    parser.add_argument(
        "-d",
        "--dest",
        dest="destination",
        default=None,
        help="Specify an alternative destination for backup as a string.",
        action="store",
    )

    # read arguments from the command line
    arguments = parser.parse_args()

    # check for --clear or -c
    if arguments.clear:
        clear_logs(*args)
        # check for --dest or -d
    if arguments.destination is not None:
        if os.path.isdir(arguments.destination):
            data_destination = arguments.destination
        else:
            print("Please enter a valid destination.")
            sys.exit(0)

    # don't run the script if the destination doesn't exist
    if not os.path.isdir(data_destination):
        print(f"The destination doesn't exist.\n({data_destination})")
        sys.exit(0)

    for source in args:
        date_now = datetime.datetime.now()
        log_format = datetime.datetime.strftime(date_now, LOG_FORMAT)
        log_filename = f"{source}/{LOG_NAME}{log_format}"
        log_option = f"--log-file={log_filename}"

        backup_source = BACKUP_CMD.copy()
        backup_source.extend([log_option])

        # files to ignore in backup
        exclude_file = f"{source}/{BACKUP_EXCLUDE}"

        if os.path.exists(exclude_file):
            exclude_option = f"--exclude-from={exclude_file}"
            backup_source.extend([exclude_option, source, data_destination])
        else:  # skips '--exclude-from' option if no file is found
            backup_source.extend([source, data_destination])

        backing_source(source, backup_source, log_filename)


###############################################################################
# EXECUTION
###############################################################################

if __name__ == "__main__":
    run_backup(*DATA_SOURCES)
