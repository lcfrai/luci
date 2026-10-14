#!/usr/bin/env python3
"""Verify/stage one immutable runtime payload and explicit core companion.

All paths are local ordinary package files. This module never calls Docker.
"""
from __future__ import annotations

import argparse
import copy
from pathlib import Path
import shutil
import tempfile
import re

import archive
from canonical_inputs import inventory_digest, selected, stage
from common import plain_directory, read_json, schema_validate, write_json
from producer import clean_source

MANIFEST = "runtime-oci-manifest-v1.json"
RECEIPT = "runtime-build-receipt.json"
SOURCE = "source-receipt.json"
PAYLOAD = "runtime-image-linux-amd64.tar"
REUSE = "runtime-source-reuse-v1.json"
PRIOR_MANIFEST = "runtime-prior-manifest-v1.json"
PRIOR_RECEIPT = "runtime-prior-build-receipt.json"
COMPANION = "runtime-payload-companion.json"


def source_receipt(value, revision):
    if not isinstance(value, dict) or set(value) != {"version", "sourceCommit", "sourceDirty"} or type(value["version"]) is not int or value["version"] != 1 or value["sourceDirty"] is not False or not isinstance(value['sourceCommit'], str) or not re.fullmatch(r"[a-f0-9]{40}", value["sourceCommit"]) or value["sourceCommit"] != revision:
        raise ValueError("installed clean source receipt mismatch")
    return value


def verify_companion(runtime):
    value = read_json(runtime / COMPANION)
    expected = {'version', 'kind', 'sourceCommit', 'manifestSHA256', 'archiveSHA256', 'archiveBytes', 'fileName', 'ordinaryLocalInstall', 'automaticSourceBuildFallback'}
    if not isinstance(value, dict) or set(value) != expected or type(value['version']) is not int or value['version'] != 1 or value['kind'] != 'luci-runtime-companion-required' or value['ordinaryLocalInstall'] != 'requires-verified-companion' or value['automaticSourceBuildFallback'] is not False or value['fileName'] != PAYLOAD or type(value['archiveBytes']) is not int or not 1024 <= value['archiveBytes'] <= archive.MAX_ARCHIVE or any(not isinstance(value[key], str) or not re.fullmatch(r'[a-f0-9]{64}', value[key]) for key in ('manifestSHA256', 'archiveSHA256')) or (runtime / PAYLOAD).exists() or (runtime / MANIFEST).exists():
        raise ValueError('invalid or duplicated management-core runtime companion')
    source_receipt(read_json(runtime / SOURCE), value['sourceCommit'])
    return value


def source_inventory(root):
    rows, total = [], 0
    for tree in ("sandboxd/cmd", "sandboxd/internal", "sandboxd/vendor"):
        plain_directory(root / tree)
        for file in sorted((root / tree).rglob("*")):
            if file.is_dir():
                plain_directory(file)
            else:
                row = archive.file_record(file, file.relative_to(root).as_posix())
                row["mode"] = "0644"
                rows.append(row)
    for name in ("sandboxd/go.mod", "sandboxd/go.sum", "images/sandbox/Dockerfile", "images/sandbox/entrypoint.sh"):
        row = archive.file_record(root / name, name)
        row["mode"] = "0644"
        rows.append(row)
    for row in rows:
        total += row["bytes"]
    if len(rows) > 8192 or total > 64 << 20:
        raise ValueError("packaged canonical runtime context exceeds its bounds")
    return sorted(rows, key=lambda row: row["path"].encode("ascii"))


def destination_file(root, name):
    archive.safe_path(name)
    target = root / name
    # Reject existing ancestor links before creating anything or overwriting
    # canonical CRLF copies from the ordinary package staging step.
    for directory in reversed(target.parent.parents):
        if directory.exists() or directory.is_symlink():
            plain_directory(directory)
    if target.parent.exists() or target.parent.is_symlink():
        plain_directory(target.parent)
    target.parent.mkdir(parents=True, exist_ok=True)
    if target.exists() or target.is_symlink():
        archive.regular(target)
    return target


