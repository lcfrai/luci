#!/usr/bin/env python3
"""Read-only Linux preflight for ONE marker-retained check-recovery fixture.

Writes only a fresh private evidence directory/archive. No signals, Docker
mutation, moves, tombstones, credential output, or ledger edits are implemented.
Preparation only: this script has not yet been executed or runtime-validated.
"""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tarfile
import time

MAX_SECONDS = 120
MAX_BYTES = 2 * 1024**3
MAX_FILES = 10000
started = time.monotonic()


def require(value, code):
    if not value:
        raise RuntimeError(code)


def budget():
    require(time.monotonic() - started < MAX_SECONDS, 'preflight-budget-exceeded')


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def stream_digest(stream):
    value = hashlib.sha256()
    while True:
        budget()
        chunk = stream.read(1024**2)
        if not chunk:
            return value.hexdigest()
        value.update(chunk)


def file_digest(path, limit=MAX_BYTES):
    require(path.resolve(strict=True) == path, 'noncanonical-hash-path')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        before = os.fstat(fd)
        require(stat.S_ISREG(before.st_mode) and before.st_size <= limit, 'unsupported-hash-file')
        with os.fdopen(os.dup(fd), 'rb') as stream:
            value = stream_digest(stream)
        after = os.fstat(fd)
        require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) ==
                (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns), 'file-changed-during-hash')
        return value
    finally:
        os.close(fd)


def ordinary(path, limit=32 * 1024**2):
    budget()
    path = Path(path)
    require(path.is_absolute() and path.resolve(strict=True) == path, 'noncanonical-input-path')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        before = os.fstat(fd)
        require(stat.S_ISREG(before.st_mode) and before.st_size <= limit, 'unsupported-input-file')
        with os.fdopen(os.dup(fd), 'rb') as stream:
            data = stream.read(limit + 1)
        after = os.fstat(fd)
        require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) ==
                (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns), 'input-changed-during-read')
        require(len(data) == before.st_size, 'input-short-or-growing-read')
        return data
    finally:
        os.close(fd)


def write_new(path, value):
    raw = (json.dumps(value, indent=2) + '\n').encode()
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(os.dup(fd), 'wb') as stream:
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
    finally:
        os.close(fd)


def identity(pid):
    try:
        raw = Path('/proc', str(pid), 'stat').read_text()
        parts = raw.rsplit(') ', 1)[1].split()
        return {'pid': pid, 'startTicks': parts[19], 'state': parts[0], 'parentPid': int(parts[1])}
    except (FileNotFoundError, ProcessLookupError):
        return None
    # Permission/parse errors intentionally remain UNKNOWN, never absent.


def minimal_facts(pid):
    first = identity(pid)
    require(first is not None, 'preexisting-identity-disappeared-during-observation')
    base = Path('/proc', str(pid))
    status = base.joinpath('status').read_text()
    uid = int(re.search(r'^Uid:\s+(\d+)', status, re.M).group(1))
    comm = base.joinpath('comm').read_text().strip()
    second = identity(pid)
    require(second is not None and first['startTicks'] == second['startTicks'], 'preexisting-identity-changed-during-observation')
    return {**second, 'uid': uid, 'comm': comm}


def preexisting_exclusion(config, pid, before, after, proof):
    expected = {'pid': 260700, 'startTicks': '2331192', 'uid': 1000, 'comm': 'gpg-agent',
                'parentPid': 1, 'evidenceKey': 'preexistingProcessProof'}
    require(config.get('preexistingUnreadableExclusions') == [expected], 'unreadable-process-exclusion-not-exact')
    require(pid == expected['pid'], 'unlisted-unreadable-process')
    wrappers = [row for row in config['knownIdentities'] if row['role'] == 'wrapper']
    require(len(wrappers) == 1 and wrappers[0]['pid'] == 2124250 and str(wrappers[0]['startTicks']) == '25466319', 'exclusion-wrapper-identity-not-exact')
    require(not any(row['pid'] == pid for row in config['knownIdentities']), 'cannot-exclude-recorded-fixture-process')
    require(config['inputs'].get('wrapperLifecycle', {}).get('sha256') == 'ec1aed4cfd706ccf012c10b02e11e612a4a4858ce709aa50641ab49f9d71b84d', 'exclusion-wrapper-proof-not-pinned')
    require(config['inputs'].get('preexistingProcessProof', {}).get('sha256') == '24648c97f3b4a5977283f7eceb7b8d1d49cdb11287762983d11f129f1cc85204', 'preexisting-proof-not-pinned')
    require(proof and proof.get('fixtureWrapperPID') == 2124250 and proof.get('fixtureWrapperStartTicks') == 25466319 and len(proof.get('observations', [])) == 2, 'preexisting-proof-incomplete')
    for observed in [*proof['observations'], before, after]:
        require(observed is not None and all(observed.get(key) == expected[key] for key in ('pid', 'startTicks', 'uid', 'comm', 'parentPid')), 'preexisting-proof-or-live-identity-mismatch')
        require(int(observed['startTicks']) < int(wrappers[0]['startTicks']), 'unreadable-process-does-not-predate-fixture')
    return {'result': 'unreadable-preexisting-not-fixture-child', 'before': before, 'after': after,
            'creationTicksBeforeWrapper': 25466319 - 2331192, 'proofInput': 'preexistingProcessProof',
            'scope': 'Exact preexisting identity only; environment remains unreadable, process remains live, not absence or whole-machine closure.'}


