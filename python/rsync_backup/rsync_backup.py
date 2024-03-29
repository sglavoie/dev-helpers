"""
Script that uses `rsync` to make a simple and convenient backup.
Note: requires Python 3.6+. No other Python third-party libraries required.
"""
import argparse
import datetime
import glob
import json
import os
import pathlib
import subprocess


def run_backup():
    """This is where all the action happens!"""
    settings = get_settings()

    # initiate the parser
    parser = argparse.ArgumentParser("backup")
    parser.add_argument(
        "-c",
        "--clear",
        help="Delete all log files for current source in DATA_SOURCES.",
        action="store_true",
    )
    parser.add_argument(
        "-s",
        "--src",
        dest="source",
        default=None,
        help="Specify an alternative source to backup as a string.",
        action="store",
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

    # check for --source or -s
    # Replace potential list of sources to backup with this one
    if arguments.source is not None:
        if os.path.isdir(arguments.source):
            settings["data_sources"] = [arguments.source]
        else:
            print("Please enter a valid source to backup.")
            exit(0)

    # check for --clear or -c
    if arguments.clear:
        clear_logs(
            data_sources=settings["data_sources"],
            log_name=settings["log_name"],
        )

    # check for --dest or -d
    # Replace destination to backup with this one
    if arguments.destination is not None:
        if os.path.isdir(arguments.destination):
            settings["data_destination"] = arguments.destination
        else:
            print("Please enter a valid destination.")
            exit(0)

    # don't run the script if the destination doesn't exist
    if not os.path.isdir(settings["data_destination"]):
        print(
            f"The destination doesn't exist.\n({settings['data_destination']})"
        )
        exit(0)

    backup_all_sources(settings)


def backup_all_sources(settings: dict) -> None:
    """Iterate over all sources to backup."""
    for source in settings["data_sources"]:
        date_now = datetime.datetime.now()
        log_format = datetime.datetime.strftime(
            date_now, settings["log_format"]
        )
        log_filename = f"{source}/{settings['log_name']}{log_format}"
        log_option = f"--log-file={log_filename}"

        backup_source = settings["backup_cmd"].copy()
        backup_source.extend([log_option])

        # files to include in backup that would otherwise be excluded with
        # --exclude-from
        include_file = f"{source}/{settings['backup_include']}"

        if os.path.exists(include_file):
            include_option = f"--include-from={include_file}"
            backup_source.append(include_option)

        # files to ignore in backup
        exclude_file = f"{source}/{settings['backup_exclude']}"

        if os.path.exists(exclude_file):
            exclude_option = f"--exclude-from={exclude_file}"
            backup_source.extend(
                [exclude_option, source, settings["data_destination"]]
            )
        else:
            # skips '--exclude-from' option if no file is found
            backup_source.extend([source, settings["data_destination"]])

        settings["source"] = source
        settings["backup_source"] = backup_source
        settings["log_filename"] = log_filename

        try:
            backing_source(settings)
        except KeyboardInterrupt:
            print("\nKeyboardInterrupt: Exiting operations.")
            exit(0)


def backing_source(settings: dict) -> None:
    """Print information to STDOUT and to `log_filename` and executes the
    rsync command."""
    print(settings["sep"] * settings["terminal_width"])

    cmd_executed = " ".join(settings["backup_source"])
    msg_executed = f"Command executed:\n{cmd_executed}\n"
    print(msg_executed)

    with open(settings["log_filename"], mode="w") as log_file:
        log_file.write(f"{msg_executed}\n")

    try:
        child = subprocess.Popen(settings["backup_source"])
        _ = child.communicate()[0]  # call communicate to get the return code
        rc = child.returncode
    except FileNotFoundError:
        print(f"FileNotFoundError: Is the `rsync` tool installed?")
        exit(1)

    print(f"\nBackup completed for: {settings['source']} (return code: {rc})")
    print(settings["sep"] * settings["terminal_width"])


def clear_logs(data_sources: list, log_name: str) -> None:
    """Clears log files for each source specified in SETTINGS."""
    for source in data_sources:
        # Retrieve a list of all matching log files in `source`
        log_files = glob.glob(f"{source}/{log_name}*")
        if log_files == []:
            print(f"\nThere is no log file to delete in {source}.")
            exit(0)
        else:
            print(f"Log files in {source}:")
            for log_file in log_files:
                print(log_file)
            if user_says_yes():
                for log_file in log_files:
                    os.remove(log_file)
                print("Log files deleted.")
        print("Exiting script...")
        exit(0)


def user_says_yes(
    msg: str = "\nDo you want to delete log files for this source? (y/n) ",
) -> bool:
    """Asks the user to enter either "y" or "n" to confirm. Returns boolean."""
    choice = None
    while choice is None:
        user_input = input(msg)
        if user_input.lower() == "y":
            choice = True
        elif user_input.lower() == "n":
            choice = False
        else:
            print('Please enter either "y" or "n".')
    return choice


def get_settings() -> dict:
    """
    Get the settings from `settings.json`.

    Returns:
        dict: Containing all settings used by the tool.
    """
    directory = pathlib.Path(__file__).parent.absolute()
    with open(directory / "settings.json") as fp:
        settings = json.load(fp)

    backup_cmd = ["rsync"]
    backup_cmd.extend(settings["rsync_options"])
    settings["backup_cmd"] = backup_cmd

    return settings


if __name__ == "__main__":
    run_backup()
