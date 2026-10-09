#!/usr/bin/env python3
"""Retained warm metadata publication; no app, private error/log/auth read or mutation."""
from pathlib import Path
import hashlib,json,os,re,subprocess
B=Path(__file__).resolve().parent
S=Path('/workspace/.luci-tools/cloud073-installed-78665270/evidence/warm-attempt2')
O=B/'capsule';C='786652704fa8bebd1ba1af41cfa1d414d7def100'
sha=lambda raw:hashlib.sha256(raw).hexdigest()
meta=lambda raw:{'bytes':len(raw),'sha256':sha(raw)}
enc=lambda obj:(json.dumps(obj,indent=2)+'\n').encode()
def read(p):
 p=Path(p);assert p.name not in ['private-first-failure.json','private-controller.stderr','private-transport-errors.jsonl','launcher.stdout','launcher.stderr']
 a=p.stat();assert p.is_file() and not p.is_symlink() and a.st_size<4*1024*1024
 raw=p.read_bytes();z=p.stat();assert (a.st_ino,a.st_size,a.st_mtime_ns)==(z.st_ino,z.st_size,z.st_mtime_ns);return raw
def get(name):return json.loads(read(S/name))
def put(name,raw):
 p=O/name;p.parent.mkdir(parents=True,exist_ok=True);assert not p.exists();p.write_bytes(raw)
assert __debug__;os.umask(0o077);assert not O.exists()
# Exact fixture-root pointer is metadata only, used exclusively for substitution.
private=read(S/'driver/private-fixture-path').decode().strip()
assert private.startswith('/workspace/.luci-tools/cloud073-installed-78665270/fixtures/')
replacement='[PRIVATE_FIXTURE_WARM_786_ATTEMPT_2]';originals=[]
def copy(source,name):
 p=Path(source);raw=read(p);public=raw.replace(private.encode(),replacement.encode());assert p.suffix!='.png' or raw==public
 put(name,public);originals.append({'source':str(p),'publishedPath':name,'original':meta(raw),'published':meta(public),'representation':'exact-original' if raw==public else 'exact-private-root-substitution','substitutions':raw.count(private.encode())})
r=get('driver/result.json');w=get('wrapper-result.json')
assert sha(read(S/'driver/result.json'))=='aa1b7d2976ad0c3425be604d3bf46394376adda06a3c6b164f64ff86735de3dc'
assert r['sourceCommit']==r['helperSourceCommit']==C
assert r['passed'] is False and 'scenario' not in r and w['passed'] is False and w['launcherExitCode']==1
assert r['failedStage']=='terminal-scenario' and len(r['pendingMutations'])==1 and len(r['cleanup'])==2
assert r['pendingMutations'][0]['method']=='POST' and r['pendingMutations'][0]['path'].endswith('/restart')
assert r['launcherSHA256']=='5f5eb8a8bc016e36932e4b8ac0649671e37e68284b31196ad42ce56d5c341524'
assert r['scenarioSHA256']=='cc0b1a4ebad01758008502368fdcdfabd3ffc36c8d1e5f6fb6e53f168788b82b'
assert w['sourceBefore']==w['sourceAfter'] and w['driverReport']['sha256']==sha(read(S/'driver/result.json'))
f=r['failureObservation'];assert f['afterCleanup'] and f['failurePresent'] and f['first']['origin']=='callback'
assert f['first']['privateRecord']['bytes']==1282 and f['first']['privateRecord']['sha256']=='7c6f16faabf81baa96e8f6a3fff221f088b55ca68db2746caf688923b12e576d'
ph=lambda n:get(f'driver/phase-{n:03d}.json')
release=ph(39);held=ph(36);fulfilled=ph(40)
assert release['reason']=='bounded-before-attachment' and release['acknowledgedMonotonicMs'] is None and release['attachedMonotonicMs'] is None
assert fulfilled['settlement']=={'attempted':'fulfill','acknowledged':True,'requestFailed':False}
assert not fulfilled['expired'] and fulfilled['releasedMonotonicMs']<fulfilled['deadlineMonotonicMs']
assert held['bodySHA256']==fulfilled['bodySHA256'] and held['session']['status']=='exited'
assert held['observedAt']<r['pendingMutations'][0]['startedAt']<fulfilled['released']
assert r['closingReadObservations']==[]
assert [x['name'] for x in r['cleanupPhases'] if x['status']=='failed-or-unsettled']==['exact-container-leftovers','final-source-and-inventory']
d=r['monitorDisposal'];last=r['monitorDisposalFinal'];obs=last['observation'];protocol=obs['protocol']
assert d['wait']['state']=='fulfilled' and d['after']['observation']==obs
assert obs['monitor']['calls']==obs['transport']['calls']==1
assert obs['monitor']['first']['settlement']['state']==obs['transport']['first']['detachSettlement']['state']==obs['transport']['first']['waitSettlement']['state']=='fulfilled'
assert len(protocol['rows'])==8 and protocol['failures']==['cdp-socket-limit']
assert len(protocol['rows'][0]['handshakes'])==1 and protocol['rows'][0]['handshakes'][0]['status']==101
assert all(x['handshakes']==[] and len(x['errors'])==1 for x in protocol['rows'][1:])
assert len([x for x in r['browserMutations'] if x['path'].endswith('/restart')])==1
assert not [x for x in r['apiActions'] if x['path'].endswith('/restart')]
assert not [x for x in r['apiActions']+r['browserMutations'] if x['method']=='DELETE' or x['path'].endswith('/stop')]
# Parent-supplied joined read-only closure must exist before publication is frozen.
closure_pin=json.loads(read(B/'closure-binding.json'));clp=Path(closure_pin['path']);clraw=read(clp)
assert sha(clraw)==closure_pin['sha256'];cl=json.loads(clraw)
O.mkdir();put('.gitattributes',b'** -text\n');put('.gitignore',b'!*.log\n')
for name in ['before-launch.json','identities.jsonl','wrapper-result.json']:copy(S/name,'run/'+name)
phases=sorted((S/'driver').glob('phase-*.json'));assert len(phases)==len(r['phases'])==47
for p in phases:copy(p,'run/driver/'+p.name)
for name in ['result.json','result.json.incomplete.json','package-provenance.json','image-provenance.json','first-ended-poll-held.png','failure.png']:copy(S/'driver'/name,'run/driver/'+name)
accepted=sorted(p for p in (S/'driver/accepted-harness').rglob('*') if p.is_file());assert len(accepted)==6
for p in accepted:copy(p,'sources/accepted-harness/'+str(p.relative_to(S/'driver/accepted-harness')))
for name,key in [('launch.mjs','launcherSHA256'),('scenario.mjs','scenarioSHA256')]:assert sha(read(S/'driver/accepted-harness'/name))==r[key]
for row in r['helperBindings']:
 raw=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+row['path']],timeout=20)
 assert meta(raw)=={k:row[k] for k in ['sha256','bytes']}
 assert read(S/'driver/accepted-harness'/row['path'].removeprefix('scripts/acceptance/'))==raw
