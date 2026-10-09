import copy
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
import urllib.error
from unittest.mock import patch

import release


class ReleaseGuardsTest(unittest.TestCase):
    def setUp(self):
        self.version = "0.7.2-cesar.1"
        self.commit = "a" * 40
        self.base = {"repository": release.UPSTREAM, "tag": "0.7.2", "sha": "b" * 40}
        self.manifest = {
            "annotations": release.expected_annotations(self.version, self.commit, self.base),
            "manifests": [{"platform": {"os": "linux", "architecture": arch}}
                          for arch in ("amd64", "arm64")],
        }
        self.digest = "sha256:" + "c" * 64

    def test_valid_and_invalid_version_tags(self):
        release.validate_version(self.version, "v" + self.version, self.base)
        for version, tag in ((self.version, "v0.7.2-cesar.2"), ("0.7.2", "v0.7.2"),
                             ("0.7.2-cesar.0", "v0.7.2-cesar.0"),
                             ("0.7.2-cesar.01", "v0.7.2-cesar.01"),
                             ("0.7.3-cesar.1", "v0.7.3-cesar.1"),
                             (self.version, "refs/tags/v" + self.version)):
            with self.subTest(version=version, tag=tag), self.assertRaises(ValueError):
                release.validate_version(version, tag, self.base)

    def test_release_notes_resume_ignores_only_line_endings(self):
        expected = "# Correções\n\nDigest: sha256:abc\n"
        for existing in (expected, expected.rstrip("\n"), expected + "\n", expected.replace("\n", "\r\n")):
            with self.subTest(existing=existing):
                release.verify_release_notes(existing, expected)
        for existing in (None, "", expected.replace("abc", "def"), expected.replace("\n\n", "\n"), expected + " "):
            with self.subTest(existing=existing), self.assertRaises(ValueError):
                release.verify_release_notes(existing, expected)

    def test_missing_and_invalid_base(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaises(FileNotFoundError):
                release.read_base(root)
            (root / ".github").mkdir()
            for change in ({"sha": "abc"}, {"repository": "another/repo"}, {"tag": "0.7.2-beta"}):
                (root / ".github/upstream-base.json").write_text(json.dumps(self.base | change))
                with self.subTest(change=change), self.assertRaises(ValueError):
                    release.read_base(root)

    def test_git_ancestry_and_absent_base_on_a_real_repository(self):
        with tempfile.TemporaryDirectory() as directory:
            def git(*args):
                return subprocess.check_output(["git", "-C", directory, *args], text=True, stderr=subprocess.PIPE).strip()

            git("init", "--initial-branch=main")
            for message in ("base", "fork"):
                git("-c", "user.name=Release Test", "-c", "user.email=test@example.invalid",
                    "commit", "--allow-empty", "-m", message)
                if message == "base":
                    base = self.base | {"sha": git("rev-parse", "HEAD")}
            commit = git("rev-parse", "HEAD")
            git("update-ref", "refs/remotes/origin/main", commit)
            with patch("release.run", side_effect=lambda *args: git(*args[1:])), patch(
                "release.subprocess.check_call", side_effect=lambda args: git(*args[1:])
            ):
                release.validate_checkout(commit, base)
                with self.assertRaises(subprocess.CalledProcessError):
                    release.validate_checkout(commit, base | {"sha": "0" * 40})
                git("update-ref", "refs/remotes/origin/main", base["sha"])
                with self.assertRaises(subprocess.CalledProcessError):
                    release.validate_checkout(commit, base)
                with self.assertRaises(ValueError):
                    release.validate_checkout(base["sha"], base)

    def test_existing_image_must_preserve_identity_and_both_platforms(self):
        release.verify_manifest(self.manifest, self.version, self.commit, self.base)
        for key in self.manifest["annotations"]:
            altered = copy.deepcopy(self.manifest)
            altered["annotations"][key] = "different"
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.verify_manifest(altered, self.version, self.commit, self.base)
        for platforms in (self.manifest["manifests"][:1], self.manifest["manifests"] * 2, []):
            with self.subTest(platforms=platforms), self.assertRaises(ValueError):
                release.verify_manifest(self.manifest | {"manifests": platforms}, self.version, self.commit, self.base)

    def test_attestations_are_not_counted_as_runtime_platforms(self):
        manifest = copy.deepcopy(self.manifest)
        manifest["manifests"].append({"platform": {"os": "unknown", "architecture": "unknown"}})
        release.verify_manifest(manifest, self.version, self.commit, self.base)

    @patch("release.inspect_image")
    def test_partial_publication_can_resume_without_rebuilding(self, inspect):
        inspect.side_effect = [None, (self.manifest, self.digest)]
        self.assertEqual(release.registry_state(self.version, self.commit, self.base), self.digest)

    @patch("release.inspect_image")
    def test_completed_publication_is_idempotent(self, inspect):
        inspect.return_value = self.manifest, self.digest
        self.assertEqual(release.registry_state(self.version, self.commit, self.base), self.digest)

    @patch("release.inspect_image")
    def test_existing_version_for_another_commit_is_rejected(self, inspect):
        altered = copy.deepcopy(self.manifest)
        altered["annotations"]["org.opencontainers.image.revision"] = "d" * 40
        inspect.return_value = altered, self.digest
        with self.assertRaises(ValueError):
            release.registry_state(self.version, self.commit, self.base)

    @patch("release.inspect_image")
    def test_conflicting_alias_digests_are_rejected(self, inspect):
        inspect.side_effect = [(self.manifest, self.digest), (self.manifest, "sha256:" + "d" * 64)]
        with self.assertRaises(ValueError):
            release.registry_state(self.version, self.commit, self.base)

    @patch("release.subprocess.run")
    def test_only_missing_manifests_are_treated_as_available_tags(self, execute):
        execute.return_value = subprocess.CompletedProcess([], 1, stderr="image: not found")
        self.assertIsNone(release.inspect_image("image"))
        for error in ("unauthorized: not found", "denied", "timeout", "TLS certificate failure"):
            execute.return_value = subprocess.CompletedProcess([], 1, stderr=error)
            with self.subTest(error=error), self.assertRaises(ValueError):
                release.inspect_image("image")

    @patch.dict("release.os.environ", {"GH_TOKEN": "test-only-token"})
    @patch("release.urllib.request.urlopen")
    def test_first_package_bootstrap_requires_confirmed_absence(self, request):
        request.side_effect = urllib.error.HTTPError("test", 404, "Not Found", {}, None)
        self.assertFalse(release.package_exists())
        for status in (401, 403, 500):
            request.side_effect = urllib.error.HTTPError("test", status, "Failure", {}, None)
            with self.subTest(status=status), self.assertRaises(ValueError):
                release.package_exists()


if __name__ == "__main__":
    unittest.main()
