#!/usr/bin/env python3
"""Stage and publish a verified sandbox recovery archive after runtime acceptance.

Root separately tags the exact accepted image as luci/sandbox:dev. This helper
never tags, loads, prunes, starts, or deletes Docker resources. Stage performs one
monitored docker save. Publish performs only verified local file replacements.
"""
import argparse
import contextlib
import fcntl
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import tarfile
import uuid

TOOLS = Path('/workspace/.luci-tools')
HELPER = TOOLS / 'prepare-sandbox-candidate-cloud016.py'
HELPER_SHA = '5018084eca4e49bb6bb22d304a46a973390d6ec2b34af1ca5cb1dedec249636e'
CANDIDATE = 'sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98'
TAG = 'luci/sandbox:dev'
ARCHIVE = TOOLS / 'luci-sandbox-dev.tar.gz'
PINS = ['sandbox-image-provenance.json', 'install.sh', 'start.md']
MAX_JSON = 16 << 20
h = None

# Narrow C012 acceptance binding, not a generic runtime or product-quality gate.
SOURCE_COMMIT = 'c4400f11736e923617a93e8e86803f15d7187bef'
CANDIDATE_RECEIPT_SHA = '5af726e6670fd6f04c0f67cf3b26b3f6072ed89c1e1e3481c652a631ae6d0a3b'
RUNTIME_RECEIPT_SHA = 'e44aaecc7ec2d16784c61b860eea8fed3981d89b108cf4001eb7f62efe60f57b'
PACKAGE_PROVENANCE = Path('/workspace/luci/artifacts/quality-swarm/continuation-c4400f11-attempt3/result/package-provenance.json')
PACKAGE_PROVENANCE_SHA = 'c1294b2643fbac9655c3be96670ebefe2fd9dc08776bfb7063058e01423f5c38'
PACKAGE_ARCHIVE_SHA = 'db6a842a9252059c5f269a3f794c0f6900b116c880ac6bed85040287482a741f'
BINARY_SHA = '08fe8d6e4dddc4bc39be873a92c51c29f3c9c6a422d71af76f74e089acf2ff26'
WEB_INDEX_SHA = '23614113b16654ec7278b735d6545795489b00fb5e4dc6fce5af2dda4a75e79c'
DRIVER_SHA = 'f7b8183877e8ceee9ada5b46c2a53ff485cca53abeba9a096a0c60e6e1525015'
RUNTIME_SCOPE = ('Exact clean c440 installed two-worker scripted continuation: native exits, '
                 'automatic report/review visibility, retained-source checkpoint, restart, '
                 'reviewed integration and independent fixture check. No live-model, sustained '
                 'performance, Windows, full-product or fresh-daemon restoration claim.')


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def sha(path):
    with Path(path).open('rb') as f:
        return hashlib.file_digest(f, 'sha256').hexdigest()


