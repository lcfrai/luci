from pathlib import Path
import json,gzip,hashlib,math,subprocess,re,base64,collections
REPO=Path('/workspace/luci')
SRC=REPO/'artifacts/quality-swarm/sustained-d20ebd29-attempt8'
PREP=Path(__file__).resolve().parent
OUT=PREP/'capsule';OUT.mkdir(parents=True,exist_ok=True)
CLOSURE=Path('<PRIVATE_TOOLING_DIRECTORY>/sustained8-closure-private/independent-closure.json')
sha=lambda b:hashlib.sha256(b).hexdigest()
def meta(p):
 h=hashlib.sha256();size=0
 with p.open('rb') as f:
  for b in iter(lambda:f.read(1024*1024),b''):h.update(b);size+=len(b)
 return {'sha256':h.hexdigest(),'bytes':size}
def load(p):return json.loads(p.read_text())
def enc(x):return (json.dumps(x,indent=2,ensure_ascii=False)+'\n').encode()
def put(name,data):
 p=OUT/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data if isinstance(data,bytes) else data.encode());return meta(p)
def jout(name,x):return put(name,enc(x))
r=load(SRC/'result/result.json');closure=load(CLOSURE);inv=load(SRC/'invocation.json')
product=r['sourceCommit'];helper=r['harnessCheckout']['commit'];fixture=Path(r['fixtureRetained']['path'])
assert product=='d20ebd292f3f1de04d6726bab0998c210987bcb5' and helper=='5f318d5178210e95cd2b773f07686d88936d603e'
assert not r['passed'] and r['actualTaskCount']==5 and set(r['actualTaskResults'])=={'build-a','build-b','build-c','build-d','amend-a'}
assert len(r['naturalCompletions'])==5 and all(x['cleanNativeExitFrame'] for x in r['naturalCompletions'])
assert r['failedStage']=='settle' and 'Measured UI latency exceeded the declared candidate targets' in r['failure']
assert r['functionalPassed'] is True and r['latencyTargetsMet'] is False and len(r['integrations'])==5
assert sha(CLOSURE.read_bytes())=='e506c6bd7f3707db32cd26d81ab24815bb27de6f82d04858647c588bcd08962d'
assert closure['fixtureUnchanged'] and closure['originalArtifactInputsUnchanged']
assert closure['activeLeaseCount']==0 and closure['dockerContainers']==[] and closure['dockerVolumes']==[]
# Private secrets are read only from this owned fixture for in-memory absence checks.
identity=load(fixture/'state/control-plane/identity.json')
secrets=[x['passwordHash'] for x in identity.get('users',{}).values() if isinstance(x,dict) and isinstance(x.get('passwordHash'),str)]
secrets.extend(identity.get('sessions',{}).keys())
secrets=[s for s in secrets if len(s)>=16]
# Every replacement has a declared original-byte binding; no other field/number is rewritten.
replacement_specs=[(str(fixture),'<PRIVATE_FAILED_FIXTURE>'),(str(fixture.parent),'<PRIVATE_FAILED_FIXTURE_PARENT>'),('<PRIVATE_TOOLING_DIRECTORY>','<PRIVATE_TOOLING_DIRECTORY>'),('<PRIVATE_RETAINED_TOOLS>','<PRIVATE_RETAINED_TOOLS>'),(str(SRC),'<PRIVATE_RUN_EVIDENCE_DIRECTORY>')]
replacements=sorted(replacement_specs,key=lambda x:-len(x[0]))
source_bindings={}; publication={}; original_paths={}
def publish_text(rel,compress=False,dest=None):
 if rel.endswith('.jsonl'):
  p=SRC/rel;original_paths[rel]=p;source_bindings[rel]=meta(p)
  target=dest or rel+('.gz' if compress else '');dp=OUT/target;dp.parent.mkdir(parents=True,exist_ok=True)
  counts={};h=hashlib.sha256();size=0
  with p.open('rb') as original,dp.open('wb') as raw:
   sink=gzip.GzipFile(filename='',mode='wb',fileobj=raw,mtime=0) if compress else raw
   try:
    for line in original:
     t=line.decode()
     for old,new in replacements:
      n=t.count(old)
      if n:counts[new]=counts.get(new,0)+n;t=t.replace(old,new)
     b=t.encode();h.update(b);size+=len(b);sink.write(b)
   finally:
    if compress:sink.close()
  publication[target]={'original':source_bindings[rel],'sourceLabel':rel,'representation':('gzip of ' if compress else '')+('exact original bytes' if not counts else 'original text with explicitly listed private-path substitutions only'),'substitutions':counts,'decodedSHA256':h.hexdigest(),'decodedBytes':size,'streamedByLine':True}
  return None
 p=SRC/rel;b=p.read_bytes();original_paths[rel]=p;source_bindings[rel]=meta(p)
 s=b.decode(); counts={}
 for i,(old,new) in enumerate(replacements):
  n=s.count(old)
  if n:counts[new]=n;s=s.replace(old,new)
 out=s.encode();target=dest or rel+('.gz' if compress else '')
 put(target,gzip.compress(out,mtime=0) if compress else out)
 publication[target]={'original':source_bindings[rel],'sourceLabel':rel,'representation':('gzip of ' if compress else '')+('exact original bytes' if not counts else 'original text with explicitly listed private-path substitutions only'),'substitutions':counts,'decodedSHA256':sha(out),'decodedBytes':len(out)}
 return out