def closure(config, destination, proof=None):
    known = []
    roles = set()
    destination.update(at=now(), known=known, fixtureOwnedLive=[], unreadableProcesses=[], preexistingUnreadableExclusions=[], otherUidProcessesNotInspected=0,
                       scope='Exact recorded identities plus same-UID fixture environment scan; other UIDs excluded, no whole-machine zero-process claim.')
    for expected in config['knownIdentities']:
        pid = expected['pid']
        require(type(pid) is int and pid > 1, 'invalid-known-pid')
        require(re.fullmatch(r'[0-9]+', str(expected['startTicks'])), 'invalid-known-start-ticks')
        require(expected['evidenceKey'] in config['inputs'], 'identity-provenance-input-missing')
        roles.add(expected['role'])
        try:
            actual = identity(pid)
        except Exception as error:
            known.append({'expected': expected, 'result': 'unknown', 'errorType': type(error).__name__})
            raise RuntimeError('known-identity-observation-unavailable') from None
        state = 'absent' if actual is None else ('reused-pid' if actual['startTicks'] != str(expected['startTicks']) else ('exited-zombie' if actual['state'] == 'Z' else 'live'))
        known.append({'expected': expected, 'observation': actual, 'result': state})
        require(state != 'live', 'known-owned-process-still-live')
        if state == 'exited-zombie':
            require(expected['role'] == 'browser' and actual['parentPid'] == 1, 'unsettled-critical-process-identity')
    require({'wrapper', 'driver', 'controller', 'browser', 'observer'} <= roles, 'required-known-identity-role-missing')
    fixture = str(Path(config['fixtureRoot']))
    same_uid = destination['fixtureOwnedLive']
    other_uid_count = 0
    for entry in Path('/proc').iterdir():
        if not entry.name.isdigit():
            continue
        budget()
        pid = int(entry.name)
        before_unreadable = None
        try:
            status = entry.joinpath('status').read_text()
            uid = int(re.search(r'^Uid:\s+(\d+)', status, re.M).group(1))
            if uid != os.getuid():
                other_uid_count += 1
                destination['otherUidProcessesNotInspected'] = other_uid_count
                continue
            first = identity(pid)
            if first is None:
                continue
            if first['state'] == 'Z':
                continue  # Known zombies are reported above, never called absent.
            if any(row.get('pid') == pid for row in config.get('preexistingUnreadableExclusions', [])):
                before_unreadable = minimal_facts(pid)
            with entry.joinpath('environ').open('rb') as stream:
                raw = stream.read(4 * 1024**2 + 1)
            require(len(raw) <= 4 * 1024**2, 'process-environment-byte-limit')
            second = identity(pid)
            if second is None or first['startTicks'] != second['startTicks']:
                continue
            env = dict(part.split(b'=', 1) for part in raw.split(b'\0') if b'=' in part)
            state = os.fsdecode(env.get(b'LUCI_STATE_DIR', b''))
            browser = os.fsdecode(env.get(b'LUCI_SWARM_BROWSER_FIXTURE', b''))
            if state == fixture + '/state' or state.startswith(fixture + '/state/') or browser == fixture:
                same_uid.append(second)
        except (FileNotFoundError, ProcessLookupError):
            continue
        except PermissionError:
            destination['unreadableProcesses'].append({'pid': pid, 'result': 'unknown'})
            if before_unreadable is None:
                raise RuntimeError('process-observation-permission-denied') from None
            after_unreadable = minimal_facts(pid)
            excluded = preexisting_exclusion(config, pid, before_unreadable, after_unreadable, proof)
            destination['unreadableProcesses'][-1]['result'] = excluded['result']
            destination['preexistingUnreadableExclusions'].append(excluded)
        # Same-UID unreadable observations fail closed except the one explicitly
        # pinned preexisting identity above. It remains live/unreadable, not gone.
    require(not same_uid, 'independent-fixture-process-scan-found-live-owner')
    root = Path(fixture)
    require(not os.path.lexists(root / 'docker-proxy.sock'), 'proxy-public-socket-not-removed')
    require(not list(root.glob('.luci-fence-proxy-*')), 'proxy-private-directory-not-removed')
    destination.update(at=now(), fixtureOwnedLive=same_uid, otherUidProcessesNotInspected=other_uid_count)
    return destination


