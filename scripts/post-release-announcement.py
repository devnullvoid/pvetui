#!/usr/bin/env python3
import argparse
import json
import os
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

    facets = build_bluesky_link_facets(message, release_url)
    if facets:
        record["facets"] = facets
    headers = {"Authorization": f"Bearer {access_jwt}"}
    payload = {"repo": did, "collection": "app.bsky.feed.post", "record": record}
    http_post_json("https://bsky.social/xrpc/com.atproto.repo.createRecord", payload, headers=headers)


def build_bluesky_link_facets(message: str, release_url: str):
    if not release_url:
        return None
    index = message.find(release_url)
    if index < 0:
        return None
    byte_start = len(message[:index].encode("utf-8"))
    byte_end = byte_start + len(release_url.encode("utf-8"))
    return [
        {
            "index": {"byteStart": byte_start, "byteEnd": byte_end},
            "features": [
                {"$type": "app.bsky.richtext.facet#link", "uri": release_url}
            ],
        }
    ]


def repo_url_from_git() -> str:
    try:
        remote = subprocess.check_output(["git", "config", "--get", "remote.origin.url"], text=True).strip()
    except Exception:
        return ""

    if remote.startswith("git@github.com:"):
        return "https://github.com/" + remote[len("git@github.com:") :].removesuffix(".git")
    if remote.startswith("https://github.com/"):
        return remote.removesuffix(".git")
    return ""


def tag_from_git() -> str:
    try:
        return subprocess.check_output(["git", "describe", "--tags", "--abbrev=0"], text=True).strip()
    except Exception:
        return ""


def main() -> int:
    parser = argparse.ArgumentParser(description="Post release announcements to Mastodon/Bluesky.")
    parser.add_argument("--tag", dest="tag", help="Release tag (e.g. v1.0.17)")
    parser.add_argument("--release-url", dest="release_url", help="Full release URL")
    parser.add_argument("--project", dest="project", default=os.environ.get("PROJECT_NAME", "pvetui"))
    parser.add_argument("--mastodon-only", action="store_true", help="Post only to Mastodon")
    parser.add_argument("--bluesky-only", action="store_true", help="Post only to Bluesky")
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
        print("Release URL is required (use --release-url or RELEASE_URL)", file=sys.stderr)
        return 1

    message = f"{args.project} {tag} is out! Check it out at {release_url} #proxmox #linux #homelab"

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
        print("No announcements posted (missing credentials or platform disabled).", file=sys.stderr)
        return 2

    print("Posted announcements to: " + ", ".join(posted))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
