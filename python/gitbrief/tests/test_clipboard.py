"""Tests for gitbrief.clipboard module."""

from unittest.mock import MagicMock, patch


from gitbrief.clipboard import copy_to_clipboard


class TestCopyToClipboard:
    def test_darwin_uses_pbcopy(self):
        mock_run = MagicMock()
        mock_run.return_value = MagicMock()
        with patch("gitbrief.clipboard.sys.platform", "darwin"):
            with patch("gitbrief.clipboard.subprocess.run", mock_run):
                copy_to_clipboard("hello")
        cmd = mock_run.call_args[0][0]
        assert cmd == ["pbcopy"]

    def test_darwin_returns_true_on_success(self):
        with patch("gitbrief.clipboard.sys.platform", "darwin"):
            with patch("gitbrief.clipboard.subprocess.run"):
                assert copy_to_clipboard("hello") is True

    def test_linux_uses_xclip_when_available(self):
        mock_run = MagicMock()
        with patch("gitbrief.clipboard.sys.platform", "linux"):
            with patch(
                "gitbrief.clipboard.shutil.which", return_value="/usr/bin/xclip"
            ):
                with patch("gitbrief.clipboard.subprocess.run", mock_run):
                    copy_to_clipboard("hello")
        cmd = mock_run.call_args[0][0]
        assert "xclip" in cmd[0]

    def test_linux_falls_back_to_xsel(self):
        mock_run = MagicMock()

        def which_side_effect(name):
            return None if name == "xclip" else "/usr/bin/xsel"

        with patch("gitbrief.clipboard.sys.platform", "linux"):
            with patch(
                "gitbrief.clipboard.shutil.which", side_effect=which_side_effect
            ):
                with patch("gitbrief.clipboard.subprocess.run", mock_run):
                    copy_to_clipboard("hello")
        cmd = mock_run.call_args[0][0]
        assert "xsel" in cmd[0]

    def test_returns_false_when_no_tool_available(self):
        with patch("gitbrief.clipboard.sys.platform", "linux"):
            with patch("gitbrief.clipboard.shutil.which", return_value=None):
                assert copy_to_clipboard("hello") is False

    def test_returns_false_on_subprocess_error(self):
        import subprocess

        with patch("gitbrief.clipboard.sys.platform", "darwin"):
            with patch(
                "gitbrief.clipboard.subprocess.run",
                side_effect=subprocess.CalledProcessError(1, "pbcopy"),
            ):
                assert copy_to_clipboard("hello") is False

    def test_returns_false_on_file_not_found(self):
        with patch("gitbrief.clipboard.sys.platform", "darwin"):
            with patch(
                "gitbrief.clipboard.subprocess.run", side_effect=FileNotFoundError
            ):
                assert copy_to_clipboard("hello") is False
