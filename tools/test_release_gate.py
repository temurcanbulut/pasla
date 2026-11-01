"""Release tests use fixture JSON; no GitHub/network request is made."""

import importlib.util
import io
import json
import unittest
from pathlib import Path
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / "tools/require_green_ci.py"
spec = importlib.util.spec_from_file_location("release_gate", SOURCE)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
SHA = "a" * 40


class ReleaseGateTests(unittest.TestCase):
    def run_gate(self, runs):
        data = io.StringIO(json.dumps({"workflow_runs": runs}))
        with patch.object(gate.urllib.request, "urlopen", return_value=data):
            gate.require_successful_ci("fixture/repository", SHA, "offline-test-token")

    def row(self, **extra):
        return {
            "head_sha": SHA,
            "head_branch": "main",
            "event": "push",
            "run_number": 1,
            "status": "completed",
            "conclusion": "success",
            "html_url": "https://example.invalid/fixture",
            **extra,
        }

    def test_matching_success_allows_release(self):
        self.run_gate([self.row()])

    def test_other_commit_or_no_run_is_rejected(self):
        for rows in ([], [self.row(head_sha="b" * 40)]):
            with self.assertRaises(SystemExit):
                self.run_gate(rows)

    def test_newer_failure_or_rerun_blocks_older_success(self):
        for newer in (
            self.row(run_number=2, conclusion="failure"),
            self.row(run_attempt=2, status="in_progress", conclusion=None),
        ):
            with self.assertRaises(SystemExit):
                self.run_gate([self.row(), newer])

    def test_pr_merge_ref_or_non_main_ci_does_not_qualify(self):
        for rows in (
            [self.row(event="pull_request")],
            [self.row(head_branch="feature")],
        ):
            with self.assertRaises(SystemExit):
                self.run_gate(rows)

    def test_invalid_repository_never_calls_network(self):
        with patch.object(gate.urllib.request, "urlopen") as request:
            with self.assertRaises(ValueError):
                gate.require_successful_ci("../bad", SHA, "offline-test-token")
            request.assert_not_called()
