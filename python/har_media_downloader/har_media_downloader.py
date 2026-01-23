#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = ["requests>=2.28.0"]
# ///
# uv run har_media_downloader.py file.har            # Download all media
# uv run har_media_downloader.py file.har --dry-run  # List without downloading
# uv run har_media_downloader.py file.har --image    # Images only
"""
HAR Media Downloader

Parses HAR (HTTP Archive) files and downloads media content (audio/images/videos)
using cookies extracted from the HAR for authenticated access.

Usage:
    ./har_media_downloader.py file.har              # Download all media
    ./har_media_downloader.py file.har --image      # Download images only
    ./har_media_downloader.py file.har --audio --video  # Download audio and video
    ./har_media_downloader.py file.har -o ./output  # Specify output directory
    ./har_media_downloader.py file.har --dry-run    # List files without downloading
"""

import argparse
import json
import re
import sys
from pathlib import Path
from urllib.parse import urlparse, unquote

import requests

MIME_EXTENSIONS = {
    "audio/mpeg": ".mp3",
    "audio/mp3": ".mp3",
    "audio/wav": ".wav",
    "audio/x-wav": ".wav",
    "audio/ogg": ".ogg",
    "audio/flac": ".flac",
    "audio/aac": ".aac",
    "audio/webm": ".webm",
    "audio/mp4": ".m4a",
    "image/jpeg": ".jpg",
    "image/png": ".png",
    "image/gif": ".gif",
    "image/webp": ".webp",
    "image/svg+xml": ".svg",
    "image/bmp": ".bmp",
    "image/tiff": ".tiff",
    "image/x-icon": ".ico",
    "image/avif": ".avif",
    "video/mp4": ".mp4",
    "video/webm": ".webm",
    "video/ogg": ".ogv",
    "video/quicktime": ".mov",
    "video/x-msvideo": ".avi",
    "video/x-matroska": ".mkv",
    "video/mpeg": ".mpeg",
}

MEDIA_CATEGORIES = {
    "audio": "audio/",
    "image": "image/",
    "video": "video/",
}


