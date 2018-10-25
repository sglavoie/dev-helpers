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
