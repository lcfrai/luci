#!/usr/bin/env python3
"""Publish retained C056 evidence without modifying historical captures or repository files."""
import argparse
import hashlib
import json
import pathlib
import shutil

parser = argparse.ArgumentParser()
parser.add_argument('--destination', required=True)
parser.add_argument('--candidate-revision')
args = parser.parse_args()
stage = pathlib.Path(__file__).resolve().parent
destination = pathlib.Path(args.destination).resolve()
if destination.exists():
    raise SystemExit('Refusing existing capsule; preserve its bytes')

def sha(data):
    return hashlib.sha256(data).hexdigest()

def read(name):
    return json.loads((stage / name).read_text())

initial = read('integrated-validation/result.json')
final = read('final-integrated-validation/result.json')
initial_tests = read('integrated-validation/frontend-report.json')
final_tests = read('final-integrated-validation/frontend-report.json')
if not final['sourceStable'] or not final['allCommandsExitZero']:
    raise SystemExit('Final validation did not pass')
if (final_tests['numPassedTests'], final_tests['numFailedTests']) != (46, 0):
    raise SystemExit('Unexpected final affected test count')
if (initial_tests['numPassedTests'], initial_tests['numFailedTests']) != (383, 0):
    raise SystemExit('Unexpected initial affected test count')

selected = [
    'source-bindings.json', 'preparation-manifest.json', 'source-review.json',
    'correction-manifest.json', 'integration-preparation-manifest.json',
    'authority-window.test.tsx', 'authority-regression.test.tsx',
    'team_session_denial_contract_test.go', 'vitest.config.ts', 'regression.config.mts',
    'run-original.py', 'run-regression.py', 'validate-integrated.py', 'validate-final.py',
    'make-capsule.py', 'make-final-capsule.py', 'cloud056.patch', 'HANDOFF.md',
    'INTEGRATION-VALIDATION.md', 'FINAL-VALIDATION.md', 'frozen', 'after',
    'repo-ready', 'repo-ready-final', 'original-attempt1',
    'regression-original-attempt1', 'regression-fixed-attempt1',
    'integrated-validation', 'episode-followup', 'final-integrated-validation',
]
for name in selected:
    source = stage / name
    if not source.exists() or source.is_symlink():
        raise SystemExit('Missing/linked selected input: ' + name)
    if source.is_dir() and any(p.is_symlink() for p in source.rglob('*')):
        raise SystemExit('Linked descendant: ' + name)
destination.mkdir(parents=True)
for name in selected:
    source, target = stage / name, destination / name
    if source.is_dir():
        shutil.copytree(source, target)
    else:
        shutil.copyfile(source, target)
shutil.copyfile(stage / 'README.md', destination / 'PREPARATION-README.md')

final_files = {
    'desktop/src/api/client.ts': 'after/desktop/src/api/client.ts',
    'desktop/src/api/teamScope.ts': 'after/desktop/src/api/teamScope.ts',
    'desktop/src/components/teams/TeamWorkspaceProvider.tsx': 'episode-followup/after.tsx',
    'desktop/src/components/teams/TeamWorkspaceAuthority.test.tsx': 'episode-followup/authority-test-final.tsx',
}
captured = {row['path']: row for row in read('final-integrated-validation/repository-inputs-before.json')}
final_bindings = []
for repository_path, source_path in final_files.items():
    content = (stage / source_path).read_bytes()
    if captured[repository_path]['sha256'] != sha(content) or captured[repository_path]['bytes'] != len(content):
        raise SystemExit('Final source differs from executed input: ' + repository_path)
    target = destination / 'final-source' / repository_path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(content)
    final_bindings.append({'repositoryPath': repository_path, 'capsulePath': str(target.relative_to(destination)),
                           'bytes': len(content), 'sha256': sha(content)})