def docker(config, *args):
    # Callers below are a fixed read-only allowlist; never accept an arbitrary
    # command or shell string from the config.
    require(args[0] in ('info', 'inspect', 'ps', 'volume'), 'unsupported-docker-read')
    env = {'PATH': os.environ['PATH'], 'DOCKER_HOST': config['dockerHost']}
    require(config['dockerHost'].startswith('unix:///'), 'nonlocal-daemon-refused')
    budget()
    result = subprocess.run(['docker', *args], env=env, capture_output=True, timeout=10)
    require(result.returncode == 0, 'docker-read-failed')
    require(len(result.stdout) <= 2 * 1024**2, 'docker-read-byte-limit')
    return result.stdout.decode().strip()


def marker(config, fence):
    require(docker(config, 'info', '--format', '{{.ID}}') == config['engineId'], 'daemon-identity-changed')
    ident = config['markerId']
    logical = 'luci-check-' + config['allocationId']
    by_id = json.loads(docker(config, 'inspect', ident))
    by_name = json.loads(docker(config, 'inspect', logical))
    require(len(by_id) == 1 and by_id == by_name, 'marker-name-identity-mismatch')
    source = config['inputs']['guardSource']['path']
    # Execute only the hash-pinned, reviewed inert shape validator; its result is
    # consumed privately. This subprocess has no Docker connection or mutation.
    code = "import {pathToFileURL} from 'node:url';let raw='';for await(const p of process.stdin)raw+=p;const x=JSON.parse(raw);const m=await import(pathToFileURL(process.argv[1]));m.validateFence(x.inspect,{record:x.fence,name:x.name,inspected:true});"
    checked = subprocess.run(['node', '--input-type=module', '-e', code, source],
                             input=json.dumps({'inspect': by_id[0], 'fence': fence, 'name': logical}).encode(),
                             capture_output=True, timeout=10)
    require(checked.returncode == 0, 'strict-marker-shape-validation-failed')
    filtered = docker(config, 'ps', '-aq', '--no-trunc', '--filter', 'label=luci.instance=' + fence['identity']['instanceId'],
                      '--filter', 'label=luci.check-allocation=' + config['allocationId']).split()
    require(filtered == [ident], 'exact-allocation-inventory-mismatch')
    containers = docker(config, 'ps', '-aq', '--no-trunc').split()
    volumes = docker(config, 'volume', 'ls', '-q').split()
    require(containers == [ident] and not volumes, 'exclusive-fixture-inventory-not-marker-only')
    return {'at': now(), 'inspection': by_id[0], 'containers': containers, 'volumes': volumes}


def fixture_manifest(root):
    rows, size = [], 0
    for current, directories, files in os.walk(root, followlinks=False):
        budget()
        for name in sorted(directories + files):
            path = Path(current, name)
            relative = str(path.relative_to(root))
            s = path.lstat()
            row = {'path': relative, 'mode': stat.S_IMODE(s.st_mode), 'device': s.st_dev,
                   'inode': s.st_ino, 'size': s.st_size, 'mtimeNs': s.st_mtime_ns, 'ctimeNs': s.st_ctime_ns}
            require(s.st_uid == os.getuid(), 'foreign-owned-fixture-node')
            if stat.S_ISDIR(s.st_mode):
                row['type'] = 'directory'
            elif stat.S_ISREG(s.st_mode):
                size += s.st_size
                require(size <= MAX_BYTES, 'fixture-byte-limit')
                row.update(type='file', sha256=file_digest(path))
            else:
                raise RuntimeError('unsupported-linked-or-special-fixture-node')
            rows.append(row)
            require(len(rows) <= MAX_FILES, 'fixture-file-limit')
    return sorted(rows, key=lambda row: row['path'])