outer=Path('/workspace/.luci-tools/cloud073-teardown-route-v2/warm/outer-run.py')
assert meta(read(outer))==w['sourceBefore'][str(outer)];copy(outer,'sources/outer-run.py')
copy(clp,'closure/independent-closure.json')
for item in closure_pin.get('additional',[]):
 p=Path(item['path']);assert sha(read(p))==item['sha256'];copy(p,item['publishedPath'])
copy(B/'closure-binding.json','closure/input-bindings.json')
parity=Path('/workspace/.luci-tools/c073-78665270-image-source-parity.json')
assert sha(read(parity))=='85928dc798516da01d6b6373ab85bf39f222689c88643805ee79aa785f90d985'
ip=json.loads(read(parity));assert ip['selectedProductSource']==C and ip['matchingImageInputCount']==106 and ip['passed'];copy(parity,'runtime-source-bindings.json')
package=get('driver/package-provenance.json');im=get('driver/image-provenance.json')
assert package['sourceCommit']==C and not package['sourceDirty'] and r['packageIntegrity']=={'verifiedMembers':964,'allMembersMatch':True}
for key in ['binarySHA256','webIndexSHA256','archiveSHA256']:assert r['package'][key]==package[key]
assert im['candidateImage']==r['daemon']['imageID']=='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
assert im['sourceCommit']=='5f4a966f6d70264c92ee2176dde912e1b43fc3c7' and im['sourcesBefore']==im['sourcesAfter'] and len(im['sourcesAfter'])==106
first=ph(22)['wire']['browser']['rows'][0];assert first['exit'][0]['reason']=={'kind':'exit-status','code':0} and first['closed']['code']==1000 and first['closed']['wasClean']
assert ph(33)['last']['containers']==[] and all(ph(33)['last']['ledger'][k]==[] for k in ['leases','checks','fences','checkFences'])
monitor=obs['monitor']['first'];transport=obs['transport']['first']
a={'productSource':C,'overallPassed':False,'scenarioCompleted':False,'launcherExitCode':1,'elapsedSeconds':w['elapsedSeconds'],'failure':r['failure'],'failedStage':r['failedStage'],'failureObservation':f,'pendingMutations':r['pendingMutations'],'cleanupErrors':r['cleanup'],'cleanupPhaseOutcomes':[{'name':x['name'],'status':x['status']} for x in r['cleanupPhases']],'closingReadObservations':[], 'firstTerminal':{'phase':22,'exitCode':0,'browserClose':first['closed']},'firstNaturalRelease':{'phase':33,'polls':ph(33)['polls'],'loopElapsedMs':ph(33)['elapsedMs'],'observedAt':ph(33)['observedAt']},'oldResponseOrdering':{'heldBeforeRestartPOST':True,'heldBodySHA256':held['bodySHA256'],'releaseRequest':release,'released':fulfilled,'restartAcknowledgementObserved':False,'oldResponseAfterRestartAcknowledgementEstablished':False,'scope':'The recorded acknowledgement is GET route.fulfill only. No completed Restart status/body/generation acknowledgement or full warm ordering is established.'},'protocol':protocol,'monitorDisposal':d,'monitorDisposalFinal':last,'disposalDurationsMs':{'monitorFirstCallToSettlement':monitor['settlement']['at']['monotonicMs']-monitor['entry']['monotonicMs'],'outerOperationEnteredToWaitSettlement':d['wait']['at']['monotonicMs']-d['operationEntered']['monotonicMs'],'detachInvocationToUnderlyingSettlement':transport['detachSettlement']['at']['monotonicMs']-transport['detachInvoked']['monotonicMs']},'browserRestartCount':1,'apiRestartCount':0,'stopOrDeleteCount':0,'originalResult':meta(read(S/'driver/result.json')),'originalWrapper':meta(read(S/'wrapper-result.json')),'independentClosureSHA256':sha(clraw),'scope':'Partial failed installed diagnostic. Protocol cap and private-capture failure do not identify the underlying replacement connection or Restart cause. No performance, Windows, provider or full-product claim.'}
put('analysis.json',enc(a));copy(B/'README.prepared.md','README.md');copy(Path(__file__),'assemble.py')
excluded=[{'path':'driver/private-first-failure.json','bytes':1282,'sha256':f['first']['privateRecord']['sha256'],'scope':'Existing writer metadata only; not read or exported.'},{'path':'driver/private-fixture-path','scope':'Metadata pointer read only for exact root substitution; pointer excluded.'},{'path':'driver/private-controller.stderr','bytes':r['controllerLog']['retainedBytes'],'sha256':r['controllerLog']['retainedPrefixSHA256'],'scope':'Existing writer metadata only; not read.'},{'path':'driver/private-transport-errors.jsonl','bytes':r['privateTransportErrors']['bytes'],'sha256':r['privateTransportErrors']['sha256'],'scope':'Existing writer metadata only; not read.'},{'path':'launcher output; fixture/auth/config files; binary/archive/image payloads','scope':'Not read or exported.'}]
put('original-bindings.json',enc({'originals':originals,'excluded':excluded,'redactionPolicy':{'replacement':replacement,'rootSHA256':sha(private.encode()),'rootBytes':len(private.encode()),'onlyExactRootSubstitution':True}}))
for row in originals:assert meta(read(row['source']))==row['original']
put('original-immutability.json',enc({'checked':len(originals),'allUnchanged':True,'scope':'Selected public originals before/after. Independent closure retains its separate fixture observation scope; no new private census.'}))
patterns={'private-key':rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----','github-token':rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})','openai-token':rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}','bearer-value':rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}','url-userinfo':rb'https?://[^/@\s:]+:[^/@\s]+@'}
findings=[]
for p in O.rglob('*'):
 if not p.is_file():continue
 raw=p.read_bytes();assert private.encode() not in raw
 if p.suffix=='.png':continue
 for kind,pattern in patterns.items():
  if re.search(pattern,raw):findings.append({'path':str(p.relative_to(O)),'pattern':kind})
