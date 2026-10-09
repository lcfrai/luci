#!/usr/bin/env python3
"""Read-only, post-join inspection of C075's exact recorded process identities.
No process census, environment reads, network requests, Docker CLI, signals,
or private credential files. One new report outside the fixture is the only write.
"""
from pathlib import Path
import argparse, datetime, hashlib, json, os, re, stat

PRODUCT = '786652704fa8bebd1ba1af41cfa1d414d7def100'
MAX_FILE = 4 * 1024 * 1024

def regular_bytes(path, limit=MAX_FILE):
    assert path.resolve() == path, 'Symlinked input refused'
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        before = os.fstat(fd)
        assert stat.S_ISREG(before.st_mode) and before.st_uid == os.getuid()
        assert before.st_nlink == 1 and before.st_size <= limit
        chunks = []
        while True:
            block = os.read(fd, min(65536, limit + 1))
            if not block: break
            chunks.append(block)
            assert sum(map(len, chunks)) <= limit
        after = os.fstat(fd)
        assert (before.st_dev, before.st_ino, before.st_mtime_ns, before.st_size) == (after.st_dev, after.st_ino, after.st_mtime_ns, after.st_size), 'Input changed while read'
        return b''.join(chunks)
    finally:
        os.close(fd)

def observation(path):
    try:
        data = regular_bytes(path)
    except FileNotFoundError:
        return {'exists': False}
    return {'exists': True, 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}

def read_json(path):
    return json.loads(regular_bytes(path))

def private_directory(path):
    assert path.resolve() == path
    value = path.lstat()
    assert stat.S_ISDIR(value.st_mode) and value.st_uid == os.getuid() and value.st_mode & 0o077 == 0