def verify(root, package=None, canonical=None, source_commit=None):
    root = plain_directory(root)
    package = plain_directory(package or root)
    canonical = plain_directory(canonical or root / "canonical")
    manifest = schema_validate(read_json(root / MANIFEST), "runtime-oci-manifest-v1.schema.json")
    receipt = schema_validate(read_json(root / RECEIPT), "runtime-oci-build-receipt-v1.schema.json")
    rows = source_inventory(canonical)
    if rows != manifest["sourceFiles"] or inventory_digest(rows) != manifest["canonicalInputDigest"]:
        raise ValueError("packaged canonical runtime input differs from manifest")
    if source_commit is not None and manifest["sourceCommit"] != source_commit:
        raise ValueError("payload source does not match the clean package source")
    if receipt["buildSourceCommit"] != manifest["producer"]["buildSourceCommit"] or receipt["canonicalInputDigest"] != manifest["canonicalInputDigest"] or receipt["imageID"] != manifest["image"]["id"] or receipt["archive"] != {key: manifest["archive"][key] for key in ("sha256", "bytes")}:
        raise ValueError("producer receipt/source/image/archive binding mismatch")
    if archive.file_record(root / RECEIPT)["sha256"] != manifest["producer"]["buildReceiptSHA256"] or next(x["sha256"] for x in rows if x["path"] == "images/sandbox/Dockerfile") != manifest["producer"]["dockerfileSHA256"]:
        raise ValueError("producer receipt or Dockerfile bytes mismatch")
    if len({row["reference"] for row in receipt["baseImages"]}) != 2 or {row["tool"] for row in receipt["probes"]} != {"sandboxd", "codex", "claudeCode", "openCode"}:
        raise ValueError("producer base/probe inventory is incomplete")
    expected_versions = dict(manifest["runtime"]["harnesses"], sandboxd="0.5.0")
    for row in receipt["probes"]:
        if row["reportedVersion"] != expected_versions[row["tool"]]:
            raise ValueError("producer reported a different upstream version")
    if manifest["runtime"] != archive.RUNTIME:
        raise ValueError("payload runtime differs from pinned release policy")
    expected_archive = manifest["archive"]
    actual = archive.file_record(root / PAYLOAD)
    if any(actual[key] != expected_archive[key] for key in ("sha256", "bytes")):
        raise ValueError("runtime archive bytes differ from source-bound manifest")
    archive.verify(root / PAYLOAD, manifest["image"], expected_archive["members"])
    names = []
    total = 0
    for row in manifest["licenses"]:
        name = archive.safe_path(row["path"])
        if not name.startswith("licenses/runtime-image/"):
            raise ValueError("runtime licence is not package-root relative")
        names.append(name)
        total += row["bytes"]
        actual = archive.file_record(package / name)
        if any(actual[key] != row[key] for key in ("sha256", "bytes")):
            raise ValueError("packaged runtime grant bytes mismatch")
    if names != sorted(names) or len(names) != len(set(names)) or total > 64 << 20:
        raise ValueError("runtime licence inventory duplicate/order/bound mismatch")
    notices = plain_directory(package / 'licenses/runtime-image')
    actual_names = []
    for file in notices.rglob('*'):
        if file.is_dir():
            plain_directory(file)
        else:
            archive.regular(file)
            actual_names.append(file.relative_to(package).as_posix())
    if sorted(actual_names) != names:
        raise ValueError('runtime licence subtree contains unmanifested grants')
    if manifest["sourceCommit"] != receipt["buildSourceCommit"]:
        evidence = schema_validate(read_json(root / REUSE), "runtime-oci-source-reuse-v1.schema.json")
        prior = schema_validate(read_json(root / PRIOR_MANIFEST), "runtime-oci-manifest-v1.schema.json")
        source_value = source_receipt(read_json(root / SOURCE), manifest["sourceCommit"])
        wanted_source = {"version": 1, "sourceCommit": manifest["sourceCommit"], "sourceDirty": False}
        if source_value != wanted_source or evidence != {"version": 1, "kind": "luci-runtime-equivalent-input-reuse", "priorManifestSHA256": archive.file_record(root / PRIOR_MANIFEST)["sha256"], "priorBuildReceiptSHA256": archive.file_record(root / PRIOR_RECEIPT)["sha256"], "priorBuildSourceCommit": receipt["buildSourceCommit"], "installedSourceCommit": manifest["sourceCommit"], "installedSourceReceiptSHA256": archive.file_record(root / SOURCE)["sha256"], "canonicalInputAlgorithm": manifest["canonicalInputAlgorithm"], "canonicalInputDigest": manifest["canonicalInputDigest"], "imageID": manifest["image"]["id"], "archiveSHA256": manifest["archive"]["sha256"], "archiveBytes": manifest["archive"]["bytes"]}:
            raise ValueError("equivalent-input source reuse receipt mismatch")
        if archive.file_record(root / REUSE)["sha256"] != manifest["producer"].get("reuseEvidenceSHA256") or archive.file_record(root / PRIOR_RECEIPT)["sha256"] != manifest["producer"]["buildReceiptSHA256"]:
            raise ValueError("prior trusted runtime receipt mismatch")
        if prior["sourceCommit"] != receipt["buildSourceCommit"] or any(prior[key] != manifest[key] for key in ("canonicalInputAlgorithm", "canonicalInputDigest", "sourceFiles", "image", "archive", "runtime", "licenses")) or prior["producer"]["buildReceiptSHA256"] != manifest["producer"]["buildReceiptSHA256"]:
            raise ValueError("prior payload does not establish exact equal-input reuse")
    elif "reuseEvidenceSHA256" in manifest["producer"]:
        raise ValueError("unexpected reuse evidence on a direct build")
    return manifest


