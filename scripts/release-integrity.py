#!/usr/bin/env python3
"""Verify candidate artifacts independently before creating a keyless attestation.

This never builds, signs, publishes, runs an artifact, or creates a signing key.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re

SHA = re.compile(r"^[0-9a-f]{64}$")
COMMIT = re.compile(r"^[0-9a-f]{40}(?:[0-9a-f]{24})?$")
NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,180}$")
GROUPS = ("linux-amd64", "darwin-arm64", "windows-portable", "windows-native")


def regular(path: Path):
    if any(parent.is_symlink() for parent in (path, *path.parents)) or not path.is_file():
        raise ValueError(f"Expected an ordinary unlinked artifact: {path.name}")


def digest(path: Path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def verify_run(run: dict, repository: str, commit: str, workflow_commit: str):
    if not COMMIT.fullmatch(commit) or commit != workflow_commit:
        raise ValueError("Run this workflow at the exact candidate source commit")
    if run.get("repository", {}).get("full_name") != repository or run.get("head_repository", {}).get("full_name") != repository:
        raise ValueError("The candidate build must belong to this repository, not a fork")
    if run.get("status") != "completed" or run.get("conclusion") != "success" or run.get("head_sha") != commit:
        raise ValueError("The candidate needs a completed successful build at the exact commit")
    if run.get("event") not in ("push", "workflow_dispatch") or run.get("path") != ".github/workflows/luci-ci.yml":
        raise ValueError("Use the trusted Luci build workflow from push or explicit dispatch")
    if not isinstance(run.get("id"), int) or run["id"] < 1:
        raise ValueError("The build run ID is invalid")
    return {"id": run["id"], "repository": repository, "commit": commit, "workflow": run["path"], "event": run["event"]}


def selected_names(group: str, version: str):
    if group == "windows-native":
        return {f"Luci_{version}_x64-setup.exe"}
    names = {"luci-cloud-linux-amd64.tar.gz"}
    if group == "windows-portable":
        return names | {"luci-windows-amd64.zip", "luci-linux-amd64.tar.gz", "luci-darwin-arm64.tar.gz"}
    return names | {f"luci-{group}.tar.gz"}


def verify_group(directory: Path, group: str, version: str, commit: str):
    manifests = list(directory.rglob("release-manifest.json"))
    if len(manifests) != 1:
        raise ValueError(f"{group} needs exactly one source-bound release manifest")
    path = manifests[0]
    regular(path)
    manifest = json.loads(path.read_text(encoding="utf-8-sig"))
    if manifest.get("version") != version or manifest.get("sourceCommit") != commit or manifest.get("sourceDirty") is not False:
        raise ValueError(f"{group} manifest does not bind the exact clean candidate source")
    licenses = manifest.get("portableLicenseComponents", [])
    if not licenses or any(not isinstance(row.get("count"), int) or row["count"] < 1 or row.get("missing") != 0 for row in licenses):
        raise ValueError(f"{group} has incomplete licence evidence")
    records = {}
    for row in manifest.get("artifacts", []):
        name = row.get("name", "")
        if not NAME.fullmatch(name) or name in records or not SHA.fullmatch(row.get("sha256", "")) or type(row.get("bytes")) is not int or row["bytes"] < 1:
            raise ValueError(f"{group} contains an unsafe or duplicate artifact record")
        records[name] = row
    checksum_path = path.parent / "SHA256SUMS"
    regular(checksum_path)
    checksums = {}
    for line in checksum_path.read_text(encoding="utf-8-sig").splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9][A-Za-z0-9._-]{0,180})", line)
        if not match or match[2] in checksums:
            raise ValueError(f"{group} checksum inventory is malformed or duplicated")
        if match[2] not in records or records[match[2]]["sha256"] != match[1]:
            raise ValueError(f"{group} checksum inventory disagrees with its manifest")
        checksums[match[2]] = match[1]
    measured = []
    for name in sorted(selected_names(group, version)):
        row = records.get(name)
        artifact = path.parent / name
        regular(artifact)
        actual = digest(artifact)
        if not row or checksums.get(name) != actual or row["sha256"] != actual or artifact.stat().st_size != row["bytes"]:
            raise ValueError(f"{group}/{name} does not match its recorded bytes")
        measured.append({"name": f"{group}/{name}", "sha256": actual, "bytes": row["bytes"]})
    if group == "windows-native" and (manifest.get("nativeRequested") is not True or manifest.get("nativeLicenseComponents", 0) < 1):
        raise ValueError("The native artifact lacks native build/licence evidence")
    # Manifest metadata is also a subject: it records requested signing, source,
    # licences and original hashes, but cannot establish publisher trust itself.
    measured.append({"name": f"{group}/release-manifest.json", "sha256": digest(path), "bytes": path.stat().st_size})
    return measured


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("run", "artifacts"))
    parser.add_argument("--run-metadata", type=Path, required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--workflow-commit", required=True)
    parser.add_argument("--version")
    parser.add_argument("--directory", type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--checksums", type=Path)
    args = parser.parse_args()
    try:
        build = verify_run(json.loads(args.run_metadata.read_text()), args.repository, args.commit, args.workflow_commit)
        if args.action == "run":
            print(json.dumps(build, sort_keys=True))
            return
        if not all((args.version, args.directory, args.output, args.checksums)) or not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", args.version):
            raise ValueError("Provide version, artifact directory, output and checksum paths")
        artifacts = []
        for group in GROUPS:
            artifacts.extend(verify_group(args.directory / group, group, args.version, args.commit))
        predicate = {"version": args.version, "build": build, "artifacts": artifacts, "scope": "Independently measured retained CI artifacts; not native publisher signing, installed acceptance or original-builder SLSA provenance."}
        args.output.write_text(json.dumps(predicate, indent=2) + "\n")
        args.checksums.write_text("".join(f"{row['sha256']}  {row['name']}\n" for row in artifacts))
        print(f"Verified {len(artifacts)} candidate artifact subjects at {args.commit}")
    except (OSError, ValueError, KeyError, TypeError) as error:
        parser.exit(1, str(error) + "\n")

if __name__ == "__main__":
    main()