status = {
    'issue': 'CLOUD-056',
    'baseProductRevision': 'edd0cb121ff6847f681ab6bf41509a497f043d41',
    'executedWorkingHEAD': read('final-integrated-validation/git-before.json')['head'],
    'candidateRevision': args.candidate_revision,
    'workingTreeCaveat': 'Three production overlays plus new test were integrated atop the recorded documentation HEAD; recorded Git status contains other root work. This is not a clean whole-checkout edd0 claim.',
    'finalSourceBindings': final_bindings,
    'runs': [
        {'path': 'original-attempt1', 'pass': 9, 'fail': 1, 'scope': 'Initial fresh denial counterexample and controls'},
        {'path': 'regression-original-attempt1', 'pass': 9, 'fail': 5, 'scope': 'Stable original fourteen-case regression'},
        {'path': 'regression-fixed-attempt1', 'pass': 14, 'fail': 0, 'scope': 'Same fourteen cases; initial three-source overlay'},
        {'path': 'integrated-validation', 'pass': 383, 'fail': 0, 'files': 14, 'typescriptExit': 2, 'goControlExit': 0,
         'scope': 'Initial integrated frontend passes; independent production registrar fixture passes; TypeScript fails on overbroad fetch mock type'},
        {'path': 'episode-followup/before-attempt1', 'pass': 0, 'fail': 4,
         'scope': 'Three valid call-count failures; fourth visibility assertion was fixture-confounded by a subsequent fresh successful membership response'},
        {'path': 'episode-followup/after-attempt1', 'pass': 4, 'fail': 0, 'scope': 'Preserved first corrected-source run against the earlier fixture'},
        {'path': 'episode-followup/before-attempt2', 'pass': 1, 'fail': 3, 'scope': 'Corrected fixture: three call-count failures, stale-success control passes'},
        {'path': 'episode-followup/after-attempt2', 'pass': 4, 'fail': 0, 'scope': 'Same corrected four cases; whole denied episode coalesces'},
        {'path': 'final-integrated-validation', 'pass': 46, 'fail': 0, 'files': 2, 'typescriptExit': 0,
         'scope': 'Final 18 authority regressions plus 28 provider tests; final three sources/test exactly match captured inputs'},
    ],
    'overlap': 'All prior, corrected and focused counts overlap. Do not sum into a unique-test total or call this a full frontend suite.',
    'finalRepositoryInputs': final['repositoryInputCount'],
    'finalSourceStable': final['sourceStable'],
    'typescriptListedInputs': final['typescriptListedInputCount'],
    'typescriptExternalScope': 'Actual listed inputs hashed after compiler exit; external declarations were not independently installed or prehashed. Existing node_modules lock observed; no npm ci in this validation.',
    'goScope': 'One controlled production management registrar test with external test-only overlay; not all compiled Go input provenance or live service/Docker behavior.',
    'behavior': 'Fresh current-scope authoritative member denial hides cached admitted UI. Reopening requires fresh current membership and projects/sessions visibility; repeated denials coalesce membership refresh but still fence old recovery. Retry/focus/periodic recovery remain.',
    'limits': [
        'Cached-display correction, not a backend authority bypass or new private-data delivery claim.',
        'Hidden mounted readers may still issue independent server-guarded reads; no blanket network suspension or erasure claim.',
        'Transient failure preserves draft/buffer bytes; confirmed membership absence retains existing disposal behavior.',
        'Controlled React/Vitest fake HTTP, WebSocket and terminal renderer; structural visibility, not browser paint or complete AppShell runtime.',
        'No installed/native/Windows/provider acceptance or responsiveness measurement.',
        'Owned test roots exited normally; no whole-machine/descendant process census claim.',
    ],
    'historicalNotes': 'PREPARATION-README.md, episode-followup/final-manifest.json and HANDOFF.md retain their earlier unexecuted-final wording. Final execution is established by final-integrated-validation and this status without rewriting old captures.',
}
(destination / 'STATUS.json').write_text(json.dumps(status, indent=2) + '\n')
(destination / 'README.md').write_text('''# CLOUD-056: current workspace denial and bounded recovery\n\nA fresh, current-generation session-list `403 team_access_denied` now hides the admitted workspace immediately. Reopening requires current membership plus fresh project/session visibility; restoring membership alone cannot show a previously cached project whose grant was removed. Stale scope results remain fenced. Failed recovery preserves drafts and retained buffers. Repeated denials in one blocked episode do not create repeated membership requests, while explicit Retry, focus and periodic refresh remain available. Hidden mounted readers can still perform independent server-guarded reads.\n\nThis is a cached-display correction. The backend already refuses fresh private reads after revocation. Neither a backend authorization bypass nor installed/browser/Windows acceptance is asserted. See `STATUS.json` for exact run scopes, failures, overlaps and remaining limits.\n\n`final-source/` is the exact final three-production-file and eighteen-case regression state captured by the final two-file validation. Initial broader validation passed 383 assertions but TypeScript failed on the new fixture's mock type; those original bytes remain intact. The typed fixture and denial-episode follow-up then passed 46 affected assertions and TypeScript. The small unchanged Go403 registrar control passed in the earlier batch and was not rerun.\n\nAll original failures are retained. The first episode fixture's fourth original assertion was confounded by its later successful membership fake; the revised control passes on both original and corrected sources. Three meaningful call-count failures persist before the change and pass afterward. Historical manifests describe their captured moment and are not rewritten to claim later execution.\n\nReproduce with `FINAL-VALIDATION.md` after applying the initial and final patches, using the repository's pinned activation and existing frozen dependency installation. Do not run tests during a separate installed performance run. Full command/config/source captures accompany every retained batch. Selected original42-file scope and later432-input frontend snapshots are different scopes; neither is a full dependency export.\n\nPublication requires explicitly adding all selected logs despite ignore rules, then verifying the publication manifest/checksum set against Git bytes. The capsule preserves controlled synthetic fixture strings and command paths, and contains no exported real application credentials or private runtime fixtures.\n''')

rows = []
for path in sorted(destination.rglob('*')):
    if path.is_file():
        data = path.read_bytes()
        rows.append({'path': str(path.relative_to(destination)), 'bytes': len(data), 'sha256': sha(data)})
manifest = {'schema': 1, 'issue': 'CLOUD-056', 'candidateRevision': args.candidate_revision,
            'scope': 'Original, corrected, initial integrated and final affected source evidence; no installed acceptance', 'files': rows}
(destination / 'publication-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
checks = rows + [{'path': 'publication-manifest.json', 'sha256': sha((destination / 'publication-manifest.json').read_bytes())}]
(destination / 'SHA256SUMS').write_text(''.join(row['sha256'] + '  ' + row['path'] + '\n' for row in checks))
print(json.dumps({'destination': str(destination), 'payloadFiles': len(rows), 'checksumEntries': len(checks),
                  'manifestSHA256': sha((destination / 'publication-manifest.json').read_bytes()),
                  'checksumFileSHA256': sha((destination / 'SHA256SUMS').read_bytes()),
                  'statusSHA256': sha((destination / 'STATUS.json').read_bytes())}))