def process_observation(pid, expected_start):
    # Inspect only a PID already present in the bounded fixture-owned registry.
    try:
        data = Path('/proc', str(pid), 'stat').read_text()
    except FileNotFoundError:
        return {'status': 'original-absent'}
    except PermissionError:
        return {'status': 'unavailable'}
    fields = data.rsplit(') ', 1)[1].split()
    actual_start, state = fields[19], fields[0]
    if actual_start != expected_start:
        return {'status': 'original-absent-pid-reused', 'observedStartTicks': actual_start, 'observedState': state}
    return {'status': 'same-original-identity', 'observedStartTicks': actual_start, 'observedState': state, 'quiescentZombie': state == 'Z'}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run-root', required=True, type=Path)
    parser.add_argument('--parent', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    root, parent, output = args.run_root.absolute(), args.parent.absolute(), args.output.absolute()
    private_directory(parent); private_directory(root); private_directory(output.parent)
    assert root.parent == parent and re.fullmatch(r'attempt-[A-Za-z0-9_-]{1,64}', root.name)
    assert not output.is_relative_to(root) and not output.exists() and output.parent.resolve() == output.parent
    receipt_names = ['driver-start.json', 'driver-owned-identities.json', 'driver-managed-scopes.json', 'driver-result.json', 'evidence/result.json', 'evidence/events.jsonl']
    selected = [root / name for name in receipt_names]
    initial_receipts = {name: observation(root / name) for name in receipt_names}
    assert all(initial_receipts[name]['exists'] for name in ['driver-start.json', 'driver-owned-identities.json']), 'Exact observed ownership records are required; never guess identities'
    missing_receipts = [name for name in receipt_names if not initial_receipts[name]['exists']]
    start = read_json(root / 'driver-start.json')
    registry = read_json(root / 'driver-owned-identities.json')
    scopes = read_json(root / 'driver-managed-scopes.json') if initial_receipts['driver-managed-scopes.json']['exists'] else None
    driver = read_json(root / 'driver-result.json') if initial_receipts['driver-result.json']['exists'] else None
    result = read_json(root / 'evidence/result.json') if initial_receipts['evidence/result.json']['exists'] else {}
    for value in [value for value in [start, registry, scopes, driver, result] if value]:
        assert value['runRoot'] == str(root)
    for value in [value for value in [start, scopes, driver, result] if value]: assert value['product'] == PRODUCT
    if driver:
        assert driver['result']['path'] == 'evidence/result.json'
        assert driver['result']['sha256'] == initial_receipts['evidence/result.json'].get('sha256')
    identities = {}
    assert 0 < len(registry['identities']) <= 256
    for row in registry['identities']:
        assert isinstance(row['pid'], int) and row['pid'] > 1
        assert re.fullmatch(r'[0-9]{1,30}', str(row['startTicks']))
        assert row['role'] in ['controller', 'browser']
        key = (row['pid'], str(row['startTicks']))
        assert key not in identities
        identities[key] = row['role']
    root_identity = start['identity']
    assert root_identity['pid'] == start['pid'] and root_identity['pid'] > 1
    assert re.fullmatch(r'[0-9]{1,30}', str(root_identity['start']))
    identities[(root_identity['pid'], str(root_identity['start']))] = 'driver'
    observed = [{'pid': pid, 'expectedStartTicks': ticks, 'role': role, 'now': process_observation(pid, ticks)} for (pid, ticks), role in identities.items()]

    # Cross-check that the driver's last successful observation covers its final
    # registry. This is not independent discovery of unrecorded descendants.
    final_phase = next((row for row in result.get('cleanupPhases', []) if row.get('name') == 'final-observations' and row.get('status') == 'completed'), None)
    final_keys = set()
    if final_phase:
        for role, rows in final_phase['result']['identities'].items():
            assert role in ['controller', 'browser']
            for row in rows: final_keys.add((row['initial']['pid'], str(row['initial']['start'])))
    registry_keys = {(row['pid'], str(row['startTicks'])) for row in registry['identities']}
    registry_covered = bool(final_phase) and final_keys == registry_keys

    # Only exact non-credential state paths beneath manifest-declared own scopes.
    # Session registries are hashed, never parsed or copied (they may hold tokens).
    state_paths = [root / 'state/sessions/sessions.json', root / 'state/control-plane/execution/leases.json']
    if scopes: assert isinstance(scopes['scopes'], list) and len(scopes['scopes']) <= 3
    managed = []
    for scope in scopes['scopes'] if scopes else []:
        assert scope['kind'] in ['user', 'team'] and re.fullmatch(r'[A-Za-z0-9_-]{1,80}', scope['id'])
        location = root / 'state/control-plane' / ('teams' if scope['kind'] == 'team' else 'users') / scope['id'] / 'runtime'
        assert scope['stateDir'] == str(location / 'state')
        state_paths.extend([location / 'state/sessions/sessions.json', location / 'worker-address.json'])
        managed.append({'kind': scope['kind'], 'id': scope['id'], 'relativeStateDir': str((location / 'state').relative_to(root))})
    initial_state = {str(path.relative_to(root)): observation(path) for path in state_paths}
    socket_path = root / 'unavailable-docker.sock'
    docker_absent_before = not os.path.lexists(socket_path)
    final_state = {str(path.relative_to(root)): observation(path) for path in state_paths}
    final_receipts = {name: observation(root / name) for name in receipt_names}
    docker_absent_after = not os.path.lexists(socket_path)
    originals_absent = all(row['now']['status'].startswith('original-absent') for row in observed)
    originals_quiescent = all(row['now']['status'].startswith('original-absent') or row['now'].get('quiescentZombie', False) for row in observed)
    unchanged = initial_state == final_state and initial_receipts == final_receipts
    boundary = result.get('routeBoundary') or {}
    route_drained = boundary.get('sealed') is True and boundary.get('pendingCallbacks') == 0 and boundary.get('pendingMutations') == []
    passed = not missing_receipts and originals_absent and unchanged and registry_covered and route_drained and docker_absent_before and docker_absent_after
    report = {
        'version': 1, 'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'product': PRODUCT,
        'inspectionCompleted': True, 'postrunClosureConfirmed': passed, 'workflowPassed': result.get('passed'),
        'originalFailedStage': result.get('failedStage'), 'driverFirstFailure': result.get('firstFailure'),
        'originalDriverResultSHA256': initial_receipts['evidence/result.json'].get('sha256'), 'missingReceipts': missing_receipts,
        'observedIdentities': observed, 'allKnownOriginalIdentitiesAbsent': originals_absent,
        'allKnownOriginalIdentitiesQuiescent': originals_quiescent, 'driverFinalRegistryCovered': registry_covered,
        'managedScopes': managed, 'routeBoundaryDrainedAsRecorded': route_drained,
        'privateDockerSocketAbsentBeforeAndAfter': docker_absent_before and docker_absent_after,
        'selectedStateBefore': initial_state, 'selectedStateAfter': final_state,
        'selectedReceiptsBefore': initial_receipts, 'selectedReceiptsAfter': final_receipts, 'selectedFilesUnchanged': unchanged,
        'scriptSHA256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        'scope': 'Independent post-join read-only inspection of exact fixture-recorded PID/start identities and selected owned state/receipt hashes. No process census, proc environments, unrelated credentials, signals, network or Docker access. File before/after means before/after this inspection, not before/after the workflow. No discovery of unrecorded processes or whole-machine closure claim. Zombies remain explicitly present and prevent strict originals-absent closure confirmation.'
    }
    data = (json.dumps(report, indent=2) + '\n').encode()
    assert len(data) < 192 * 1024
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        offset = 0
        while offset < len(data): offset += os.write(fd, data[offset:])
        os.fsync(fd)
    finally: os.close(fd)
    print(json.dumps({'receipt': str(output), 'sha256': hashlib.sha256(data).hexdigest(), 'postrunClosureConfirmed': passed, 'identitiesInspected': len(observed), 'workflowPassed': result.get('passed')}))
    return 0 if passed else 2

if __name__ == '__main__':
    raise SystemExit(main())
