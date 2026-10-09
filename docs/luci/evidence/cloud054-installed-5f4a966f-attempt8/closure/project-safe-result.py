"""Publish only enumerated observation fields; never read private stderr/error files."""
from pathlib import Path
import hashlib, json

base = Path('/workspace/.luci-tools/cloud054-terminal-close/actual-5f4a966f/evidence/attempt8')
out = Path(__file__).with_name('safe-projection.json')
def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()
driver = json.loads((base / 'driver/result.json').read_text())
wrapper = json.loads((base / 'wrapper-result.json').read_text())
closure = json.loads((base / 'independent-closure.json').read_text())
assert sha(base / 'driver/result.json') == closure['originalResultSHA256']
assert sha(base / 'wrapper-result.json') == closure['originalWrapperSHA256']
observations = []
for phase in driver['phases']:
    if phase['kind'] not in ['typed-exit-before-poll', 'scenario-failure-observation']:
        continue
    wire = phase['wire']
    rows = []
    for row in wire['browser']['rows']:
        exits = []
        for event in row['exit']:
            assert event['reason']['kind'] == 'exit-status'
            assert type(event['reason']['code']) is int
            exits.append({'kind': 'exit-status', 'code': event['reason']['code'], 'sequence': event['sequence']})
        close = row.get('closed')
        rows.append({'ordinal': row['ordinal'], 'error': row['error'],
            'close': None if close is None else {'code': close['code'], 'wasClean': close['wasClean']},
            'exit': exits, 'markers': {k: {'sequence': v['sequence']} for k, v in row['markers'].items()},
            'closeEvents': [{'sequence': e['sequence'], 'code': e['code'], 'wasClean': e['wasClean']}
                for e in row['events'] if e['kind'] == 'close']})
    ui = phase['ui']
    observations.append({'phase': phase['kind'], 'observedAt': phase['observedAt'],
        'browserSocketCount': len(rows), 'browserRows': rows,
        'browserObserverFailureCount': len(wire['browser']['failures']),
        'cdpSocketCount': len(wire['protocol']['rows']),
        'cdpCloseFrameCount': sum(len(r['closeFrames']) for r in wire['protocol']['rows']),
        'ui': {'labelEndsRunning': ui['label'].endswith(' — running'),
               'labelEndsExited': ui['label'].endswith(' — exited'),
               'termPresent': ui['termPresent'], 'restartCount': ui['restartCount']}})
value = {'sourceCommit': driver['sourceCommit'], 'imageID': driver['daemon']['imageID'],
    'workflowPassed': driver['passed'], 'failedStage': driver['failedStage'],
    'failedStep': next(p['step'] for p in driver['phases'] if p['kind'] == 'scenario-failure'),
    'driverResultSHA256': closure['originalResultSHA256'], 'wrapperResultSHA256': closure['originalWrapperSHA256'],
    'independentClosureSHA256': sha(base / 'independent-closure.json'), 'observations': observations,
    'cleanupKinds': [row['kind'] for row in driver['cleanup']],
    'exactHelperRemovalIsNotProductRelease': any(row.get('notProductRelease') is True for row in driver['cleanup']),
    'cleanupPhases': [{'name': row['name'], 'status': row['status']} for row in driver['cleanupPhases']],
    'wrapperExitCode': wrapper['launcherExitCode'], 'wrapperSignalCount': len(wrapper['signals']),
    'pendingMutationCount': len(driver['pendingMutations']),
    'closure': {k: closure[k] for k in ['identities', 'knownLiveProcessCount', 'knownZombieCount',
        'knownUnavailableProcessCount', 'unreadableEnvironmentObservations', 'knownContainers', 'containers', 'volumes',
        'ledger', 'sameEngine', 'immutableImageVerified', 'originalFilesUnchanged', 'fixtureFilesUnchanged', 'sourceBindingsUnchanged']},
    'scope': 'Whitelisted fields from completed receipts only. No raw controller stderr, transport errors, frame payloads, environment values or fixture contents. First browser normal closure does not establish completed Restart or product-owned cleanup. CDP close-frame count is reported separately from browser CloseEvent evidence.'}
with out.open('x') as f:
    json.dump(value, f, indent=2)
    f.write('\n')
print(json.dumps({'path': str(out), 'sha256': sha(out), 'failedStep': value['failedStep'],
    'observations': observations, 'cleanupKinds': value['cleanupKinds']}))
