#!/usr/bin/env python3
"""Contract tests using frozen private row copies, never live fixture readers."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
BASE = HERE.parent


def load(path, name):
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


old = load(HERE / 'before-preflight.py', 'before_projection_checker')
new = load(BASE / 'preflight.py', 'after_projection_checker')
config = json.loads((BASE / 'attempt6-root-review-config-retry1.json').read_text())
config['inputs']['evidenceProjectionSource'] = {'sha256': '00f5d99d4c4fdbab969a235b46e92e711ae7dd15c5bc0ba0cabce6f9e6ddc309'}
config['inputs']['checkReadSource'] = {'sha256': 'cd7a2a28c0125566a3ddeccef823ea12676a77232d14423a0cd54609db5e999e'}
receipt = json.loads(Path(config['inputs']['recoveryReceipt']['path']).read_text())
closure = json.loads(Path(config['inputs']['independentClosure']['path']).read_text())
root = Path(config['fixtureRoot'])
raw = {str(root / config[key]): (HERE / (key + '.raw.json')).read_bytes()
       for key in ('targetHistoryRelativePath', 'historyHistoryRelativePath')}
old.ordinary = lambda path: raw[str(path)]
new.ordinary = lambda path: raw[str(path)]
cases = []


def expect(name, callback, reject=False):
    try:
        result = callback()
        assert not reject, 'Expected refusal did not occur'
        cases.append({'name': name, 'passed': True})
        return result
    except RuntimeError as error:
        assert reject, str(error)
        cases.append({'name': name, 'passed': True, 'expectedRefusal': str(error)})


expect('original checker rejects exact preserved raw/API history at derived fields',
       lambda: old.check_partial_history(config, root, receipt), True)
expect('corrected checker validates exact raw inventory and stored/API execution contract',
       lambda: new.check_partial_history(config, root, receipt, closure))

diagnostic = json.loads((HERE / 'diagnostic-private.json').read_text())
stored = diagnostic['captures']['historyHistoryRelativePath']['stored']
public = diagnostic['captures']['historyHistoryRelativePath']['recordedAPI']
expect('only two documented derived display fields may differ', lambda: new.compare_stored_public_history(stored, public))

mutations = [
    ('revision', lambda row: row.update(revision=row['revision'] + 1)),
    ('output', lambda row: row['commands'][0].update(output='ALTERED')),
    ('source digest', lambda row: row.update(sourceDigest='0' * 64)),
    ('source commit', lambda row: row.update(commit='0' * 40)),
    ('specification revision', lambda row: row.update(specificationRevision=row['specificationRevision'] + 1)),
    ('design digest', lambda row: row.update(designPlanDigest='0' * 64)),
    ('allocation identity', lambda row: row['commands'][0].update(allocationId='different')),
    ('command exit', lambda row: row['commands'][0].update(exitCode=9)),
    ('command arguments', lambda row: row['commands'][0].update(args=['other'])),
    ('historical timestamp', lambda row: row.update(updatedAt='different')),
]
for name, mutation in mutations:
    changed = copy.deepcopy(stored)
    mutation(changed)
    expect('reject changed stored ' + name, lambda changed=changed: new.compare_stored_public_history(changed, public), True)

history_path = str(root / config['historyHistoryRelativePath'])
original = raw[history_path]
for field in ('revision', 'evidenceStatus'):
    changed = json.loads(original)
    changed[0][field] = changed[0][field] + 1 if field == 'revision' else 'changed-stored-derived-value'
    raw[history_path] = json.dumps(changed).encode()
    expect('raw inventory still rejects altered ' + field + ' bytes', lambda: new.check_partial_history(config, root, receipt, closure), True)
    raw[history_path] = original

other_closure = copy.deepcopy(closure)
other_closure['fixtureInventoryAfter']['rows'] = []
expect('reject incomplete or changed original raw retention witness', lambda: new.check_partial_history(config, root, receipt, other_closure), True)

result = {'passed': True, 'mode': 'Frozen private copies and fake file readers only; no live fixture/API/Docker/process observation.',
          'caseCount': len(cases), 'cases': cases,
          'beforeSourceSHA256': hashlib.sha256((HERE / 'before-preflight.py').read_bytes()).hexdigest(),
          'afterSourceSHA256': hashlib.sha256((BASE / 'preflight.py').read_bytes()).hexdigest(),
          'privateDiagnosticSHA256': hashlib.sha256((HERE / 'diagnostic-private.json').read_bytes()).hexdigest(),
          'liveFixtureReads': 0, 'archiveOrMutation': False}
with (HERE / 'controlled-result.json').open('x') as output:
    json.dump(result, output, indent=2); output.write('\n')
print(json.dumps({'passed': True, 'cases': len(cases), 'originalMismatchRetained': True}))
