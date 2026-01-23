#!/usr/bin/env python3
"""Find and remove duplicate files based on content hash.

This tool scans a directory for files with identical content and helps
clean up duplicates, keeping the most likely "original" based on filename
patterns.

Usage:
    ./dedupe_files.py /path/to/directory              # Dry-run: list duplicates
    ./dedupe_files.py /path/to/directory --delete     # Delete duplicates
    ./dedupe_files.py /path/to/directory --move trash/  # Move to trash dir
    ./dedupe_files.py /path/to/directory -r           # Recursive search
    ./dedupe_files.py /path/to/directory --hash sha256  # Use SHA256

# /// script
# requires-python = ">=3.9"
# ///
"""

from __future__ import annotations

import argparse
import hashlib
import re
import sys
from collections import defaultdict
from pathlib import Path


CHUNK_SIZE = 8192  # 8KB chunks for reading files


def hash_file(path: Path, algorithm: str = "md5") -> str | None:
    """Compute the content hash of a file using chunked reads.

    Args:
        path: Path to the file to hash.
        algorithm: Hash algorithm to use ('md5' or 'sha256').

    Returns:
        Hex digest of the file content, or None if file cannot be read.
    """
    hasher = hashlib.new(algorithm)
    try:
        with open(path, "rb") as f:
            while chunk := f.read(CHUNK_SIZE):
                hasher.update(chunk)
        return hasher.hexdigest()
    except (OSError, PermissionError) as e:
        print(f"Warning: Cannot read {path}: {e}", file=sys.stderr)
        return None


def scan_directory(directory: Path, recursive: bool = False) -> list[Path]:
    """Collect all files in the directory.

    Args:
        directory: Directory to scan.
        recursive: Whether to descend into subdirectories.

    Returns:
        List of Path objects for all files found.
    """
    if recursive:
        return [p for p in directory.rglob("*") if p.is_file()]
    return [p for p in directory.iterdir() if p.is_file()]


def group_by_hash(
    files: list[Path], algorithm: str = "md5"
) -> dict[str, list[Path]]:
    """Build a dict mapping hash -> list of file paths.

    Args:
        files: List of file paths to hash.
        algorithm: Hash algorithm to use.

    Returns:
        Dict where keys are hashes and values are lists of files with that hash.
        Only includes groups with more than one file (duplicates).
    """
    hash_groups: dict[str, list[Path]] = defaultdict(list)
    total = len(files)

    for i, path in enumerate(files, 1):
        print(f"\rHashing files: {i}/{total}", end="", flush=True)
        file_hash = hash_file(path, algorithm)
        if file_hash:
            hash_groups[file_hash].append(path)

    print()  # Newline after progress

    # Filter to only duplicate groups
    return {h: paths for h, paths in hash_groups.items() if len(paths) > 1}


def get_suffix_number(filename: str) -> int | None:
    """Extract the _N suffix number from a filename if present.

    Examples:
        'photo_1.jpg' -> 1
        'photo_12.jpg' -> 12
        'photo.jpg' -> None
    """
    stem = Path(filename).stem
    match = re.search(r"_(\d+)$", stem)
    return int(match.group(1)) if match else None


def select_keeper(file_group: list[Path]) -> tuple[Path, list[Path]]:
    """Choose which file to keep from a group of duplicates.

    Strategy (in order of preference):
    1. Files without _N suffix are preferred
    2. Shorter filenames are preferred
    3. Alphabetically first filename as tiebreaker

    Args:
        file_group: List of duplicate file paths.

    Returns:
        Tuple of (keeper_path, list_of_duplicates_to_remove).
    """

    def sort_key(path: Path) -> tuple[int, int, str]:
        name = path.name
        suffix_num = get_suffix_number(name)
        has_suffix = 0 if suffix_num is None else 1
        return (has_suffix, len(name), name.lower())

    sorted_files = sorted(file_group, key=sort_key)
    return sorted_files[0], sorted_files[1:]


