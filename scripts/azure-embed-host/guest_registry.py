#!/usr/bin/env python3
"""Immutable source guest registry, prepared only on the owned image builder.

The runtime host serves public software bytes on literal loopback. It cannot
upload, delete, proxy or rebuild images. This restores source, never task state.
"""
import argparse
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import socket
import stat
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.parse
import urllib.request
import urllib.error

ROOT = Path('/opt/luci/guest-registry')
SOURCE = '9ed95e7365224025ee89365e12cf128e1f1bf965'
ARCHIVE_SHA = '61c9a2c0d5981a78482025b6b69728521fbc78506d68b223d4a2eb825de5ca3d'
LICENSE_SHA = 'cb5e8e7e5f4a3988e1063c142c60dc2df75605f4c46515e776e3aca6df976e14'
ORIGIN = 'http://127.0.0.1:5055'
REPOSITORY = 'luci/guest'
HEX = re.compile(r'[0-9a-f]{64}\Z')
MAX_TOTAL = 16 << 30
MAX_BLOB = 4 << 30
MEDIA = 'application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json'


def sha_file(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def bounded_file(path, maximum):
    path = Path(path)
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_size < 1 or info.st_size > maximum:
        raise ValueError('Registry file is not ordinary bounded source data')
    return path.read_bytes()


def configuration(root, readonly=True):
    # Explicitly omit auth, TLS, proxy and debug. Public software is readable
    # only on loopback; write admission is disabled after builder seeding.
    return ('version: 0.1\nlog:\n  level: error\nstorage:\n'
            '  filesystem:\n    rootdirectory: ' + json.dumps(str(Path(root) / 'data')) + '\n'
            '  delete:\n    enabled: false\n  maintenance:\n'
            '    uploadpurging:\n      enabled: false\n'
            '    readonly:\n      enabled: ' + ('true' if readonly else 'false') + '\n'
            'http:\n  addr: 127.0.0.1:5055\n  draintimeout: 5s\n').encode()


UNIT = '''[Unit]
Description=Immutable Luci guest source on runtime host loopback
After=network.target
Before=luci-azure-embed-host.service
[Service]
Type=simple
ExecStartPre=/usr/bin/python3 /opt/luci/azure-embed/package/bin/guest_registry.py verify --root /opt/luci/guest-registry
ExecStart=/opt/luci/guest-registry/bin/registry serve /opt/luci/guest-registry/config.yaml
Restart=on-failure
RestartSec=3
DynamicUser=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadOnlyPaths=/opt/luci/guest-registry
RestrictAddressFamilies=AF_INET AF_UNIX
IPAddressDeny=any
IPAddressAllow=localhost
Environment=OTEL_TRACES_EXPORTER=none
UMask=0077
[Install]
WantedBy=multi-user.target
'''


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        raise ValueError('Local guest registry redirects are forbidden')


def local_read(path, expected=None, size=None, maximum=MAX_BLOB):
    if not path.startswith('/v2/') or '?' in path or '#' in path:
        raise ValueError('Only the fixed local registry API is available')
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    request = urllib.request.Request(ORIGIN + path, headers={'Accept': MEDIA})
    measured, total, small = hashlib.sha256(), 0, bytearray()
    with client.open(request, timeout=15) as response:
        if response.status != 200: raise ValueError('Local registry source is unavailable')
        while chunk := response.read(1 << 20):
            total += len(chunk)
            if total > maximum: raise ValueError('Registry response exceeds its bound')
            measured.update(chunk)
            if maximum <= 1 << 20: small.extend(chunk)
    if expected is not None and measured.hexdigest() != expected or size is not None and total != size:
        raise ValueError('Registry response bytes differ from selected source')
    return bytes(small)


def image_graph(manifest_bytes, read_blob):
    manifest = json.loads(manifest_bytes)
    if manifest.get('schemaVersion') != 2 or manifest.get('mediaType') not in MEDIA.split(', '):
        raise ValueError('Only an exact single-platform OCI/Docker manifest is allowed')
    rows = [manifest.get('config', {}), *manifest.get('layers', [])]
    if not 1 <= len(rows) <= 256: raise ValueError('Guest OCI graph exceeds its bound')
    inventory, total = {}, 0
    for row in rows:
        checksum = row.get('digest', '').removeprefix('sha256:')
        size = row.get('size')
        if not HEX.fullmatch(checksum) or isinstance(size, bool) or not isinstance(size, int) or not 0 < size <= MAX_BLOB:
            raise ValueError('Guest OCI blob identity is invalid')
        total += size
        if total > MAX_TOTAL: raise ValueError('Guest OCI bytes exceed the image bound')
        read_blob(checksum, size, row is rows[0])
        inventory[checksum] = size
    return inventory


def verify(root=ROOT, source_commit=None, served=False):
    root = Path(root)
    if root.is_symlink(): raise ValueError('Registry root must be an owned ordinary directory')
    receipt = json.loads(bounded_file(root / 'receipt.json', 128 << 10))
    checksum = receipt.get('imageDigest', '')
    if (receipt.get('version') != 1 or receipt.get('state') != 'immutable_guest_source_prepared' or
            receipt.get('sourceCommit') != (source_commit or receipt.get('sourceCommit')) or
            not re.fullmatch(r'[0-9a-f]{40}', receipt.get('sourceCommit', '')) or not HEX.fullmatch(checksum) or
            receipt.get('fromImage') != '127.0.0.1:5055/' + REPOSITORY + '@sha256:' + checksum or
            receipt.get('registrySourceCommit') != SOURCE or receipt.get('registryArchiveSHA256') != ARCHIVE_SHA or
            receipt.get('registryLicenseSHA256') != LICENSE_SHA or receipt.get('readOnlyRegistryObserved') is not True or
            receipt.get('microVMExecutionVerified') is not False or receipt.get('taskStateRestored') is not False):
        raise ValueError('Registry source/artifact receipt changed')
    versions = receipt.get('harnessVersions', {})
    if any(version not in versions.get(name, '') for name, version in (('codex', '0.160.0'), ('claude', '2.1.274'), ('opencode', '1.18.34'))) or not versions.get('sandboxd'):
        raise ValueError('Actual guest harness version receipt is incomplete')
    if os.name != 'nt' and root.stat().st_mode & 0o222 or any((root / name).stat().st_mode & 0o222 for name in ('receipt.json', 'config.yaml', 'bin/registry', 'LICENSE')):
        raise ValueError('Runtime registry source must remain read-only')
    if bounded_file(root / 'config.yaml', 4096) != configuration(root):
        raise ValueError('Registry must retain exact loopback read-only configuration')
    if sha_file(root / 'bin/registry') != receipt.get('registryBinarySHA256') or sha_file(root / 'LICENSE') != LICENSE_SHA:
        raise ValueError('Registry executable or original licence changed')
    inventory = receipt.get('files', {})
    actual, total = {}, 0
    for index, path in enumerate((root / 'data').rglob('*')):
        if index >= 16384: raise ValueError('Registry path inventory exceeds its bound')
        if path.is_symlink(): raise ValueError('Registry data link is forbidden')
        if (os.name != 'nt' or path.is_file()) and path.stat().st_mode & 0o222: raise ValueError('Runtime registry data must remain read-only')
        if path.is_file():
            info = path.stat()
            if not 0 < info.st_size <= MAX_BLOB: raise ValueError('Registry data file exceeds its bound')
            total += info.st_size
            if len(actual) >= 8192 or total > MAX_TOTAL: raise ValueError('Registry inventory exceeds its bound')
            actual[path.relative_to(root / 'data').as_posix()] = {'sha256': sha_file(path), 'size': info.st_size}
    if actual != inventory or not actual: raise ValueError('Registry source data is missing, substituted or changed')
    def blob_path(digest):
        return root / 'data/docker/registry/v2/blobs/sha256' / digest[:2] / digest / 'data'
    manifest_bytes = bounded_file(blob_path(checksum), 1 << 20)
    if hashlib.sha256(manifest_bytes).hexdigest() != checksum: raise ValueError('Guest manifest digest changed')
    def read_blob(digest, size, is_config):
        path = blob_path(digest)
        if path.is_symlink() or path.stat().st_size != size or sha_file(path) != digest:
            raise ValueError('Guest config/layer bytes changed')
        if is_config:
            config = json.loads(bounded_file(path, 1 << 20))
            if config.get('architecture') != 'amd64' or config.get('os') != 'linux':
                raise ValueError('Guest source is not Linux amd64')
        if served: local_read('/v2/' + REPOSITORY + '/blobs/sha256:' + digest, digest, size)
    blobs = image_graph(manifest_bytes, read_blob)
    if receipt.get('blobs') != blobs: raise ValueError('Guest manifest graph changed')
    if served:
        if bounded_file('/etc/systemd/system/luci-azure-guest-registry.service', 8192) != UNIT.encode():
            raise ValueError('Registry service ownership/configuration changed')
        local_read('/v2/', maximum=1 << 20)
        local_read('/v2/' + REPOSITORY + '/manifests/sha256:' + checksum, checksum, len(manifest_bytes), 1 << 20)
    return receipt


def readonly_probe(checksum=None):
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    if checksum is not None:
        request = urllib.request.Request(ORIGIN + '/v2/' + REPOSITORY + '/manifests/sha256:' + checksum, method='HEAD', headers={'Accept': MEDIA})
        with client.open(request, timeout=5) as response:
            if response.status != 200 or response.headers.get('Docker-Content-Digest') != 'sha256:' + checksum:
                raise ValueError('Registry HEAD does not acknowledge the immutable manifest')
    for method, path in (('POST', '/blobs/uploads/'), ('PATCH', '/blobs/uploads/unknown'),
                         ('PUT', '/manifests/forbidden'), ('DELETE', '/manifests/sha256:' + '0' * 64)):
        request = urllib.request.Request(ORIGIN + '/v2/' + REPOSITORY + path, method=method, data=b'')
        try:
            with client.open(request, timeout=5):
                raise ValueError('Read-only registry accepted a write')
        except urllib.error.HTTPError as error:
            if error.code != 405: raise ValueError('Registry write refusal was not exact read-only mode') from None


def service_identity():
    value = subprocess.run(['systemctl', 'show', 'luci-azure-guest-registry.service', '--property=MainPID', '--value'],
                           check=True, capture_output=True, text=True, timeout=10).stdout.strip()
    if not re.fullmatch(r'[1-9][0-9]{0,9}', value): raise ValueError('Owned registry service is not running')
    process = Path('/proc') / value
    if (process / 'exe').resolve() != ROOT / 'bin/registry': raise ValueError('Registry service executable changed')
    if (process / 'cmdline').read_bytes().split(b'\0')[:-1] != [str(ROOT / 'bin/registry').encode(), b'serve', str(ROOT / 'config.yaml').encode()]:
        raise ValueError('Registry service command changed')
    listeners = []
    for table in ('/proc/net/tcp', '/proc/net/tcp6'):
        for row in Path(table).read_text().splitlines()[1:]:
            fields = row.split()
            if fields[3] == '0A' and fields[1].split(':')[-1] == '13BF': listeners.append((fields[1], fields[9]))
    owned = {path.readlink().as_posix() for path in (process / 'fd').iterdir() if path.is_symlink()}
    if len(listeners) != 1 or listeners[0][0] != '0100007F:13BF' or 'socket:[' + listeners[0][1] + ']' not in owned:
        raise ValueError('Registry must own only the literal loopback listener')


def assert_port_free():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as listener:
        listener.bind(('127.0.0.1', 5055))


class PublicRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, newurl):
        parsed = urllib.parse.urlsplit(newurl)
        if parsed.scheme != 'https' or parsed.hostname not in ('github.com', 'release-assets.githubusercontent.com', 'objects.githubusercontent.com'):
            raise ValueError('Registry artifact redirect escaped its official origin')
        return super().redirect_request(request, fp, code, message, headers, newurl)


