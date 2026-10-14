"""Executed without credentials/network inside the fresh producer image.

Emit a bounded tar of actual installed grants plus component metadata. The host
verifies every ordinary member before copying it into package-root licenses/.
"""
import hashlib
import io
import json
from pathlib import Path
import re
import sys
import tarfile
import subprocess
import base64

MAX_FILES, MAX_BYTES = 4096, 64 << 20


def find_fallback(name, version, grant):
    return next((item for item in globals().get('SOURCE_GRANT_FALLBACKS', []) if item['name'] == name and version in item['versions'] and item['grant'] == grant), None)


def distribution_notice(metadata, package):
    """Only the exact OpenCode distribution's own binary siblings share MIT."""
    if package.get('name') not in ('opencode-linux-x64', 'opencode-linux-x64-baseline') or package.get('version') != '1.18.34' or package.get('license') is not None:
        return None
    parent = metadata.parent.parent.parent
    if metadata.parent.parent.name != 'node_modules' or parent.name != 'opencode-ai':
        return None
    value = json.loads((parent / 'package.json').read_text(encoding='utf-8'))
    if value.get('name') != 'opencode-ai' or value.get('version') != '1.18.34' or value.get('license') != 'MIT' or value.get('optionalDependencies', {}).get(package['name']) != '1.18.34':
        return None
    path = parent / 'LICENSE'
    if not path.is_file() or hashlib.sha256(path.read_bytes()).hexdigest() != '625f0f619133f89bbbb2abe37369613dfa1885eba1e50d02170deb62bb42cb6b':
        raise ValueError('exact OpenCode parent grant differs')
    return path


