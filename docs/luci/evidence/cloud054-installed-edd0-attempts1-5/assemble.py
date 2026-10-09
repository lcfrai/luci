#!/usr/bin/env python3
"""Publication only: reads frozen evidence; never runs Luci/Docker or changes fixtures."""
from pathlib import Path
import hashlib,json,subprocess,datetime,shutil
ROOT=Path('/workspace/.luci-tools/cloud054-terminal-close')
SOURCE=ROOT/'actual-edd0/evidence'
OUT=Path(__file__).parent/'capsule'
PRODUCT='edd0cb121ff6847f681ab6bf41509a497f043d41'

def sha(b):return hashlib.sha256(b).hexdigest()
def meta(b):return {'sha256':sha(b),'bytes':len(b)}
def dump(v):return (json.dumps(v,indent=2,ensure_ascii=False)+'\n').encode()
def write(p,b):p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(b)
def read(v):return json.loads(v.read_bytes())
assert not OUT.exists(),'Do not overwrite a frozen publication'
OUT.mkdir(mode=0o700)
originals=[];roots=[];summaries=[]
image=read(SOURCE/'attempt5/driver/image-provenance.json')
image_bindings={}
for name,expected in image['sourcesBefore'].items():
 b=subprocess.check_output(['git','-C','/workspace/luci','show',PRODUCT+':'+name]);assert sha(b)==expected
 assert image['sourcesAfter'][name]==expected
 image_bindings[name]=meta(b)
assert len(image_bindings)==104
write(OUT/'image-source-binding.json',dump({'sourceCommit':PRODUCT,'imageID':image['candidateImage'],'files':image_bindings,'scope':'Exact historical image source inputs match product Git bytes; no image rebuild/download or new runtime assertion.'}))

scope={
1:('Unknown initial refusal','Initial POST /v1/sessions returned409 before a known session/allocation/browser. The original body was discarded: cause, body hash and atomic admission operands remain unavailable. This attempt stays failed. No conclusion is backfilled from attempt2 or later environment changes.'),
2:('Captured disk refusal','Initial POST /v1/sessions returned409 with the new bounded Problem projection: execution_disk_pressure. No session/allocation/browser is credited. The complete original body is hash-bound but not retained; numerical actual admission operands are unavailable in that POST schema. C057 has installed diagnostic scope here, not admission correctness or a full terminal pass.'),
3:('Legitimate mount rejected by helper','Initial POST /v1/sessions returned201 and a real owned shell allocation existed. The helper rejected the legitimate read-only primary .git/worktrees bind. No browser ran. Helper cleanup failed the same guard. Separate root-owned immutable-ID stop/non-force removal was performed with a private recovery archive; it did not release the exact backend-settled1024MiB/1000mCPU lease or repair the ledger. The original failed fixture,30 file hashes and retained charge remain unchanged.'),
4:('Cleanup obscured terminal failure evidence','The real browser entered SessionPane with a running shell and held a matching running session-list response. The final error was route.fulfill: Route is already handled!; the original helper finally could replace an earlier scenario error. No primary failure or passive wire snapshot was retained, so its hidden original cause remains unknown. Screenshot shows ready/body-lookalike and ended footer; it is not reconstructed wire evidence. No Restart occurred. Supported Stop cleanup succeeded; two exact Chromium PID1 zombies remain.'),
5:('Typed exit followed by abnormal close','The real browser received FIRST stdout sequence71, native typed exit-status0 sequence73 (13-byte payload7146fc872b9a9dba5098f80e211bc7a8af3d9b2fbf8a3e5d9447db130fcaa3b2), error sequence74 and unclean1006 close sequence75. The primary assertion first.error===false failed at first-wire-assertions. C059 preserved this failure and passive state before successful gate release; the held-read deadline had not expired. UI snapshot says ended/no overlay while the held row still says running. No sustained no-reconnect/no-stdin interval, Restart, nonzero second exit or full terminal pass is credited. Supported Stop cleanup succeeded; two exact Chromium PID1 zombies remain.')}