def archive_fixture(root, output, manifest):
    archive = output / 'fixture.tar'
    with tarfile.open(archive, 'x', format=tarfile.PAX_FORMAT, dereference=False) as stream:
        for row in manifest:
            budget()
            path = root / row['path']
            info = stream.gettarinfo(str(path), arcname=row['path'])
            # Do not encode hardlink aliases: preserve each file's exact bytes.
            if row['type'] == 'file':
                info.type = tarfile.REGTYPE
                info.linkname = ''
                info.size = row['size']
                fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
                with os.fdopen(fd, 'rb') as source:
                    current = os.fstat(source.fileno())
                    require(stat.S_ISREG(current.st_mode) and (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns) ==
                            (row['device'], row['inode'], row['size'], row['mtimeNs']), 'archive-source-identity-changed')
                    stream.addfile(info, source)
            else:
                stream.addfile(info)
    os.chmod(archive, 0o600)
    descriptor = os.open(archive, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)
    with tarfile.open(archive, 'r:') as stream:
        members = stream.getmembers()
        require(len(members) == len(manifest), 'archive-entry-count-mismatch')
        for row, item in zip(manifest, members):
            budget()
            require(item.name == row['path'] and item.mode == row['mode'], 'archive-path-or-mode-mismatch')
            if row['type'] == 'file':
                require(item.isfile() and item.size == row['size'], 'archive-file-shape-mismatch')
                source = stream.extractfile(item)
                require(source is not None and stream_digest(source) == row['sha256'], 'archive-content-mismatch')
            else:
                require(item.isdir(), 'archive-directory-shape-mismatch')
    require(fixture_manifest(root) == manifest, 'fixture-changed-during-archive')
    return {'path': str(archive), 'bytes': archive.stat().st_size, 'sha256': file_digest(archive, MAX_BYTES + 64 * 1024**2)}