def collect():
    rows, bodies, total = [], {}, 0

    def add_body(data, component, version, grant, provenance, suffix):
        nonlocal total
        if not 0 < len(data) <= 16 << 20:
            raise ValueError('missing/oversized actual runtime grant')
        name = re.sub(r"[^A-Za-z0-9._-]", "_", component + "-" + version)
        relative = 'licenses/runtime-image/' + name + '/' + suffix
        if relative in bodies:
            raise ValueError('duplicate runtime grant destination')
        total += len(data)
        if len(rows) >= MAX_FILES or total > MAX_BYTES or len(relative) > 256:
            raise ValueError('runtime licence inventory exceeds its bounds')
        bodies[relative] = data
        rows.append({'path': relative, 'sha256': hashlib.sha256(data).hexdigest(), 'bytes': len(data),
                     'component': component, 'version': version, 'grant': grant[:160], 'provenance': provenance[:512]})

    def add(path, component, version, grant, provenance, suffix=None):
        original = Path(path)
        path = original.resolve()
        if not path.is_file() or path.stat().st_size == 0 or path.stat().st_size > 16 << 20:
            raise ValueError("missing/oversized installed runtime grant: " + str(original))
        if not any(path.is_relative_to(Path(root)) for root in ("/usr/share/doc", "/usr/share/common-licenses", "/usr/local/lib/node_modules", "/usr/local/go", '/usr/local/share/doc/node')) and path != Path('/usr/local/LICENSE'):
            raise ValueError("runtime grant escapes installed notice roots")
        add_body(path.read_bytes(), component, version, grant, provenance, suffix or original.name)

    status = Path("/var/lib/dpkg/status").read_text(encoding="utf-8")
    for paragraph in status.split("\n\n"):
        fields = dict(line.split(": ", 1) for line in paragraph.splitlines() if ": " in line and not line.startswith(" "))
        if fields.get("Status") != "install ok installed":
            continue
        name, version = fields["Package"], fields["Version"]
        notice = Path("/usr/share/doc") / name / "copyright"
        if not notice.is_file():
            raise ValueError("installed Debian component lacks copyright: " + name)
        raw = notice.read_text(encoding="utf-8", errors="replace")
        grants = sorted(set(re.findall(r"^License:\s*(.+)$", raw, re.M)))
        grant = "; ".join(grants) or "Upstream grants retained in Debian copyright"
        add(notice, "debian:" + name, version, grant, "Installed Debian package " + name + "=" + version + "; " + str(notice))
    for notice in sorted(Path("/usr/share/common-licenses").iterdir()):
        if notice.is_file():
            add(notice, "debian-common-licenses", "bookworm", notice.name, "Debian common grant text " + str(notice))
    node_notice = next((path for path in (Path('/usr/local/LICENSE'), Path('/usr/local/share/doc/node/LICENSE')) if path.is_file()), None)
    if node_notice is None:
        raise ValueError('Node distribution lacks its retained upstream grant')
    node_version = subprocess.check_output(['node', '--version'], text=True, timeout=5).strip()
    add(node_notice, 'nodejs-distribution', node_version, 'Node.js and bundled third-party grants retained verbatim', str(node_notice))

    modules = Path("/usr/local/lib/node_modules")
    seen = set()
    for metadata in sorted(modules.rglob("package.json")):
        parts = metadata.parent.relative_to(modules).parts
        installed_root = len(parts) == 1 or len(parts) == 2 and parts[0].startswith("@") or len(parts) >= 2 and parts[-2] == "node_modules" or len(parts) >= 3 and parts[-3] == "node_modules" and parts[-2].startswith("@")
        if not installed_root:
            continue
        package = json.loads(metadata.read_text(encoding="utf-8"))
        name, version = package.get("name"), package.get("version")
        if not isinstance(name, str) or not isinstance(version, str) or (name, version) in seen:
            continue
        seen.add((name, version))
        grant = package.get("license")
        if not isinstance(grant, str):
            grant = "Upstream grants retained in bundled licence files"
        notices = [p for p in metadata.parent.iterdir() if p.is_file() and re.match(r"^(licen[cs]e|copying|notice|copyright)([._-].*)?$", p.name, re.I)]
        for child in (metadata.parent / "licenses", metadata.parent / "vendor"):
            if child.is_dir():
                notices.extend(p for p in child.rglob("*") if p.is_file() and re.match(r"^(licen[cs]e|copying|notice|copyright)([._-].*)?$", p.name, re.I))
        shared = distribution_notice(metadata, package) if not notices else None
        if shared:
            grant = 'MIT'
            add(shared, 'npm:' + name, version, grant, 'Exact opencode-ai@1.18.34 optionalDependencies binary sibling; installed distribution LICENSE; registry tarball SHA512 verified', 'DISTRIBUTION-LICENSE')
        fallback = find_fallback(name, version, grant)
        if (not notices and not shared) or (fallback and fallback.get('supplemental')):
            if fallback is None:
                raise ValueError("installed npm component lacks packaged grant: " + name + "@" + version)
            for item in fallback['files']:
                raw = base64.b64decode(item['data'], validate=True)
                if len(raw) != item['bytes'] or hashlib.sha256(raw).hexdigest() != item['sha256']:
                    raise ValueError('source-controlled fallback grant hash mismatch')
                add_body(raw, 'npm:' + name, version, grant, 'Exact installed npm metadata; retained authoritative source attribution ' + item['url'], item['file'])
        for index, notice in enumerate(sorted(set(notices))):
            add(notice, "npm:" + name, version, grant, "Installed npm " + name + "@" + version + "; " + str(notice), str(index) + "-" + notice.name)
    go_version = Path("/usr/local/go/VERSION").read_text(encoding="utf-8").splitlines()[0]
    add("/usr/local/go/LICENSE", "go-toolchain", go_version, "BSD-3-Clause", "Go distribution /usr/local/go/LICENSE")
    for optional in ("PATENTS", "NOTICE"):
        path = Path("/usr/local/go") / optional
        if path.is_file() and path.stat().st_size:
            add(path, "go-toolchain", go_version, "Go distribution supplemental notice", str(path))
    return sorted(rows, key=lambda x: x["path"]), bodies


def main():
    rows, bodies = collect()
    bodies["runtime-license-inventory.json"] = (json.dumps(rows, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
    with tarfile.open(fileobj=sys.stdout.buffer, mode="w|", format=tarfile.USTAR_FORMAT) as output:
        for name in sorted(bodies):
            raw = bodies[name]
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.mtime = len(raw), 0o644, 0
            output.addfile(info, io.BytesIO(raw))


if __name__ == "__main__":
    main()
