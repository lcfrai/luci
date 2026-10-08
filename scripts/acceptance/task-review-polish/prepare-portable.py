#!/usr/bin/env python3
"""Prepare a separate Linux reproduction; never starts Node, Vite or Chromium."""
import argparse
import hashlib
import json
from pathlib import Path
import sys


ORIGINAL_SHA = "550fa1e3c9e376479f90e2f277285ad65de3ad10129feb675597617b4c484b93"
FIXTURE_SHA = "72e784c08a67117472b7fea6803c279c040c8471ee9f1e06454d8e99ab71e97a"


def digest(value):
    return hashlib.sha256(value).hexdigest()


def replace_once(source, before, after):
    if source.count(before) != 1:
        raise ValueError("The pinned adaptation anchor is absent or ambiguous")
    return source.replace(before, after, 1)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[3])
    parser.add_argument("--output", required=True, type=Path, help="Fresh private output directory")
    args = parser.parse_args()
    if sys.platform != "linux":
        parser.error("The retained witness supports Linux Chromium only")
    repo = args.repo.resolve(strict=True)
    publication = repo / "scripts/acceptance/task-review-polish"
    original = (publication / "original/copy-browser.mjs").read_bytes()
    fixture = (publication / "original/fixture.tsx").read_bytes()
    if digest(original) != ORIGINAL_SHA or digest(fixture) != FIXTURE_SHA:
        parser.error("Pinned original driver or decoded fixture bytes changed")
    source = original.decode("utf-8")
    if source.count("const fixtureSource=") != 1:
        parser.error("Original fixture declaration is ambiguous")
    embedded, _ = json.JSONDecoder().raw_decode(source.split("const fixtureSource=", 1)[1])
    if not isinstance(embedded, str) or embedded.encode("utf-8") != fixture:
        parser.error("Published fixture differs from the original embedded literal")
    required = ["desktop/vite.config.ts", "desktop/node_modules/vite/dist/node/index.js",
                "desktop/node_modules/playwright/index.mjs", "desktop/node_modules/axe-core/axe.min.js"]
    if any(not (repo / name).is_file() for name in required):
        parser.error("Use a Luci checkout with its locked desktop dependencies installed")
    output = args.output.resolve()
    if output.exists():
        parser.error("Choose a fresh output directory; historical evidence is never overwritten")

    source = replace_once(source,
        "const here=resolve('/workspace/luci/scripts/acceptance/task-review-ui'),repo=resolve(here,'../../..');",
        "const repo=resolve(" + json.dumps(str(repo)) + "),here=join(repo,'scripts/acceptance/task-review-ui');")
    source = replace_once(source,
        "'desktop','scripts/acceptance/task-review-ui'",
        "'desktop','scripts/acceptance/task-review-ui','scripts/acceptance/task-review-polish'")
    reproduction = {
        "kind": "portable-path-adaptation-of-historical-controlled-source-witness",
        "originalDriverSHA256": ORIGINAL_SHA,
        "originalFixtureSHA256": FIXTURE_SHA,
        "adapterSHA256": digest(Path(__file__).read_bytes()),
        "changes": ["Resolve a caller-selected checkout instead of /workspace/luci",
                    "Include the published original/adapter in before/after source fingerprints",
                    "Add explicit adaptation provenance to the new receipt"],
        "scope": "Linux source components with controlled callbacks; no installed/backend/native acceptance",
    }
    marker = "receipt.helperSHA256=sha(readFileSync(fileURLToPath(import.meta.url)));"
    source = replace_once(source, marker, marker + "receipt.reproduction=" + json.dumps(reproduction) + ";")
    derived = source.encode("utf-8")
    manifest = {
        "version": 1, **reproduction,
        "preparedOnly": True,
        "browserExecutedByPreparation": False,
        "repo": str(repo),
        "derivedDriver": "copy-browser.portable.mjs",
        "derivedDriverSHA256": digest(derived),
        "derivedDriverBytes": len(derived),
        "originalFixtureBytes": len(fixture),
        "limitations": ["Current checkout source may differ from the historical successful witness",
                        "The original browser assertions and cleanup behavior are retained",
                        "Preparation is not syntax, browser, backend or native validation"],
    }
    # New files only; keep a partial fresh directory if a write fails.
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    with (output / "copy-browser.portable.mjs").open("xb") as stream:
        stream.write(derived)
    with (output / "adaptation.json").open("x") as stream:
        json.dump(manifest, stream, indent=2)
        stream.write("\n")
    print(json.dumps({"preparedOnly": True, "output": str(output),
                      "derivedDriverSHA256": digest(derived)}, indent=2))


if __name__ == "__main__":
    main()
