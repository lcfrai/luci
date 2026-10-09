from pathlib import Path
import json,hashlib
R=Path(__file__).resolve().parent;sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
r=json.loads((R/'inert-final-attempt3.receipt.json').read_text());assert r['passed'] and r['inputsUnchanged']
for name,value in r['inputHashesAfter'].items():assert sha(R/name)==value,name
binding=json.loads((R/'actual/external-source-binding.json').read_text())
for name,value in binding['localSources'].items():assert sha(R/'actual'/name)==value,name
for name,value in binding['repositoryDependencies'].items():assert sha(Path('/workspace/luci')/name)==value,name
for name in ['layout-trace-capture.mjs','layout-trace-projector.mjs']:assert (R/name).read_bytes()==(R/'actual'/name).read_bytes()
assert 'tests 39' in (R/'inert-final-attempt3-1.stdout.log').read_text()
readiness={'readyToLaunch':True,'runtimeLaunched':False,'productSource':binding['prospectiveProduct'],'helperSource':binding['prospectiveHelper'],'scope':'Eight diagnostic rounds/32 original actions only. No primary benchmark or latency acceptance. Root exclusive runtime grant required.','tests':{'uniqueFinalPassed':39,'receipt':'inert-final-attempt3.receipt.json','receiptSHA256':sha(R/'inert-final-attempt3.receipt.json'),'inputsUnchanged':True},'commandSHA256':sha(R/'launch-command.json'),'launcherSHA256':sha(R/'launch.py'),'driverSHA256':sha(R/'actual/sustained.mjs'),'captureSHA256':sha(R/'actual/layout-trace-capture.mjs'),'projectorSHA256':sha(R/'actual/layout-trace-projector.mjs'),'bindingSHA256':sha(R/'actual/external-source-binding.json'),'childReviewScope':'Pure projector schema/clock/private-field/stack boundary implementation and review,17 final controls. Parent implemented capture + actual orchestration,22 additional unique controls. No actual browser/build.'}
(R/'readiness.json').write_text(json.dumps(readiness,indent=2)+'\n')
files=[{'path':str(p.relative_to(R)),'bytes':p.stat().st_size,'sha256':sha(p)} for p in sorted(R.rglob('*')) if p.is_file() and p.name not in {'manifest.json','SHA256SUMS'}]
(R/'manifest.json').write_text(json.dumps({'version':1,'runtimeLaunched':False,'scope':'Exact external source/preparation/test history; no actual trace/runtime evidence yet.','files':files},indent=2)+'\n')
rows=[(x['path'],x['sha256']) for x in files]+[('manifest.json',sha(R/'manifest.json'))]
(R/'SHA256SUMS').write_text(''.join(h+'  '+n+'\n' for n,h in sorted(rows)))
for n,h in rows:assert sha(R/n)==h
print(json.dumps({'filesIncludingChecksum':len(rows)+1,'manifestSHA256':sha(R/'manifest.json'),'checksumSHA256':sha(R/'SHA256SUMS'),'readiness':readiness}))