def public_download(url, checksum, target, limit):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != 'https' or parsed.hostname not in ('github.com', 'raw.githubusercontent.com'):
        raise ValueError('Only pinned official registry source is downloadable')
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), PublicRedirect())
    with client.open(url, timeout=30) as response, Path(target).open('xb') as output:
        total = 0
        while chunk := response.read(1 << 20):
            total += len(chunk)
            if total > limit: raise ValueError('Official registry artifact exceeds its bound')
            output.write(chunk)
    if sha_file(target) != checksum: raise ValueError('Official registry artifact checksum changed')


def install_artifact(root, fetch=public_download):
    root = Path(root)
    (root / 'bin').mkdir(parents=True)
    with tempfile.TemporaryDirectory() as directory:
        archive = Path(directory) / 'registry.tar.gz'
        fetch('https://github.com/distribution/distribution/releases/download/v3.0.0/registry_3.0.0_linux_amd64.tar.gz', ARCHIVE_SHA, archive, 32 << 20)
        with tarfile.open(archive, 'r:gz') as source:
            found, expanded = False, 0
            for index, entry in enumerate(source):
                expanded += entry.size
                if index >= 16 or not entry.isfile() or entry.size > 128 << 20 or expanded > 160 << 20 or Path(entry.name).is_absolute() or '..' in Path(entry.name).parts:
                    raise ValueError('Official registry archive has an unsafe inventory')
                if entry.name != 'registry': continue
                if found or entry.size < 64: raise ValueError('Official registry executable is missing or duplicated')
                found = True
                data = source.extractfile(entry).read((128 << 20) + 1)
                if data[:6] != b'\x7fELF\x02\x01' or data[18:20] != b'\x3e\x00':
                    raise ValueError('Registry executable is not Linux amd64 ELF')
                (root / 'bin/registry').write_bytes(data)
                (root / 'bin/registry').chmod(0o755)
            if not found: raise ValueError('Official registry executable is missing')
    fetch('https://raw.githubusercontent.com/distribution/distribution/' + SOURCE + '/LICENSE', LICENSE_SHA, root / 'LICENSE', 128 << 10)