def format_size(size_bytes: int) -> str:
    """Format byte size as human-readable string."""
    for unit in ["B", "KB", "MB", "GB"]:
        if size_bytes < 1024:
            return f"{size_bytes:.1f} {unit}"
        size_bytes /= 1024
    return f"{size_bytes:.1f} TB"


def process_duplicates(
    groups: dict[str, list[Path]],
    action: str,
    dest: Path | None = None,
) -> None:
    """Process duplicate groups according to the specified action.

    Args:
        groups: Dict mapping hash -> list of duplicate file paths.
        action: One of 'dry-run', 'delete', or 'move'.
        dest: Destination directory for 'move' action.
    """
    total_removed = 0
    total_bytes_freed = 0

    for file_hash, file_group in groups.items():
        keeper, duplicates = select_keeper(file_group)
        file_size = keeper.stat().st_size

        print(f"\nDuplicate group ({len(file_group)} files, {format_size(file_size)} each):")
        print(f"  KEEP: {keeper}")

        for dup in duplicates:
            print(f"  DUP:  {dup}")
            total_removed += 1
            total_bytes_freed += file_size

            if action == "delete":
                try:
                    dup.unlink()
                except OSError as e:
                    print(f"  Error deleting {dup}: {e}", file=sys.stderr)

            elif action == "move" and dest:
                try:
                    dest.mkdir(parents=True, exist_ok=True)
                    target = dest / dup.name
                    # Handle name collision in destination
                    counter = 1
                    while target.exists():
                        stem = dup.stem
                        suffix = dup.suffix
                        target = dest / f"{stem}_{counter}{suffix}"
                        counter += 1
                    dup.rename(target)
                except OSError as e:
                    print(f"  Error moving {dup}: {e}", file=sys.stderr)

    # Summary
    action_verb = {
        "dry-run": "would be removed",
        "delete": "removed",
        "move": "moved",
    }[action]

    print(f"\nSummary: {len(groups)} duplicate groups, {total_removed} files {action_verb}, {format_size(total_bytes_freed)} freed")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Find and remove duplicate files based on content hash.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s /path/to/dir              Dry-run: show duplicates without removing
  %(prog)s /path/to/dir --delete     Delete duplicate files
  %(prog)s /path/to/dir --move trash Move duplicates to 'trash' directory
  %(prog)s /path/to/dir -r           Search recursively in subdirectories
  %(prog)s /path/to/dir --hash sha256 Use SHA256 instead of MD5
""",
    )
    parser.add_argument(
        "directory",
        type=Path,
        help="Directory to scan for duplicates",
    )
    parser.add_argument(
        "-r", "--recursive",
        action="store_true",
        help="Search subdirectories recursively",
    )
    parser.add_argument(
        "--hash",
        choices=["md5", "sha256"],
        default="md5",
        dest="algorithm",
        help="Hash algorithm to use (default: md5)",
    )

    action_group = parser.add_mutually_exclusive_group()
    action_group.add_argument(
        "--delete",
        action="store_true",
        help="Delete duplicate files (keep one copy)",
    )
    action_group.add_argument(
        "--move",
        type=Path,
        metavar="DIR",
        help="Move duplicates to specified directory",
    )

    args = parser.parse_args()

    # Validate directory
    if not args.directory.exists():
        print(f"Error: Directory does not exist: {args.directory}", file=sys.stderr)
        return 1

    if not args.directory.is_dir():
        print(f"Error: Not a directory: {args.directory}", file=sys.stderr)
        return 1

    # Scan for files
    print(f"Scanning {args.directory}{'recursively' if args.recursive else ''}...")
    files = scan_directory(args.directory, args.recursive)

    if not files:
        print("No files found.")
        return 0

    print(f"Found {len(files)} files. Computing {args.algorithm.upper()} hashes...")

    # Group by hash
    groups = group_by_hash(files, args.algorithm)

    if not groups:
        print("No duplicates found.")
        return 0

    # Determine action
    if args.delete:
        action = "delete"
    elif args.move:
        action = "move"
    else:
        action = "dry-run"

    # Process duplicates
    process_duplicates(groups, action, args.move)

    return 0


if __name__ == "__main__":
    sys.exit(main())
