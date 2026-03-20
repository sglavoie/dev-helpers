import datetime
import re
import subprocess
from pathlib import Path


def validate_repo(path: str) -> str | None:
    """Returns error message if path is not a valid git repo, None otherwise."""
    p = Path(path)
    if not p.exists():
        return f"Path does not exist: {path}"
    if not (p / ".git").exists():
        return f"Not a git repository: {path}"
    return None


def get_git_user_email(repo_path: str) -> str | None:
    """Returns the git user.email configured for the given repo."""
    try:
        result = subprocess.run(
            ["git", "-C", repo_path, "config", "user.email"],
            capture_output=True,
            text=True,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except FileNotFoundError:
        pass
    return None


def validate_date_string(s: str) -> bool:
    """Returns True if s is a valid YYYY-MM-DD date string."""
    try:
        datetime.date.fromisoformat(s)
        return True
    except ValueError:
        return False


def parse_duration(duration: str) -> str:
    """Converts duration string or named preset to ISO date string.

    Supported units: d (days), w (weeks), m (months ~30d), y (years ~365d)
    Named presets: today, yesterday, this-week, last-week, this-month, last-month
    """
    today = datetime.date.today()

    presets = {
        "today": today,
        "yesterday": today - datetime.timedelta(days=1),
        "this-week": today - datetime.timedelta(days=today.weekday()),
        "last-week": today - datetime.timedelta(days=today.weekday() + 7),
        "this-month": today.replace(day=1),
        "last-month": (
            today.replace(month=today.month - 1, day=1)
            if today.month > 1
            else today.replace(year=today.year - 1, month=12, day=1)
        ),
    }
    if duration in presets:
        return presets[duration].isoformat()

    match = re.fullmatch(r"(\d+)([dwmy])", duration)
    if not match:
        raise ValueError(
            f"Invalid duration format: '{duration}'. "
            "Use e.g. 1d, 1w, 2w, 1m, 1y or a preset like today, this-week."
        )

    amount = int(match.group(1))
    unit = match.group(2)

    if unit == "d":
        delta = datetime.timedelta(days=amount)
    elif unit == "w":
        delta = datetime.timedelta(weeks=amount)
    elif unit == "m":
        delta = datetime.timedelta(days=amount * 30)
    elif unit == "y":
        delta = datetime.timedelta(days=amount * 365)
    else:
        raise ValueError(f"Unknown unit: {unit}")

    return (today - delta).isoformat()


MAX_COMMITS = 100

_REF_HASH_PATTERN = re.compile(r"#(\d+)")
_REF_URL_PATTERN = re.compile(r"github\.com/[^/\s]+/[^/\s]+/(?:issues|pull)/(\d+)")


def _parse_shortstat(line: str) -> dict | None:
    """Parse a git --shortstat line into files_changed/insertions/deletions."""
    files_match = re.search(r"(\d+) files? changed", line)
    if not files_match:
        return None
    ins_match = re.search(r"(\d+) insertions?\(\+\)", line)
    del_match = re.search(r"(\d+) deletions?\(-\)", line)
    return {
        "files_changed": int(files_match.group(1)),
        "insertions": int(ins_match.group(1)) if ins_match else 0,
        "deletions": int(del_match.group(1)) if del_match else 0,
    }


def _extract_diff_stats(
    repo_path: str,
    since: str,
    author: str | None = None,
    until: str | None = None,
) -> dict[str, dict]:
    """Return {sha: {files_changed, insertions, deletions}} for commits in range."""
    cmd = ["git", "-C", repo_path, "log", f"--since={since}", "--format=%H", "--shortstat"]
    if until:
        cmd.append(f"--until={until}")
    if author:
        cmd.append(f"--author={author}")

    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
    except FileNotFoundError:
        return {}

    if result.returncode != 0:
        return {}

    stats: dict[str, dict] = {}
    current_sha: str | None = None
    for line in result.stdout.splitlines():
        stripped = line.strip()
        if re.fullmatch(r"[0-9a-f]{40}", stripped):
            current_sha = stripped
        elif current_sha and stripped:
            stat = _parse_shortstat(stripped)
            if stat:
                stats[current_sha] = stat
    return stats


def _extract_branch_tips(
    repo_path: str,
    since: str,
    author: str | None = None,
    until: str | None = None,
) -> dict[str, str]:
    """Return {sha: branch_name} for commits at branch tips in range."""
    cmd = ["git", "-C", repo_path, "log", f"--since={since}", "--format=%H|%D"]
    if until:
        cmd.append(f"--until={until}")
    if author:
        cmd.append(f"--author={author}")

    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
    except FileNotFoundError:
        return {}

    if result.returncode != 0:
        return {}

    branch_tips: dict[str, str] = {}
    for line in result.stdout.splitlines():
        if "|" not in line:
            continue
        sha, decorations = line.split("|", 1)
        sha = sha.strip()
        decorations = decorations.strip()
        if not decorations:
            continue
        # HEAD -> name takes priority
        head_match = re.search(r"HEAD -> ([^,]+)", decorations)
        if head_match:
            branch_tips[sha] = head_match.group(1).strip()
            continue
        # Fall back to origin/name (skip generic names)
        origin_match = re.search(r"origin/([^,]+)", decorations)
        if origin_match:
            name = origin_match.group(1).strip()
            if name not in ("HEAD", "main", "master"):
                branch_tips[sha] = name
    return branch_tips


def _extract_refs(text: str) -> list[str]:
    """Extract PR/issue refs (#123) from commit subject/body text."""
    seen: set[str] = set()
    refs: list[str] = []
    for m in _REF_HASH_PATTERN.finditer(text):
        ref = f"#{m.group(1)}"
        if ref not in seen:
            refs.append(ref)
            seen.add(ref)
    for m in _REF_URL_PATTERN.finditer(text):
        ref = f"#{m.group(1)}"
        if ref not in seen:
            refs.append(ref)
            seen.add(ref)
    return refs


def extract_commits(
    repo_path: str,
    since: str,
    author: str | None = None,
    until: str | None = None,
) -> list[dict]:
    """Extract commits from a git repo since a given date."""
    cmd = [
        "git",
        "-C",
        repo_path,
        "log",
        f"--since={since}",
        "--format=%H|%s|%b%x00",
    ]
    if until:
        cmd.append(f"--until={until}")
    if author:
        cmd.append(f"--author={author}")

    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
    except FileNotFoundError:
        return []

    if result.returncode != 0:
        return []

    commits = []
    records = result.stdout.split("\0")
    for record in records:
        record = record.strip()
        if not record:
            continue
        parts = record.split("|", 2)
        if len(parts) < 3:
            continue
        sha = parts[0]
        subject = parts[1]
        body = parts[2].strip()
        commits.append({
            "sha": sha,
            "subject": subject,
            "body": body,
            "refs": _extract_refs(subject + " " + body),
        })
        if len(commits) >= MAX_COMMITS:
            break

    if not commits:
        return commits

    diff_stats = _extract_diff_stats(repo_path, since, author, until)
    branch_tips = _extract_branch_tips(repo_path, since, author, until)

    for commit in commits:
        sha = commit["sha"]
        if sha in diff_stats:
            commit.update(diff_stats[sha])
        if sha in branch_tips:
            commit["branch"] = branch_tips[sha]

    return commits