def check_receipts(config, values):
    receipt = values['recoveryReceipt']
    partial = config.get('partialAttempt6Retirement')
    if partial is not None:
        # A single root-explicit forensic retirement exception. This does not
        # change the run outcome, accept another failure, or skip live closure.
        require(partial == {'acknowledgeOriginalRunFailed': True,
                            'receiptSHA256': 'f2b05ffcd3acad5228c5d3ab4681610cc2021e777b6771a74ab161f9b5e2118b',
                            'allowance': 'exact-da8a-attempt6-final-stats-refusal-assertion-only'}, 'partial-retirement-exception-not-exact')
        exact_inputs = {
            'recoveryReceipt': partial['receiptSHA256'],
            'wrapperExit': 'd2da8c7dbdce432266f449291d8e47be8cbd81f7836d4afb41f741f2ae619e89',
            'wrapperLifecycle': 'ec1aed4cfd706ccf012c10b02e11e612a4a4858ce709aa50641ab49f9d71b84d',
            'wrapperStderr': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
            'sourceBinding': '7ffad5e400ee4413505706d8530bd71a113ad571a10c7895cf8afb924c41bc72',
            'scenarioSource': 'f69a7d5bbf1fd1019106f5a24170f4946e182c78f741d6ad8415b5a90d386e24',
            'installedSource': '1f4acfbe56c91daadbc8968ac5c1a8db0fbd75e1a066c0bae50c84d748ad1c81',
            'guardSource': '5bb86009d73cdd158637a716c4a3a348edb30337e9e2a8cd93ebef82773e1b35',
        }
        require(all(config['inputs'].get(key, {}).get('sha256') == value for key, value in exact_inputs.items()), 'partial-retirement-evidence-pins-mismatch')
        require(config['productCommit'] == config['helperCommit'] == 'da8a6ff29d5f4b15b674087076855404456f2d9d', 'partial-retirement-source-not-exact')
        require(values['wrapperExit']['exitCode'] == 1 and receipt['passed'] is False and receipt.get('scenario') is None, 'original-failed-outcome-not-preserved')
        failure = receipt.get('failure', {})
        require(failure.get('errorName') == 'AssertionError' and failure.get('errorCode') == 'ERR_ASSERTION' and failure.get('locations', [None])[0] == {'file': 'recovery-scenario.mjs', 'line': 241, 'column': 10}, 'partial-retirement-failure-not-exact')
        require(receipt.get('failedStage') == 'explicit-current-owner-recovery', 'partial-retirement-stage-not-exact')
    else:
        require(values['wrapperExit']['exitCode'] == 0 and receipt['passed'] is True and not receipt.get('failure'), 'recovery-not-passed')
    require(receipt['mode'] == 'installed-check-create-recovery', 'wrong-recovery-mode')
    require(receipt['sourceCommit'] == config['productCommit'] and receipt['helperSourceCommit'] == config['helperCommit'], 'source-pin-mismatch')
    require(receipt['fixtureRetained'] is True and receipt['permanentMarkerRemoved'] is False, 'fixture-already-retired-or-unretained')
    require(receipt['cleanup'] == [], 'cleanup-failure')
    require(receipt['controllersStopped'] is True and receipt['dockerRequestsDrained'] is True, 'transport-or-controller-not-settled')
    require(receipt['cleanupPhases'] and all(row['status'] == 'completed' for row in receipt['cleanupPhases']), 'cleanup-phases-incomplete')
    resources = receipt['startupResources']
    require(resources and all(row['state'] == 'closed' and not row.get('late') and not row.get('closeTimedOut') for row in resources), 'startup-resources-unsettled')
    proxy = receipt['proxyClosure']
    require(proxy['drained'] is True and proxy['uncertainMutation'] is False and all(proxy[key] == 0 for key in ('pendingRequests', 'activeHandlers', 'pendingBodyBytes')), 'proxy-drain-not-certain')
    observations = receipt['dockerTransportObservations']
    expected_failures = ['sdk-check-request-shape-refused'] if partial else []
    require(observations['cleanup'] == proxy and observations['dropped'] == 0 and observations['failures'] == expected_failures, 'transport-observation-incomplete')
    rows = observations['requests']
    require(all(row.get('completedAt') and not row.get('uncertainMutation') and not row.get('failure') for row in rows if row['method'] not in ('GET', 'HEAD') and row.get('forwardedAt')), 'forwarded-mutation-not-settled')
    if partial:
        require(observations['counts'] == {'total': 467, 'create': 3, 'worker': 0, 'fence': 1, 'start': 1, 'check': 2}, 'partial-retirement-effect-counts-not-exact')
        failures = [row for row in rows if row.get('failure')]
        require([row['requestSequence'] for row in failures] == [193, 394, 399, 404, 410, 416], 'partial-retirement-refusals-not-exact')
        known_creates = {row['id']: row for row in rows if row.get('kind') == 'check' and row.get('status') == 201 and row['method'] == 'POST' and row['path'].endswith('/containers/create')}
        for row in failures:
            original = known_creates.get(row.get('ref'))
            require(original and row['method'] == 'GET' and row['path'] == '/v1.51/containers/' + row['ref'] + '/stats' and row['kind'] == 'check' and row['failure'] == 'sdk-check-request-shape-refused' and row['proxyStatus'] == 503 and row.get('completedAt') and not row.get('forwardedAt') and not row.get('uncertainMutation') and row['labels'] == original['labels'], 'partial-retirement-error-is-not-unforwarded-exact-check-stats')
        retired = [row for row in receipt['phases'] if row['phase'] == 'exact-target-charge-retired']
        require(len(retired) == 1 and retired[0]['checkId'] == config['targetCheckId'] and retired[0]['allocationId'] == config['allocationId'] and retired[0]['fenceId'] == config['markerId'] and retired[0]['fenceState'] == 'retired' and retired[0]['resourcesHeld'] is False and retired[0]['cleanupConfirmed'] is True and retired[0]['witnessAllocationId'] is None and retired[0]['beforeCharge'] == {'memoryMiB': 1024, 'cpuMillis': 1000} and retired[0]['afterCharge'] == {'memoryMiB': 0, 'cpuMillis': 0}, 'partial-retirement-positive-release-witness-missing')
        charge = [row for row in receipt['phases'] if row['phase'] == 'exact-shared-ledger-charge'][-1]
        require(charge['totals'] == {'count': 0, 'memoryMiB': 0, 'cpuMillis': 0}, 'partial-retirement-final-charge-not-zero')
    else:
        scenario = receipt['scenario']
        require(scenario['retainedMarker'] == {'id': config['markerId'], 'allocationId': config['allocationId']}, 'marker-pin-mismatch')
        check_target(config, scenario['target'])
    creates = [row for row in rows if row['method'] == 'POST' and row['path'].endswith('/containers/create')]
    originals = [row for row in creates if row.get('kind') == 'check' and row.get('labels', {}).get('luci.check-id') == config['targetCheckId']]
    require(len(originals) == 1 and originals[0]['status'] == 201 and originals[0]['id'] == config['originalContainerId'] and originals[0].get('discardedAt'), 'original-lost-ack-not-exact')
    require(not any(row['method'] == 'POST' and row['path'].endswith('/start') and row.get('ref') == config['originalContainerId'] for row in rows), 'target-command-was-started')
    deletes = [row for row in rows if row['method'] == 'DELETE' and row.get('ref') == config['originalContainerId']]
    require(len(deletes) == 1 and deletes[0]['status'] == 204, 'original-delete-not-exact')
    fences = [row for row in creates if row.get('kind') == 'fence']
    require(len(fences) == 1 and fences[0]['status'] == 201 and fences[0]['id'] == config['markerId'] and fences[0].get('deliveredAt') and not fences[0].get('discardedAt'), 'marker-create-ack-not-exact')
    return receipt