for rel in ['result/result.json','result/result.json.incomplete.json','result/diagnostic-journal.jsonl','observer/samples.jsonl','invocation.json']:
 publish_text(rel,True)
for rel in ['wrapper-result.json','outer-process.json','wrapper.stderr','wrapper.stdout','observer-lifecycle.jsonl','observer-observer-self-cost.json','observer/metadata.json','observer/summary.json','result/package-provenance.json','result/image-provenance.json']:
 publish_text(rel)
# The marker file exposes only the private instance path; it is hash-bound but not published.
source_bindings['observer-instance-path.txt']=meta(SRC/'observer-instance-path.txt');original_paths['observer-instance-path.txt']=SRC/'observer-instance-path.txt'
# Exact immutable executed source snapshots including the observer.
helpers={'scripts/acceptance/sustained-four-project/sustained.mjs':r['driverSHA256']}
helpers.update({'scripts/acceptance/sustained-four-project/'+k:v for k,v in r['helperSHA256'].items()})
helpers.update(r['reusedHelperSHA256']);helper_checks={}
for name,expected in helpers.items():
 file_name=name.removeprefix('scripts/acceptance/sustained-four-project/') if name.startswith('scripts/acceptance/sustained-four-project/') else name.removeprefix('scripts/acceptance/').replace('/','_')
 rel='result/accepted-harness/'+file_name;p=SRC/rel
 assert sha(p.read_bytes())==expected
 pinned=subprocess.check_output(['git','show',f'{helper}:{name}'],cwd=REPO)
 assert sha(pinned)==expected
 publish_text(rel)
 helper_checks[name]={'sha256':expected,'bytes':len(pinned),'acceptedMember':rel,'matchesPinnedHelperGitBlob':True}
sampler=SRC/'observer/accepted-sampler.py'; sampler_expected=helpers['scripts/acceptance/four-project-queue/sample-linux-owned.py']
assert sha(sampler.read_bytes())==sampler_expected
publish_text('observer/accepted-sampler.py')
# Bind the recorded runtime source inventory to immutable product bytes; no image retrieval.
image=load(SRC/'result/image-provenance.json')
assert len(image['sourcesBefore'])==104 and image['sourcesBefore']==image['sourcesAfter']
for path,expected in image['sourcesBefore'].items():
 assert sha(subprocess.check_output(['git','show',product+':'+path],cwd=REPO))==expected
jout('image-source-comparison.json',{'sourceCommit':product,'sources':image['sourcesBefore'],'count':104,'allImmutableProductBlobsMatch':True,'scope':'Exact source comparisons only; recorded image contents/provenance are not independently rebuilt or fetched by publication.'})
# Screenshot reviewed visually: only synthetic acceptance projects, no private path/credential UI.
for rel in ['result/failed-stage.png','result/amended-project-desktop.png','result/four-project-phone.png']:
 photo=SRC/rel;source_bindings[rel]=meta(photo);original_paths[rel]=photo;put(rel,photo.read_bytes());publication[rel]={'original':meta(photo),'representation':'exact original image; reviewed synthetic fixture UI'}
