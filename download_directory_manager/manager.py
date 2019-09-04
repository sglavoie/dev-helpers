"""Script to manage download folder."""
# Standard library imports
import time
import os
from pathlib import Path
import logging

# Third-party imports: `pip install watchdog`
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

# Script imports
from config_file import FOLDER_TO_TRACK, FORMATS


class MyHandler(FileSystemEventHandler):
    def on_created(self, event):
        file_path = Path(event.src_path)
        file_ext = file_path.suffix.lower()  # so it works with uppercase too
        file_name = file_path.name
        if not event.is_directory:  # only check for files, not directories
            try:
                # Check against all known file formats in `config_file`
                for key, values in FORMATS.items():
                    for value in values:
                        if file_ext in value:
                            file_directory = f"{FOLDER_TO_TRACK}/{key}"
                            new_destination = (
                                f"{FOLDER_TO_TRACK}/{key}/{file_name}"
                            )
                            if not os.path.exists(file_directory):
                                # os.rename won't create the directory for us
                                os.makedirs(file_directory)
                            os.rename(file_path, new_destination)
            # FileNotFoundError can occur when creating many files at once
            except FileNotFoundError:
                pass

    def on_moved(self, event):
        pass

    def on_modified(self, event):
        pass

    def on_deleted(self, event):
        pass


def main():
    event_handler = MyHandler()
    observer = Observer()
    observer.schedule(event_handler, FOLDER_TO_TRACK, recursive=True)
    observer.start()

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        observer.stop()
    observer.join()


if __name__ == "__main__":
    # logging.basicConfig(level=logging.DEBUG)
    main()
