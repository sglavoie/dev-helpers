# Standard library imports
import sys
import uuid

sys.path.insert(0, "../src")

# Third-party library imports
import pytest

# Local imports
from src import zsh_history_cleaner


def test_empty_file_returns_zero_line_length(tmpdir):
    temp_file = tmpdir.join("temp.txt")
    temp_file.write_text("", encoding="utf-8")
    assert zsh_history_cleaner.file_length(temp_file) == 0


def test_file_length_returns_expected_int(tmpdir):
    temp_file = tmpdir.join("temp.txt")
    temp_file.write_text("line one\nline two", encoding="utf-8")
    assert zsh_history_cleaner.file_length(temp_file) == 2
    temp_file.write_text("line one\n\n\nline four", encoding="utf-8")
    assert zsh_history_cleaner.file_length(temp_file) == 4
    temp_file.write_text("\n\n\n\n\n", encoding="utf-8")
    assert zsh_history_cleaner.file_length(temp_file) == 5
    temp_file.write_text("_", encoding="utf-8")
    assert zsh_history_cleaner.file_length(temp_file) == 1
