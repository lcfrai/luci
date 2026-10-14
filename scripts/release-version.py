#!/usr/bin/env python3
"""Check or update Luci's authoritative release version declarations."""
import argparse
import json
from pathlib import Path
import re
import sys

VERSION = re.compile(r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$")
JSON_SLOTS = {
    "desktop/package.json": [("version",)],
    "desktop/package-lock.json": [("version",), ("packages", "", "version")],
    "desktop/src-tauri/tauri.conf.json": [("version",)],
}
TEXT_SLOTS = {
    "desktop/src-tauri/Cargo.toml": r'(?ms)^\[package\]\n(?:(?!^\[).)*?^version = "(?P<version>[^\"]+)"',
    "desktop/src-tauri/Cargo.lock": r'(?m)^name = "luci"\nversion = "(?P<version>[^\"]+)"',
    "control-plane/internal/product/server.go": r'const Version = "(?P<version>[^\"]+)"',
    "sandboxd/cmd/sandboxd/main.go": r'const sandboxVersion = "(?P<version>[^\"]+)"',
    ".github/workflows/luci-ci.yml": r"LUCI_VERSION:\s*'(?P<version>[^']+)'",
    "scripts/build-release.sh": r'version=\$\{1:-(?P<version>[^}]+)\}',
    "scripts/build-release.ps1": r"param\(\[string\]\$Version = '(?P<version>[^']+)'",
    "distribution/docker/compose.yaml": r'\$\{LUCI_VERSION:-(?P<version>[^}]+)\}',
}

def declarations(root: Path):
    values = {}
    for name, slots in JSON_SLOTS.items():
        data = json.loads((root / name).read_text(encoding='utf-8'))
        for slot in slots:
            value = data
            for key in slot:
                value = value[key]
            values[name + ":" + "/".join(slot)] = value
    for name, pattern in TEXT_SLOTS.items():
        matches = list(re.finditer(pattern, (root / name).read_text(encoding='utf-8')))
        expected = 2 if name == "distribution/docker/compose.yaml" else 1
        if len(matches) != expected:
            raise ValueError(f"Expected {expected} version declaration(s) in {name}")
        for i, match in enumerate(matches):
            values[f"{name}:{i}"] = match["version"]
    return values

def check(root: Path, version: str):
    if not VERSION.fullmatch(version):
        raise ValueError("Use a semantic version such as 0.5.0 or 0.5.0-rc.1")
    mismatches = {name: value for name, value in declarations(root).items() if value != version}
    if mismatches:
        raise ValueError("Release version mismatch: " + json.dumps(mismatches, sort_keys=True))
    return version

def set_version(root: Path, version: str):
    if not VERSION.fullmatch(version):
        raise ValueError("Invalid release version")
    # Read and validate every input before writing; preserve unrelated versions,
    # dependencies, historical evidence and lockfile resolution metadata.
    declarations(root)
    changes = {}
    for name, slots in JSON_SLOTS.items():
        data = json.loads((root / name).read_text(encoding='utf-8'))
        for slot in slots:
            target = data
            for key in slot[:-1]:
                target = target[key]
            target[slot[-1]] = version
        changes[name] = json.dumps(data, indent=2, ensure_ascii=False) + "\n"
    for name, pattern in TEXT_SLOTS.items():
        text = (root / name).read_text(encoding='utf-8')
        for match in reversed(list(re.finditer(pattern, text))):
            start, end = match.span("version")
            text = text[:start] + version + text[end:]
        changes[name] = text
    for name, content in changes.items():
        (root / name).write_text(content, encoding='utf-8')
    return check(root, version)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("check", "set"))
    parser.add_argument("version")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    try:
        version = set_version(args.root, args.version) if args.action == "set" else check(args.root, args.version)
    except (ValueError, KeyError, OSError) as error:
        parser.exit(1, str(error) + "\n")
    print(f"Luci release declarations agree: {version}")

if __name__ == "__main__":
    main()
