'''
=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
SETTINGS
=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
'''
# `rsync` options that can be configured independently for each source.
MAIN_OPTIONS = ['-vaHh', '--delete', '--ignore-errors', '--force',
                '--prune-empty-dirs', '--delete-excluded']
# MAIN_OPTIONS_2 = ['-vaHh', '--delete', '--ignore-errors', '--force',
# '--prune-empty-dirs', '--delete-excluded']


# Directories to back up, supplied as a dictionary where {key: value}
# corresponds to {'source_to_back_up': ['list', 'of', 'rsync', 'options']}
# If the source directory contains a slash at the end, the CONTENT will be
# copied without recreating the source directory.
# Each source can be specified with its own rsync options.
DATA_SOURCES = {
    '/tmp/backup': [MAIN_OPTIONS],
    '/tmp/backup2': [MAIN_OPTIONS],
    # '/tmp/backup3': [MAIN_OPTIONS_2]  # Example with other options
}


# Single destination of the files to back up, supplied as a string
# This can be overridden when passing option '-d' or '--dest' to the script
# DATA_DESTINATION = '/media/sgdlavoie/Elements'
DATA_DESTINATION = '/tmp/destination'

# Line length in the terminal, used for printing separators
TERMINAL_WIDTH = 40

# Separator to use along with TERMINAL_WIDTH
SEP = '……'  # using 2 characters, we have to divide TERMINAL_WIDTH by 2 also

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
PLAY_WAIT_TIME = 3

# Path of sound to play
SOUND_PATH = '/home/sglavoie/Music/.level_up.wav'
