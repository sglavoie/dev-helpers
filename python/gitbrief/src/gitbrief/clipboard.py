import shutil
import subprocess
import sys


def copy_to_clipboard(text: str) -> bool:
    """Copy text to system clipboard. Returns True on success."""
    if sys.platform == "darwin":
        cmd = ["pbcopy"]
    elif shutil.which("xclip"):
        cmd = ["xclip", "-selection", "clipboard"]
    elif shutil.which("xsel"):
        cmd = ["xsel", "--clipboard", "--input"]
    else:
        return False

    try:
        subprocess.run(cmd, input=text, text=True, check=True)
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False
