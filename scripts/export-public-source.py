#!/usr/bin/env python3
"""Export a review candidate from a clean pinned Luci revision; never publish it.

Python 3.11+ and Git only. Reads Git blobs, not working files or Git archives with
export-subst filters. See docs/luci/SOURCE-EXPORT.md for scope and release checks.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tomllib
import zipfile


ROOT_FILES = {
    ".dockerignore", ".gitattributes", ".gitignore", "AGENTS.md", "CLAUDE.md",
    "CONTEXT.md", "LICENSE", "NOTICE", "README.md", "go.work", "go.work.sum",
    "docs/DESIGN-SYSTEM.md", "docs/PROGRAM.md", "docs/RUNBOOK.md", "docs/SEAMS.md",
}
PREFIXES = (
    ".github/", "control-plane/", "desktop/", "sandboxd/", "images/sandbox/",
    "scripts/", "third-party/licenses/", "docs/luci/",
)
OMISSIONS = {
    "desktop/src/seams.test.ts": "Historical-document seam audit loads excluded phase-2 "
    "contracts, plans and verification reports. All other selected tests are retained.",
}
EXCLUDED_PREFIXES = (
    "desktop/.vscode/", "desktop/src-tauri/binaries/", "desktop/src-tauri/runtime/",
    "desktop/src-tauri/cloud/", "desktop/src-tauri/licenses/",
    "desktop/public/design-assets/",
)
FORBIDDEN_PARTS = {
    ".aws", ".azure", ".git", ".luci", ".ssh", ".terraform", ".venv",
    "__pycache__", "artifacts", "dist", "node_modules", "target",
}
REQUIRED = {
    "LICENSE", "NOTICE", "go.work", "control-plane/go.mod", "control-plane/go.sum",
    "sandboxd/go.mod", "sandboxd/go.sum", "sandboxd/vendor/modules.txt",
    "desktop/package.json", "desktop/package-lock.json", "desktop/src-tauri/Cargo.toml",
    "desktop/src-tauri/Cargo.lock", "desktop/src-tauri/tauri.conf.json",
    "images/sandbox/Dockerfile", "images/sandbox/entrypoint.sh",
    "scripts/build-release.ps1", "scripts/build-release.sh", "scripts/collect-licenses.py",
    "scripts/package.py", "desktop/scripts/prepare-design-assets.mjs",
    "third-party/licenses/manifest.json", "control-plane/internal/product/server.go",
    ".github/workflows/luci-ci.yml",
}
GENERATED = {
    "EXPORT-ALLOWLIST.json", "EXPORT-CHECKS.json", "PROVENANCE.md",
    "SOURCE-INVENTORY.json", "SOURCE-SHA256SUMS", "EXPORT-SHA256SUMS",
}
# Signatures supplement source review; they cannot establish that arbitrary secrets
# or proprietary material are absent. Never include matched values in diagnostics.
SIGNATURES = {
    "private_key_header": re.compile(rb"-----BEGIN (?:[A-Z0-9]+ )?PRIVATE KEY-----"),
    "github_token": re.compile(rb"\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{60,})\b"),
    "aws_access_key_id": re.compile(rb"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b"),
    "anthropic_api_key": re.compile(rb"\bsk-ant-[A-Za-z0-9_-]{40,}\b"),
}
# Exact reviewed synthetic value in the API credential-serialization regression.
# Path + signature + matched-value hash must all agree; never permit a prefix or
# an entire test file. Keep values out of emitted evidence and failure messages.
SYNTHETIC_SIGNATURES = {
    ("control-plane/internal/api/sessions_agent_credentials_test.go", "anthropic_api_key"):
        ("96880ec50b36c64aa08ebfa90d97866ddc47a716097c20768c13863576a6ddaf",
         "Explicitly synthetic personal-key fixture for API response/log/session isolation; exact matched-value SHA256 verified."),
}


class ExportError(Exception):
    pass


def git(repo: Path, *args: str) -> bytes:
    result = subprocess.run(
        ["git", "--no-optional-locks", "-c", "core.fsmonitor=false", "-C", str(repo), *args],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False, timeout=120,
    )
    if result.returncode:
        raise ExportError(f"Git {args[0]} failed (exit {result.returncode}); no Git output copied into evidence")
    return result.stdout


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def json_bytes(value: object) -> bytes:
    return (json.dumps(value, indent=2, ensure_ascii=False) + "\n").encode("utf-8")


def safe_path(name: str) -> bool:
    parts = name.split("/")
    return bool(name) and all(
        part and part not in (".", "..") and not part.endswith((".", " "))
        and not re.match(r"(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)", part)
        and not any(ord(c) < 32 or c in '\\:*?"<>|' for c in part)
        for part in parts
    )


def selected(name: str) -> bool:
    if name not in ROOT_FILES and not name.startswith(PREFIXES):
        return False
    if name in OMISSIONS or name.startswith(EXCLUDED_PREFIXES):
        return False
    parts = name.lower().split("/")
    if any(part in FORBIDDEN_PARTS or part == ".env" or part.startswith(".env.") for part in parts):
        return False
    return not name.lower().endswith((".exe", ".log", ".tfstate", ".tfstate.backup", ".pyc"))


def ensure_clean(repo: Path, revision: str) -> None:
    if git(repo, "rev-parse", "--verify", "HEAD").decode().strip() != revision:
        raise ExportError("The requested full commit must be the checkout's HEAD")
    if git(repo, "status", "--porcelain=v1", "--untracked-files=all", "--ignore-submodules=none"):
        raise ExportError("Source checkout is dirty; commit or remove outstanding changes before exporting")


def read_source(repo: Path, revision: str) -> tuple[dict[str, bytes], list[dict]]:
    files: dict[str, bytes] = {}
    inventory = []
    folded: set[str] = set()
    for row in git(repo, "ls-tree", "-r", "-z", "--full-tree", revision).split(b"\0"):
        if not row:
            continue
        attrs, raw_name = row.split(b"\t", 1)
        name = raw_name.decode("utf-8", errors="strict")
        if not selected(name):
            continue
        mode, kind, oid = attrs.decode("ascii").split()
        if not safe_path(name) or name.casefold() in folded:
            raise ExportError(f"Unsafe or case-colliding selected path: {name!r}")
        if kind != "blob" or mode not in ("100644", "100755"):
            raise ExportError(f"Selected symlink, submodule or special file is unsupported: {name}")
        data = git(repo, "cat-file", "blob", oid)
        if len(data) > 64 * 1024 * 1024:
            raise ExportError(f"Selected file exceeds 64 MiB source limit: {name}")
        files[name] = data
        folded.add(name.casefold())
        inventory.append({"path": name, "gitBlob": oid, "gitMode": mode,
                          "bytes": len(data), "sha256": sha(data)})
    return files, sorted(inventory, key=lambda row: row["path"])


def verify_source(files: dict[str, bytes], version: str) -> dict:
    missing = REQUIRED - files.keys()
    if missing:
        raise ExportError("Required source files are missing: " + ", ".join(sorted(missing)))
    if b"MIT License" not in files["LICENSE"] or b"Permission is hereby granted" not in files["LICENSE"]:
        raise ExportError("The selected Luci root licence is not the expected MIT grant")
    package = json.loads(files["desktop/package.json"])
    lock = json.loads(files["desktop/package-lock.json"])
    cargo = tomllib.loads(files["desktop/src-tauri/Cargo.toml"].decode())
    cargo_lock = tomllib.loads(files["desktop/src-tauri/Cargo.lock"].decode())
    rust_packages = [p for p in cargo_lock["package"] if p["name"] == "luci"]
    versions = {
        "desktop/package.json": package["version"],
        "desktop/package-lock.json": lock["version"],
        "desktop/package-lock.json root package": lock["packages"][""]["version"],
        "desktop/src-tauri/Cargo.toml": cargo["package"]["version"],
        "desktop/src-tauri/Cargo.lock": rust_packages[0]["version"] if len(rust_packages) == 1 else None,
        "desktop/src-tauri/tauri.conf.json": json.loads(files["desktop/src-tauri/tauri.conf.json"])["version"],
    }
    for name, pattern in (
        ("control-plane/internal/product/server.go", rb'const Version = "([^"]+)"'),
        (".github/workflows/luci-ci.yml", rb"LUCI_VERSION:\s*['\"]([^'\"]+)['\"]"),
    ):
        match = re.search(pattern, files[name])
        versions[name] = match[1].decode() if match else None
    if any(value != version for value in versions.values()):
        raise ExportError("Source version mismatch: " + json.dumps(versions, sort_keys=True))
    if package.get("license") != "MIT" or cargo["package"].get("license") != "MIT":
        raise ExportError("Luci package licence metadata must identify MIT")
    references = []
    for entry in json.loads(files["third-party/licenses/manifest.json"]):
        for grant in entry["files"]:
            path = "third-party/licenses/" + grant["path"]
            if not safe_path(path) or path not in files or sha(files[path]) != grant["sha256"]:
                raise ExportError(f"Missing or changed retained third-party grant: {path}")
            references.append({"path": path, "sha256": grant["sha256"]})
    if not references:
        raise ExportError("Third-party licence manifest has no retained grants")
    findings = []
    for name, data in files.items():
        for signature, pattern in SIGNATURES.items():
            matches = list(pattern.finditer(data))
            if not matches:
                continue
            synthetic = SYNTHETIC_SIGNATURES.get((name, signature))
            if synthetic and len(matches) == 1 and sha(matches[0].group()) == synthetic[0]:
                findings.append({"path": name, "signature": signature,
                                 "matchedValueSha256": synthetic[0], "review": synthetic[1]})
                continue
            # The existing Git host test intentionally writes just a header.
            # Reject any payload/end marker or additional header in this file.
            fixture = (name == "control-plane/cmd/api/githostconfig_test.go"
                       and signature == "private_key_header" and len(matches) == 1
                       and (b'"-----BEGIN ' + b'OPENSSH PRIVATE KEY-----\\n"') in data
                       and b"-----END " not in data)
            if not fixture:
                raise ExportError(f"Credential signature requires review: {name} ({signature}); value withheld")
            findings.append({"path": name, "signature": signature,
                             "review": "Synthetic opening-header-only fixture; no key payload or closing marker."})
    local_deps = []
    for name, data in files.items():
        if name.endswith("package.json"):
            obj = json.loads(data)
            for field in ("dependencies", "devDependencies", "optionalDependencies"):
                for dep, value in obj.get(field, {}).items():
                    if isinstance(value, str) and value.startswith(("file:", "link:", "workspace:")):
                        local_deps.append(f"{name}:{dep}")
        if name.endswith("go.mod") and re.search(rb"=>\s*(?:\.{1,2}[/\\]|[/\\]|[A-Za-z]:)", data):
            local_deps.append(name + ":local Go replacement")
    if local_deps:
        raise ExportError("Local dependency references need an explicit export boundary review: " + ", ".join(local_deps))
    return {"versionsVerified": versions, "retainedLicenseReferencesVerified": len(references),
            "credentialSignaturePatterns": sorted(SIGNATURES), "credentialSignatureFindings": findings,
            "unresolvedCredentialSignatureFindings": 0, "localDependencyReferences": local_deps,
            "requiredBuildInputsVerified": sorted(REQUIRED)}


def export(repo: Path, revision: str, version: str, output: Path, plan: bool = False) -> dict:
    if not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", revision):
        raise ExportError("--revision must be a full lowercase Git commit ID")
    if not re.fullmatch(r"\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?", version):
        raise ExportError("--version must be a semantic release version")
    repo = repo.resolve(strict=True)
    if Path(git(repo, "rev-parse", "--show-toplevel").decode().strip()).resolve() != repo:
        raise ExportError("--repo must be the actual source repository root")
    if git(repo, "rev-parse", "--verify", revision + "^{commit}").decode().strip() != revision:
        raise ExportError("The revision is not an exact commit")
    ensure_clean(repo, revision)
    output = output.resolve()
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", output.name):
        raise ExportError("Output directory name must use letters, digits, dots, underscores or hyphens")
    if output == repo or ".git" in (part.lower() for part in output.parts):
        raise ExportError("Output must not replace the repository or be inside Git metadata")
    archive = output.with_name(output.name + ".zip")
    checksum = archive.with_name(archive.name + ".sha256")
    if any(path.exists() or path.is_symlink() for path in (output, archive, checksum)):
        raise ExportError("Output directory, ZIP or checksum already exists; choose a new output name")
    files, inventory = read_source(repo, revision)
    checks = verify_source(files, version)
    if GENERATED & files.keys():
        raise ExportError("Generated metadata would overwrite selected source")
    commit_time = dt.datetime.fromtimestamp(int(git(repo, "show", "-s", "--format=%ct", revision)), dt.timezone.utc)
    source_bytes = sum(len(data) for data in files.values())
    checks.update({"sourceCommit": revision, "version": version, "sourceDirty": False,
                   "originalSourceFileCount": len(inventory), "originalSourceBytes": source_bytes,
                   "includedBlobsVerifiedAgainstGit": True, "includedModes": sorted({r["gitMode"] for r in inventory}),
                   "gitHistoryIncluded": False, "symlinksSubmodulesSpecialFiles": 0,
                   "omittedHistoricalAuditTests": sorted(OMISSIONS), "fullCleanRebuildRerun": False,
                   "publicPublicationAuthorizedOrPerformed": False, "ownershipOrLegalClearanceClaimed": False})
    result = {"sourceCommit": revision, "version": version, "files": len(inventory),
              "bytes": source_bytes, "licenseReferences": checks["retainedLicenseReferencesVerified"], "planOnly": plan}
    if plan:
        return result
    allowlist = {"rootFiles": sorted(ROOT_FILES), "treePrefixes": list(PREFIXES), "omittedFiles": OMISSIONS,
                 "excludedWithinAllowedPrefixes": list(EXCLUDED_PREFIXES), "forbiddenPathParts": sorted(FORBIDDEN_PARTS),
                 "envFilesExcluded": True, "defaultForAllOtherTrackedPaths": "exclude"}
    files["EXPORT-ALLOWLIST.json"] = json_bytes(allowlist)
    files["EXPORT-CHECKS.json"] = json_bytes(checks)
    files["SOURCE-INVENTORY.json"] = json_bytes({"sourceCommit": revision, "files": inventory})
    files["SOURCE-SHA256SUMS"] = "".join(f'{row["sha256"]}  {row["path"]}\n' for row in inventory).encode()
    files["PROVENANCE.md"] = f"""# Luci {version} source review candidate