def check_target(config, target):
    require(target['id'] == config['targetCheckId'] and target['status'] == 'cancelled' and target['reasonCode'] == 'check_stopped_by_review' and target['resourcesHeld'] is False and target['evidenceStatus'] == 'unverified', 'target-recovery-state-mismatch')
    require(len(target['commands']) == 1 and target['commands'][0]['cleanupConfirmed'] is True and target['commands'][0]['allocationId'] == config['allocationId'] and target['commands'][0]['status'] == 'cancelled' and target['commands'][0].get('exitCode') is None, 'target-command-cleanup-mismatch')


def compare_stored_public_history(stored, public):
    # evidenceRunView snapshots the run and recomputes ONLY these two top-level
    # display fields. Execution, revision, source, approvals and commands remain
    # exact; the separate raw-file binding also preserves these stored fields.
    derived = {'evidenceStatus', 'staleReasons'}
    for row in (stored, public):
        require(derived <= row.keys() and isinstance(row['evidenceStatus'], str) and isinstance(row['staleReasons'], list) and all(isinstance(reason, str) for reason in row['staleReasons']), 'invalid-evidence-projection-fields')
    require({key: value for key, value in stored.items() if key not in derived} ==
            {key: value for key, value in public.items() if key not in derived}, 'stored-history-contract-differs-from-public-witness')