def stage_payload(payload, repo, revision, package):
    payload, repo, package = Path(payload).absolute(), Path(repo).absolute(), plain_directory(package)
    clean_source(repo, revision)
    original = verify(payload)
    runtime = package / "runtime"
    runtime.mkdir(exist_ok=True)
    plain_directory(runtime)
    with tempfile.TemporaryDirectory(prefix="luci-canonical-", dir=package.parent) as temporary:
        inputs = stage(repo, revision, Path(temporary) / "inputs")
        if inputs["canonicalInputDigest"] != original["canonicalInputDigest"] or inputs["sourceFiles"] != original["sourceFiles"]:
            raise ValueError("new package changed runtime inputs; produce a new image")
        for row in inputs["sourceFiles"]:
            destination = destination_file(runtime, row['path'])
            shutil.copyfile(Path(temporary) / "inputs" / row["path"], destination)
    for name in (PAYLOAD, RECEIPT):
        if (runtime / name).exists():
            raise ValueError("runtime payload destination already exists")
        shutil.copyfile(payload / name, runtime / name)
    for row in original["licenses"]:
        destination = destination_file(package, row['path'])
        shutil.copyfile(payload / row["path"], destination)
    write_json(runtime / SOURCE, {"version": 1, "sourceCommit": revision, "sourceDirty": False})
    manifest = copy.deepcopy(original)
    manifest["sourceCommit"] = revision
    if revision != original["producer"]["buildSourceCommit"]:
        if original["sourceCommit"] != original["producer"]["buildSourceCommit"]:
            raise ValueError("reuse must start from the original trusted build output")
        shutil.copyfile(payload / MANIFEST, runtime / PRIOR_MANIFEST)
        shutil.copyfile(payload / RECEIPT, runtime / PRIOR_RECEIPT)
        reuse = {"version": 1, "kind": "luci-runtime-equivalent-input-reuse", "priorManifestSHA256": archive.file_record(runtime / PRIOR_MANIFEST)["sha256"], "priorBuildReceiptSHA256": archive.file_record(runtime / PRIOR_RECEIPT)["sha256"], "priorBuildSourceCommit": original["producer"]["buildSourceCommit"], "installedSourceCommit": revision, "installedSourceReceiptSHA256": archive.file_record(runtime / SOURCE)["sha256"], "canonicalInputAlgorithm": inputs["algorithm"], "canonicalInputDigest": inputs["canonicalInputDigest"], "imageID": original["image"]["id"], "archiveSHA256": original["archive"]["sha256"], "archiveBytes": original["archive"]["bytes"]}
        schema_validate(reuse, "runtime-oci-source-reuse-v1.schema.json")
        write_json(runtime / REUSE, reuse)
        manifest["producer"]["reuseEvidenceSHA256"] = archive.file_record(runtime / REUSE)["sha256"]
    write_json(runtime / MANIFEST, manifest)
    verify(runtime, package=package, canonical=runtime, source_commit=revision)
    clean_source(repo, revision)
    return manifest