# Publish only bounded independent closure projection; private inventory rows remain external.
c={k:v for k,v in closure.items() if k not in ['fixture','fixtureInventoryBefore','fixtureInventoryAfter','originalArtifactInputs']}
c['fixture']='<PRIVATE_FAILED_FIXTURE>'
for k in ['fixtureInventoryBefore','fixtureInventoryAfter']:
 c[k]={n:v for n,v in closure[k].items() if n!='rows'}
c['publication']={'representation':'allowlisted independent read-only closure; private fixture root and inventory rows omitted','original':meta(CLOSURE),'privateInventoryOriginalsRemainExternal':True,'originalArtifactInputsVerifiedUnchangedByRoot':closure['originalArtifactInputsUnchanged']}
# Avoid exporting private tool directory if later closure metadata contains one.
def sanitize(x):
 if isinstance(x,dict):return {k:sanitize(v) for k,v in x.items()}
 if isinstance(x,list):return [sanitize(v) for v in x]
 if isinstance(x,str):
  for old,new in replacements:x=x.replace(old,new)
 return x
jout('independent-closure.projection.json',sanitize(c))
source_bindings['external:independent-closure.json']=meta(CLOSURE);original_paths['external:independent-closure.json']=CLOSURE
jout('analysis/task-settlement.projection.json',{'scope':'Exact selected retained-receipt proof fields, with private-path substitutions only if present. Five accepted task receipts, five natural completions and five reviewed integrations include the four initial projects and the A amendment. They overlap as observations of the same work, not fifteen results. The final phone navigation passed and functionalPassed=true; missing handoff reports and independent whole-build verification remain explicit. Overall performance acceptance still failed. Queue/allocation and cleanup fields are retained exactly; successful independent closure does not turn the failed overall run into a pass.','originalResult':source_bindings['result/result.json'],'fields':sanitize({k:r.get(k) for k in ['taskReceiptSettlements','naturalCompletions','integrations','preIntegrationProgress','activeRestarts','allocationOrder','queueControls','queueCancellationGuard','cleanup','runtimeCleanup']}),'fieldPresence':{k:k in r for k in ['taskReceiptSettlements','naturalCompletions','integrations','preIntegrationProgress','activeRestarts','allocationOrder','queueControls','queueCancellationGuard','cleanup','runtimeCleanup']}})
# Projection retains every row and every collected node-stage/browser-frame timing/reference.
rows=[]
for i,x in enumerate(r['interactions']):
 rows.append({'index':i,**{k:x[k] for k in ['phase','kind','at','success','elapsedMs','diagnostics'] if k in x}})
assert len(rows)==2116
summary={}
for key,expected in r['interactionSummary'].items():
 xs=[x for x in rows if x['phase']+':'+x['kind']==key];vs=sorted(x['elapsedMs'] for x in xs);q=lambda p:vs[max(0,math.ceil(len(vs)*p)-1)]
 actual={'count':len(xs),'successful':sum(x['success'] is True for x in xs),'failed':sum(x['success'] is not True for x in xs),'p50Ms':q(.5),'p95Ms':q(.95),'p99Ms':q(.99),'maxMs':max(vs)}
 assert all(actual[k]==expected[k] for k in actual),(key,actual,expected)
 summary[key]=actual
