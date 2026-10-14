"""Prepare a fresh, bounded Docker build context from a verified portable release."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import subprocess
import tarfile
import zipfile
import sys

sys.path.insert(0, str(Path(__file__).parent / 'runtime-oci'))
import payload as runtime_payload

VERSION = re.compile(r"^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$")
TOP_LEVEL = {"luci", "web", "runtime", "cloud", "licenses", "docs", "LICENSE", "NOTICE", "README.md", "DESIGN.md", "start-luci.sh", "package-identity"}
REQUIRED = {"luci", "web/index.html", "runtime/images/sandbox/Dockerfile", "runtime/images/sandbox/entrypoint.sh", "cloud/luci-cloud-linux-amd64.tar.gz", "LICENSE", "NOTICE", "licenses/manifest.json", "licenses/MISSING-LICENSES.txt"}
PACKAGING_FILES = ("Dockerfile", "compose.yaml", "entrypoint.sh", "engine-entrypoint.sh", "workspace-init.sh", "healthcheck.sh", "start-luci.sh", "Start-Luci.ps1", "Start-Luci.cmd", "README.md", ".dockerignore")
MAX_FILES = 20000
MAX_BYTES = 512 * 1024 * 1024
MAX_RUNTIME_BYTES = 8 * 1024 * 1024 * 1024


def digest(path: Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def regular_path(path: Path) -> None:
    info = path.lstat()
    if path.is_symlink() or getattr(info, "st_file_attributes", 0) & getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0):
        raise ValueError(f"Linked release input refused: {path.name}")
    if not stat.S_ISREG(info.st_mode) and not stat.S_ISDIR(info.st_mode):
        raise ValueError(f"Special release input refused: {path.name}")


def safe_member(name: str) -> str:
    normalized = name.rstrip("/")
    candidate = PurePosixPath(normalized)
    if not normalized or candidate.is_absolute() or "\\" in normalized or ":" in normalized:
        raise ValueError("Invalid release archive path")
    if any(part in {".", "..", ".git", "node_modules", "artifacts", "target"} or part.startswith(".env") for part in candidate.parts):
        raise ValueError("Private or escaping release archive path")
    if candidate.parts[0] not in TOP_LEVEL or candidate.as_posix() != normalized:
        raise ValueError("Unexpected release bundle input")
    return normalized


def verify_release(release: Path, version: str) -> tuple[dict, dict[str, tuple[str, int, int]]]:
    regular_path(release)
    manifest_path = release / "release-manifest.json"
    regular_path(manifest_path)
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("version") != version or manifest.get("sourceDirty") is not False:
        raise ValueError("Docker packaging requires the matching clean-source release manifest")
    if not re.fullmatch(r"[0-9a-f]{40}(?:[0-9a-f]{24})?", manifest.get("sourceCommit", "")):
        raise ValueError("The release manifest needs a full source commit")
    record = next((item for item in manifest.get("artifacts", []) if item.get("name") == "luci-linux-amd64.tar.gz"), None)
    if not record or not re.fullmatch(r"[0-9a-f]{64}", record.get("sha256", "")):
        raise ValueError("The release manifest does not attest a Linux amd64 archive")
    archive_path = release / record["name"]
    regular_path(archive_path)
    if archive_path.stat().st_size != record.get("bytes") or digest(archive_path) != record["sha256"]:
        raise ValueError("The Linux portable archive does not match its release manifest")
    bundle = release / "luci-linux-amd64"
    regular_path(bundle)
    expected: dict[str, tuple[str, int, int]] = {}
    seen: set[str] = set()
    total = 0
    runtime_total = 0
    with tarfile.open(archive_path, "r:gz") as archive:
        for member in archive:
            relative = safe_member(member.name)
            if relative in seen:
                raise ValueError("Duplicate release archive member")
            seen.add(relative)
            if len(seen) > MAX_FILES or not (member.isfile() or member.isdir()):
                raise ValueError("Linked, special or oversized release archive refused")
            if member.isdir():
                continue
            if relative == 'runtime/' + runtime_payload.PAYLOAD:
                runtime_total += member.size
            else:
                total += member.size
            if member.size < 0 or total > MAX_BYTES or runtime_total > MAX_RUNTIME_BYTES:
                raise ValueError("Release archive exceeds the bounded context size")
            stream = archive.extractfile(member)
            if stream is None:
                raise ValueError("Release archive member cannot be read")
            value = hashlib.sha256()
            for block in iter(lambda: stream.read(1024 * 1024), b""):
                value.update(block)
            expected[relative] = (value.hexdigest(), member.size, member.mode)
    actual: set[str] = set()
    for source in sorted(bundle.rglob("*")):
        regular_path(source)
        relative = safe_member(source.relative_to(bundle).as_posix())
        if source.is_dir():
            continue
        actual.add(relative)
        wanted = expected.get(relative)
        if wanted is None or source.stat().st_size != wanted[1] or digest(source) != wanted[0]:
            raise ValueError(f"Extracted release file differs from its archive: {relative}")
    if actual != set(expected) or not REQUIRED.issubset(actual):
        raise ValueError("The portable bundle is incomplete or has unexpected files")
    with (bundle / "luci").open("rb") as source:
        header = source.read(20)
    if len(header) != 20 or header[:6] != b"\x7fELF\x02\x01" or header[18:20] != b"\x3e\x00":
        raise ValueError("The controller must be a Linux amd64 ELF executable")
    licenses = json.loads((bundle / "licenses/manifest.json").read_text(encoding="utf-8"))
    if licenses.get("missingLicenses") != [] or (bundle / "licenses/MISSING-LICENSES.txt").stat().st_size != 0:
        raise ValueError("The portable bundle has incomplete licence evidence")
    runtime_payload.verify(bundle / 'runtime', package=bundle, canonical=bundle / 'runtime', source_commit=manifest['sourceCommit'])
    runtime_payload.source_receipt(runtime_payload.read_json(bundle / 'runtime' / runtime_payload.SOURCE), manifest['sourceCommit'])
    return manifest, expected


def verify_checkout(repo: Path, manifest: dict) -> None:
    def git(*args: str) -> str:
        return subprocess.check_output(["git", "-C", str(repo), *args], text=True, stderr=subprocess.DEVNULL).strip()
    if git("rev-parse", "--verify", "HEAD") != manifest["sourceCommit"]:
        raise ValueError("Build the release from the current source commit before preparing Docker")
    if git("status", "--porcelain=v1", "--untracked-files=all"):
        raise ValueError("Commit the complete source batch and build its clean release before Docker packaging")


def bounded_output(repo: Path, output: Path) -> Path:
    allowed = Path(os.path.abspath(repo / "artifacts/docker"))
    target = Path(os.path.abspath(output))
    if target == allowed or not target.is_relative_to(allowed):
        raise ValueError("Docker output must be a fresh directory beneath artifacts/docker")
    current = target
    while current != repo.absolute():
        if current.exists() or current.is_symlink():
            regular_path(current)
        current = current.parent
    if target.exists():
        raise ValueError("Docker output already exists; choose a fresh output directory")
    return target


def prepare(repo: Path, release: Path, version: str, output: Path, manifest: dict, files: dict[str, tuple[str, int, int]]) -> dict:
    target = bounded_output(repo, output)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.mkdir()
    for name in PACKAGING_FILES:
        source = repo / "distribution/docker" / name
        regular_path(source)
        content = source.read_bytes()
        # Git may have CRLF checkout settings on Windows. Shell and Docker files need LF.
        if name.endswith(".sh") or name in {"Dockerfile", "compose.yaml", ".dockerignore"}:
            content = content.replace(b"\r\n", b"\n")
        if name == "compose.yaml":
            content = re.sub(rb"\$\{LUCI_VERSION:-[^}]+\}", ("${LUCI_VERSION:-" + version + "}").encode("ascii"), content)
        destination = target / name
        destination.write_bytes(content)
        if name.endswith(".sh"):
            destination.chmod(0o755)
    bundle = target / "bundle"
    bundle.mkdir()
    for relative, (_, _, mode) in sorted(files.items()):
        source = release / "luci-linux-amd64" / relative
        destination = bundle / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, destination)
        destination.chmod(0o755 if mode & 0o111 else 0o644)
    output_files = [path for path in target.rglob("*") if path.is_file()]
    result = {
        "kind": "luci-docker-distribution", "version": version,
        "sourceCommit": manifest["sourceCommit"], "sourceDirty": False,
        "portableArchiveSha256": next(item["sha256"] for item in manifest["artifacts"] if item["name"] == "luci-linux-amd64.tar.gz"),
        "files": [{"path": path.relative_to(target).as_posix(), "bytes": path.stat().st_size, "sha256": digest(path)} for path in sorted(output_files)],
    }
    (target / "distribution-manifest.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    return result


def archive_distribution(repo: Path, output: Path) -> dict:
    allowed = Path(os.path.abspath(repo / "artifacts/docker"))
    target = Path(os.path.abspath(output))
    archive = target.with_name(target.name + ".zip")
    if not target.is_relative_to(allowed) or not archive.is_relative_to(allowed):
        raise ValueError("Docker archive must stay beneath artifacts/docker")
    regular_path(target)
    if archive.exists() or archive.is_symlink():
        raise ValueError("Docker archive already exists; choose a fresh output directory")
    with zipfile.ZipFile(archive, "x", zipfile.ZIP_DEFLATED, compresslevel=6) as packaged:
        for source in sorted(target.rglob("*")):
            regular_path(source)
            if source.is_dir():
                continue
            relative = source.relative_to(target).as_posix()
            record = zipfile.ZipInfo(relative)
            record.create_system = 3
            record.compress_type = zipfile.ZIP_DEFLATED
            record.external_attr = (0o100755 if relative.endswith(".sh") or relative == "bundle/luci" else 0o100644) << 16
            # The sole OCI payload can be several GiB; never materialize it in RAM.
            with source.open('rb') as incoming, packaged.open(record, 'w', force_zip64=True) as outgoing:
                shutil.copyfileobj(incoming, outgoing, 1024 * 1024)
    return {"name": archive.name, "bytes": archive.stat().st_size, "sha256": digest(archive)}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--archive", action="store_true", help="Also create a portable Docker-distribution ZIP beside the prepared directory")
    args = parser.parse_args()
    if not VERSION.fullmatch(args.version):
        parser.error("Version must be a semantic version")
    repo = Path(__file__).resolve().parent.parent
    release = repo / "artifacts/releases" / args.version
    try:
        # Validate ancestors before inspecting a generated release tree.
        for directory in (repo / "artifacts", repo / "artifacts/releases", release):
            regular_path(directory)
        manifest, files = verify_release(release, args.version)
        verify_checkout(repo, manifest)
        output = args.output or repo / "artifacts/docker" / args.version
        result = prepare(repo, release, args.version, output, manifest, files)
        receipt = {"version": result["version"], "sourceCommit": result["sourceCommit"], "files": len(result["files"]), "output": str(output)}
        if args.archive:
            receipt["archive"] = archive_distribution(repo, output)
        print(json.dumps(receipt))
    except (OSError, ValueError, json.JSONDecodeError, subprocess.SubprocessError, tarfile.TarError) as cause:
        parser.exit(1, f"Docker distribution preparation failed: {cause}\n")


if __name__ == "__main__":
    main()
