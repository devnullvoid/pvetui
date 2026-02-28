#!/usr/bin/env python3
import argparse
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone


def env(name: str) -> str:
    return os.environ.get(name, "")


def require_env(names):
    missing = [n for n in names if not env(n)]
    if missing:
        raise RuntimeError("Missing required env vars: " + ", ".join(missing))


def http_post_json(url: str, payload: dict, headers: dict | None = None) -> dict:
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    if headers:
        for k, v in headers.items():
            req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            body = resp.read().decode("utf-8")
            return json.loads(body) if body else {}
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        raise RuntimeError(f"HTTP {e.code} from {url}: {body}") from e


def http_timeout_seconds(default: int = 20) -> int:
    raw = env("ANNOUNCE_AI_TIMEOUT_SECONDS").strip()
    if not raw:
        return default
    try:
        value = int(raw)
    except ValueError:
        return default
    return max(3, min(value, 60))


def ai_summary_enabled() -> bool:
    val = env("ANNOUNCE_AI_ENABLED").strip().lower()
    return val in {"1", "true", "yes", "on"}


def ai_debug_enabled() -> bool:
    val = env("ANNOUNCE_AI_DEBUG").strip().lower()
    return val in {"1", "true", "yes", "on"}


def summarize_highlights_with_ai(
    tag: str, highlights: list[str], max_chars: int
) -> tuple[str, str]:
    if not highlights:
        return "", "no highlights available"

    base_url = env("ANNOUNCE_AI_BASE_URL").strip()
    model = env("ANNOUNCE_AI_MODEL").strip()
    api_key = env("ANNOUNCE_AI_API_KEY").strip()
    if not (base_url and model and api_key):
        return (
            "",
            "missing ANNOUNCE_AI_BASE_URL / ANNOUNCE_AI_MODEL / ANNOUNCE_AI_API_KEY",
        )

    prompt_lines = "\n".join(f"- {h}" for h in highlights[:8])
    payload = {
        "model": model,
        "messages": [
            {
                "role": "system",
                "content": (
                    "You summarize release notes into one short line. "
                    "Return plain text only, no markdown, no hashtags, no quotes."
                ),
            },
            {
                "role": "user",
                "content": (
                    f"Summarize these release highlights for {tag} in <= {max_chars} characters.\n"
                    "Focus on the most user-visible improvements.\n"
                    f"{prompt_lines}"
                ),
            },
        ],
        "temperature": 1.0,
    }

    url = base_url.rstrip("/") + "/chat/completions"
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "application/json")
    req.add_header("User-Agent", "pvetui-release-announcer/1.0")
    req.add_header("Authorization", f"Bearer {api_key}")
    try:
        with urllib.request.urlopen(req, timeout=http_timeout_seconds()) as resp:
            body = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        body = ""
        try:
            body = exc.read().decode("utf-8", errors="replace").strip()
        except Exception:
            body = ""
        if len(body) > 240:
            body = body[:237] + "..."
        if body:
            return "", f"AI HTTP error: {exc.code} body={body}"
        return "", f"AI HTTP error: {exc.code}"
    except urllib.error.URLError as exc:
        return "", f"AI URL error: {exc.reason}"
    except TimeoutError:
        return "", "AI request timed out"

    try:
        parsed = json.loads(body)
    except json.JSONDecodeError:
        return "", "AI response was not valid JSON"

    choices = parsed.get("choices") or []
    if not choices:
        return "", "AI response had no choices"
    message = choices[0].get("message") or {}
    text = (message.get("content") or "").strip()
    if not text:
        return "", "AI response content was empty"

    text = text.replace("\n", " ").replace("\r", " ").strip()
    text = re.sub(r"\s+", " ", text)
    text = re.sub(r"`([^`]+)`", r"\1", text)
    text = re.sub(r"\*\*([^*]+)\*\*", r"\1", text)
    text = text.strip(" \"'")
    if len(text) > max_chars:
        text = text[: max_chars - 3].rstrip() + "..."
    return text, ""


def post_mastodon(message: str):
    require_env(["MASTODON_SERVER", "MASTODON_ACCESS_TOKEN"])
    server = env("MASTODON_SERVER").rstrip("/")
    token = env("MASTODON_ACCESS_TOKEN")
    url = f"{server}/api/v1/statuses"
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"status": message}
    http_post_json(url, payload, headers=headers)


