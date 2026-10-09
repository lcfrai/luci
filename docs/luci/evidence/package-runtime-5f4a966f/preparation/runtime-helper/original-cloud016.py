#!/usr/bin/env python3
"""Build a reversible, source-fenced sandbox candidate; never retire anything.

Run after sourcing activate.sh. `build` does not export/start a container.
`verify-export RECEIPT` is a separately budgeted, streaming image-file check.
Neither operation updates setup pins, tags the old image, prunes, or removes.
"""
import argparse
import contextlib
import datetime as dt
import gzip
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import signal
import stat
import subprocess
import sys
import tarfile
import threading
import time
import uuid

ROOT = Path('/workspace/luci')
TOOLS = Path('/workspace/.luci-tools')
DOCKER = '/usr/local/bin/docker'
GO = str(TOOLS / 'go1.26.3/bin/go')
BASE = 'sha256:9e0fd9f731408c4405c4b9e97308bf807c8088cf521a138c0b1921667a831351'
OLD = 'sha256:42597a985b20f2aee31a629a320980f02180bde307b6ffdd3ec60b257a82af5b'
ARCHIVE_SHA = 'c593f37eaa3795358aa1b76dd355828ddaadfcf503b9a5e3a1aea6976c947c91'
ENTRY_SHA = 'c3d925a1b08874079c44820cbc70e50fa370c4af26ecddfbb656687a04eebe3a'
GIB = 1 << 30
FLOOR = GIB
STOP_RESERVE = FLOOR + (256 << 20)
CHUNK = 1 << 20


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def write_json(path, value):
    temp = path.with_suffix(path.suffix + '.new')
    temp.write_text(json.dumps(value, indent=2) + '\n')
    temp.replace(path)


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def free(path):
    info = os.statvfs(path)
    return info.f_bavail * info.f_frsize


def docker_json(*args):
    return json.loads(subprocess.check_output([DOCKER, *args], text=True, timeout=60))


def inspect(identity):
    return docker_json('image', 'inspect', identity)[0]


def image_inventory():
    data = subprocess.check_output([DOCKER, 'image', 'ls', '--no-trunc', '--format', '{{json .}}'], text=True, timeout=60)
    return sorted((row['Repository'], row['Tag'], row['ID']) for row in map(json.loads, data.splitlines()) if row['Repository'] != '<none>')


def add_verified_base_tag(tag):
    require(not any(repo + ':' + name == tag for repo, name, _ in image_inventory()), 'Refusing to replace an existing base tag')
    require(inspect(BASE)['Id'] == BASE, 'Trusted base identity changed before tagging')
    argv = [DOCKER, 'image', 'tag', BASE, tag]
    subprocess.run(argv, check=True, timeout=30)
    require(inspect(tag)['Id'] == BASE, 'New local base tag does not name the trusted image')
    return argv


def source_hashes():
    result = {}
    for directory, folders, files in os.walk(ROOT / 'sandboxd', followlinks=False):
        for name in folders + files:
            require(not (Path(directory) / name).is_symlink(), 'Unexpected symlink in sandbox source')
        for name in sorted(files):
            path = Path(directory) / name
            require(path.is_file(), 'Unexpected non-regular source entry')
            result[str(path.relative_to(ROOT))] = digest(path)
    for name in ['images/sandbox/entrypoint.sh', 'images/sandbox/Dockerfile']:
        path = ROOT / name
        require(path.is_file() and not path.is_symlink(), 'Unexpected sandbox image source')
        result[name] = digest(path)
    return dict(sorted(result.items()))


def preserved_state():
    archive = TOOLS / 'luci-sandbox-dev.tar.gz'
    require(archive.is_file() and not archive.is_symlink(), 'Old recovery archive must be a retained regular file')
    require(digest(archive) == ARCHIVE_SHA, 'Old recovery archive checksum mismatch')
    return {'archiveSHA256': ARCHIVE_SHA, 'archiveBytes': archive.stat().st_size,
            'provenanceSHA256': digest(TOOLS / 'sandbox-image-provenance.json'),
            'installSHA256': digest(TOOLS / 'install.sh'), 'startupSHA256': digest(TOOLS / 'start.md')}