for n in range(1,6):
 src=SOURCE/f'attempt{n}';dst=OUT/f'attempt{n}';dst.mkdir()
 root=(src/'driver/private-fixture-path').read_text().strip();assert root.startswith(str(ROOT/'actual-edd0/fixtures')+'/');roots.append(root)
 replacement=f'[PRIVATE_FIXTURE_ATTEMPT_{n}]';bindings=[];excluded=[]
 def capture(p,rel=None):
  raw=p.read_bytes();public=raw.replace(root.encode(),replacement.encode())
  if p.suffix in ['.png','.gz']:assert public==raw
  name=rel or str(p.relative_to(src));write(dst/name,public)
  item={'originalRelativePath':str(p.relative_to(SOURCE)) if p.is_relative_to(SOURCE) else str(p.relative_to(ROOT)),'publicPath':name,'original':meta(raw),'published':meta(public),'representation':'byte-identical' if raw==public else 'exact-private-fixture-root-substitution','substitutions':raw.count(root.encode())}
  bindings.append(item);originals.append((str(p),meta(raw)));return item
 for p in sorted(src.rglob('*')):
  if not p.is_file():continue
  if p.name=='private-fixture-path' or p.name=='private-original-fixture.tar.gz':
   excluded.append({'relativePath':str(p.relative_to(src)),**meta(p.read_bytes()),'reason':'private fixture path pointer' if p.name=='private-fixture-path' else 'private original fixture archive; never exported'})
   continue
  capture(p)
 # Exact observer and helper originals remain retrievable, no binaries included.
 capture(ROOT/'outer-v2/run.py','sources/outer-wrapper.py')
 if n>=2:
  closure=read(src/'independent-closure.json');assert sha((ROOT/'independent-closure.py').read_bytes())==closure['scriptSHA256']
  capture(ROOT/'independent-closure.py','sources/independent-closure.py')
 if n==3:capture(ROOT/'attempt3-manual-cleanup.py','sources/manual-cleanup.py')
 result=read(src/'driver/result.json');wrapper=read(src/'wrapper-result.json');closure=read(src/'independent-closure.json')
 assert result['passed'] is False and wrapper['passed'] is False and result['sourceCommit']==PRODUCT and result['helperSourceCommit']==PRODUCT
 assert wrapper['sourceBefore']==wrapper['sourceAfter']
 for path,expected in wrapper['sourceBefore'].items():assert meta(Path(path).read_bytes())==expected
 assert sha((src/'driver/accepted-harness/launch.mjs').read_bytes())==result['launcherSHA256']
 assert sha((src/'driver/accepted-harness/scenario.mjs').read_bytes())==result['scenarioSHA256']
 helper_binding=[]
 for item in result['helperBindings']:
  b=subprocess.check_output(['git','-C','/workspace/luci','show',item['commit']+':'+item['path']]);assert meta(b)=={k:item[k] for k in ['sha256','bytes']}
  local=src/'driver/accepted-harness'/item['path'].removeprefix('scripts/acceptance/');assert local.read_bytes()==b;helper_binding.append(item)
 assert closure['originalResultSHA256']==sha((src/'driver/result.json').read_bytes())
 assert closure['originalWrapperSHA256']==sha((src/'wrapper-result.json').read_bytes())
 problems=[p['problem'] for p in result['phases'] if p.get('kind')=='http-problem-diagnostic']
 creates=[x for x in result['apiActions'] if x.get('method')=='POST' and x.get('path')=='/v1/sessions']
 stops=[x for x in result['apiActions'] if x.get('method')=='DELETE' and x.get('path','').startswith('/v1/sessions/')]
 restarts=[x for x in result['browserMutations'] if x.get('path','').endswith('/restart')]
 summary={'attempt':n,'sourceCommit':PRODUCT,'helperVersion':{1:2,2:3,3:3,4:4,5:6}[n],'passed':False,'title':scope[n][0],'scope':scope[n][1],'originalResult':meta((src/'driver/result.json').read_bytes()),'failedStage':result['failedStage'],'failure':result['failure'],'phaseCount':len(result['phases']),'initialSessionPOSTStatuses':[x['status'] for x in creates],'supportedStopStatuses':[x['status'] for x in stops],'browserRestartCount':len(restarts),'problemProjections':problems,'cleanupPhases':result['cleanupPhases'],'independentClosure':{'originalReceipt':meta((src/'independent-closure.json').read_bytes()),'identities':closure['identities'],'ledger':closure['ledger'],'containers':closure['containers'],'volumes':closure['volumes'],'sameEngine':closure['sameEngine'],'fixtureFilesCount':closure['fixtureFilesCount'],'fixtureFilesUnchanged':closure['fixtureFilesUnchanged'],'originalFilesUnchanged':closure['originalFilesUnchanged']},'sourceAndHelperBindings':helper_binding,'packageAndImageScope':'Original build/provenance and in-run verification receipts are published; package archives, binaries, image layers and private file originals remain publisher-held, not downloadable in this capsule.','observerScope':'Outer identity watchdog, not CPU/RAM/performance acceptance. Repeated unreadable environment observations are not unique process counts. Exact zombie/unknown entries remain literal.'}
 if n==3:
  before=read(src/'manual-recovery/before.json');after=read(src/'manual-recovery/after.json');archive=next(x for x in excluded if x['relativePath'].endswith('.tar.gz'))
  assert archive['sha256']==before['privateArchiveSHA256'] and archive['bytes']==before['privateArchiveBytes']
  assert before['ledgerSHA256']==after['ledgerSHA256']==closure['ledger']['sha256']
  summary['manualCleanup']={'privateArchive':archive,'originalLedgerSHA256':before['ledgerSHA256'],'retainedLease':after['retainedLease'],'fixtureFilesUnchanged':after['fixtureFilesUnchanged'],'originalReportsUnchanged':after['originalReportsUnchanged'],'scope':'Separately recorded operator cleanup; immutable-ID stop/non-force removal; no product capacity release, replay, fence, ledger edit or archive resumption.'}
 if n==5:
  failure=next(p for p in result['phases'] if p.get('kind')=='scenario-failure');observation=next(p for p in result['phases'] if p.get('kind')=='scenario-failure-observation');wire=observation['wire'];row=wire['browser']['rows'][0]
  assert failure['step']=='first-wire-assertions' and row['error'] is True and row['closed']['code']==1006 and row['closed']['wasClean'] is False
  assert row['exit'][0]['reason']=={'kind':'exit-status','code':0} and row['exit'][0]['payload']['sha256']==sha(b'exit status 0')
  assert row['markers']['first']['sequence']<row['exit'][0]['sequence']<next(e['sequence'] for e in row['events'] if e['kind']=='error')<next(e['sequence'] for e in row['events'] if e['kind']=='close')
  summary['passiveFailureObservation']={'stage':failure['step'],'wire':wire,'ui':observation['ui'],'heldGates':failure['gates'],'scope':'Listener-only original observations; browser and driver monotonic clocks have distinct origins. No cross-clock subtraction or backend-cause attribution.'}
 summary=json.loads(dump(summary).replace(root.encode(),replacement.encode()))
 write(dst/'analysis.json',dump(summary));summaries.append({'attempt':n,'title':scope[n][0],'scope':scope[n][1],'originalResult':summary['originalResult']})
 write(dst/'original-bindings.json',dump({'version':1,'bindings':bindings,'excluded':excluded,'redactionPolicy':{'kind':'exact-private-fixture-root-substitution','replacement':replacement,'originalRootSHA256':sha(root.encode()),'originalRootBytes':len(root.encode()),'rawRootPublished':False},'scope':'All selected original rows/fields retained except exact private fixture root substitution; no secret/private state/archive bytes exported. Transformed hashes never replace original hashes.'}))
 text=f'''# Installed terminal attempt {n}: {scope[n][0]}\n\n**Overall result: failed.** Product and utility helper source `{PRODUCT}`; external installed helper version {summary['helperVersion']}.\n\n{scope[n][1]}\n\nThe original complete driver result, every phase checkpoint, wrapper/identity observations, exact accepted helpers, package/image provenance and separate independent closure are retained below. `analysis.json` is a derived summary; original records take precedence. `{summary['originalResult']['sha256']}` is the original result hash. `original-bindings.json` distinguishes exact bytes from private-root substitutions and excludes private path pins and the recovery archive. Screenshots, when present, are unchanged and contain only the disposable shell fixture; their visual content does not substitute for missing wire observations.\n\nThe four accepted repository helpers bind to exact Git bytes; all 104 unchanged image source inputs are checked in the parent capsule. Original wrapper source-before/after hashes match. No binary/archive/image layer is exported or claimed retrievable here. Private fixture contents/file hashes and raw ledger assertions remain publisher-held except their bounded closure projections. No current allocation, provider, Windows, resource-efficiency or complete local acceptance is asserted.\n\nOnly verify these receipts. Do not run archived launch/cleanup helpers against preserved fixtures or a shared user Docker daemon. The archived sources document executed code and require a freshly pinned isolated setup for any future authorized run.\n'''
 write(dst/'README.md',text.encode())
 manifest={'version':1,'attempt':n,'passed':False,'files':{str(p.relative_to(dst)):meta(p.read_bytes()) for p in sorted(dst.rglob('*')) if p.is_file()},'scope':scope[n][1]}
 write(dst/'publication.json',dump(manifest));write(dst/'SHA256SUMS',(''.join(v['sha256']+'  '+k+'\n' for k,v in manifest['files'].items())+sha((dst/'publication.json').read_bytes())+'  publication.json\n').encode())