def post_bluesky(message: str, release_url: str):
    require_env(["BLUESKY_USERNAME", "BLUESKY_APP_PASSWORD"])
    identifier = env("BLUESKY_USERNAME")
    password = env("BLUESKY_APP_PASSWORD")

    session = http_post_json(
        "https://bsky.social/xrpc/com.atproto.server.createSession",
        {"identifier": identifier, "password": password},
    )
    did = session.get("did")
    access_jwt = session.get("accessJwt")
    if not did or not access_jwt:
        raise RuntimeError("Failed to create Bluesky session")

    record = {
        "text": message,
        "createdAt": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
    }

    facets = build_bluesky_facets(message, release_url)
    if facets:
        record["facets"] = facets
    headers = {"Authorization": f"Bearer {access_jwt}"}
    payload = {"repo": did, "collection": "app.bsky.feed.post", "record": record}
    http_post_json(
        "https://bsky.social/xrpc/com.atproto.repo.createRecord",
        payload,
        headers=headers,
    )


def build_bluesky_facets(message: str, release_url: str):
    facets = []

    if release_url:
        index = message.find(release_url)
        if index >= 0:
            byte_start = len(message[:index].encode("utf-8"))
            byte_end = byte_start + len(release_url.encode("utf-8"))
            facets.append(
                {
                    "index": {"byteStart": byte_start, "byteEnd": byte_end},
                    "features": [
                        {"$type": "app.bsky.richtext.facet#link", "uri": release_url}
                    ],
                }
            )

    for match in re.finditer(r"#[A-Za-z0-9_]+", message):
        tag = match.group()[1:]
        if not tag:
            continue
        byte_start = len(message[: match.start()].encode("utf-8"))
        byte_end = len(message[: match.end()].encode("utf-8"))
        facets.append(
            {
                "index": {"byteStart": byte_start, "byteEnd": byte_end},
                "features": [{"$type": "app.bsky.richtext.facet#tag", "tag": tag}],
            }
        )

    return facets or None


def repo_url_from_git() -> str:
    try:
        remote = subprocess.check_output(
            ["git", "config", "--get", "remote.origin.url"], text=True
        ).strip()
    except Exception:
        return ""

    if remote.startswith("git@github.com:"):
        return "https://github.com/" + remote[len("git@github.com:") :].removesuffix(
            ".git"
        )
    if remote.startswith("https://github.com/"):
        return remote.removesuffix(".git")
    return ""


def tag_from_git() -> str:
    try:
        return subprocess.check_output(
            ["git", "describe", "--tags", "--abbrev=0"], text=True
        ).strip()
    except Exception:
        return ""


