"""Collect installed runtime dependency notices without inventing licence grants.

Usage: python collect-licenses.py REPOSITORY OUTPUT [--native] [--allow-missing]
The default exits nonzero when any included component has no retained licence.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import subprocess
import sys
import tarfile


def json_stream(text):
    decoder = json.JSONDecoder()
    while text.strip():
        text = text.lstrip()
        item, end = decoder.raw_decode(text)
        yield item
        text = text[end:]


def command(args, cwd, **env):
    return subprocess.check_output(
        args, cwd=cwd, env={**os.environ, **env}, text=True, encoding="utf-8"
    )


def licence_name(name):
    return bool(re.match(r"^(licen[cs]e|copying|copyright|notice|ofl)(?:[._-].*)?$", name, re.I)) or name in {
        "Apache-2.0.txt", "BSD-2-Clause.txt", "CC-BY-SA-4.0.txt",
        "GCC-exception-3.1.txt", "GPL-2.0-only.txt", "GPL-3.0-or-later.txt",
        "ISC.txt", "LLVM-exception.txt", "MIT.txt", "NCSA.txt",
        "OFL-1.1.txt", "Unicode-3.0.txt",
    }


def collect(root, output, native):
    root, output = root.resolve(), output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    records, seen = [], set()
    overrides = []
    for overrides_file in sorted((root / "third-party" / "licenses").rglob("manifest.json")):
        overrides.extend(json.loads(overrides_file.read_text(encoding="utf-8")))

    def add(ecosystem, name, version, directory, declared=None, extra=(), source_url=None, retain_source=False):
        key = (ecosystem, name, version)
        if key in seen:
            return
        seen.add(key)
        directory = pathlib.Path(directory).resolve()
        record = {"ecosystem": ecosystem, "name": name, "version": version, "declaredLicense": declared, "files": []}
        if source_url:
            record["sourceArchiveURL"] = source_url
        candidates = []
        # Root/docs/licences and bundled font licences, excluding nested packages.
        for path in directory.rglob("*"):
            relative = path.relative_to(directory)
            if "node_modules" in relative.parts or ".git" in relative.parts:
                continue
            if len(relative.parts) <= 2 or "fonts" in relative.parts or "licenses" in relative.parts:
                if path.is_file() and licence_name(path.name):
                    candidates.append((path, relative.as_posix(), None))
        for path in extra:
            path = pathlib.Path(path).resolve()
            if path.is_file():
                candidates.append((path, path.name, None))
        for override in overrides:
            if (override["ecosystem"], override["name"], override["version"]) == key:
                if record["declaredLicense"] is None:
                    record["declaredLicense"] = override.get("declaredLicense")
                record["retainedEvidence"] = {field: override[field] for field in ("basis", "provenance", "packageMetadata", "fontFiles") if field in override}
                if ecosystem == "font" and override.get("fontFiles"):
                    actual_fonts = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in directory.rglob("*.woff2")}
                    if actual_fonts != override["fontFiles"]:
                        raise RuntimeError(f"Bundled font bytes differ from retained licence evidence: {name}")
                for retained in override["files"]:
                    path = (root / "third-party" / "licenses" / retained["path"]).resolve()
                    path.relative_to((root / "third-party" / "licenses").resolve())
                    if path.is_file():
                        if retained.get("sha256") and hashlib.sha256(path.read_bytes()).hexdigest() != retained["sha256"]:
                            raise RuntimeError(f"Retained licence snapshot hash changed: {retained['path']}")
                        # Keep subdirectories: bundled fonts can have distinct
                        # grants with the same basename (for example OFL.txt).
                        candidates.append((path, "retained/" + pathlib.PurePosixPath(retained["path"]).as_posix(), retained["source"]))
        destination = output / ecosystem / (re.sub(r"[^a-zA-Z0-9._-]", "_", name) + "-" + re.sub(r"[^a-zA-Z0-9._-]", "_", version))
        for path, relative, source in candidates:
            if path.is_symlink() or path.stat().st_size > 2 << 20:
                continue
            body = path.read_bytes()
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(body)
            item = {"path": target.relative_to(output).as_posix(), "sha256": hashlib.sha256(body).hexdigest()}
            if source:
                item["source"] = source
            if item not in record["files"]:
                record["files"].append(item)
        # A NOTICE alone can supplement a grant but is not the licence itself.
        record["missingLicense"] = not any(licence_name(pathlib.PurePosixPath(item["path"]).name) and not pathlib.PurePosixPath(item["path"]).name.lower().startswith("notice") for item in record["files"])
        if retain_source:
            archive_path = destination / "source.tar.gz"
            archive_path.parent.mkdir(parents=True, exist_ok=True)
            with tarfile.open(archive_path, "w:gz") as archive:
                for path in sorted(directory.rglob("*")):
                    if path.is_file() and not path.is_symlink() and not any(part in {".git", "target"} for part in path.relative_to(directory).parts):
                        archive.add(path, arcname=path.relative_to(directory).as_posix(), recursive=False)
            record["sourceArchive"] = {"path": archive_path.relative_to(output).as_posix(), "sha256": hashlib.sha256(archive_path.read_bytes()).hexdigest(), "modified": False}
        records.append(record)

    lock = json.loads((root / "desktop" / "package-lock.json").read_text(encoding="utf-8"))
    for relative, meta in sorted(lock["packages"].items()):
        if not relative or meta.get("dev", False):
            continue
        directory = root / "desktop" / relative
        package_file = directory / "package.json"
        if not package_file.exists() and meta.get("optional", False):
            continue
        if not package_file.exists():
            raise RuntimeError(f"Run npm ci before collecting runtime package {relative}")
        package = json.loads(package_file.read_text(encoding="utf-8"))
        add("npm", package["name"], package["version"], directory, package.get("license"), source_url=meta.get("resolved"))
        if package["name"] == "@excalidraw/excalidraw":
            # Inventory what npm run build will actually ship. Preparation may
            # intentionally exclude upstream server-only font families.
            fonts = root / "desktop" / "dist" / "design-assets" / "fonts"
            if not fonts.is_dir():
                raise RuntimeError("Run npm run build in desktop before collecting bundled font licences")
            for family in sorted(fonts.iterdir()):
                if family.is_dir():
                    add("font", family.name, "excalidraw-" + package["version"], family)

    modules = {}
    for cwd, args, env in [
        (root, ["go", "list", "-deps", "-json", "./control-plane/cmd/api"], {}),
        (root / "sandboxd", ["go", "list", "-deps", "-json", "./cmd/sandboxd"], {"GOWORK": "off", "GOOS": "linux", "CGO_ENABLED": "0", "GOFLAGS": "-mod=vendor"}),
    ]:
        for package in json_stream(command(args, cwd, **env)):
            module = package.get("Module")
            if not module or module.get("Main"):
                continue
            effective = module.get("Replace", module)
            directory = effective.get("Dir")
            if not directory:
                # Vendored packages have package dirs but no module directory.
                import_path = package["ImportPath"]
                suffix = import_path[len(module["Path"]):].strip("/")
                directory = pathlib.Path(package["Dir"])
                for _ in pathlib.PurePosixPath(suffix).parts if suffix else ():
                    directory = directory.parent
            modules[(module["Path"], module.get("Version", "local"))] = directory
    for (name, version), directory in sorted(modules.items()):
        add("go", name, version, directory)
    goroot = pathlib.Path(command(["go", "env", "GOROOT"], root).strip())
    add("go", "standard-library", command(["go", "env", "GOVERSION"], root).strip(), goroot, "BSD-3-Clause", [goroot / "LICENSE"])

    if native:
        metadata = json.loads(command(["cargo", "metadata", "--locked", "--format-version", "1", "--filter-platform", "x86_64-pc-windows-msvc"], root / "desktop" / "src-tauri"))
        nodes = {node["id"]: node for node in metadata["resolve"]["nodes"]}
        pending, reachable = list(metadata["workspace_members"]), set()
        while pending:
            identifier = pending.pop()
            if identifier in reachable:
                continue
            reachable.add(identifier)
            pending.extend(edge["pkg"] for edge in nodes.get(identifier, {}).get("deps", []))
        for package in metadata["packages"]:
            if package["id"] not in reachable or package["id"] in metadata["workspace_members"]:
                continue
            directory = pathlib.Path(package["manifest_path"]).parent
            extra = [directory / package["license_file"]] if package.get("license_file") else []
            declared = package.get("license")
            add("cargo", package["name"], package["version"], directory, declared, extra,
                source_url=f'https://crates.io/api/v1/crates/{package["name"]}/{package["version"]}/download',
                retain_source=declared == "MPL-2.0")
        rustroot = pathlib.Path(command(["rustc", "--print", "sysroot"], root).strip())
        rust_version = command(["rustc", "--version"], root).split()[1]
        add("rust", "standard-library", rust_version, rustroot / "share" / "doc" / "rust", "MIT OR Apache-2.0 (see bundled library notices)")

    missing = [f'{r["ecosystem"]}:{r["name"]}@{r["version"]}' for r in records if r["missingLicense"]]
    manifest = {"schemaVersion": 1, "scope": "Installed npm runtime packages, bundled Excalidraw font families, compiled Go modules and standard library" + (", reachable native Windows Cargo packages and installed Rust library notices" if native else ""), "reviewRequired": "Collected notices are evidence, not a compatibility or legal approval. Separately installed software and container images need their own review.", "missingLicenses": missing, "components": records}
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    (output / "MISSING-LICENSES.txt").write_text("\n".join(missing) + ("\n" if missing else ""), encoding="utf-8")
    (output / "README.txt").write_text(
        "Luci dependency notices\n\n"
        "manifest.json identifies component versions, retained licence files and their hashes.\n"
        "Original third-party terms remain in force; Luci's licence does not replace them.\n"
        "For npm and Cargo packages, sourceArchiveURL identifies the published upstream source.\n"
        "For MPL-2.0 Rust dependencies, sourceArchive identifies a bundled archive of the\n"
        "unmodified covered source under the MPL-2.0 terms retained alongside it.\n"
        "The Rust library notice collection can mention toolchain or optional components\n"
        "beyond the final executable; it is retained conservatively.\n"
        "MISSING-LICENSES.txt must be empty in a distributable candidate.\n",
        encoding="utf-8",
    )
    return len(records), missing


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("repository", type=pathlib.Path)
    parser.add_argument("output", type=pathlib.Path)
    parser.add_argument("--native", action="store_true")
    parser.add_argument("--allow-missing", action="store_true", help="Diagnostic report only; not a distributable release gate")
    args = parser.parse_args()
    count, missing = collect(args.repository, args.output, args.native)
    print(f"Collected licence evidence for {count} components; {len(missing)} require licence files.")
    for component in missing:
        print(component)
    if missing and not args.allow_missing:
        sys.exit(2)