class Monitor:
    """Stop admission early; client cancellation does not certify daemon drain."""
    def __init__(self, proc, stage, operation, timeout, temp_reserve=256 << 20):
        self.proc, self.stage, self.operation, self.timeout = proc, stage, operation, timeout
        self.done = threading.Event()
        self.failure = None
        self.temp_reserve = temp_reserve
        self.thread = threading.Thread(target=self.watch, daemon=True)

    def __enter__(self):
        self.thread.start()
        return self

    def watch(self):
        try:
            self.watch_checked()
        except BaseException as error:
            # Losing the evidence sink must not silently disable the guard.
            self.failure = 'Disk monitor failed: ' + str(error)
            stop_client(self.proc)

    def watch_checked(self):
        began, interrupted = time.monotonic(), None
        with (self.stage / (self.operation + '-disk.jsonl')).open('w') as log:
            while not self.done.is_set():
                try:
                    available, temp_available = free('/workspace'), free('/tmp')
                except OSError:
                    available, temp_available = 0, 0
                elapsed = time.monotonic() - began
                log.write(json.dumps({'at': now(), 'elapsed': elapsed, 'workspaceFreeBytes': available,
                                      'tmpFreeBytes': temp_available}) + '\n')
                log.flush()
                if interrupted is None and (available < STOP_RESERVE or temp_available < self.temp_reserve or elapsed > self.timeout):
                    self.failure = ('Disk cancellation reserve reached' if available < STOP_RESERVE or temp_available < self.temp_reserve else 'Operation timed out')
                    interrupted = time.monotonic()
                    with contextlib.suppress(ProcessLookupError):
                        os.killpg(self.proc.pid, signal.SIGINT)
                elif interrupted is not None and time.monotonic() - interrupted > 8:
                    with contextlib.suppress(ProcessLookupError):
                        os.killpg(self.proc.pid, signal.SIGKILL)
                self.done.wait(0.25)

    def __exit__(self, *_):
        self.done.set()
        self.thread.join()