def stop_process(process):
    if process.poll() is None:
        process.terminate()
        try: process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill(); process.wait(timeout=5)


def owned_version_probe(command, image_id, source_commit, operation, name, root):
    container = 'luci-guest-probe-' + operation + '-' + name
    journal = Path(root) / 'probe-pending.json'
    if journal.exists(): raise ValueError('Owned guest probe outcome is retained; no replay')
    if command(['docker', 'container', 'ls', '--all', '--filter', 'name=^/' + container + '$', '--format', '{{.ID}}']):
        raise ValueError('Guest probe name is already present; no adoption')
    # The outer installation journal is already durable. Retain the exact local
    # child identity too, so a lost create/start acknowledgment cannot be hidden.
    journal.write_text(json.dumps({'container': container, 'image': image_id, 'sourceCommit': source_commit, 'operation': operation}))
    try:
        command(['docker', 'create', '--name', container, '--network', 'none', '--memory', '536870912', '--memory-swap', '536870912', '--cpus', '1',
            '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges', '--label', 'luci.azure-guest-source=' + source_commit,
            '--label', 'luci.azure-guest-probe=' + operation, '--entrypoint', name, image_id, '--version'])
        rows = json.loads(command(['docker', 'container', 'inspect', container]))
        if len(rows) != 1 or rows[0].get('Image') != image_id or rows[0].get('Config', {}).get('Labels', {}).get('luci.azure-guest-probe') != operation:
            raise ValueError('Guest probe container ownership differs')
        return command(['docker', 'start', '--attach', container], timeout=60)[:4096]
    finally:
        rows = json.loads(command(['docker', 'container', 'inspect', container]))
        if (len(rows) != 1 or rows[0].get('Image') != image_id or
                rows[0].get('Config', {}).get('Labels', {}).get('luci.azure-guest-source') != source_commit or
                rows[0].get('Config', {}).get('Labels', {}).get('luci.azure-guest-probe') != operation):
            raise ValueError('Probe cleanup is uncertain or foreign; image capture remains blocked')
        command(['docker', 'container', 'rm', '--force', rows[0]['Id']])
        if command(['docker', 'container', 'ls', '--all', '--filter', 'name=^/' + container + '$', '--format', '{{.ID}}']):
            raise ValueError('Owned probe removal remains uncertain')
        journal.unlink()