Source commit: `{revision}`. Source commit time: `{commit_time.isoformat()}`.
The exporter required that exact HEAD and a clean checkout. {len(inventory)} original
files ({source_bytes} bytes) came directly from Git blobs. SHA256 and Git blob IDs
are in SOURCE-INVENTORY.json. Executable modes and original line endings are retained.
Luci's own licence is MIT; upstream grants and attribution remain unchanged.

This is a candidate only: no public publication, ownership clearance, successful
rebuild, production acceptance or absence of every possible secret is claimed.
The credential-signature scan is limited and does not replace ownership/source review.
No private Git history, repository config, local state, credentials, installed
dependencies, compiled outputs, archived corporate plans or prior exports are copied.
EXPORT-ALLOWLIST.json defines the complete selection; new top-level paths default to excluded.

## Test and historical-document boundary

`desktop/src/seams.test.ts` is deliberately omitted: it loads excluded historical
phase-2 contracts, plans and verification reports. All other selected tests remain.
This candidate therefore does not contain the entire original repository test suite.
Historical references in active documentation, source comments and error messages
remain truthful provenance; references to omitted archives will not resolve here.
The export does not rewrite application source or dependency/licence texts.

## Rebuild and verification

Follow docs/luci/RELEASE.md and docs/luci/SOURCE-EXPORT.md. Use the locked dependencies
and required Go/Node/Python toolchains; native Windows additionally needs Rust/MSVC
and Tauri prerequisites. Dependency installation requires network access. Generated
fonts, web assets, licence inventories and runtime resources are rebuilt by the
normal scripts. No installed node_modules, toolchains or provider credentials are included.