def core_companion(package, payload_manifest):
    """A management core intentionally lacks the heavy local Docker payload."""
    package = plain_directory(package)
    runtime = package / "runtime"
    if (runtime / PAYLOAD).exists() or (runtime / MANIFEST).exists():
        raise ValueError("management core would duplicate the outer runtime payload")
    source = source_receipt(read_json(runtime / SOURCE), read_json(payload_manifest)["sourceCommit"])
    manifest = schema_validate(read_json(payload_manifest), "runtime-oci-manifest-v1.schema.json")
    if source != {"version": 1, "sourceCommit": manifest["sourceCommit"], "sourceDirty": False}:
        raise ValueError("core and required companion source differ")
    receipt = {"version": 1, "kind": "luci-runtime-companion-required", "sourceCommit": source["sourceCommit"], "manifestSHA256": archive.file_record(payload_manifest)["sha256"], "archiveSHA256": manifest["archive"]["sha256"], "archiveBytes": manifest["archive"]["bytes"], "fileName": PAYLOAD, "ordinaryLocalInstall": "requires-verified-companion", "automaticSourceBuildFallback": False}
    write_json(runtime / COMPANION, receipt)
    return receipt


def core_from_package(package, destination):
    package, destination = plain_directory(package), Path(destination).absolute()
    plain_directory(destination.parent)
    if destination.exists() or destination.is_relative_to(package):
        raise ValueError("choose a fresh separate management-core directory")
    excluded = {"runtime/" + name for name in (PAYLOAD, MANIFEST, RECEIPT, REUSE, PRIOR_MANIFEST, PRIOR_RECEIPT)}
    destination.mkdir()
    for source in sorted(package.rglob("*")):
        relative = source.relative_to(package).as_posix()
        if relative in excluded or relative.startswith("licenses/runtime-image/") or relative == "licenses/runtime-image" or relative == "package-identity" or relative.startswith("cloud/") or relative == "cloud":
            continue
        if source.is_dir():
            plain_directory(source)
            (destination / relative).mkdir(parents=True, exist_ok=True)
        else:
            archive.regular(source)
            (destination / relative).parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, destination / relative)
    return core_companion(destination, package / "runtime" / MANIFEST)


def distribution_record(package):
    """Small release provenance record; all evidence was verified at staging."""
    runtime = plain_directory(Path(package) / 'runtime')
    manifest = schema_validate(read_json(runtime / MANIFEST), 'runtime-oci-manifest-v1.schema.json')
    source_receipt(read_json(runtime / SOURCE), manifest['sourceCommit'])
    return {'manifestSHA256': archive.file_record(runtime / MANIFEST)['sha256'],
            'buildReceiptSHA256': archive.file_record(runtime / RECEIPT)['sha256'],
            'sourceReceiptSHA256': archive.file_record(runtime / SOURCE)['sha256'],
            'sourceCommit': manifest['sourceCommit'], 'buildSourceCommit': manifest['producer']['buildSourceCommit'],
            'canonicalInputDigest': manifest['canonicalInputDigest'], 'imageID': manifest['image']['id'],
            'archiveSHA256': manifest['archive']['sha256'], 'archiveBytes': manifest['archive']['bytes'],
            'payloadRole': 'one-per-outer-package', 'embeddedManagementCore': 'requires-verified-companion',
            'automaticSourceBuildFallback': False}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="action", required=True)
    check = commands.add_parser("verify")
    check.add_argument("--payload", type=Path, required=True)
    describe = commands.add_parser('describe-package')
    describe.add_argument('--package', type=Path, required=True)
    install = commands.add_parser("stage")
    for name in ("payload", "repo", "package"):
        install.add_argument("--" + name, type=Path, required=True)
    install.add_argument("--revision", required=True)
    companion = commands.add_parser("core-companion")
    companion.add_argument("--package", type=Path, required=True)
    companion.add_argument("--manifest", type=Path, required=True)
    core = commands.add_parser("core-from-package")
    core.add_argument("--package", type=Path, required=True)
    core.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.action == "verify":
            verify(args.payload)
        elif args.action == 'describe-package':
            import json
            print(json.dumps(distribution_record(args.package), sort_keys=True))
            raise SystemExit(0)
        elif args.action == "stage":
            stage_payload(args.payload, args.repo, args.revision, args.package)
        elif args.action == "core-companion":
            core_companion(args.package, args.manifest)
        else:
            core_from_package(args.package, args.output)
    except (ValueError, OSError) as exc:
        parser.exit(1, "Runtime payload refusal: " + str(exc) + "\n")
    print("Verified runtime payload operation: " + args.action)