def extract_changelog_highlights(
    changelog_path: str, tag: str, max_items: int = 3
) -> list[str]:
    version = tag[1:] if tag.startswith("v") else tag
    if not version:
        return []

    try:
        with open(changelog_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return []

    section_pattern = re.compile(
        rf"(?ms)^##\s+\[{re.escape(version)}\][^\n]*\n(.*?)(?=^##\s+\[|\Z)"
    )
    match = section_pattern.search(content)
    if not match:
        return []

    body = match.group(1)
    highlights: list[str] = []
    for raw_line in body.splitlines():
        line = raw_line.strip()
        if not line.startswith("- "):
            continue
        text = line[2:].strip()
        if not text:
            continue
        text = re.sub(r"\*\*([^*]+)\*\*", r"\1", text)
        text = re.sub(r"`([^`]+)`", r"\1", text)
        text = re.sub(r"\[([^\]]+)\]\([^)]+\)", r"\1", text)
        text = re.sub(r"\s+", " ", text).strip()
        if text:
            highlights.append(text)
        if len(highlights) >= max_items:
            break

    return highlights


def build_announcement_message(
    project: str, tag: str, release_url: str, highlights: list[str], max_len: int = 300
) -> str:
    base = f"{project} {tag} is out!"
    tail = f"Full notes: {release_url} #proxmox #linux #homelab"

    if highlights:
        # Keep highlights concise so we can include at least one line in
        # constrained post lengths (e.g., Bluesky 300-char limit).
        compact = []
        for h in highlights:
            h = h.strip()
            if len(h) > 92:
                h = h[:89].rstrip() + "..."
            compact.append(h)

        for count in range(len(compact), 0, -1):
            bullet_lines = "\n".join(f"• {h}" for h in compact[:count])
            candidate = f"{base}\nHighlights:\n{bullet_lines}\n{tail}"
            if len(candidate) <= max_len:
                return candidate

        # If even one compact highlight doesn't fit, try an aggressively short
        # single-line highlight before falling back to the bare release message.
        short = compact[0]
        if len(short) > 56:
            short = short[:53].rstrip() + "..."
        candidate = f"{base}\nHighlights:\n• {short}\n{tail}"
        if len(candidate) <= max_len:
            return candidate

    candidate = f"{base} {tail}"
    if len(candidate) <= max_len:
        return candidate

    # Keep URL and hashtags; truncate the project prefix as needed.
    overflow = len(candidate) - max_len
    shortened_base = base
    if overflow > 0 and len(shortened_base) > overflow + 3:
        shortened_base = (
            shortened_base[: len(shortened_base) - overflow - 3].rstrip() + "..."
        )
    return f"{shortened_base} {tail}"


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Post release announcements to Mastodon/Bluesky."
    )
    parser.add_argument("--tag", dest="tag", help="Release tag (e.g. v1.0.17)")
    parser.add_argument("--release-url", dest="release_url", help="Full release URL")
    parser.add_argument(
        "--project", dest="project", default=os.environ.get("PROJECT_NAME", "pvetui")
    )
    parser.add_argument(
        "--changelog",
        dest="changelog",
        default="CHANGELOG.md",
        help="Path to changelog file",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print generated message and exit without posting",
    )
    parser.add_argument(
        "--mastodon-only", action="store_true", help="Post only to Mastodon"
    )
    parser.add_argument(
        "--bluesky-only", action="store_true", help="Post only to Bluesky"
    )
    args = parser.parse_args()

    if args.mastodon_only and args.bluesky_only:
        print("Cannot use --mastodon-only and --bluesky-only together", file=sys.stderr)
        return 1

    tag = args.tag or env("RELEASE_TAG") or tag_from_git()
    if not tag:
        print("Release tag is required (use --tag or RELEASE_TAG)", file=sys.stderr)
        return 1

    release_url = args.release_url or env("RELEASE_URL")
    if not release_url:
        repo_url = repo_url_from_git()
        if repo_url:
            release_url = f"{repo_url}/releases/tag/{tag}"

    if not release_url:
        print(
            "Release URL is required (use --release-url or RELEASE_URL)",
            file=sys.stderr,
        )
        return 1

    highlights = extract_changelog_highlights(args.changelog, tag)

    ai_summary = ""
    ai_reason = ""
    if ai_summary_enabled():
        # Reserve room for base/tail/newlines and use AI for a short single-line highlight.
        ai_summary, ai_reason = summarize_highlights_with_ai(
            tag, highlights, max_chars=96
        )
        if ai_summary:
            highlights = [ai_summary]
        elif ai_debug_enabled():
            print(f"[announce-ai] fallback to changelog: {ai_reason}", file=sys.stderr)

    message = build_announcement_message(args.project, tag, release_url, highlights)

    if args.dry_run:
        source = "ai" if ai_summary else "changelog"
        print(f"[dry-run] source={source}")
        if ai_summary_enabled() and not ai_summary:
            print(f"[dry-run] ai_fallback_reason={ai_reason or 'unknown'}")
        print(message)
        return 0

    posted = []
    if not args.bluesky_only:
        if env("MASTODON_SERVER") and env("MASTODON_ACCESS_TOKEN"):
            post_mastodon(message)
            posted.append("mastodon")

    if not args.mastodon_only:
        if env("BLUESKY_USERNAME") and env("BLUESKY_APP_PASSWORD"):
            post_bluesky(message, release_url)
            posted.append("bluesky")

    if not posted:
        print(
            "No announcements posted (missing credentials or platform disabled).",
            file=sys.stderr,
        )
        return 2

    print("Posted announcements to: " + ", ".join(posted))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