projection={'scope':'All original 2116 interaction rows. No numeric filtering/subtraction. diagnostics includes every original nodeStages, nodeClockReads, browserFrames and limitations entry. Project identity is omitted here but remains in the sanitized full result.','originalResult':source_bindings['result/result.json'],'rows':rows,'recomputedSummaries':summary,'matchesAllOriginalSummaries':True}
pb=enc(projection);put('analysis/interactions-with-spans.projection.json.gz',gzip.compress(pb,mtime=0))
jout('analysis/interaction-summary.json',{'scope':'Nearest rank over every original duration; no instrumentation-cost subtraction. Idle, loaded, amendment and settled rows are included; final successful phone navigation is outside these timed action rows. Full acceptance failed the latency gate.','originalResult':source_bindings['result/result.json'],'summary':summary,'rows':len(rows),'allSuccessful':all(x['success'] for x in rows),'targets':{'switchP95Ms':300,'inputP95Ms':100,'p99StrictlyBelowMs':1000},'noSustainedAcceptance':True})
# Exact helper/runtime/package observations, not an independent artifact retrieval claim.
jout('source-bindings.json',{'functionalPassedPropertyPresent':'functionalPassed' in r,'originalWorkflowOperationUnsettled':r['workflowOperationUnsettled'],'originalReceiptComplete':r['originalReceiptComplete'],'providerCalls':r['providerCalls'],'externalModelCalls':r['externalModelCalls'],'reviewedAmendmentCheckPassed':any(x['passed'] and x['name'].startswith('Reviewed amendment') for x in r['checks']),'productSource':product,'helperSource':helper,'separateProductAndHelper':product!=helper,'executedHelpers':helper_checks,'samplerMatchesAcceptedHelper':True,'packageIntegrity':r['packageIntegrity'],'binarySHA256':r['binarySHA256'],'webIndexSHA256':r['webIndexSHA256'],'image':r['image'],'helperChecksumCount':len(helper_checks),'runtimeMode':'Actual native Codex CLI and Docker with scripted provider; zero external model calls.','acceptedTaskResults':5,'acceptedTaskKeys':list(r['actualTaskResults']),'naturalExitZeroCompletions':5,'integratedTasks':5,'overallFailedStage':'settle-latency-gate','functionalPassedPropertyPresent':True,'functionalPassed':r['functionalPassed'],'taskChildrenCoverage':'Five actual tasks across four projects, one worker at a time, including the reviewed A amendment. Distinct exit/receipt/integration counts overlap. No parallel speed or live-model quality claim.','packageAndImageByteRetrievalReperformed':False})
# A light public verifier can recompute the projected rows without app/network/process work.
put('recompute-interactions.py','''#!/usr/bin/env python3
from pathlib import Path
import json,gzip,math
root=Path(__file__).resolve().parent
x=json.loads(gzip.decompress((root/'analysis/interactions-with-spans.projection.json.gz').read_bytes()))
assert len(x['rows'])==2116
for key,expected in x['recomputedSummaries'].items():
 rows=[r for r in x['rows'] if r['phase']+':'+r['kind']==key]; vals=sorted(r['elapsedMs'] for r in rows);q=lambda p:vals[max(0,math.ceil(len(vals)*p)-1)]
 actual=dict(count=len(vals),successful=sum(r['success'] is True for r in rows),failed=sum(r['success'] is not True for r in rows),p50Ms=q(.5),p95Ms=q(.95),p99Ms=q(.99),maxMs=max(vals))
 assert actual==expected,key
print(json.dumps({'rows':len(x['rows']),'summaryGroups':len(x['recomputedSummaries']),'allOriginalNumericRowsIncluded':True,'sustainedAcceptance':'failed'}))
''')
# Actual run compact task receipts/natural completions remain in the full result; no earlier-run retrospective outputs imported.
# Additional final analysis is copied from the parent's frozen output via an explicit manifest.
analysis_manifest=PREP/'analysis-inputs.json'
assert analysis_manifest.is_file(),'Waiting for frozen parent analysis manifest'
for item in load(analysis_manifest):
 p=Path(item['path']);assert meta(p)['sha256']==item['sha256']
 b=p.read_bytes();target=item['member'];original_paths['external:'+target]=p;source_bindings['external:'+target]=meta(p)
 if item.get('sanitize'):
  t=b.decode()
  for old,new in replacements:t=t.replace(old,new)
  # Declared generic substitution preserves values/hashes while removing external analysis paths.
  t=re.sub(r'/workspace/\.luci-tools/[A-Za-z0-9_./-]+','<PRIVATE_EXTERNAL_ANALYSIS_PATH>',t)
  b=t.encode()
 if item.get('gzip'):b=gzip.compress(b,mtime=0)
 put(target,b)
 publication[target]={'original':meta(p),'representation':'parent frozen offline analysis; '+('sanitized private analysis paths' if item.get('sanitize') else 'exact original analysis bytes')+('; gzip-compressed' if item.get('gzip') else '')}