def load_helper():
    require(sha(HELPER) == HELPER_SHA, 'Candidate verifier changed; independent re-review required')
    spec = importlib.util.spec_from_file_location('candidate_verifier', HELPER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def regular(path):
    info = Path(path).lstat()
    require(stat.S_ISREG(info.st_mode), 'Expected a regular file, not a symlink: ' + str(path))
    return info


def read_json(path):
    require(regular(path).st_size <= MAX_JSON, 'Oversized receipt')
    return json.loads(Path(path).read_bytes())


def sync_dir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def durable_bytes(path, data, mode=0o600):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    identity = os.fstat(fd)
    try:
        with os.fdopen(fd, 'wb') as f:
            f.write(data)
            f.flush()
            os.fsync(f.fileno())
        sync_dir(Path(path).parent)
    except BaseException:
        with contextlib.suppress(FileNotFoundError):
            current = Path(path).lstat()
            if (current.st_dev, current.st_ino) == (identity.st_dev, identity.st_ino):
                Path(path).unlink()
        raise


def journal(path, data):
    temp = path.parent / (path.name + '.' + uuid.uuid4().hex + '.new')
    durable_bytes(temp, (json.dumps(data, indent=2) + '\n').encode())
    try:
        os.replace(temp, path)
        sync_dir(path.parent)
    finally:
        with contextlib.suppress(FileNotFoundError):
            temp.unlink()


def admission(image_size):
    require(type(image_size) is int and 0 < image_size < 4 << 30, 'Unexpected image size')
    # Include tar headers/padding and a conservative gzip expansion allowance.
    raw_limit = image_size + (256 << 20)
    gzip_limit = raw_limit + raw_limit // 1000 + (1 << 20)
    return {'rawLimitBytes': raw_limit, 'gzipLimitBytes': gzip_limit,
            'workspaceRequiredBytes': raw_limit + gzip_limit + h.STOP_RESERVE + (64 << 20),
            'tmpRequiredBytes': raw_limit + h.STOP_RESERVE + (64 << 20),
            'hardFloorBytes': h.FLOOR, 'cancelAtFreeBytes': h.STOP_RESERVE}


def exact_receipt_inputs(candidate_path, runtime_path):
    for path, expected in [(candidate_path, CANDIDATE_RECEIPT_SHA), (runtime_path, RUNTIME_RECEIPT_SHA),
                           (PACKAGE_PROVENANCE, PACKAGE_PROVENANCE_SHA)]:
        require(regular(path).st_size <= MAX_JSON and sha(path) == expected, 'Pinned acceptance input changed: ' + str(path))
    package = read_json(PACKAGE_PROVENANCE)
    require(package.get('sourceCommit') == SOURCE_COMMIT and package.get('sourceDirty') is False, 'Package source binding changed')
    require(package.get('archiveSHA256') == PACKAGE_ARCHIVE_SHA and package.get('binarySHA256') == BINARY_SHA and
            package.get('webIndexSHA256') == WEB_INDEX_SHA, 'Package subjects changed')


def runtime_gate(runtime, candidate):
    require(candidate == CANDIDATE and runtime.get('passed') is True and runtime.get('image', {}).get('id') == candidate,
            'Passing installed receipt for exact333eff required')
    require(runtime.get('mode') == 'continuation-installed-scripted' and type(runtime.get('actualTaskCount')) is int and
            runtime['actualTaskCount'] == 2 and set(runtime.get('actualTaskResults', {})) == {'partial', 'finish'},
            'Exact two-worker continuation result required')
    require(runtime.get('sourceCommit') == SOURCE_COMMIT and runtime.get('sourceDirty') is False, 'Clean c440 product source required')
    harness = runtime.get('harnessCheckout', {})
    require(harness.get('commit') == SOURCE_COMMIT and harness.get('expectedSource') == SOURCE_COMMIT and
            harness.get('productSource') == SOURCE_COMMIT and harness.get('dirty') is False and
            harness.get('separateHelperRevision') is False, 'Exact clean c440 helper binding required')
    integrity = runtime.get('packageIntegrity', {})
    require(integrity.get('allMembersMatch') is True and type(integrity.get('verifiedMembers')) is int and
            integrity['verifiedMembers'] == 1203 and integrity.get('archiveSHA256') == PACKAGE_ARCHIVE_SHA and
            runtime.get('binarySHA256') == BINARY_SHA and runtime.get('webIndexSHA256') == WEB_INDEX_SHA and
            runtime.get('driverSHA256') == DRIVER_SHA, 'Installed package/helper integrity changed')
    require(runtime.get('controllersStopped') is True and runtime.get('dockerRequestsDrained') is True,
            'Stopped controllers and drained Docker requests required')
    require(runtime.get('cleanup') == [] and runtime.get('browserErrors') == [] and runtime.get('signal') is None and
            not any(runtime.get(key) for key in ['failure', 'backgroundFailure', 'failedStage']), 'Failed/uncertain acceptance remains')
    cleanup = runtime.get('runtimeCleanup', {})
    require(all(type(cleanup.get(k)) is int and cleanup[k] == 0 for k in
                ['containers', 'volumes', 'ownedServiceProcesses', 'ownedBrowserProcesses']), 'Runtime resources remain or cleanup is unknown')
    require(cleanup.get('protectedImagePackageAndHelpersUnchanged') is True, 'Protected identities were not verified')
    transport = runtime.get('dockerTransportObservations', {})
    counts, drained = transport.get('counts', {}), transport.get('cleanup', {})
    require(transport.get('dropped') == 0 and transport.get('failures') == [] and
            all(type(counts.get(k)) is int and counts[k] == v for k,v in [('create',2),('worker',2),('fence',0),('start',2)]),
            'Missing or unexpected transport observations')
    require(drained.get('drained') is True and drained.get('uncertainMutation') is False and
            all(type(drained.get(k)) is int and drained[k] == 0 for k in ['pendingRequests','activeHandlers','pendingBodyBytes']),
            'Transport drain is incomplete')
    natural = runtime.get('naturalCompletions', [])
    require(len(natural) == 2 and {x.get('task') for x in natural} == {'partial','finish'} and
            all(x.get('sessionStatus') == 'exited' and x.get('cleanNativeExitFrame') is True and
                x.get('codingDispatches') == 1 and x.get('integratedBeforeExit') is False for x in natural),
            'Both native one-shot results must precede integration')
    terminal = runtime.get('terminalObservations', [])
    require(len(terminal) == 2 and {x.get('sessionID') for x in terminal} == {x.get('sessionID') for x in natural} and
            all(x.get('exit') == {'classification':'exit-status','code':0} and x.get('exitFrames') == 1 and
                x.get('trustEnterSent') is False for x in terminal), 'Explicit native zero exits required')
    automatic = runtime.get('automaticCompletions', [])
    require(len(automatic) == 2 and {x.get('variant') for x in automatic} == {'partial','finish'} and
            all(x.get('recordedStatus') == 'review-required' and x.get('manualReconcile') is False and
                x.get('manualCollection') is False and x.get('homeVisible') is True and x.get('integrated') is False and
                x.get('inPlace', {}).get('recordedStatus') == 'Awaiting review' for x in automatic),
            'Automatic in-place and Home result visibility required')
    checkpoint, final = runtime.get('checkpoint', {}), runtime.get('final', {})
    require(checkpoint.get('replyFault', {}).get('status') == 201 and checkpoint.get('explicitIdempotentReadbackHTTP') == 200,
            'Lost checkpoint response/idempotent readback evidence required')
    require(final.get('retainedOriginalUnchanged') is True and final.get('independentCheck', {}).get('exitCode') == 0 and
            final['independentCheck'].get('stdout') == 'CONTINUATION_CHECK_OK final\n' and
            final['independentCheck'].get('sourceCommit') == final.get('primaryCommit'), 'Independent exact-source fixture result required')
    lifecycle = runtime.get('lifecycle', [])
    require([x.get('action') for x in lifecycle] == ['start','stop','start','stop'] and
            all(x.get('stopped') is True and x.get('exitCode') == 0 and x.get('forced') is False
                for x in lifecycle if x['action'] == 'stop'), 'Orderly restart and final controller closure required')
    require(runtime.get('externalModelCalls') == 0 and runtime.get('cloudResourcesCreated') == 0,
            'This narrow scope uses scripted provider and no cloud resources')


class LimitedWriter:
    def __init__(self, file, limit):
        self.file, self.limit, self.count = file, limit, 0

    def write(self, data):
        require(self.count + len(data) <= self.limit, 'Compressed archive bound exceeded')
        written = self.file.write(data)
        self.count += written
        return written

    def flush(self):
        self.file.flush()


def verify_archive(path, candidate, inputs, stage):
    layers, metadata, metadata_bytes, members = {}, {}, 0, 0
    with gzip.open(path, 'rb') as stream:
        with tarfile.open(fileobj=stream, mode='r|') as archive:
            for member in archive:
                archive.members.clear()
                members += 1
                require(members <= 1100, 'Excessive export member count')
                name = h.clean_member(member.name)
                if member.isdir():
                    continue
                require(member.isfile(), 'Non-regular Docker export member')
                content = archive.extractfile(member)
                if name.endswith('/layer.tar') or (name.startswith('blobs/sha256/') and member.size > 2 << 20):
                    require(name not in layers and len(layers) < 16, 'Duplicate/excessive layers')
                    layers[name] = h.read_layer(content)
                else:
                    require(member.size <= 2 << 20 and name not in metadata, 'Oversized/duplicate metadata')
                    metadata_bytes += member.size
                    require(metadata_bytes <= MAX_JSON, 'Excessive metadata aggregate')
                    metadata[name] = content.read()
        # Read through the gzip footer: parsing tar alone does not verify CRC.
        while stream.read(h.CHUNK):
            pass
    diagnostics = h.retain_export_diagnostics(stage, candidate, layers, metadata)
    manifests = json.loads(metadata['manifest.json'])
    require(len(manifests) == 1, 'Archive must contain exactly one image')
    manifest = manifests[0]
    require(manifest.get('RepoTags') == [TAG], 'Archive does not restore exactly the pinned dev tag')
    raw = metadata[manifest['Config']]
    require('sha256:' + hashlib.sha256(raw).hexdigest() == CANDIDATE, 'Config bytes do not identify exact candidate')
    config = json.loads(raw)
    comparison = h.compare_export_config(config['config'], candidate['Config'])
    require(not comparison['differences'], 'Runtime config drift')
    require(config['rootfs'] == {'type': candidate['RootFS']['Type'], 'diff_ids': candidate['RootFS']['Layers']}, 'RootFS mismatch')
    require(config['architecture'] == candidate['Architecture'] and config['os'] == candidate['Os'], 'Platform mismatch')
    require(set(layers) == set(manifest['Layers']), 'Unreferenced or absent archive layers')
    require([layers[n]['diffID'] for n in manifest['Layers']] == candidate['RootFS']['Layers'], 'Layer bytes do not match diffIDs')
    final = layers[manifest['Layers'][-1]]
    targets = {'usr/local/bin/' + k: v for k, v in inputs.items()}
    require(final['files'] == targets and final['nonDirectoryCount'] == len(targets) and final['nonTargetCount'] == 0, 'Runtime file bytes/modes/ownership differ')
    require(all(v == {'mode': '0755', 'uid': 0, 'gid': 0} for v in final['directories'].values()), 'Runtime ancestor mode/ownership changed')
    return {'status': 'passed', 'gzipCRCVerified': True, 'archiveSHA256': sha(path), 'archiveBytes': regular(path).st_size,
            'candidateImage': CANDIDATE, 'restoredTags': manifest['RepoTags'], 'configComparison': comparison,
            'layerDiffIDs': candidate['RootFS']['Layers'], 'verifiedFiles': final['files'], 'diagnostics': diagnostics}


def prepare_pins(stage, receipt, candidate_receipt, runtime):
    old_provenance = read_json(stage / 'old-sandbox-image-provenance.json')
    provenance = dict(old_provenance)
    # Previous evidence remains in the retained old provenance. Do not carry
    # stale browser/source receipts forward as acceptance of this new image.
    for key in ['browserEvidence', 'sourceRuntimeEvidence']:
        provenance.pop(key, None)
    provenance.update(retainedImage=CANDIDATE, tag=TAG, sourceCommit=runtime['sourceCommit'],
                      sandboxdSHA256=candidate_receipt['inputFiles']['sandboxd']['sha256'],
                      archiveSHA256=receipt['archiveVerification']['archiveSHA256'], gzipVerified=True,
                      dockerLoadVerified=False, archiveManifestVerified=True, config=receipt['candidate']['Config'],
                      runtimeEvidence=receipt['runtimeReceipt'], candidateEvidence=receipt['candidateReceipt'],
                      verifiedAt=h.now(), scope=RUNTIME_SCOPE + ' Full upstream Dockerfile rebuild and docker load of this new archive are not exercised.',
                      previousProvenance=str(stage / 'old-sandbox-image-provenance.json'))
    durable_bytes(stage / 'new-sandbox-image-provenance.json', (json.dumps(provenance, indent=2) + '\n').encode())
    for name in ['install.sh', 'start.md']:
        old = (stage / ('old-' + name)).read_text()
        require(old.count(h.ARCHIVE_SHA) == 1 and old.count(h.OLD) == 1, 'Old pin occurrence differs: ' + name)
        updated = old.replace(h.ARCHIVE_SHA, provenance['archiveSHA256']).replace(h.OLD, CANDIDATE)
        durable_bytes(stage / ('new-' + name), updated.encode(), regular(TOOLS / name).st_mode & 0o777)
    receipt['newPins'] = {name: sha(stage / ('new-' + name)) for name in PINS}


def stage_archive(candidate_path, runtime_path):
    exact_receipt_inputs(candidate_path, runtime_path)
    cr, runtime = read_json(candidate_path), read_json(runtime_path)
    require(cr.get('candidateImage') == CANDIDATE and cr.get('exportVerification', {}).get('status') == 'passed', 'Exact candidate export must have passed')
    runtime_gate(runtime, CANDIDATE)
    candidate = h.inspect(TAG)
    require(candidate['Id'] == CANDIDATE, 'Root must first tag the exact accepted candidate as luci/sandbox:dev')
    require({k: candidate[k] for k in cr['candidate']} == cr['candidate'], 'Candidate config changed')
    require(not subprocess.check_output([h.DOCKER, 'ps', '-aq'], text=True).strip(), 'Containers remain; exclusive Docker window required')
    require(h.preserved_state() == cr['preservedBefore'], 'Old archive/provenance/install/start pins changed')
    require(h.source_hashes() == cr['sourcesBefore'], 'Sandbox sources differ from accepted image')
    budget = admission(candidate['Size'])
    require(h.free('/workspace') >= budget['workspaceRequiredBytes'], 'Insufficient workspace for daemon staging plus bounded gzip output')
    require(h.free('/tmp') >= budget['tmpRequiredBytes'], 'Insufficient /tmp for unknown daemon temporary override')
    stage = TOOLS / ('runtime-archive-' + uuid.uuid4().hex[:12])
    stage.mkdir(mode=0o700)
    sync_dir(TOOLS)
    receipt_path = stage / 'receipt.json'
    receipt = {'status': 'staging', 'startedAt': h.now(), 'stage': str(stage), 'candidateImage': CANDIDATE,
               'candidate': cr['candidate'], 'candidateReceipt': str(candidate_path), 'candidateReceiptSHA256': sha(candidate_path),
               'runtimeReceipt': str(runtime_path), 'runtimeReceiptSHA256': sha(runtime_path),
               'verifierSHA256': sha(Path(__file__)), 'budget': budget, 'preservedBefore': cr['preservedBefore'],
               'oldPins': {name: sha(TOOLS / name) for name in PINS}, 'inputFiles': cr['inputFiles'],
               'packageProvenance': str(PACKAGE_PROVENANCE), 'packageProvenanceSHA256': PACKAGE_PROVENANCE_SHA,
               'originalCandidateVerifierSHA256': HELPER_SHA, 'runtimeScope': RUNTIME_SCOPE}
    journal(receipt_path, receipt)
    partial = stage / 'archive.partial.tar.gz'
    proc = None
    try:
        # A hardlink is a retained regular rollback file without a second 1 GiB
        # copy; neither name is ever modified in place.
        regular(ARCHIVE)
        os.link(ARCHIVE, stage / 'old-luci-sandbox-dev.tar.gz', follow_symlinks=False)
        sync_dir(stage)
        require(sha(stage / 'old-luci-sandbox-dev.tar.gz') == h.ARCHIVE_SHA, 'Rollback archive hash mismatch')
        for name in PINS:
            durable_bytes(stage / ('old-' + name), (TOOLS / name).read_bytes(), regular(TOOLS / name).st_mode & 0o777)
        with partial.open('xb') as output, (stage / 'docker-save.stderr').open('xb') as stderr:
            bounded = LimitedWriter(output, budget['gzipLimitBytes'])
            proc = subprocess.Popen([h.DOCKER, 'image', 'save', TAG], stdout=subprocess.PIPE, stderr=stderr, start_new_session=True)
            try:
                with h.Monitor(proc, stage, 'docker-save', 1800, temp_reserve=h.STOP_RESERVE) as monitor:
                    raw_count = 0
                    with gzip.GzipFile(filename='', fileobj=bounded, mode='wb', compresslevel=6, mtime=0) as compressed:
                        for chunk in iter(lambda: proc.stdout.read(h.CHUNK), b''):
                            raw_count += len(chunk)
                            require(raw_count <= budget['rawLimitBytes'], 'Docker archive exceeds admitted bound')
                            require(monitor.failure is None, monitor.failure or 'Disk monitor failed')
                            compressed.write(chunk)
                    code = proc.wait(timeout=60)
                require(monitor.failure is None and code == 0, monitor.failure or 'Docker save failed')
                output.flush()
                os.fsync(output.fileno())
            except BaseException:
                h.stop_client(proc)
                raise
            finally:
                proc.stdout.close()
        sync_dir(stage)
        receipt['archiveVerification'] = verify_archive(partial, candidate, cr['inputFiles'], stage)
        require(h.preserved_state() == cr['preservedBefore'], 'Old archive/pins changed during export')
        require(h.source_hashes() == cr['sourcesBefore'], 'Sandbox sources changed during export')
        require(h.inspect(TAG)['Id'] == CANDIDATE, 'Dev tag changed during export')
        exact_receipt_inputs(candidate_path, runtime_path)
        require(sha(runtime_path) == receipt['runtimeReceiptSHA256'] and sha(candidate_path) == receipt['candidateReceiptSHA256'], 'Acceptance receipts changed')
        complete = stage / 'verified-luci-sandbox-dev.tar.gz'
        os.replace(partial, complete)
        sync_dir(stage)
        receipt['stagedArchive'] = str(complete)
        prepare_pins(stage, receipt, cr, runtime)
        receipt.update(status='verified-staged-not-published', finishedAt=h.now())
        journal(receipt_path, receipt)
        print(json.dumps({'receipt': str(receipt_path), 'status': receipt['status'], 'archiveSHA256': receipt['archiveVerification']['archiveSHA256']}, indent=2))
    except BaseException as error:
        if proc is not None:
            h.stop_client(proc)
        # Remove exactly our incomplete output only; keep rollback/evidence.
        if partial.exists():
            regular(partial)
            partial.unlink()
            sync_dir(stage)
        receipt.update(status='failed', error=str(error), failedAt=h.now(), daemonDrain='Client stopped; daemon work may still drain. Recheck capacity before another allocation.')
        journal(receipt_path, receipt)
        raise


def publish(receipt_path):
    r = read_json(receipt_path)
    stage = receipt_path.parent.resolve(strict=True)
    require(stage.parent == TOOLS and stage.name.startswith('runtime-archive-') and r.get('stage') == str(stage), 'Unowned stage')
    require(r.get('status') == 'verified-staged-not-published', 'Only a verified stage can be published')
    require(sha(Path(r['candidateReceipt'])) == r['candidateReceiptSHA256'] and sha(Path(r['runtimeReceipt'])) == r['runtimeReceiptSHA256'], 'Acceptance receipts changed')
    require(r.get('verifierSHA256') == sha(Path(__file__)) and r.get('originalCandidateVerifierSHA256') == HELPER_SHA, 'Archive helper changed')
    require(r.get('packageProvenance') == str(PACKAGE_PROVENANCE) and r.get('packageProvenanceSHA256') == PACKAGE_PROVENANCE_SHA, 'Package receipt identity changed')
    exact_receipt_inputs(Path(r['candidateReceipt']), Path(r['runtimeReceipt']))
    runtime_gate(read_json(Path(r['runtimeReceipt'])), CANDIDATE)
    require(h.preserved_state() == r['preservedBefore'], 'Old archive/pins changed before publication')
    require(h.inspect(TAG)['Id'] == CANDIDATE, 'Dev tag differs from accepted candidate')
    require(not subprocess.check_output([h.DOCKER, 'ps', '-aq'], text=True).strip(), 'Containers remain')
    staged = stage / 'verified-luci-sandbox-dev.tar.gz'
    regular(staged)
    require(sha(staged) == r['archiveVerification']['archiveSHA256'], 'Staged archive changed')
    require(sha(stage / 'old-luci-sandbox-dev.tar.gz') == h.ARCHIVE_SHA, 'Rollback archive changed')
    for name in PINS:
        regular(stage / ('old-' + name))
        regular(stage / ('new-' + name))
        require(sha(stage / ('old-' + name)) == r['oldPins'][name] and sha(stage / ('new-' + name)) == r['newPins'][name], 'Rollback or prepared pin changed')
    # Individual replacements are atomic. The journal records the multi-file
    # transaction; during interruption the old installer fails closed on new
    # archive bytes. Retained old and new files permit explicit recovery.
    r.update(status='publishing', replacementOrder=['archive', *PINS], completedReplacements=[])
    journal(receipt_path, r)
    owned_temps = []
    try:
        new_archive = TOOLS / ('.runtime-archive-' + stage.name + '.new')
        os.link(staged, new_archive, follow_symlinks=False)
        owned_temps.append(new_archive)
        os.replace(new_archive, ARCHIVE)
        sync_dir(TOOLS)
        r['completedReplacements'].append('archive')
        journal(receipt_path, r)
        for name in PINS:
            temp = TOOLS / ('.runtime-pin-' + stage.name + '-' + name + '.new')
            durable_bytes(temp, (stage / ('new-' + name)).read_bytes(), regular(TOOLS / name).st_mode & 0o777)
            owned_temps.append(temp)
            os.replace(temp, TOOLS / name)
            sync_dir(TOOLS)
            r['completedReplacements'].append(name)
            journal(receipt_path, r)
        regular(ARCHIVE)
        require(sha(ARCHIVE) == r['archiveVerification']['archiveSHA256'], 'Published archive hash mismatch')
        require(all(sha(TOOLS / name) == r['newPins'][name] for name in PINS), 'Published pins mismatch')
        r.update(status='published', publishedAt=h.now(), rollbackDirectory=str(stage), dockerLoadVerified=False)
        journal(receipt_path, r)
        print(json.dumps({'receipt': str(receipt_path), 'status': r['status'], 'archive': str(ARCHIVE), 'archiveSHA256': r['archiveVerification']['archiveSHA256']}, indent=2))
    except BaseException as error:
        r.update(status='publication-interrupted', error=str(error), recovery='Do not run installer until all pins match. Old regular archive and old/new pin files retained in rollbackDirectory.', rollbackDirectory=str(stage))
        journal(receipt_path, r)
        raise
    finally:
        for temp in owned_temps:
            with contextlib.suppress(FileNotFoundError):
                regular(temp)
                temp.unlink()
        sync_dir(TOOLS)


def main():
    global h
    os.umask(0o077)
    h = load_helper()
    signal.signal(signal.SIGTERM, lambda signum, frame: (_ for _ in ()).throw(SystemExit(128 + signum)))
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    stage = commands.add_parser('stage')
    stage.add_argument('--candidate-receipt', required=True, type=Path)
    stage.add_argument('--runtime-receipt', required=True, type=Path)
    pub = commands.add_parser('publish')
    pub.add_argument('receipt', type=Path)
    args = parser.parse_args()
    with (TOOLS / '.runtime-archive.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.command == 'stage':
            stage_archive(args.candidate_receipt.resolve(strict=True), args.runtime_receipt.resolve(strict=True))
        else:
            publish(args.receipt.resolve(strict=True))


if __name__ == '__main__':
    main()
