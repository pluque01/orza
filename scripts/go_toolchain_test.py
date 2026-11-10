"""Behavior tests for coordinated compiler updates, without network or Nix builds."""

import importlib.util
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("toolchain", pathlib.Path(__file__).with_name("go-toolchain.py"))
toolchain = importlib.util.module_from_spec(spec)
spec.loader.exec_module(toolchain)


class ToolchainTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.previous = pathlib.Path.cwd()
        os.chdir(self.directory.name)
        self.addCleanup(self.directory.cleanup)
        self.addCleanup(os.chdir, self.previous)
        self.old_revision = "a" * 40
        self.new_revision = "b" * 40
        pathlib.Path("flake.nix").write_text(
            'goVersion = "1.26.10";\n'
            f'goNixpkgs.url = "github:NixOS/nixpkgs/{self.old_revision}";\n'
        )
        self.lock = {
            "root": "root", "version": 7,
            "nodes": {
                "root": {"inputs": {"goNixpkgs": "compiler", "nixpkgs": "base"}},
                "compiler": {"locked": {"rev": self.old_revision}},
                "base": {"locked": {"rev": "c" * 40}},
            },
        }
        pathlib.Path("flake.lock").write_text(json.dumps(self.lock))
        pathlib.Path("go.mod").write_text("module example.invalid/test\n\ngo 1.26.0\n\ntoolchain go1.27.3\n\n")
        pathlib.Path("go.sum").write_text("original sums\n")
        self.calls = []
        self.failure = None

    def snapshot(self):
        return {name: pathlib.Path(name).read_bytes() for name in toolchain.FILES}

    def command(self, *args):
        self.calls.append(args)
        if args[:3] == ("nix", "flake", "metadata"):
            return json.dumps({"locked": {"rev": self.new_revision}})
        if args[:3] == ("nix", "flake", "update"):
            self.lock["nodes"]["compiler"]["locked"]["rev"] = self.new_revision
            if self.failure == "unrelated-lock":
                self.lock["nodes"]["base"]["locked"]["rev"] = "d" * 40
            pathlib.Path("flake.lock").write_text(json.dumps(self.lock))
        if args[:2] == ("nix", "develop"):
            self.assertEqual(args[2], "path:" + str(pathlib.Path.cwd()))
            self.assertIn("--no-update-lock-file", args)
            if args[-1] == "scripts/update-go-dependencies.sh":
                pathlib.Path("go.sum").write_text("new sums\n")
                if self.failure == "hash":
                    raise subprocess.CalledProcessError(1, args)
            if args[-1] == "scripts/check-go-toolchain.sh" and self.failure == "check":
                raise subprocess.CalledProcessError(1, args)
        return ""

    def test_patch_and_minor_keep_the_declared_policy(self):
        for target, expected in [("1.26.11", "1.26.0"), ("1.27.3", "1.27.0")]:
            with self.subTest(target=target), mock.patch.object(toolchain, "available", return_value=True), mock.patch.object(toolchain, "run", side_effect=self.command):
                toolchain.update(target)
                self.assertEqual(toolchain.version(), target)
                manifest = pathlib.Path("go.mod").read_text()
                self.assertIn(f"go {expected}\n", manifest)
                self.assertNotIn("toolchain ", manifest)
                self.assertNotIn(("nix", "flake", "update", "goNixpkgs"), self.calls)
                self.assertEqual(self.calls[-1][-1], "scripts/check-go-toolchain.sh")

    def test_resolves_and_locks_only_the_compiler_input(self):
        with mock.patch.object(toolchain, "available", side_effect=[False, True]), mock.patch.object(toolchain, "run", side_effect=self.command):
            toolchain.update("1.27.3")
        self.assertIn(self.new_revision, pathlib.Path("flake.nix").read_text())
        self.assertIn(("nix", "flake", "update", "goNixpkgs"), self.calls)
        self.assertEqual(json.loads(pathlib.Path("flake.lock").read_text())["nodes"]["base"], {"locked": {"rev": "c" * 40}})

    def test_unavailable_version_leaves_no_partial_update(self):
        before = self.snapshot()
        with mock.patch.object(toolchain, "available", return_value=False), mock.patch.object(toolchain, "run", side_effect=self.command):
            with self.assertRaisesRegex(ValueError, "not yet available"):
                toolchain.update("1.28.0")
        self.assertEqual(before, self.snapshot())

    def test_failures_restore_every_coordinated_file(self):
        for failure in ["hash", "check", "unrelated-lock"]:
            with self.subTest(failure=failure):
                self.failure = failure
                before = self.snapshot()
                with mock.patch.object(toolchain, "available", side_effect=[False, True]), mock.patch.object(toolchain, "run", side_effect=self.command):
                    with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                        toolchain.update("1.27.3")
                self.assertEqual(before, self.snapshot())

    def test_check_rejects_wrong_minimum_toolchain_and_compiler(self):
        for manifest, compiler, valid in [
            ("go 1.26.0\n", "go1.26.10", True),
            ("go 1.25.0\n", "go1.26.10", False),
            ("go 1.26.0\ntoolchain go1.27.3\n", "go1.26.10", False),
            ("go 1.26.0\n", "go1.27.3", False),
        ]:
            with self.subTest(manifest=manifest, compiler=compiler), mock.patch.object(toolchain, "run", return_value=compiler):
                pathlib.Path("go.mod").write_text(manifest)
                if valid:
                    toolchain.check()
                else:
                    with self.assertRaises(ValueError):
                        toolchain.check()


if __name__ == "__main__":
    unittest.main()