assert not findings,findings
put('privacy-scan.json',enc({'exactPrivateRootAbsent':True,'boundedPatternFindings':findings,'screenshotsManuallyReviewed':['first-ended-poll-held.png','failure.png'],'scope':'Common credential scan and screenshot review, not exhaustive. No raw private error/log/auth/fixture read by publisher; separately supplied allowlisted diagnostic projection retains its own extraction scope.'}))
files={str(p.relative_to(O)):meta(p.read_bytes()) for p in sorted(O.rglob('*')) if p.is_file()}
put('publication.json',enc({'version':1,'productSource':C,'overallPassed':False,'scenarioCompleted':False,'scope':'Original warm786 attempt2 failed during the single Restart; pending mutation, failed cleanup and protocol/capture limits retained. Complete warm ordering not established.','files':files}))
put('SHA256SUMS',(''.join(v['sha256']+'  '+k+'\n' for k,v in files.items())+sha((O/'publication.json').read_bytes())+'  publication.json\n').encode())
for line in (O/'SHA256SUMS').read_text().splitlines():
 digest,name=line.split('  ',1);assert sha((O/name).read_bytes())==digest
result={'capsule':str(O),'files':len(files)+2,'publicationSHA256':sha((O/'publication.json').read_bytes()),'checksumsSHA256':sha((O/'SHA256SUMS').read_bytes()),'bytes':sum(p.stat().st_size for p in O.rglob('*') if p.is_file())}
(B/'assembly-result.json').write_bytes(enc(result));print(json.dumps(result,indent=2))
