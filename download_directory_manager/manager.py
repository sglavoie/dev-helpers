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
from config_file import *


class MyHandler(FileSystemEventHandler):
    def on_created(self, event):
        file_ext = Path(event.src_path).suffix[1:]  # e.g. 'png' for '.png'
        if not event.is_directory:  # only check for files, not directories
            for key, values in FORMATS.items():
                for value in values:
                    if file_ext in value:
                        print(event.src_path)
                        print(FOLDER_TO_TRACK)
                        print(file_ext)

        # for filename in os.listdir(FOLDER_TO_TRACK):
        #     pass

        # src = FOLDER_TO_TRACK + "/" + filename
        # new_destination = folder_destination + "/" + filename
        # os.rename(src, new_destination)

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
