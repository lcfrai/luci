#!/usr/bin/env python3
"""Publish exact completed frontend outputs and source bindings, without rerunning."""
from pathlib import Path
from collections import Counter
import datetime,gzip,hashlib,json,re,shutil,subprocess
ROOT=Path('/workspace/luci');BASE=Path('/workspace/.luci-tools/frontend-finalcombined-d52');RUN=BASE/'20261009T001048Z';DEST=ROOT/'docs/luci/evidence/frontend-finalcombined-20261009T001048Z-d52f7fe3'
sha=lambda b:hashlib.sha256(b).hexdigest()
read=lambda name:json.loads((RUN/name).read_text())
complete=read('completed.json');commands=read('commands.json');report=read('vitest.json');before=read('frontend-source-before.json');after=read('typecheck-source-after.json')
assert complete['commandsCompleted']==2
expected={p for p in before['files']if re.fullmatch(r'desktop/src/.*\.test\.(ts|tsx)',p)}
reported={str(Path(r['name']).relative_to(ROOT))for r in report['testResults']}
status=Counter(a['status']for r in report['testResults']for a in r['assertionResults']);suites=Counter(r['status']for r in report['testResults'])
failures=[{'file':str(Path(r['name']).relative_to(ROOT)),'suiteStatus':r['status'],'suiteMessage':r.get('message',''),'assertions':[a for a in r['assertionResults']if a['status']=='failed']}for r in report['testResults']if r['status']=='failed'or any(a['status']=='failed'for a in r['assertionResults'])]
changed=[p for p in sorted(set(before['files'])|set(after['files']))if before['files'].get(p)!=after['files'].get(p)]
counts={k:report.get(k)for k in ['numTotalTestSuites','numPassedTestSuites','numFailedTestSuites','numPendingTestSuites','numTotalTests','numPassedTests','numFailedTests','numPendingTests','numTodoTests','success','wasInterrupted']}
summary={'version':1,'finishedAt':complete['finishedAt'],'sourceCommitAtStart':before['commit'],'sourceCommitAtEnd':after['commit'],'scope':'One full configured desktop Vitest suite, maxWorkers2, plus one npm run typecheck. No browser, installed app, build, performance or full product acceptance execution.','sourceBinding':{'frontendInputsUnchanged':not any(p.startswith('desktop/')for p in changed),'allCapturedInputsUnchanged':not changed,'changedPaths':changed,'initialCheckoutDirty':bool(before['statusPorcelain']),'initialStatus':before['statusPorcelain'],'finalStatus':after['statusPorcelain'],'note':'The frontend source started at d52f7fe3; concurrent backend/quality work makes whole-checkout clean-revision claims invalid. Exact imported external raw-source inputs are also hashed before/after.'},'commands':commands,'vitestReportedCounts':counts,'testFileCoverage':{'configuredFiles':len(expected),'reportedFiles':len(reported),'missingFiles':sorted(expected-reported),'unexpectedFiles':sorted(reported-expected),'everyConfiguredFileReported':expected==reported,'scope':'Configured include src/**/*.test.{ts,tsx}, compared with raw reporter file names.'},'assertionStatusCounts':dict(status),'fileStatusCounts':dict(suites),'failedFiles':failures,'limitations':['Component/jsdom tests and typechecking do not establish installed latency, browser responsiveness, real-model quality, platform behavior or sustained acceptance.','Original raw stdout/stderr retained even when empty. Harness warnings are distinct from failed assertions.','Expected-failure classification is retained when exposed by the raw reporter; no unexposed classification is inferred from a passed assertion.']}
summary['validationPassed']=all(c['exitCode']==0 for c in commands)and report.get('success')is True and not failures and expected==reported and not(status.get('pending',0)or status.get('todo',0)or status.get('skipped',0))and summary['sourceBinding']['frontendInputsUnchanged']
DEST.mkdir(parents=True,exist_ok=False)
def put(name,b):p=DEST/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(b)
def save(name,data):put(name,(json.dumps(data,indent=2)+'\n').encode())
for name in ['invocation.json','commands.json','completed.json','frontend-source-before.json','frontend-source-after.json','typecheck-source-before.json','typecheck-source-after.json','frontend.stdout','frontend.stderr','typecheck.stdout','typecheck.stderr']:put(name,(RUN/name).read_bytes())
raw=(RUN/'vitest.json').read_bytes();put('vitest.json.gz',gzip.compress(raw,mtime=0));assert gzip.decompress((DEST/'vitest.json.gz').read_bytes())==raw
save('result.json',summary)
for name in ['desktop/package.json','desktop/package-lock.json','desktop/tsconfig.json','desktop/tsconfig.node.json','desktop/vitest.config.ts','desktop/vite.config.ts']:
    raw_input=(ROOT/name).read_bytes();assert sha(raw_input)==before['files'][name]['sha256']==after['files'][name]['sha256'];put('inputs/'+name,raw_input)
