from pathlib import Path
import json,hashlib
R=Path(__file__).resolve().parent;sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
r=json.loads((R/'v2-inert-attempt1.receipt.json').read_text());assert r['passed'] and r['inputsUnchanged']
for name,value in r['inputHashesAfter'].items():assert sha(R/name)==value,name
binding=json.loads((R/'actual/external-source-binding.json').read_text())
for name,value in binding['localSources'].items():assert sha(R/'actual'/name)==value,name
for name,value in binding['repositoryDependencies'].items():assert sha(Path('/workspace/luci')/name)==value,name
for name in ['layout-trace-capture.mjs','layout-trace-projector.mjs']:assert (R/name).read_bytes()==(R/'actual'/name).read_bytes()
assert 'tests 23' in (R/'v2-inert-attempt1-1.stdout.log').read_text()
readiness={'readyToLaunch':True,'runtimeLaunched':False,'productSource':binding['prospectiveProduct'],'helperSource':binding['prospectiveHelper'],'scope':'V2 coarse Layout/style caller attribution: fine invalidationTracking omitted, exact four categories/exclude-all. Eight diagnostic rounds/32 original actions; unchanged guards. No primary acceptance. Root exclusive runtime grant required.','tests':{'affectedCurrentPassed':23,'unchangedProjectorHistoricalPassed':17,'receipt':'v2-inert-attempt1.receipt.json','receiptSHA256':sha(R/'v2-inert-attempt1.receipt.json'),'inputsUnchanged':True},'commandSHA256':sha(R/'launch-command.json'),'launcherSHA256':sha(R/'launch.py'),'driverSHA256':sha(R/'actual/sustained.mjs'),'captureSHA256':sha(R/'actual/layout-trace-capture.mjs'),'projectorSHA256':sha(R/'actual/layout-trace-projector.mjs'),'bindingSHA256':sha(R/'actual/external-source-binding.json'),'childReviewScope':'Pure projector unchanged from17 V1 controls. Local protocol/client review found no observed default-only leakage; V2 omits fine invalidation detail explicitly.23 affected current controls include one new exact category-transmission regression. No actual browser/build.'}
(R/'readiness.json').write_text(json.dumps(readiness,indent=2)+'\n')
files=[{'path':str(p.relative_to(R)),'bytes':p.stat().st_size,'sha256':sha(p)} for p in sorted(R.rglob('*')) if p.is_file() and p.name not in {'manifest.json','SHA256SUMS'}]
(R/'manifest.json').write_text(json.dumps({'version':1,'runtimeLaunched':False,'scope':'Exact external source/preparation/test history; no actual trace/runtime evidence yet.','files':files},indent=2)+'\n')
rows=[(x['path'],x['sha256']) for x in files]+[('manifest.json',sha(R/'manifest.json'))]
(R/'SHA256SUMS').write_text(''.join(h+'  '+n+'\n' for n,h in sorted(rows)))
for n,h in rows:assert sha(R/n)==h
print(json.dumps({'filesIncludingChecksum':len(rows)+1,'manifestSHA256':sha(R/'manifest.json'),'checksumSHA256':sha(R/'SHA256SUMS'),'readiness':readiness}))