Run the retained Go and frontend suites and build from this separate folder before
accepting the candidate. Record results separately against this exact commit and ZIP
hash; old private-checkout or earlier export results are not current export evidence.
A Git-less package build correctly reports sourceCommit/sourceDirty as null; this
provenance and inventory identify its exported origin. The source exporter itself
requires Git metadata and is intended to run from the original clean checkout.

SOURCE-SHA256SUMS covers original source. EXPORT-SHA256SUMS covers source and generated
metadata except itself; the adjacent ZIP checksum covers the complete archive.
No checksum is a publisher signature. The ZIP uses stable source-commit timestamps.
""".encode()
    files["EXPORT-SHA256SUMS"] = "".join(f"{sha(data)}  {name}\n" for name, data in sorted(files.items())).encode()
    # Verify the source did not change while its pinned blobs were collected.
    ensure_clean(repo, revision)
    output.mkdir(parents=True, exist_ok=False)
    modes = {row["path"]: int(row["gitMode"], 8) & 0o777 for row in inventory}
    for name, data in sorted(files.items()):
        path = output.joinpath(*PurePosixPath(name).parts)
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("xb") as stream:
            stream.write(data)
        os.chmod(path, modes.get(name, 0o644))
        if sha(path.read_bytes()) != sha(data):
            raise ExportError(f"Written-file hash mismatch: {name}; partial output retained for inspection")
    stamp = max(dt.datetime(1980, 1, 1, tzinfo=dt.timezone.utc), commit_time).timetuple()[:6]
    with zipfile.ZipFile(archive, "x", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as bundle:
        for name, data in sorted(files.items()):
            info = zipfile.ZipInfo(output.name + "/" + name, date_time=stamp)
            info.create_system = 3
            info.external_attr = (0o100000 | modes.get(name, 0o644)) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            bundle.writestr(info, data)
    digest = sha(archive.read_bytes())
    with checksum.open("x", encoding="utf-8", newline="\n") as stream:
        stream.write(f"{digest}  {archive.name}\n")
    return {**result, "output": str(output), "archive": str(archive), "archiveSha256": digest}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--revision", required=True, help="Full commit ID, equal to clean checkout HEAD")
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", required=True, type=Path, help="New candidate directory; adjacent ZIP/checksum are also created")
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parent.parent)
    parser.add_argument("--plan", action="store_true", help="Validate clean pinned source and report selection without writing")
    args = parser.parse_args()
    try:
        result = export(args.repo, args.revision, args.version, args.output, args.plan)
    except (ExportError, OSError, ValueError, KeyError, subprocess.TimeoutExpired) as exc:
        print(f"Source export refused: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(result, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