def run_monitored(argv, stage, operation, *, cwd=None, env=None, timeout=600):
    require(free('/workspace') >= STOP_RESERVE, 'Workspace has insufficient cancellation headroom')
    with (stage / (operation + '.log')).open('wb') as log:
        proc = subprocess.Popen(argv, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            with Monitor(proc, stage, operation, timeout) as monitor:
                code = proc.wait()
        except BaseException:
            stop_client(proc)
            raise
        require(monitor.failure is None, (monitor.failure or '') + '; client stopped, daemon work may need to drain before any other allocation')
        require(code == 0, f'{operation} exited {code}; see retained log')


def stop_client(proc):
    if proc.poll() is not None:
        return
    with contextlib.suppress(ProcessLookupError):
        os.killpg(proc.pid, signal.SIGINT)
    with contextlib.suppress(subprocess.TimeoutExpired):
        proc.wait(timeout=8)
    if proc.poll() is None:
        with contextlib.suppress(ProcessLookupError):
            os.killpg(proc.pid, signal.SIGKILL)
        proc.wait()


def build():
    os.umask(0o077)
    run_id = dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex[:8]
    stage = TOOLS / ('sandbox-candidate-' + run_id)
    stage.mkdir(mode=0o700)
    receipt_path = stage / 'receipt.json'
    receipt = {'version': 1, 'startedAt': now(), 'stage': str(stage), 'status': 'preflight',
               'scope': 'Source-fenced candidate and additive verified base tag; no container started; no old-image/cache removal; no setup-pin update',
               'tag': 'luci/sandbox:verified-candidate-' + run_id.lower(), 'baseImage': BASE,
               'baseTag': 'luci/sandbox:provider-base-' + run_id.lower(),
               'exportVerification': {'status': 'not-run', 'reason': 'Separately budgeted daemon export; use verify-export after headroom is established'},
               'runtimeVerification': 'not-run'}
    write_json(receipt_path, receipt)
    try:
        require(ROOT.is_dir(), 'Checkout missing')
        version = subprocess.check_output([GO, 'version'], text=True).strip()
        require('go1.26.3 linux/amd64' in version, 'Pinned Go version mismatch')
        require(docker_json('info', '--format', '{{json .Driver}}') == 'vfs', 'Unexpected Docker driver; re-evaluate the budget')
        require(not subprocess.check_output([DOCKER, 'ps', '-aq'], text=True).strip(), 'A container is present; coordinate exclusive Docker admission')
        base, old = inspect(BASE), inspect(OLD)
        require(base['Id'] == BASE and old['Id'] == OLD, 'Image identity mismatch')
        require(base['Architecture'] == 'amd64' and base['Os'] == 'linux', 'Unexpected base platform')
        require(base['Config'] == old['Config'], 'Base/runtime config drift; review before refresh')
        require(len(base['RootFS']['Layers']) == 1, 'Expected the reviewed flat base')
        receipt['preservedBefore'] = preserved_state()
        receipt['tagsBefore'] = image_inventory()
        require(not any(repo + ':' + tag == receipt['tag'] for repo, tag, _ in receipt['tagsBefore']), 'Candidate tag already exists')
        require(not any(repo + ':' + tag == receipt['baseTag'] for repo, tag, _ in receipt['tagsBefore']), 'Base tag already exists')
        receipt['baseConfig'] = base['Config']
        receipt['goVersion'] = version
        receipt['sourcesBefore'] = source_hashes()
        require(receipt['sourcesBefore']['images/sandbox/entrypoint.sh'] == ENTRY_SHA, 'Entrypoint changed from trusted runtime provenance')
        # Budget one full VFS COPY plus cancellation reserve and 256 MiB compile margin.
        # This is an admission estimate, not a guarantee against asynchronous daemon allocation.
        required = base['Size'] + STOP_RESERVE + (256 << 20)
        receipt['buildAdmission'] = {'workspaceFreeBytes': free('/workspace'), 'requiredBytes': required,
                                     'estimatedFullCopyBytes': base['Size'], 'hardFloorBytes': FLOOR,
                                     'cancelAtFreeBytes': STOP_RESERVE, 'sampleSeconds': 0.25}
        require(free('/workspace') >= required, 'Insufficient space for one full VFS COPY and cancellation headroom')
        snapshot = stage / 'source'
        shutil.copytree(ROOT / 'sandboxd', snapshot)
        require(source_hashes() == receipt['sourcesBefore'], 'Sources changed while copying; no Docker build performed')
        for name, expected in receipt['sourcesBefore'].items():
            if name.startswith('sandboxd/'):
                require(digest(snapshot / name.removeprefix('sandboxd/')) == expected, 'Source snapshot differs')
        context = stage / 'context'
        (context / 'bin').mkdir(parents=True, mode=0o700)
        (context / 'bin').chmod(0o755)
        entry = context / 'bin/entrypoint.sh'
        entry.write_bytes((ROOT / 'images/sandbox/entrypoint.sh').read_bytes())
        require(b'\r' not in entry.read_bytes() and digest(entry) == ENTRY_SHA, 'Entrypoint bytes are not the reviewed unchanged LF bytes')
        entry.chmod(0o755)
        temp = stage / 'go-tmp'
        temp.mkdir()
        environment = os.environ.copy()
        environment.update({'CGO_ENABLED': '0', 'GOWORK': 'off', 'GOFLAGS': '-mod=vendor', 'GOTOOLCHAIN': 'local',
                            'GOOS': 'linux', 'GOARCH': 'amd64', 'GOAMD64': 'v1',
                            'GOMAXPROCS': '2', 'TMPDIR': str(temp), 'GOTMPDIR': str(temp), 'GOPROXY': 'off'})
        # Vendor is checked against the original source hash set; no downloads.
        binary = context / 'bin/sandboxd'
        command = [GO, 'build', '-trimpath', '-buildvcs=false', '-p=2', '-o', str(binary), './cmd/sandboxd']
        run_monitored(command, stage, 'compile', cwd=snapshot, env=environment)
        binary.chmod(0o755)
        require(source_hashes() == receipt['sourcesBefore'], 'Sources changed during compile; no Docker build performed')
        receipt['compileCommand'] = command
        receipt['inputFiles'] = {p.name: {'sha256': digest(p), 'bytes': p.stat().st_size, 'mode': '0755', 'uid': 0, 'gid': 0} for p in [binary, entry]}
        dockerfile = context / 'Dockerfile'
        # BuildKit interprets FROM sha256:<id> as a registry reference. Supply
        # an additive local tag and fence its exact image ID before/after build.
        dockerfile.write_text('FROM ' + receipt['baseTag'] + '\nCOPY --chown=0:0 --chmod=0755 bin/ /usr/local/bin/\n')
        receipt['dockerfileSHA256'] = digest(dockerfile)
        receipt['dockerfile'] = dockerfile.read_text()
        receipt['status'] = 'building'
        write_json(receipt_path, receipt)
        require(free('/workspace') >= base['Size'] + STOP_RESERVE, 'Headroom changed before Docker COPY')
        receipt['baseTagCommand'] = add_verified_base_tag(receipt['baseTag'])
        receipt['baseTagVerifiedBefore'] = BASE
        write_json(receipt_path, receipt)
        argv = [DOCKER, 'build', '--pull=false', '--network=none', '--progress=plain',
                '--tag', receipt['tag'], '--file', str(dockerfile), str(context)]
        receipt['buildCommand'] = argv
        run_monitored(argv, stage, 'docker-build', timeout=600)
        candidate = inspect(receipt['tag'])
        require(inspect(receipt['baseTag'])['Id'] == BASE, 'Local base tag changed during the build')
        receipt['baseTagVerifiedAfter'] = BASE
        receipt['candidateImage'] = candidate['Id']
        require(candidate['Id'] not in [BASE, OLD], 'Candidate image unexpectedly reused an old identity')
        require(candidate['Config'] == base['Config'], 'Candidate image config differs from trusted base')
        require(candidate['Architecture'] == base['Architecture'] and candidate['Os'] == base['Os'], 'Candidate platform differs')
        require(candidate['RootFS']['Layers'][:-1] == base['RootFS']['Layers'], 'Candidate does not add exactly one layer to the trusted base')
        receipt['candidate'] = {key: candidate[key] for key in ['Id', 'Config', 'RootFS', 'Size', 'Architecture', 'Os']}
        receipt['sourcesAfter'] = source_hashes()
        require(receipt['sourcesAfter'] == receipt['sourcesBefore'], 'Source changed before candidate verification')
        require(all(digest(context / 'bin' / name) == value['sha256'] for name, value in receipt['inputFiles'].items()), 'Staged build input changed')
        receipt['preservedAfter'] = preserved_state()
        require(receipt['preservedBefore'] == receipt['preservedAfter'], 'Recovery archive or setup provenance/pins changed')
        tags_after = image_inventory()
        require(all(tuple(item) in tags_after for item in receipt['tagsBefore']), 'A pre-existing image tag changed')
        receipt['status'] = 'built-config-verified-export-and-runtime-pending'
        receipt['finishedAt'] = now()
        receipt['workspaceFreeBytesAfter'] = free('/workspace')
        write_json(receipt_path, receipt)
        print(json.dumps({'receipt': str(receipt_path), 'candidate': candidate['Id'], 'status': receipt['status']}, indent=2))
    except BaseException as error:
        receipt.update(status='failed', failedAt=now(), error=str(error), daemonDrain='If cancellation occurred, do not assume daemon work stopped with the client')
        write_json(receipt_path, receipt)
        raise


class HashingReader(io.RawIOBase):
    def __init__(self, stream):
        self.stream, self.hash, self.count = stream, hashlib.sha256(), 0

    def readable(self):
        return True

    def readinto(self, buffer):
        chunk = self.stream.read(len(buffer))
        if not chunk:
            return 0
        buffer[:len(chunk)] = chunk
        self.hash.update(chunk)
        self.count += len(chunk)
        return len(chunk)


def clean_member(name):
    path = PurePosixPath(name)
    require(not path.is_absolute() and '..' not in path.parts, 'Unsafe archive member name')
    return str(path)


def read_layer(stream):
    raw = HashingReader(stream)
    buffered = io.BufferedReader(raw, buffer_size=CHUNK)
    compressed = buffered.peek(2)[:2] == b'\x1f\x8b'
    content = gzip.GzipFile(fileobj=buffered, mode='rb') if compressed else buffered
    decoded = HashingReader(content)
    payload = io.BufferedReader(decoded, buffer_size=CHUNK)
    interesting, directories, non_directories, other_files = {}, {}, 0, 0
    with tarfile.open(fileobj=payload, mode='r|') as archive:
        for member in archive:
            # Python 3.12 retains TarInfo even in streaming mode. We never look
            # up or extract links, so discard that hidden per-member cache.
            archive.members.clear()
            name = clean_member(member.name)
            if member.isdir() and name in ['usr', 'usr/local', 'usr/local/bin']:
                require(name not in directories, 'Duplicate runtime ancestor directory')
                directories[name] = {'mode': format(member.mode & 0o7777, '04o'), 'uid': member.uid, 'gid': member.gid}
            if not member.isdir():
                non_directories += 1
                require(non_directories < 500000, 'Unreasonable layer member count')
                if name not in ['usr/local/bin/sandboxd', 'usr/local/bin/entrypoint.sh']:
                    other_files += 1
            if name not in ['usr/local/bin/sandboxd', 'usr/local/bin/entrypoint.sh']:
                continue
            require(member.isfile() and name not in interesting, 'Target entry is duplicated or not a regular file')
            require(member.size <= 32 << 20, 'Unexpected runtime file size')
            file_hash = hashlib.sha256()
            file = archive.extractfile(member)
            for chunk in iter(lambda: file.read(CHUNK), b''):
                file_hash.update(chunk)
            interesting[name] = {'sha256': file_hash.hexdigest(), 'bytes': member.size,
                                  'mode': format(member.mode & 0o7777, '04o'), 'uid': member.uid, 'gid': member.gid}
    while payload.read(CHUNK):
        pass
    if compressed:
        content.close()
    while buffered.read(CHUNK):
        pass
    return {'rawSHA256': raw.hash.hexdigest(), 'diffID': 'sha256:' + decoded.hash.hexdigest(),
            'files': interesting, 'directories': directories, 'nonDirectoryCount': non_directories, 'nonTargetCount': other_files}


def compare_export_config(exported, inspected):
    # Proven omissions in this Docker/OCI image schema, not generic empty-value
    # normalization: raw JSON omits these nil slices/maps, while full image
    # inspect materializes null. Empty arrays/maps and non-null values stay exact.
    nullable_omissions = {'Cmd', 'Labels', 'OnBuild', 'Volumes'}
    require(isinstance(exported, dict) and isinstance(inspected, dict), 'Image config must be an object')
    accepted, differences = [], []
    for key in sorted(exported.keys() | inspected.keys()):
        if key not in exported and key in nullable_omissions and inspected.get(key) is None:
            accepted.append(key)
        elif key not in exported or key not in inspected or exported[key] != inspected[key]:
            differences.append({'field': key, 'exportedPresent': key in exported, 'inspectedPresent': key in inspected,
                                'exported': exported.get(key), 'inspected': inspected.get(key)})
    return {'acceptedMissingToNull': accepted, 'differences': differences}


def retain_export_diagnostics(attempt, candidate, layers, metadata):
    # Preserve bounded raw metadata before interpreting it or accepting any
    # equality. The export itself is never retained as a full archive.
    entries = {}
    for name, raw in metadata.items():
        identity = hashlib.sha256(raw).hexdigest()
        path = attempt / ('metadata-' + identity + '.raw')
        path.write_bytes(raw)
        entries[name] = {'sha256': identity, 'bytes': len(raw), 'path': str(path)}
    diagnostics = {'candidate': candidate, 'layers': layers, 'metadata': entries}
    config_entry = next((entry for entry in entries.values() if 'sha256:' + entry['sha256'] == candidate['Id']), None)
    if config_entry:
        diagnostics['rawConfigPath'] = config_entry['path']
        try:
            raw_config = json.loads(Path(config_entry['path']).read_bytes())
            diagnostics['rawConfig'] = raw_config
            diagnostics['configComparison'] = compare_export_config(raw_config.get('config'), candidate['Config'])
            diagnostics['expectedRootFS'] = {'type': candidate['RootFS']['Type'], 'diff_ids': candidate['RootFS']['Layers']}
        except (ValueError, TypeError, RuntimeError) as error:
            diagnostics['configDiagnosticError'] = str(error)
    path = attempt / 'diagnostics.json'
    write_json(path, diagnostics)
    return str(path)


def verify_export(receipt_path):
    receipt_path = receipt_path.resolve(strict=True)
    stage = receipt_path.parent
    require(stage.parent == TOOLS and stage.name.startswith('sandbox-candidate-'), 'Receipt is not an owned candidate stage')
    receipt = json.loads(receipt_path.read_text())
    require(receipt['status'] in ['built-config-verified-export-and-runtime-pending', 'export-verified-runtime-pending'], 'Candidate build was not verified')
    candidate = inspect(receipt['candidateImage'])
    require({key: candidate[key] for key in receipt['candidate']} == receipt['candidate'], 'Candidate image identity/config changed')
    # Docker save can stage its own uncompressed archive in the daemon. Streaming
    # avoids our own full archive, not that daemon allocation. Budget both honestly.
    required = candidate['Size'] + STOP_RESERVE + (256 << 20)
    require(free('/workspace') >= required, 'Export deferred: daemon archive staging plus cancellation headroom unavailable')
    # The root daemon environment is unreadable. Its implementation defaults to
    # /var/lib/docker/tmp, but DOCKER_TMPDIR can override it. Reserve both known
    # filesystems instead of asserting that streaming eliminates this unknown.
    require(free('/tmp') >= required, 'Export deferred: daemon temp override is unknown and /tmp lacks full archive headroom')
    require(preserved_state() == receipt['preservedBefore'], 'Old recovery archive/pins changed')
    attempt = stage / ('export-' + dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex[:8])
    attempt.mkdir(mode=0o700)
    # Preserve the original failed receipt and monitor evidence on every retry.
    shutil.copyfile(receipt_path, attempt / 'receipt-before.json')
    layers, metadata, metadata_bytes = {}, {}, 0
    export = {'startedAt': now(), 'requiredFreeBytesPerFilesystem': required, 'workspaceFreeBytesBefore': free('/workspace'),
              'tmpFreeBytesBefore': free('/tmp'), 'daemonTempLocation': 'Unknown override; both known filesystems budgeted',
              'attemptDirectory': str(attempt), 'verifierSHA256': digest(Path(__file__))}
    log = attempt / 'docker-save.stderr'
    with log.open('wb') as stderr:
        proc = subprocess.Popen([DOCKER, 'image', 'save', candidate['Id']], stdout=subprocess.PIPE, stderr=stderr, start_new_session=True)
        try:
            with Monitor(proc, attempt, 'docker-save', 1800, temp_reserve=STOP_RESERVE) as monitor:
                with tarfile.open(fileobj=proc.stdout, mode='r|') as outer:
                    for member in outer:
                        outer.members.clear()
                        name = clean_member(member.name)
                        if member.isdir():
                            continue
                        require(member.isfile(), 'Unexpected non-regular Docker export entry')
                        stream = outer.extractfile(member)
                        if name.endswith('/layer.tar') or (name.startswith('blobs/sha256/') and member.size > 2 << 20):
                            require(name not in layers and len(layers) < 16, 'Duplicate or excessive layer member')
                            layers[name] = read_layer(stream)
                        else:
                            require(member.size <= 2 << 20, 'Oversized Docker metadata')
                            require(name not in metadata and len(metadata) < 1000, 'Duplicate or excessive Docker metadata')
                            metadata_bytes += member.size
                            require(metadata_bytes <= 16 << 20, 'Excessive aggregate Docker metadata')
                            metadata[name] = stream.read()
                # Drain the outer tar trailer so Docker can finish without a full pipe.
                while proc.stdout.read(CHUNK):
                    pass
                code = proc.wait(timeout=60)
            export['diagnostics'] = retain_export_diagnostics(attempt, candidate, layers, metadata)
            require(monitor.failure is None, (monitor.failure or '') + '; daemon drain must be checked')
            require(code == 0, f'docker save exited {code}')
            manifests = json.loads(metadata['manifest.json'])
            wanted = [m for m in manifests if hashlib.sha256(metadata.get(m['Config'], b'')).hexdigest() == candidate['Id'].split(':')[1]]
            require(len(wanted) == 1, 'Export does not identify exactly the candidate config')
            manifest = wanted[0]
            config = json.loads(metadata[manifest['Config']])
            comparison = compare_export_config(config['config'], candidate['Config'])
            export['configComparison'] = comparison
            require(not comparison['differences'], 'Export config mismatch; see retained raw metadata and field differences')
            require(config['rootfs'] == {'type': candidate['RootFS']['Type'], 'diff_ids': candidate['RootFS']['Layers']}, 'Export rootfs mismatch')
            require(config['architecture'] == candidate['Architecture'] and config['os'] == candidate['Os'], 'Export platform mismatch')
            require([layers[name]['diffID'] for name in manifest['Layers']] == candidate['RootFS']['Layers'], 'Exported layer bytes do not match image diff IDs')
            final = layers[manifest['Layers'][-1]]
            targets = {'usr/local/bin/' + name: value for name, value in receipt['inputFiles'].items()}
            require(final['files'] == targets and final['nonDirectoryCount'] == len(targets) and final['nonTargetCount'] == 0, 'Candidate layer payload, ownership or mode differs from exact staged inputs')
            require(all(info == {'mode': '0755', 'uid': 0, 'gid': 0} for info in final['directories'].values()), 'Candidate changes runtime ancestor directory access')
            export.update(status='passed', finishedAt=now(), candidateImage=candidate['Id'], verifiedFiles=final['files'],
                          layerDiffIDs=[layers[name]['diffID'] for name in manifest['Layers']], workspaceFreeBytesAfter=free('/workspace'))
            receipt.update(status='export-verified-runtime-pending', exportVerification=export)
            write_json(receipt_path, receipt)
            print(json.dumps({'receipt': str(receipt_path), 'status': receipt['status'], 'verifiedFiles': final['files']}, indent=2))
        except BaseException as error:
            stop_client(proc)
            try:
                export['diagnostics'] = retain_export_diagnostics(attempt, candidate, layers, metadata)
            except BaseException as diagnostic_error:
                export['diagnosticsError'] = str(diagnostic_error)
            export.update(status='failed', error=str(error), failedAt=now(), daemonDrain='Do not assume daemon operations stopped with the client')
            receipt['exportVerification'] = export
            write_json(receipt_path, receipt)
            raise


def main():
    def interrupted(signum, _frame):
        raise SystemExit(128 + signum)
    signal.signal(signal.SIGTERM, interrupted)
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    commands.add_parser('build', help='Compile and build a candidate; preserve existing runtime/pins; no export/container')
    check = commands.add_parser('verify-export', help='Separately budgeted streaming layer-byte verification; no container')
    check.add_argument('receipt', type=Path)
    args = parser.parse_args()
    if args.command == 'build':
        build()
    else:
        verify_export(args.receipt)


if __name__ == '__main__':
    main()
