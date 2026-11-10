"""Keep the Nix compiler, module minimum, and Renovate proposals coordinated."""

import json
import pathlib
import re
import signal
import subprocess
import sys


VERSION = r"[1-9][0-9]*\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
VERSION_ASSIGNMENT = rf'goVersion = "({VERSION})";'
PIN = r'goNixpkgs.url = "github:NixOS/nixpkgs/([0-9a-f]{40})";'
SYSTEMS = ["x86_64-linux", "aarch64-linux", "x86_64-darwin", "aarch64-darwin"]
FILES = ["flake.nix", "flake.lock", "go.mod", "go.sum"]


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def one(pattern, text):
    matches = re.findall(pattern, text, re.MULTILINE)
    if len(matches) != 1:
        raise ValueError(f"expected exactly one match for {pattern}")
    return matches[0]


def version():
    return one(VERSION_ASSIGNMENT, pathlib.Path("flake.nix").read_text())


def minimum(target):
    return target.rsplit(".", 1)[0] + ".0"


def check():
    target = version()
    manifest = pathlib.Path("go.mod").read_text()
    if re.search(r"^toolchain\s", manifest, re.MULTILINE):
        raise ValueError("go.mod must not contain a toolchain directive; Nix owns the compiler")
    if one(r"^go (\S+)$", manifest) != minimum(target):
        raise ValueError(f"go.mod must declare go {minimum(target)}")
    if run("go", "env", "GOVERSION") != "go" + target:
        raise ValueError(f"expected the Nix compiler Go {target}; enter the updated nix develop shell")
    print(f"toolchain: Go {target}; module minimum {minimum(target)}; GOTOOLCHAIN=local")


def available(revision, target):
    root = json.dumps(str(pathlib.Path.cwd()))
    expression = (
        f'let root = {root}; base = builtins.getFlake ("path:" + root); '
        f'candidate = builtins.getFlake "github:NixOS/nixpkgs/{revision}"; '
        'in map (system: (import (root + "/scripts/go-toolchain.nix") { '
        'nixpkgs = base.inputs.nixpkgs; goNixpkgs = candidate; '
        f'goVersion = "{target}"; inherit system; }}).version) '
        + "[ " + " ".join(json.dumps(system) for system in SYSTEMS) + " ]"
    )
    try:
        return json.loads(run("nix", "eval", "--impure", "--json", "--expr", expression)) == [target] * len(SYSTEMS)
    except subprocess.CalledProcessError:
        return False


def update(target=None):
    target = target or version()
    if not re.fullmatch(VERSION, target):
        raise ValueError("expected an exact stable Go version, e.g. 1.27.3")
    snapshot = {name: pathlib.Path(name).read_bytes() for name in FILES}
    try:
        flake = snapshot["flake.nix"].decode()
        current = one(PIN, flake)
        revision = current
        if not available(current, target):
            # Resolve the branch once, then evaluate and retain only its immutable revision.
            metadata = json.loads(run("nix", "flake", "metadata", "--json", "github:NixOS/nixpkgs/nixpkgs-unstable"))
            revision = metadata["locked"]["rev"]
            if not re.fullmatch(r"[0-9a-f]{40}", revision) or not available(revision, target):
                raise ValueError(f"Go {target} is not yet available on all supported systems in nixpkgs-unstable; retry a later cycle")
        one(VERSION_ASSIGNMENT, flake)
        flake = re.sub(VERSION_ASSIGNMENT, f'goVersion = "{target}";', flake)
        flake = re.sub(PIN, f'goNixpkgs.url = "github:NixOS/nixpkgs/{revision}";', flake)
        pathlib.Path("flake.nix").write_text(flake)
        if revision != current:
            run("nix", "flake", "update", "goNixpkgs")
        before = json.loads(snapshot["flake.lock"])
        after = json.loads(pathlib.Path("flake.lock").read_text())
        node = before["nodes"][before["root"]]["inputs"]["goNixpkgs"]
        if after["nodes"][node]["locked"]["rev"] != revision:
            raise ValueError("goNixpkgs lock does not match the selected revision")
        before["nodes"].pop(node)
        after["nodes"].pop(node)
        if before != after:
            raise ValueError("toolchain update changed unrelated Nix inputs")
        manifest = snapshot["go.mod"].decode()
        one(r"^go (\S+)$", manifest)
        manifest = re.sub(r"^go \S+$", f"go {minimum(target)}", manifest, flags=re.MULTILINE)
        manifest = re.sub(r"^toolchain [^\n]+\n\n?", "", manifest, flags=re.MULTILINE)
        pathlib.Path("go.mod").write_text(manifest)
        # The parent Renovate process still has the OLD development shell on PATH.
        # path: also includes newly added files in a local, unstaged maintenance checkout.
        shell = ("nix", "develop", "path:" + str(pathlib.Path.cwd()), "--no-update-lock-file", "--command")
        run(*shell, "bash", "scripts/update-go-dependencies.sh")
        run(*shell, "bash", "scripts/check-go-toolchain.sh")
        print(f"toolchain: prepared Go {target} at nixpkgs {revision}")
    except BaseException:
        for name, contents in snapshot.items():
            pathlib.Path(name).write_bytes(contents)
        raise


def main():
    command, *args = sys.argv[1:]
    if command == "version" and not args:
        print(version())
    elif command == "check" and not args:
        check()
    elif command == "update" and len(args) <= 1:
        update(*args)
    else:
        raise ValueError("usage: go-toolchain.py version|check|update [VERSION]")


if __name__ == "__main__":
    def interrupted(signum, _frame):
        raise SystemExit(128 + signum)

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGHUP, interrupted)
    try:
        main()
    except (ValueError, KeyError, subprocess.CalledProcessError) as error:
        sys.exit(f"toolchain: {error}")
