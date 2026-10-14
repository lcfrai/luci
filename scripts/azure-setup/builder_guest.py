#!/usr/bin/env python3
"""Explicit temporary image-builder guest operations; no task execution.

The source archive arrives over a private AAD Blob read. Source and licence
inputs are immutable; account/generation configuration is installed later.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import http.client
import io
import json
import os
from pathlib import Path
import re
import shutil
import ssl
import subprocess
import tarfile
import tempfile
import time
import urllib.parse
import urllib.request

RUNTIME = 'cf536c284434fdd2f09bbab35ec807a5dacc87e4'
ROOT = Path('/var/lib/luci-image-builder')
HEX = re.compile(r'[0-9a-f]{64}\Z')
REV = re.compile(r'[0-9a-f]{40}\Z')


def sha_file(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def save(path, value):
    path = Path(path)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(dir=path.parent)
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, 'wb') as output:
        output.write(json.dumps(value, sort_keys=True, separators=(',', ':')).encode())
        output.flush(); os.fsync(output.fileno())
    os.replace(temporary, path)


def command(arguments, timeout=1800, cwd=None):
    # Build diagnostics can contain registry authentication material. Keep them
    # off ARM stdout and bound private disk output while the process runs.
    with tempfile.TemporaryFile() as output:
        process = subprocess.Popen(arguments, cwd=cwd, stdout=output, stderr=output, stdin=subprocess.DEVNULL)
        try:
            deadline = time.monotonic() + timeout
            while process.poll() is None:
                if time.monotonic() >= deadline or output.seek(0, 2) > 8 << 20:
                    raise RuntimeError('Image preparation command timed out or exceeded its output bound')
                time.sleep(0.1)
            if process.returncode or output.seek(0, 2) > 8 << 20:
                raise RuntimeError('Image preparation command failed or exceeded its output bound')
            output.seek(0)
            return output.read().decode().strip()
        finally:
            if process.poll() is None: process.kill()
            process.wait()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args): raise ValueError('Private source redirect refused')


def download(url, expected, target):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != 'https' or not re.fullmatch(r'[a-z0-9]{3,24}\.blob\.core\.windows\.net', parsed.netloc) or parsed.query or parsed.fragment or not HEX.fullmatch(expected):
        raise ValueError('Exact private AAD source blob and digest required')
    import ipaddress, socket
    addresses = {row[4][0] for row in socket.getaddrinfo(parsed.hostname, 443, type=socket.SOCK_STREAM)}
    networks = [ipaddress.ip_network(v) for v in ('10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16')]
    if not addresses or any(not any(ipaddress.ip_address(v) in n for n in networks) for v in addresses):
        raise ValueError('Builder source route must be private')
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    request = urllib.request.Request('http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https%3A%2F%2Fstorage.azure.com%2F', headers={'Metadata': 'true'})
    with client.open(request, timeout=5) as response:
        raw = response.read(16385)
    if len(raw) > 16384: raise ValueError('Builder identity response exceeded bound')
    token = json.loads(raw)['access_token']
    class PrivateConnection(http.client.HTTPSConnection):
        def connect(self):
            if self.host != parsed.hostname: raise ValueError('Private storage destination changed')
            candidates = {row[4][0] for row in socket.getaddrinfo(self.host, 443, type=socket.SOCK_STREAM)}
            if not candidates or any(not any(ipaddress.ip_address(v) in n for n in networks) for v in candidates):
                raise ValueError('Private storage DNS changed')
            self.sock = self._context.wrap_socket(socket.create_connection((sorted(candidates)[0], 443), timeout=self.timeout), server_hostname=self.host)
    class PrivateHandler(urllib.request.HTTPSHandler):
        def https_open(self, request): return self.do_open(PrivateConnection, request, context=ssl.create_default_context())
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect(), PrivateHandler())
    request = urllib.request.Request(url, headers={'Authorization': 'Bearer ' + token, 'x-ms-version': '2023-11-03'})
    with client.open(request, timeout=30) as response, Path(target).open('xb') as output:
        total = 0
        while chunk := response.read(1 << 20):
            total += len(chunk)
            if total > 256 << 20: raise ValueError('Source payload exceeds builder bounds')
            output.write(chunk)
    Path(target).chmod(0o600)
    if sha_file(target) != expected: raise ValueError('Private source bytes changed')


def extract(path, target):
    target.mkdir(mode=0o700)
    with tarfile.open(path) as archive:
        entries = archive.getmembers()
        if len(entries) > 65536 or sum(row.size for row in entries) > 512 << 20:
            raise ValueError('Source archive exceeds bounded inventory')
        for row in entries:
            name = Path(row.name)
            if name.is_absolute() or '..' in name.parts or not (row.isfile() or row.isdir() or row.issym()) or row.size > 128 << 20:
                raise ValueError('Source archive entry unsafe')
            if row.issym() and (Path(row.linkname).is_absolute() or '..' in Path(row.linkname).parts):
                raise ValueError('Source link escapes its checkout')
        archive.extractall(target, filter='data')


def checkout(archive, commit_bytes, revision, target):
    if not REV.fullmatch(revision) or hashlib.sha1(b'commit ' + str(len(commit_bytes)).encode() + b'\0' + commit_bytes).hexdigest() != revision:
        raise ValueError('Source commit bytes differ')
    extract(archive, target)
    command(['git', 'init', '-q'], cwd=target)
    commit = target / '.git/source-commit'
    commit.write_bytes(commit_bytes)
    observed = command(['git', 'hash-object', '-t', 'commit', '-w', str(commit)], cwd=target)
    command(['git', 'add', '-f', '-A'], cwd=target)
    tree = command(['git', 'write-tree'], cwd=target)
    expected_tree = commit_bytes.splitlines()[0].decode().removeprefix('tree ')
    if observed != revision or tree != expected_tree:
        raise ValueError('Source checkout tree differs from immutable commit')
    command(['git', 'update-ref', 'refs/heads/image-source', revision], cwd=target)
    command(['git', 'symbolic-ref', 'HEAD', 'refs/heads/image-source'], cwd=target)
    if command(['git', 'status', '--porcelain'], cwd=target): raise ValueError('Image source checkout is dirty')


def prepare(config, provider_download=download):
    if config.get('version') != 1 or config.get('role') not in ('runtime', 'controller') or not REV.fullmatch(config.get('sourceCommit', '')) or not HEX.fullmatch(config.get('payloadSHA256', '')):
        raise ValueError('Exact role/source builder selection required')
    state = ROOT / 'state.json'
    if state.exists(): raise ValueError('Image installation intent is retained; inspect without replay')
    ROOT.mkdir(mode=0o700, parents=True, exist_ok=True)
    save(state, {'version': 1, 'state': 'installation_pending', 'sourceCommit': config['sourceCommit'], 'role': config['role'], 'payloadSHA256': config['payloadSHA256']})
    payload = ROOT / 'payload.tar'
    provider_download(config['payloadURL'], config['payloadSHA256'], payload)
    source = ROOT / 'payload'
    extract(payload, source)
    manifest = json.loads((source / 'manifest.json').read_bytes())
    wanted = {'luci.tar', 'luci.commit', 'runtime.tar', 'runtime.commit', 'envoy', 'envoy-license'}
    if set(manifest.get('files', {})) != wanted or manifest.get('sourceCommit') != config['sourceCommit'] or manifest.get('runtimeRevision') != RUNTIME:
        raise ValueError('Role payload source/licence inventory changed')
    for name, binding in manifest['files'].items():
        path = source / name
        if not path.is_file() or path.is_symlink() or path.stat().st_size != binding['size'] or sha_file(path) != binding['sha256']:
            raise ValueError('Source payload bytes changed')
    luci, upstream = ROOT / 'luci', ROOT / 'runtime'
    command(['apt-get', 'update'])
    command(['apt-get', 'install', '-y', 'git', 'golang-go', 'python3-yaml', 'docker.io', 'docker-compose-v2', 'skopeo', 'openssl', 'ca-certificates'])
    checkout(source / 'luci.tar', (source / 'luci.commit').read_bytes(), config['sourceCommit'], luci)
    checkout(source / 'runtime.tar', (source / 'runtime.commit').read_bytes(), RUNTIME, upstream)
    environment = os.environ.copy()
    os.environ.update(GOTOOLCHAIN='go1.25.0', GOMAXPROCS='2', GOMEMLIMIT='2GiB')
    try:
        binaries = {'luci-api': './control-plane/cmd/api'} if config['role'] == 'controller' else {
            'azure-embed-host-bootstrap': './control-plane/cmd/azure-embed-host-bootstrap', 'azure-gateway-config': './control-plane/cmd/azure-gateway-config'}
        for name, package in binaries.items():
            command(['go', 'build', '-trimpath', '-mod=readonly', '-p', '1', '-o', str(ROOT / name), package], cwd=luci)
    finally:
        os.environ.clear(); os.environ.update(environment)
    package = ROOT / 'role.tar'
    if config['role'] == 'controller':
        script = luci / 'scripts/azure-controller-disk/package.py'
        digest = command(['python3', str(script), 'build', '--source', str(luci), '--revision', config['sourceCommit'], '--api', str(ROOT / 'luci-api'), '--output', str(package)])
        command(['python3', str(script), 'install-image', '--archive', str(package), '--sha256', digest, '--root', '/', '--mount', '/mnt/luci-controller'])
    else:
        compose = json.loads(command(['docker', 'compose', '-f', 'compose.yaml', '--profile', '*', 'config', '--format', 'json'], cwd=upstream / 'embed/compose'))
        pins = {}
        for name, service in compose['services'].items():
            image = service['image']
            digest = command(['skopeo', 'inspect', '--format', '{{.Digest}}', 'docker://' + image], timeout=120)
            if not re.fullmatch(r'sha256:[0-9a-f]{64}', digest): raise ValueError('Immutable runtime image digest unavailable')
            pins[name] = image.split('@')[0] + '@' + digest
        save(ROOT / 'image-digests.json', pins)
        script = luci / 'scripts/azure-embed-host/package.py'
        digest = command(['python3', str(script), 'build', '--upstream', str(upstream), '--luci', str(luci), '--luci-revision', config['sourceCommit'],
            '--image-digests', str(ROOT / 'image-digests.json'), '--bootstrap', str(ROOT / 'azure-embed-host-bootstrap'), '--gateway', str(ROOT / 'azure-gateway-config'),
            '--envoy', str(source / 'envoy'), '--envoy-license', str(source / 'envoy-license'), '--output', str(package)])
        command(['python3', str(script), 'install-image', '--archive', str(package), '--sha256', digest, '--root', '/'])
        command(['docker', 'compose', 'version'])
    if not HEX.fullmatch(digest) or sha_file(package) != digest: raise ValueError('Installed package differs from measured source')
    licences = Path('/opt/luci/licences')
    licences.mkdir(parents=True, exist_ok=True)
    for name, origin in (('LUCI-LICENSE', luci / 'LICENSE'), ('LUCI-NOTICE', luci / 'NOTICE'), ('E2B-LICENSE', upstream / 'LICENSE'), ('ENVOY-LICENSE', source / 'envoy-license')):
        if not origin.is_file() or origin.is_symlink(): raise ValueError('Original licence/NOTICE unavailable')
        shutil.copyfile(origin, licences / name)
    if any(Path(path).exists() for path in ('/etc/luci/operator-template.json', '/etc/luci/azure-controller.env', '/etc/luci/azure-private-ca.crt', '/etc/luci/operator.json', '/var/lib/luci/azure-embed/host-state.json')):
        raise ValueError('Generic image contains active controller/runtime configuration')
    receipt = {'version': 1, 'state': 'role_prepared_guest_unverified', 'sourceCommit': config['sourceCommit'], 'role': config['role'],
        'runtimeRevision': RUNTIME, 'payloadSHA256': config['payloadSHA256'], 'packageSHA256': digest, 'microVMExecutionVerified': False,
        'observedAtUTC': datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')}
    save('/opt/luci/image-source-receipt.json', receipt)
    save(state, receipt)
    return receipt


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=('prepare', 'inspect', 'deprovision'))
    parser.add_argument('selection', type=Path)
    args = parser.parse_args()
    config = json.loads(args.selection.read_bytes())
    if args.action == 'prepare': value = prepare(config)
    else:
        value = json.loads((ROOT / 'state.json').read_bytes())
        if value.get('state') != 'role_prepared_guest_unverified' or any(value.get(key) != config.get(key) for key in ('role', 'sourceCommit', 'payloadSHA256')):
            raise ValueError('Image preparation is incomplete or uncertain')
        if args.action == 'inspect' and (ROOT / 'deprovision.json').exists():
            value = json.loads((ROOT / 'deprovision.json').read_bytes())
        if args.action == 'deprovision':
            if (ROOT / 'deprovision.json').exists(): raise ValueError('Deprovision intent retained; never replay')
            save(ROOT / 'deprovision.json', {'state': 'deprovision_pending'})
            # Build sources, temporary credentials/cache and account state do not
            # belong in the image. Keep the generic source/licence receipt only.
            for name in ('luci', 'runtime', 'payload', 'payload.tar', 'role.tar', 'image-digests.json'):
                path = ROOT / name
                if path.is_dir(): shutil.rmtree(path)
                elif path.exists(): path.unlink()
            command(['waagent', '-deprovision+user', '-force'], timeout=120)
            value = dict(value, state='role_deprovisioned_capture_pending')
            save(ROOT / 'deprovision.json', value)
    print('LUCI_AZURE_BUILDER_RECEIPT=' + json.dumps(value, separators=(',', ':')))


if __name__ == '__main__':
    main()
