from pathlib import Path
import hashlib,json,subprocess,shutil,datetime
base=Path(__file__).resolve().parent;repo=Path('/workspace/luci');head='5f318d5178210e95cd2b773f07686d88936d603e';product='d20ebd292f3f1de04d6726bab0998c210987bcb5'
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def git(*args):return subprocess.check_output(['git',*args],cwd=repo,timeout=15)
reports={name:json.loads((base/'receipts'/name/'browser/report.json').read_text()) for name in ['controlled-final','controlled-lifecycle-final']}
assert all(r['passed'] and r['sourceUnchanged'] for r in reports.values())
assert [len(r['checks']) for r in reports.values()]==[12,13]
observer=sha(base/'observer/autonomous-readiness.mjs');ledger=sha(base/'observer/readiness-ledger.mjs')
for name in reports:
 command=json.loads((base/'receipts'/name/'command.json').read_text());assert command['exitCode']==0 and command['unchanged'];assert command['sourceSHA256']['observer/autonomous-readiness.mjs']==observer;assert command['sourceSHA256']['observer/readiness-ledger.mjs']==ledger
assert (base/'actual/autonomous-readiness.mjs').read_bytes()==(base/'observer/autonomous-readiness.mjs').read_bytes()
assert (base/'actual/readiness-ledger.mjs').read_bytes()==(base/'observer/readiness-ledger.mjs').read_bytes()
files=set(next(iter(reports.values()))['sourceBefore'])|{'desktop/src/components/projects/ProjectsHome.tsx','desktop/src/components/lens/DiscoverLens.tsx','desktop/src/components/journey/shared.tsx','desktop/src/components/ui/tabs-subtle.tsx','desktop/src/styles/app.css','desktop/src/components/orchestrator/fixtures.ts','desktop/src/test/fixtures.ts'}
# Bind exact Git source rather than racing later owner-authorized source changes.
source={}
for name in sorted(files):
 try:data=git('show',head+':'+name)
 except subprocess.CalledProcessError:
  # Only two extension variations are permitted for existing source-contract references.
  assert name in ['desktop/src/components/orchestrator/fixtures.ts','desktop/src/test/fixtures.ts'];continue
 target=base/'source-contract'/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data);source[name]={'sha256':sha(target),'bytes':len(data)}
for report in reports.values():
 for name,value in report['sourceAfter'].items():assert source[name]['sha256']==value
sameTrees={tree:git('rev-parse',head+':'+tree).strip()==git('rev-parse',product+':'+tree).strip() for tree in ['desktop','scripts/acceptance']}
assert all(sameTrees.values())
binding={'version':1,'productSource':product,'helperAtControlledValidation':head,'committedSourceTreesIdentical':sameTrees,'sourceContracts':source,'observerSHA256':observer,'ledgerSHA256':ledger,'validationScope':'Actual styled source components with synthetic API/provider fixture, no installed app/backend/worker/provider/performance run. Source unchanged in final browser receipts; root independently owns documentation publication and subsequent product edits.'}
(base/'source-binding.json').write_text(json.dumps(binding,indent=2)+'\n')
commands=[]
for name in ['actual/sustained.mjs','actual/autonomous-readiness.mjs','actual/readiness-ledger.mjs']:
 result=subprocess.run(['node','--check',str(base/name)],capture_output=True,text=True,timeout=15);commands.append({'command':['node','--check',name],'exitCode':result.returncode,'stdout':result.stdout,'stderr':result.stderr});assert result.returncode==0
compile((base/'launch.py').read_text(),'launch.py','exec');compile((base/'prepare.py').read_text(),'prepare.py','exec')
(base/'receipts/syntax.json').write_text(json.dumps({'commands':commands,'pythonCompileOnly':['launch.py','prepare.py'],'installedRuntimeExecuted':False},indent=2)+'\n')
main=reports['controlled-final'];proof=next(v for v in main['checks'] if v['label']=='autonomous-witness-precedes-controlled-late-driver')
summary={'browserChecks':{'controlled-final':12,'controlled-lifecycle-final':13},'ledgerTests':6,'uniqueCountClaim':False,'repeatedAudits':'Same-origin GET and forbidden API source scan repeated in both browser batches. Prior passing intermediates are not additional cases.','controlledLateDriver':proof,'retainedFailures':['controlled-attempt1: Chromium socket path too long, no action ran','controlled-lifecycle-attempt1: old-node replacement safely censored as observed disconnection before test expected retained-editor'],'originalLateOnly':'No autonomous witness, expected limitation; not a fabricated production before-failure.','installedRuntimeExecuted':False}
(base/'validation-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
# Freeze all meaningful sources and receipts; ignore regenerable browser cache.
capsule=base/'capsule';assert not capsule.exists();capsule.mkdir()
rootnames=['README.md','PLAN-original.md','peer-review.md','prepare.py','launch.py','freeze.py','integration.patch','preparation.json','template-capture.json','source-binding.json','validation-summary.json']
paths=[base/name for name in rootnames]
for directory in ['observer','controlled','original','actual','source-contract','receipts']:
 for p in (base/directory).rglob('*'):
  if p.is_file() and not p.is_symlink() and 'vite-cache' not in p.parts and '__pycache__' not in p.parts and not any(x.startswith('profile-') for x in p.parts):paths.append(p)
for p in paths:
 target=capsule/p.relative_to(base);target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(p,target)
rows=[{'path':str(p.relative_to(capsule)),'bytes':p.stat().st_size,'sha256':sha(p)} for p in sorted(capsule.rglob('*')) if p.is_file()]
manifest={'version':1,'createdAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Frozen external review capsule; no installed run. Original failures retained; reproducible Vite caches excluded.','members':rows,'payloadBytes':sum(r['bytes'] for r in rows)}
(capsule/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
(capsule/'SHA256SUMS').write_text(''.join(r['sha256']+'  '+r['path']+'\n' for r in rows)+sha(capsule/'manifest.json')+'  manifest.json\n')
print(json.dumps({'capsule':str(capsule),'payloadMembers':len(rows),'payloadBytes':manifest['payloadBytes'],'manifestSHA256':sha(capsule/'manifest.json'),'checksumsSHA256':sha(capsule/'SHA256SUMS'),'observerSHA256':observer,'integrationPatchSHA256':sha(base/'integration.patch'),'controlledLateDriver':proof}))