def parse_har_file(har_path: Path) -> dict:
    """Load and validate a HAR JSON file."""
    try:
        with open(har_path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except FileNotFoundError:
        sys.exit(f"Error: HAR file not found: {har_path}")
    except json.JSONDecodeError as e:
        sys.exit(f"Error: Invalid JSON in HAR file: {e}")

    if "log" not in data or "entries" not in data.get("log", {}):
        sys.exit("Error: Invalid HAR format - missing 'log' or 'entries'")

    return data


def extract_media_entries(har_data: dict, media_types: set[str]) -> list[dict]:
    """
    Filter HAR entries by MIME type categories.

    Args:
        har_data: Parsed HAR data
        media_types: Set of media type prefixes to match (e.g., {"audio/", "image/"})

    Returns:
        List of HAR entries matching the specified media types
    """
    entries = har_data.get("log", {}).get("entries", [])
    media_entries = []

    for entry in entries:
        response = entry.get("response", {})
        content = response.get("content", {})
        mime_type = content.get("mimeType", "")

        # Handle MIME types with parameters (e.g., "image/jpeg; charset=utf-8")
        mime_type = mime_type.split(";")[0].strip().lower()

        if any(mime_type.startswith(prefix) for prefix in media_types):
            media_entries.append(entry)

    return media_entries


def extract_cookies_for_domain(har_data: dict, target_url: str) -> dict[str, str]:
    """
    Collect cookies from HAR entries matching the target URL's domain.

    HAR stores cookies in request.cookies[] with name, value, domain, and path fields.
    This function aggregates cookies using domain matching rules.
    """
    target_parsed = urlparse(target_url)
    target_domain = target_parsed.netloc.lower()

    cookies = {}
    entries = har_data.get("log", {}).get("entries", [])

    for entry in entries:
        request = entry.get("request", {})
        entry_url = request.get("url", "")
        entry_parsed = urlparse(entry_url)
        entry_domain = entry_parsed.netloc.lower()

        # Check if domains match (including subdomain matching)
        if domain_matches(entry_domain, target_domain):
            for cookie in request.get("cookies", []):
                name = cookie.get("name")
                value = cookie.get("value")
                if name and value is not None:
                    cookies[name] = value

    return cookies


def domain_matches(cookie_domain: str, target_domain: str) -> bool:
    """
    Check if a cookie domain matches the target domain.

    Handles subdomain matching (e.g., .example.com matches www.example.com).
    """
    cookie_domain = cookie_domain.lstrip(".").lower()
    target_domain = target_domain.lower()

    if cookie_domain == target_domain:
        return True

    # Check if target is a subdomain of cookie domain
    if target_domain.endswith("." + cookie_domain):
        return True

    # Check if cookie is a subdomain of target
    if cookie_domain.endswith("." + target_domain):
        return True

    # Check if they share the same base domain
    cookie_parts = cookie_domain.split(".")
    target_parts = target_domain.split(".")

    if len(cookie_parts) >= 2 and len(target_parts) >= 2:
        cookie_base = ".".join(cookie_parts[-2:])
        target_base = ".".join(target_parts[-2:])
        return cookie_base == target_base

    return False


def build_session_with_cookies(cookies: dict[str, str]) -> requests.Session:
    """Create a requests Session with the provided cookies."""
    session = requests.Session()
    for name, value in cookies.items():
        session.cookies.set(name, value)
    return session


def sanitize_filename(filename: str) -> str:
    """Remove or replace invalid filename characters."""
    # Replace invalid characters with underscores
    sanitized = re.sub(r'[<>:"/\\|?*]', "_", filename)
    # Remove control characters
    sanitized = re.sub(r"[\x00-\x1f\x7f]", "", sanitized)
    # Collapse multiple underscores
    sanitized = re.sub(r"_+", "_", sanitized)
    # Remove leading/trailing whitespace and dots
    sanitized = sanitized.strip(" .")
    # Limit length
    if len(sanitized) > 200:
        sanitized = sanitized[:200]
    return sanitized or "unnamed"


def generate_filename(url: str, mime_type: str, existing_names: set[str]) -> str:
    """
    Derive a unique filename from URL and MIME type.

    Handles duplicates by appending _1, _2, etc.
    """
    parsed = urlparse(url)
    path = unquote(parsed.path)

    # Extract filename from URL path
    if path and path != "/":
        filename = Path(path).name
    else:
        # Use domain + hash if no path
        filename = parsed.netloc.replace(".", "_")

    filename = sanitize_filename(filename)

    # Check if extension exists and is appropriate
    current_ext = Path(filename).suffix.lower()
    expected_ext = MIME_EXTENSIONS.get(mime_type.split(";")[0].strip().lower(), "")

    if not current_ext and expected_ext:
        filename = filename + expected_ext
    elif current_ext and expected_ext and current_ext != expected_ext:
        # Keep original extension but add expected if very different
        pass  # Keep original for now

    # Handle duplicates
    if filename not in existing_names:
        existing_names.add(filename)
        return filename

    base = Path(filename).stem
    ext = Path(filename).suffix
    counter = 1

    while True:
        new_name = f"{base}_{counter}{ext}"
        if new_name not in existing_names:
            existing_names.add(new_name)
            return new_name
        counter += 1


def download_file(
    session: requests.Session, url: str, dest_path: Path, timeout: int = 30
) -> bool:
    """
    Download a file using streaming HTTP GET.

    Returns True on success, False on failure.
    """
    try:
        response = session.get(url, stream=True, timeout=timeout)
        response.raise_for_status()

        with open(dest_path, "wb") as f:
            for chunk in response.iter_content(chunk_size=8192):
                if chunk:
                    f.write(chunk)

        return True
    except requests.RequestException as e:
        print(f"  Failed to download {url}: {e}")
        return False
    except OSError as e:
        print(f"  Failed to save {dest_path}: {e}")
        return False


def print_entry_info(entry: dict, filename: str) -> None:
    """Print information about a media entry."""
    url = entry.get("request", {}).get("url", "unknown")
    mime_type = entry.get("response", {}).get("content", {}).get("mimeType", "unknown")
    size = entry.get("response", {}).get("content", {}).get("size", 0)

    size_str = format_size(size) if size else "unknown size"
    print(f"  {filename} ({mime_type}, {size_str})")
    print(f"    URL: {url[:100]}{'...' if len(url) > 100 else ''}")


def format_size(size_bytes: int) -> str:
    """Format byte size as human-readable string."""
    for unit in ["B", "KB", "MB", "GB"]:
        if size_bytes < 1024:
            return f"{size_bytes:.1f} {unit}"
        size_bytes /= 1024
    return f"{size_bytes:.1f} TB"


def main():
    parser = argparse.ArgumentParser(
        description="Download media files from HAR (HTTP Archive) files.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s recording.har              Download all media
  %(prog)s recording.har --image      Download images only
  %(prog)s recording.har --audio --video  Download audio and video
  %(prog)s recording.har -o ./output  Specify output directory
  %(prog)s recording.har --dry-run    List files without downloading
        """,
    )

    parser.add_argument("har_file", type=Path, help="Path to the HAR file")
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        help="Output directory (default: <har_filename>_media/)",
    )
    parser.add_argument(
        "--audio", action="store_true", help="Download audio files only"
    )
    parser.add_argument(
        "--image", action="store_true", help="Download image files only"
    )
    parser.add_argument(
        "--video", action="store_true", help="Download video files only"
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="List files without downloading",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=30,
        help="Download timeout in seconds (default: 30)",
    )

    args = parser.parse_args()

    # Determine which media types to download
    media_types = set()
    if args.audio:
        media_types.add(MEDIA_CATEGORIES["audio"])
    if args.image:
        media_types.add(MEDIA_CATEGORIES["image"])
    if args.video:
        media_types.add(MEDIA_CATEGORIES["video"])

    # If no specific type requested, download all
    if not media_types:
        media_types = set(MEDIA_CATEGORIES.values())

    # Parse HAR file
    print(f"Parsing HAR file: {args.har_file}")
    har_data = parse_har_file(args.har_file)

    # Extract media entries
    media_entries = extract_media_entries(har_data, media_types)
    print(f"Found {len(media_entries)} media entries")

    if not media_entries:
        print("No media files found matching the specified criteria.")
        return

    # Determine output directory
    if args.output:
        output_dir = args.output
    else:
        output_dir = Path(f"{args.har_file.stem}_media")

    if args.dry_run:
        print(f"\nDry run - would download to: {output_dir}")
    else:
        output_dir.mkdir(parents=True, exist_ok=True)
        print(f"Downloading to: {output_dir}")

    # Process entries
    existing_names: set[str] = set()
    successful = 0
    failed = 0

    print("\nMedia files:")
    for entry in media_entries:
        url = entry.get("request", {}).get("url", "")
        mime_type = (
            entry.get("response", {}).get("content", {}).get("mimeType", "").lower()
        )

        filename = generate_filename(url, mime_type, existing_names)
        print_entry_info(entry, filename)

        if args.dry_run:
            continue

        # Extract cookies for this URL's domain
        cookies = extract_cookies_for_domain(har_data, url)
        session = build_session_with_cookies(cookies)

        dest_path = output_dir / filename
        if download_file(session, url, dest_path, args.timeout):
            successful += 1
        else:
            failed += 1

    # Print summary
    if not args.dry_run:
        print(f"\nDownload complete: {successful} successful, {failed} failed")
    else:
        print(f"\nDry run complete: {len(media_entries)} files would be downloaded")


if __name__ == "__main__":
    main()