put('run-verification.py',(BASE/'run-verification.py').read_bytes());put('publish-verification.py',Path(__file__).read_bytes())
if(BASE/'idle-launch-procedure.md').exists():put('idle-launch-procedure.md',(BASE/'idle-launch-procedure.md').read_bytes())
raw_hash=sha(raw)
text=f'''# Combined frontend verification — 9 October 2026

One full current desktop Vitest run with **two workers** completed, followed by TypeScript. The raw reporter records **{counts['numPassedTests']} passed**, **{counts['numFailedTests']} failed**, **{counts['numPendingTests']} pending**, and **{counts.get('numTodoTests',0)} todo** tests across **{len(reported)} reported files**. All **{len(expected)} configured test files** are accounted for; missing files: **{len(expected-reported)}**. Vitest exit code: **{commands[0]['exitCode']}**. TypeScript exit code: **{commands[1]['exitCode']}**. Final validation passed: **{summary['validationPassed']}**.

The frontend source includes the current CLOUD-033 cleanup messaging, CLOUD-034 shared-reservation summary, and later definition/team-context render reductions. It is bound to source commit **{before['commit']}** and explicit before/after file hashes. Frontend/config/lock inputs stayed unchanged: **{summary['sourceBinding']['frontendInputsUnchanged']}**. All captured inputs stayed unchanged: **{summary['sourceBinding']['allCapturedInputsUnchanged']}**. The checkout included concurrent backend/quality work; this is not represented as a clean whole-checkout revision. Any changed imported Go/document inputs are listed in `result.json`.

The exact commands, working directory, installed tool versions, timestamps, durations and exit codes are in `invocation.json` and `commands.json`. Source snapshots bind all frontend source, tests, scripts, package/config/lock files and the external Go/document/fixture files imported by the frontend contract tests. Stable config/lock bytes are included under `inputs/`. Source bytes are otherwise retrievable by commit plus the recorded status/hashes.

`vitest.json.gz` decompresses to the complete original JSON reporter bytes (SHA-256 **{raw_hash}**). Original stdout and stderr are preserved, including empty files. React `act` and jsdom canvas warnings, when present, remain in stderr and are not converted into failed assertions. `result.json` keeps command, file and assertion outcomes distinct rather than treating an empty or partial run as success. `SHA256SUMS` and `manifest.json` bind the included artifacts.

No browser, installed application, package build or performance diagnostic was launched. Passing these checks does not establish the original 300 ms switch or 100 ms input p95 targets; the latest measured d348 result still failed them. An optional attached idle-launch note is preparation only and requires the root's clean matching package/helper pin and exclusive runtime window.
'''
put('README.md',text.encode())
files={str(p.relative_to(DEST)):{'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())}for p in sorted(DEST.rglob('*'))if p.is_file()}
save('manifest.json',{'version':1,'originalJSONBytes':len(raw),'originalJSONSHA256':raw_hash,'files':files})
put('SHA256SUMS',''.join(sha(p.read_bytes())+'  '+str(p.relative_to(DEST))+'\n'for p in sorted(DEST.rglob('*'))if p.is_file()and p.name!='SHA256SUMS').encode())
for p,v in files.items():assert sha((DEST/p).read_bytes())==v['sha256']
print(json.dumps({'publication':str(DEST),'summary':summary,'bytes':sum(p.stat().st_size for p in DEST.rglob('*')if p.is_file())},indent=2))
