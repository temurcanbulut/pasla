"""Release gate: require successful CI for the exact tagged commit."""

import json
import os
import re
import urllib.request


def require_successful_ci(repo, sha, token, workflow="ci.yml"):
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+", repo) or not re.fullmatch(
        r"[a-f0-9]{40}", sha
    ):
        raise ValueError("Invalid repository/commit")
    request = urllib.request.Request(
        f"https://api.github.com/repos/{repo}/actions/workflows/{workflow}/runs?head_sha={sha}&per_page=100",
        headers={
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {token}",
        },
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        runs = json.load(response)["workflow_runs"]
    matching = [
        r
        for r in runs
        if r["head_sha"] == sha
        and r["event"] in {"push", "pull_request", "workflow_dispatch"}
    ]
    if not matching:
        raise SystemExit("No CI run for this exact source commit; release refused.")
    latest = max(
        matching, key=lambda r: (r.get("run_number", 0), r.get("run_attempt", 1))
    )
    if latest["status"] != "completed" or latest["conclusion"] != "success":
        raise SystemExit("Latest exact-commit CI is not successful; release refused.")
    print(f"Verified successful source CI: {latest['html_url']}")


if __name__ == "__main__":
    require_successful_ci(
        os.environ["GITHUB_REPOSITORY"],
        os.environ["GITHUB_SHA"],
        os.environ["GITHUB_TOKEN"],
    )