def prepare(luci, source_commit, command, root=ROOT):
    """Existing temporary runtime-builder operation; never called at host boot."""
    luci, root = Path(luci), Path(root)
    if root.exists(): raise ValueError('Registry source preparation already retained; inspect without rebuild')
    assert_port_free()
    root.mkdir(mode=0o755, parents=True)
    install_artifact(root)
    if 'v3.0.0' not in command([str(root / 'bin/registry'), '--version'], timeout=10):
        raise ValueError('Registry executable version differs')
    dockerfile = bounded_file(luci / 'images/microvm/Dockerfile', 128 << 10)
    pins, prepared = {}, []
    for line in dockerfile.decode().splitlines():
        match = re.fullmatch(r'FROM ([^ ]+)( AS [a-zA-Z0-9_-]+)?', line)
        if match:
            image = match[1]
            checksum = command(['skopeo', 'inspect', '--override-os', 'linux', '--override-arch', 'amd64', '--format', '{{.Digest}}', 'docker://' + image], timeout=120)
            if not re.fullmatch(r'sha256:[0-9a-f]{64}', checksum): raise ValueError('Guest base image digest unavailable')
            pins[image] = image.split('@')[0] + '@' + checksum
            line = 'FROM ' + pins[image] + (match[2] or '')
        prepared.append(line)
    if len(pins) != 2: raise ValueError('Pinned guest Dockerfile base inventory changed')
    # Inject original application/source notices without changing guest logic.
    # The final source is still measured separately from the reviewed Dockerfile.
    prepared.insert(-1, 'COPY LICENSE NOTICE /usr/local/share/luci/source-notices/')
    prepared.insert(-1, 'COPY sandboxd/vendor /usr/local/share/luci/sandboxd-vendor/')
    prepared.insert(-1, 'COPY third-party/licenses /usr/local/share/luci/upstream-grants/')
    prepared.insert(-1, 'COPY --from=guest-build /usr/local/go/LICENSE /usr/local/go/VERSION /usr/local/go/')
    old_path, old_bytecode = list(sys.path), sys.dont_write_bytecode
    try:
        sys.path.insert(0, str(luci / 'scripts/runtime-oci')); sys.dont_write_bytecode = True
        spec = importlib.util.spec_from_file_location('luci_guest_grants', luci / 'scripts/runtime-oci/grant_fallbacks.py')
        fallback = importlib.util.module_from_spec(spec); spec.loader.exec_module(fallback)
        grants = fallback.load(luci)
    finally:
        sys.path[:] = old_path; sys.dont_write_bytecode = old_bytecode
    collector = ('SOURCE_GRANT_FALLBACKS = ' + repr(grants) + '\n').encode() + bounded_file(luci / 'scripts/runtime-oci/image_licenses.py', 128 << 10)
    encoded = base64.b64encode(collector).decode()
    prepared.insert(-1, 'RUN python3 -c "import base64; exec(compile(base64.b64decode(\'' + encoded + '\'),\'source-license-inventory\',\'exec\'))" > /usr/local/share/luci/installed-grants.tar')
    generated = root / 'guest.Dockerfile'
    generated.write_text('\n'.join(prepared) + '\n')
    operation = hashlib.sha256((source_commit + str(root)).encode()).hexdigest()[:24]
    image = 'luci-azure-guest:' + operation
    command(['docker', 'build', '--platform', 'linux/amd64', '--label', 'luci.azure-guest-source=' + source_commit,
             '-f', str(generated), '-t', image, str(luci)], timeout=7200)
    metadata = json.loads(command(['docker', 'image', 'inspect', image]))
    if len(metadata) != 1 or metadata[0].get('Architecture') != 'amd64' or metadata[0].get('Os') != 'linux' or metadata[0].get('Config', {}).get('Labels', {}).get('luci.azure-guest-source') != source_commit:
        raise ValueError('Owned guest image identity/architecture differs')
    image_id = metadata[0]['Id']
    versions = {}
    for name in ('sandboxd', 'codex', 'claude', 'opencode'):
        # No task, project, network or model credential is supplied to probes.
        versions[name] = owned_version_probe(command, image_id, source_commit, operation, name, root)
    for name, version in (('codex', '0.160.0'), ('claude', '2.1.274'), ('opencode', '1.18.34')):
        if version not in versions[name]: raise ValueError('Installed guest harness version differs')
    (root / 'data').mkdir()
    writable = root / 'seed.yaml'
    writable.write_bytes(configuration(root, False))
    environment = os.environ.copy(); environment['OTEL_TRACES_EXPORTER'] = 'none'
    with tempfile.TemporaryFile() as output:
        process = subprocess.Popen([str(root / 'bin/registry'), 'serve', str(writable)], stdout=output, stderr=output, env=environment)
        try:
            for _ in range(50):
                if process.poll() is not None: raise ValueError('Owned registry seed process failed')
                try: local_read('/v2/', maximum=1 << 20); break
                except OSError: time.sleep(0.1)
            else: raise ValueError('Owned registry seed process is unavailable')
            command(['skopeo', 'copy', '--dest-tls-verify=false', 'docker-daemon:' + image, 'docker://127.0.0.1:5055/' + REPOSITORY + ':source'], timeout=1800)
            manifest_bytes = local_read('/v2/' + REPOSITORY + '/manifests/source', maximum=1 << 20)
            checksum = hashlib.sha256(manifest_bytes).hexdigest()
            blobs = image_graph(manifest_bytes, lambda value, size, _: local_read('/v2/' + REPOSITORY + '/blobs/sha256:' + value, value, size))
        finally: stop_process(process)
    writable.unlink()
    (root / 'config.yaml').write_bytes(configuration(root))
    with tempfile.TemporaryFile() as output:
        process = subprocess.Popen([str(root / 'bin/registry'), 'serve', str(root / 'config.yaml')], stdout=output, stderr=output, env=environment)
        try:
            for _ in range(50):
                if process.poll() is not None: raise ValueError('Read-only registry validation process failed')
                try: local_read('/v2/', maximum=1 << 20); break
                except OSError: time.sleep(0.1)
            else: raise ValueError('Read-only registry validation process unavailable')
            local_read('/v2/' + REPOSITORY + '/manifests/sha256:' + checksum, checksum, len(manifest_bytes), 1 << 20)
            readonly_probe(checksum)
        finally: stop_process(process)
    files, total = {}, 0
    for index, path in enumerate((root / 'data').rglob('*')):
        if index >= 16384: raise ValueError('Registry path inventory exceeds its bound')
        if path.is_symlink(): raise ValueError('Registry seed created a source link')
        if path.is_file():
            size = path.stat().st_size; total += size
            if not 0 < size <= MAX_BLOB or total > MAX_TOTAL or len(files) >= 8192: raise ValueError('Registry source inventory exceeds its bound')
            files[path.relative_to(root / 'data').as_posix()] = {'sha256': sha_file(path), 'size': size}
    receipt = {'version': 1, 'state': 'immutable_guest_source_prepared', 'sourceCommit': source_commit,
        'imageDigest': checksum, 'fromImage': '127.0.0.1:5055/' + REPOSITORY + '@sha256:' + checksum,
        'registrySourceCommit': SOURCE, 'registryArchiveSHA256': ARCHIVE_SHA, 'registryLicenseSHA256': LICENSE_SHA,
        'registryBinarySHA256': sha_file(root / 'bin/registry'), 'dockerfileSHA256': hashlib.sha256(dockerfile).hexdigest(),
        'preparedDockerfileSHA256': sha_file(generated), 'baseImages': pins, 'harnessVersions': versions,
        'blobs': blobs, 'files': files, 'readOnlyRegistryObserved': True,
        'microVMExecutionVerified': False, 'taskStateRestored': False}
    (root / 'receipt.json').write_text(json.dumps(receipt, sort_keys=True, separators=(',', ':')))
    if (root / 'receipt.json').stat().st_size > 128 << 10: raise ValueError('Registry source receipt exceeds its bound')
    for path in root.rglob('*'):
        path.chmod(0o555 if path.is_dir() or path == root / 'bin/registry' else 0o444)
    root.chmod(0o555)
    verify(root, source_commit)
    units = Path('/etc/systemd/system')
    target = units / 'luci-azure-guest-registry.service'
    if target.exists(): raise ValueError('Registry unit already exists; preserve its owner')
    target.write_text(UNIT)
    command(['docker', 'image', 'rm', image])
    return receipt


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=('verify', 'ready'))
    parser.add_argument('--root', type=Path, default=ROOT)
    parser.add_argument('--source-commit')
    args = parser.parse_args()
    if args.action == 'ready':
        service_identity()
        spec = importlib.util.spec_from_file_location('luci_registry_host', Path(__file__).with_name('service.py'))
        host = importlib.util.module_from_spec(spec); spec.loader.exec_module(host)
        manifest = host.pinned_package(host.config(host.HOST_CONFIG))
        if 'guest-source-receipt.json' not in manifest['files']:
            raise ValueError('Host package has no source-bound registry receipt')
        if bounded_file(args.root / 'receipt.json', 128 << 10) != bounded_file(host.PACKAGE / 'guest-source-receipt.json', 128 << 10):
            raise ValueError('Registry source receipt differs from immutable host package')
    print(json.dumps(verify(args.root, args.source_commit, args.action == 'ready'), sort_keys=True))


if __name__ == '__main__': main()