# Original immutability is checked again after publication; observations are not repeated.
for path,expected in originals:assert meta(Path(path).read_bytes())==expected
write(OUT/'source-immutability.json',dump({'originalsChecked':len(originals),'allUnchanged':True,'scope':'Original evidence/helper files read before and after assembly; no runtime/private fixture mutation or new closure observation.'}))
write(OUT/'summary.json',dump({'sourceCommit':PRODUCT,'allAttemptsFailed':True,'attempts':summaries,'limits':['No Windows/live-provider/performance or full terminal pass','Attempt1 original body/cause not reconstructed','Attempt3 exact charge retained despite separate container removal','Attempt4 original primary failure unknown','Attempt5 typed exit0 does not erase abnormal1006','Attempts4/5 preserve two exact Chromium PID1 zombies each']}))
# No private root reaches any publication member.
for p in OUT.rglob('*'):
 if p.is_file():
  b=p.read_bytes()
  for root in roots:assert root.encode() not in b,(p,'private root escaped')
write(OUT/'README.md',b'''# Five failed installed terminal attempts\n\nThese independent Linux interactive-shell attempts use product edd0cb121ff6847f681ab6bf41509a497f043d41. Each subdirectory is a separate hash-bound publication with its own result, limitations and cleanup evidence. All five remain failed. No result is combined into an overall terminal pass.\n\n1. Initial HTTP409; original body/cause unavailable.\n2. HTTP409 with captured execution_disk_pressure.\n3. Real allocation; helper rejected legitimate read-only Git guard, later separate manual cleanup retains charged lease.\n4. Real browser; cleanup error obscured the original failure evidence.\n5. Actual stdout and typed exit0 precede socket error/unclean1006; primary failure and passive state retained.\n\nC057/C058 controlled source corrections are published separately. C059 controlled source evidence is separate from attempt5's useful installed diagnostics. In attempt5 the UI was ended/no-overlay at the snapshot, but no extended reconnect/input guard, same-ID restart or second nonzero exit was reached. No transport fix or backend cause is asserted by this publication.\n\nRun `python3 verify.py` for byte/hash checks only. It never launches applications, tests, Docker or cleanup. Original private fixture/archive/ledger bytes and package binaries are excluded; source and derived-representation bindings state what is independently retrievable. Screenshots were visually checked for disposable-fixture-only content and retained unchanged. No unchanged tests or workloads were rerun to assemble this capsule.\n''')
verify='''#!/usr/bin/env python3\nfrom pathlib import Path\nimport hashlib,json\nr=Path(__file__).resolve().parent\ncount=0\nfor manifest in [r/'publication.json',*sorted(r.glob('attempt*/publication.json'))]:\n v=json.loads(manifest.read_bytes());base=manifest.parent\n for name,want in v['files'].items():\n  p=base/name;assert p.resolve().is_relative_to(base.resolve());b=p.read_bytes();assert len(b)==want['bytes'] and hashlib.sha256(b).hexdigest()==want['sha256'],str(p);count+=1\nprint(json.dumps({'verifiedBindings':count,'runtimeStarted':False}))\n'''
write(OUT/'verify.py',verify.encode())
write(OUT/'assemble.py',Path(__file__).read_bytes())
manifest={'version':1,'sourceCommit':PRODUCT,'allAttemptsFailed':True,'files':{str(p.relative_to(OUT)):meta(p.read_bytes()) for p in sorted(OUT.rglob('*')) if p.is_file()},'limits':'Publication verification only; no private source-byte or whole-machine cleanup claim.'}
write(OUT/'publication.json',dump(manifest));write(OUT/'SHA256SUMS',(''.join(v['sha256']+'  '+k+'\n' for k,v in manifest['files'].items())+sha((OUT/'publication.json').read_bytes())+'  publication.json\n').encode())
print(json.dumps({'publication':meta((OUT/'publication.json').read_bytes()),'members':len(manifest['files']),'originalsChecked':len(originals),'path':str(OUT)}))