def check_partial_history(config, root, receipt, original_closure):
    # These root-supplied exact paths are read only when root later executes the
    # preflight. They are not discovered or read while preparing this script.
    current = {}
    snapshots = {}
    require(config['inputs'].get('evidenceProjectionSource', {}).get('sha256') == '00f5d99d4c4fdbab969a235b46e92e711ae7dd15c5bc0ba0cabce6f9e6ddc309' and
            config['inputs'].get('checkReadSource', {}).get('sha256') == 'cd7a2a28c0125566a3ddeccef823ea12676a77232d14423a0cd54609db5e999e', 'stored-public-projection-contract-not-pinned')
    require(config['inputs'].get('independentClosure', {}).get('sha256') == 'a5ee75e8e8aa9fefc7d96c3e7dfb9a6af78e48e8ac60c5005db9b429688e57d5' and
            original_closure['fixture'] == str(root) and original_closure['fixtureUnchanged'] is True and
            original_closure['fixtureInventoryBefore']['rows'] == original_closure['fixtureInventoryAfter']['rows'], 'original-raw-history-inventory-not-bound')
    for name in ('targetHistoryRelativePath', 'historyHistoryRelativePath'):
        relative = Path(config[name])
        require(not relative.is_absolute() and '..' not in relative.parts and relative.parts[0] == 'state', 'history-path-outside-exact-fixture')
        path = root / relative
        data = ordinary(path)
        witnesses = [row for row in original_closure['fixtureInventoryBefore']['rows'] if row[0] == str(relative)]
        require(len(witnesses) == 1 and witnesses[0][1] == len(data) and witnesses[0][2] == digest(data), 'stored-history-bytes-differ-from-original-closure')
        rows = json.loads(data)
        require(isinstance(rows, list) and len(rows) == 1, 'unexpected-partial-fixture-history')
        current[name] = rows[0]
        snapshots[name] = {'relativePath': str(relative), 'rawSHA256': digest(data)}
    target = current['targetHistoryRelativePath']
    check_target(config, target)
    before = [row['check'] for row in receipt['phases'] if row['phase'] == 'terminal-check-capacity' and row['check']['id'] == config['targetCheckId']]
    require(len(before) == 1 and before[0]['status'] == 'unknown' and before[0]['resourcesHeld'] is True, 'original-target-uncertainty-witness-missing')
    for key in ('id', 'projectId', 'commit', 'sourceDigest'):
        require(target[key] == before[0][key], 'retained-target-context-changed')
    for key in ('allocationId', 'label', 'executable', 'args', 'output', 'truncated'):
        require(target['commands'][0].get(key) == before[0]['commands'][0].get(key), 'retained-target-command-evidence-changed')
    history = current['historyHistoryRelativePath']
    original = [row['check'] for row in receipt['phases'] if row['phase'] == 'terminal-check-capacity' and row['check']['id'] == history['id']]
    require(len(original) == 1, 'unrelated-historical-public-witness-missing')
    compare_stored_public_history(history, original[0])
    require(history['status'] == 'passed' and history['resourcesHeld'] is False and history['commands'][0]['cleanupConfirmed'] is True and history['commands'][0]['exitCode'] == 0 and history['commands'][0]['output'] == 'RECOVERY_CHECK_OK\n', 'unrelated-historical-success-not-preserved')
    require(history['projectId'] != target['projectId'], 'history-witness-not-independent-project')
    snapshots['privateDerivedProjectionComparison'] = {
        'scope': 'Stored and previously captured public values only; no current source/evidence recomputation or fresh API observation.',
        'stored': {key: history[key] for key in ('evidenceStatus', 'staleReasons')},
        'recordedAPI': {key: original[0][key] for key in ('evidenceStatus', 'staleReasons')}}
    return snapshots


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('config', type=Path)
    args = parser.parse_args()
    require(sys.platform == 'linux', 'linux-only')
    os.umask(0o077)
    config_bytes = ordinary(args.config.absolute())
    config = json.loads(config_bytes)
    for key in ('productCommit', 'helperCommit'):
        require(re.fullmatch('[a-f0-9]{40}', config[key]), 'invalid-source-pin')
    for key in ('markerId', 'originalContainerId'):
        require(re.fullmatch('[a-f0-9]{64}', config[key]), 'invalid-container-pin')
    root = Path(config['fixtureRoot'])
    require(root.is_absolute() and root.resolve(strict=True) == root and root.is_dir() and root.name.startswith('luci-check-recovery-'), 'exact-fresh-fixture-root-required')
    require(stat.S_IMODE(root.stat().st_mode) & 0o077 == 0 and root.stat().st_uid == os.getuid(), 'fixture-root-not-private-owned')
    output = Path(config['output'])
    require(output.is_absolute() and output.parent.resolve(strict=True) == output.parent and root not in output.parents and output != root, 'private-external-output-required')
    output.mkdir(mode=0o700)
    result = {'version': 1, 'passed': False, 'at': now(), 'configSHA256': digest(config_bytes),
              'scope': 'Read-only preparation; no fixture retirement, Docker deletion, or authorization to mutate.', 'errors': []}
    try:
        values = {}
        pinned_dir = output / 'pinned-inputs'
        pinned_dir.mkdir(mode=0o700)
        for key, pin in config['inputs'].items():
            require(re.fullmatch('[A-Za-z][A-Za-z0-9_-]{0,63}', key), 'invalid-input-key')
            data = ordinary(pin['path'])
            require(digest(data) == pin['sha256'], 'pinned-input-hash-mismatch')
            (pinned_dir / key).write_bytes(data)
            if key in ('recoveryReceipt', 'wrapperExit', 'preexistingProcessProof', 'independentClosure'):
                values[key] = json.loads(data)
        receipt = check_receipts(config, values)
        result['originalRunPassed'] = receipt['passed']
        result['originalFailurePreserved'] = receipt.get('failure')
        result['partialRetirementAllowance'] = config.get('partialAttempt6Retirement')
        result['closureBefore'] = {}
        closure(config, result['closureBefore'], values.get('preexistingProcessProof'))
        ledger_path = root / 'state/control-plane/execution/leases.json'
        ledger_bytes = ordinary(ledger_path)
        ledger = json.loads(ledger_bytes)
        if config.get('partialAttempt6Retirement') is not None:
            require(digest(json.dumps(ledger, separators=(',', ':'), ensure_ascii=False).encode()) == receipt['independentClosure']['ledgerSHA256'], 'current-ledger-differs-from-original-closure')
            result['partialHistoryBefore'] = check_partial_history(config, root, receipt, values['independentClosure'])
        require(ledger['version'] == 1 and ledger.get('leases', []) == [] and not [row for row in ledger.get('checks', []) if row['state'] != 'finished'], 'retained-resource-charge')
        fences = [row for row in ledger.get('checkFences', []) if row['identity']['allocationId'] == config['allocationId']]
        require(len(fences) == 1 and len(ledger.get('checkFences', [])) == 1, 'unexpected-fence-inventory')
        fence = fences[0]
        require(fence['state'] == 'retired' and fence['proof']['fenceRef'] == config['markerId'] and fence['identity']['checkId'] == config['targetCheckId'] and fence['identity']['engineId'] == config['engineId'], 'retired-proof-identity-mismatch')
        require(fence['proof']['identity'] == fence['identity'] and fence['proof']['intentId'] == fence['intentId'], 'retired-proof-binding-mismatch')
        proof_phases = [row for row in receipt['phases'] if row['phase'] == 'positive-fence-proof']
        require(len(proof_phases) == 1 and proof_phases[0]['proof'] == fence['proof'] and proof_phases[0]['fenceId'] == config['markerId'] and proof_phases[0]['originalId'] == config['originalContainerId'], 'current-proof-differs-from-successful-recovery')
        require(receipt['independentClosure']['containers'] == [config['markerId']] and receipt['independentClosure']['volumes'] == [] and receipt['independentClosure']['activeChecks'] == [], 'original-closure-inventory-mismatch')
        result['markerBefore'] = marker(config, fence)
        result['ledgerRawSHA256'] = digest(ledger_bytes)
        result['retiredProofJSONSHA256'] = digest(json.dumps(fence, sort_keys=True, separators=(',', ':')).encode())
        manifest = fixture_manifest(root)
        write_new(output / 'fixture-manifest.json', manifest)
        result['archive'] = archive_fixture(root, output, manifest)
        result['closureAfter'] = {}
        closure(config, result['closureAfter'], values.get('preexistingProcessProof'))
        result['markerAfter'] = marker(config, fence)
        require(ordinary(ledger_path) == ledger_bytes, 'ledger-changed-after-archive')
        if config.get('partialAttempt6Retirement') is not None:
            result['partialHistoryAfter'] = check_partial_history(config, root, receipt, values['independentClosure'])
            require(result['partialHistoryAfter'] == result['partialHistoryBefore'], 'partial-history-changed-during-preflight')
        require(fixture_manifest(root) == manifest, 'fixture-changed-after-final-closure')
        require(result['markerBefore']['inspection'] == result['markerAfter']['inspection'], 'marker-inspection-changed')
        result['knownExitedZombies'] = [row for row in result['closureAfter']['known'] if row['result'] == 'exited-zombie']
        result['passed'] = True
        result['rootReviewRequired'] = ['Known exited PID1 browser zombies are not absent or reaped.', 'Read-only observations are point-in-time; repeat immediately before any move or immutable-ID deletion.', 'No automatic move/tombstone/delete is supplied. Keep the permanent marker until separate terminal nonresumption steps are reviewed.']
    except Exception as error:
        # Do not copy path-bearing exception text or credential-bearing values.
        result['errors'].append({'type': type(error).__name__, 'code': str(error) if isinstance(error, RuntimeError) else 'observation-or-input-error'})
    result['finishedAt'] = now()
    write_new(output / 'preflight-result.json', result)
    print(json.dumps({'passed': result['passed'], 'result': str(output / 'preflight-result.json'), 'fixtureMutationPerformed': False}))
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