put('README.md',(PREP/'README.md').read_bytes())
# Recursive text/JSON/gzip + base64 scans against exact owned fixture roots and retained credential hashes.
private=[old for old,new in replacements]; counts={'files':0,'decodedGzip':0,'jsonStrings':0,'base64DecodedValues':0,'knownPrivatePatterns':len(private),'retainedCredentialValues':len(secrets),'privateMatches':0,'credentialMatches':0}
def scan_bytes(b,label):
 for needle in private:
  assert needle.encode() not in b,('private path',label)
 for secret in secrets:assert secret.encode() not in b,('retained credential',label)
def walk(x):
 if isinstance(x,str):
  counts['jsonStrings']+=1;scan_bytes(x.encode(),'json string')
  if len(x)>=32 and len(x)%4==0 and re.fullmatch('[A-Za-z0-9+/]+={0,2}',x):
   try:
    d=base64.b64decode(x,validate=True);counts['base64DecodedValues']+=1;scan_bytes(d,'decoded base64')
   except ValueError:pass
 elif isinstance(x,dict):
  for k,v in x.items():walk(k);walk(v)
 elif isinstance(x,list):
  for v in x:walk(v)
for p in OUT.rglob('*'):
 if not p.is_file() or p.name in ['publication.json','SHA256SUMS','privacy-scan.json']:continue
 counts['files']+=1
 if p.name.endswith('.jsonl') or p.name.endswith('.jsonl.gz'):
  if p.suffix=='.gz':counts['decodedGzip']+=1
  stream=gzip.open(p,'rb') if p.suffix=='.gz' else p.open('rb')
  with stream:
   for line in stream:
    scan_bytes(line,p.name);walk(json.loads(line))
  continue
 b=p.read_bytes()
 if p.suffix=='.gz':b=gzip.decompress(b);counts['decodedGzip']+=1
 if p.suffix=='.png':continue
 scan_bytes(b,p.name)
 try:
  if p.name.endswith('.jsonl') or p.name.endswith('.jsonl.gz'):
   for line in b.decode().splitlines():walk(json.loads(line))
  else:walk(json.loads(b))
 except (json.JSONDecodeError,UnicodeDecodeError):pass
counts['checkCodeSHA256']=meta(Path(__file__))['sha256']
counts['scope']='Known private roots and owned-fixture password/session hashes checked in memory only; all JSON strings, decoded gzip and plausible base64 values scanned. Original private inventory/credentials excluded. Screenshot separately reviewed. No universal secret guarantee.'
jout('privacy-scan.json',counts)
assert all(meta(path)==source_bindings[key] for key,path in original_paths.items())
entries={p.relative_to(OUT).as_posix():meta(p) for p in sorted(OUT.rglob('*')) if p.is_file() and p.name not in ['publication.json','SHA256SUMS']}
manifest={'schemaVersion':1,'scope':'Installed sustained attempt 8: functional workflow passed, overall failed on latency, preserved without rerun. Offline publication only, outside checkout. No product/helper correction, runtime, browser, Docker, compiler or test execution.','productSource':product,'helperSource':helper,'originalSources':source_bindings,'representations':publication,'allOriginalArtifactsUnchanged':True,'interactionProjectionDecoded':{'sha256':sha(pb),'bytes':len(pb),'rows':2116},'independentClosureScope':'All999 fixture files unchanged; empty fixture ledger and Docker/volumes; fifteen known identities include thirteen absent and two PID1 browser zombies, with three other unreadable environments. Scoped independent closure is not whole-machine absence or full acceptance; the distinct functional workflow passed.','entries':entries}
jout('publication.json',manifest)
sums=''.join(f'{sha(p.read_bytes())}  {p.relative_to(OUT).as_posix()}\n' for p in sorted(OUT.rglob('*')) if p.is_file() and p.name!='SHA256SUMS');put('SHA256SUMS',sums)
final={'members':len(entries)+2,'checksumEntries':len(sums.splitlines()),'publication':meta(OUT/'publication.json'),'SHA256SUMS':meta(OUT/'SHA256SUMS'),'originalResult':source_bindings['result/result.json'],'allOriginalsUnchanged':True,'privacy':counts}
(PREP/'result.json').write_bytes(enc(final));print(json.dumps(final,indent=2))
