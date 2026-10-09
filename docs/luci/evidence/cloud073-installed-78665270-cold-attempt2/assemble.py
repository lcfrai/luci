#!/usr/bin/env python3
"""Publish retained cold metadata/source only; never read raw private error/log/auth files."""
from pathlib import Path
import hashlib, json, os, re, subprocess
B=Path(__file__).resolve().parent
S=Path('/workspace/.luci-tools/cloud073-installed-78665270/evidence/cold-attempt2')
O=B/'capsule'
C='786652704fa8bebd1ba1af41cfa1d414d7def100'
I='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
sha=lambda raw:hashlib.sha256(raw).hexdigest()
meta=lambda raw:{'bytes':len(raw),'sha256':sha(raw)}
enc=lambda obj:(json.dumps(obj,indent=2)+'\n').encode()
def read(p):
    p=Path(p)
    assert p.name not in ['private-first-failure.json','private-controller.stderr','private-transport-errors.jsonl','launcher.stdout','launcher.stderr']
    a=p.stat();assert p.is_file() and not p.is_symlink() and a.st_size<4*1024*1024
    raw=p.read_bytes();z=p.stat();assert (a.st_ino,a.st_size,a.st_mtime_ns)==(z.st_ino,z.st_size,z.st_mtime_ns);return raw
def get(name):return json.loads(read(S/name))
def put(name,raw):
    p=O/name;p.parent.mkdir(parents=True,exist_ok=True);assert not p.exists();p.write_bytes(raw)
assert __debug__;os.umask(0o077);assert not O.exists()
# The pointer is metadata, not fixture/auth content; only used for exact substitution.
private=read(S/'driver/private-fixture-path').decode().strip()
assert private.startswith('/workspace/.luci-tools/cloud073-installed-78665270/fixtures/')
replacement='[PRIVATE_FIXTURE_COLD_786_ATTEMPT_2]';originals=[]
def copy(source,name):
    p=Path(source);raw=read(p);public=raw.replace(private.encode(),replacement.encode())
    assert p.suffix!='.png' or raw==public
    put(name,public);originals.append({'source':str(p),'publishedPath':name,'original':meta(raw),'published':meta(public),'representation':'exact-original' if raw==public else 'exact-private-root-substitution','substitutions':raw.count(private.encode())})

r=get('driver/result.json');w=get('wrapper-result.json');cl=get('independent-closure.json')
assert sha(read(S/'driver/result.json'))=='cc5b94e025462b829faf016756f832260aa1cd4792dfea3680cffd630e953fa2'
assert sha(read(S/'independent-closure.json'))=='8b463fead6625e34fc506ac758bb6c274f54e5e9eae977203718ab32aede3325'
assert r['sourceCommit']==r['helperSourceCommit']==C
assert r['passed'] is True and r['scenario']['passed'] is True and w['passed'] is True and w['launcherExitCode']==0
assert r.get('failure') is None and r.get('failedStage') is None and w['failure'] is None
assert r['cleanup']==r['pendingMutations']==[] and all(x['status']=='completed' for x in r['cleanupPhases'])
assert r['launcherSHA256']=='5312bf202104ba1316272c5e05594547bc3538208e57d3eb8adb9ae346ec83e4'
assert r['scenarioSHA256']=='7af700a6e90262629e1ea5687b883430b4acfd966ec41e305319c963ba3bcf5e'
f=r['failureObservation']
assert f['afterCleanup'] and not f['failurePresent'] and f['first'] is None
assert w['sourceBefore']==w['sourceAfter']
assert w['driverReport']['sha256']==cl['originalResultSHA256']==sha(read(S/'driver/result.json'))
assert cl['originalWrapperSHA256']==sha(read(S/'wrapper-result.json'))
assert r['closingReadObservations']==[{'method':'GET','routeClass':'session-list','pathnameSHA256':'7383f8056aab77dcb32bb7a5cd252830f41f75486b2250c65aba210324678c3c','startedAt':'2026-10-09T17:25:03.115Z','abort':'acknowledged','settledAt':'2026-10-09T17:25:03.122Z'}]
assert sha(b'/v1/sessions')==r['closingReadObservations'][0]['pathnameSHA256']
disposal=r['monitorDisposal'];last=r['monitorDisposalFinal']
assert disposal['wait']['state']=='fulfilled' and disposal['before']['available'] and disposal['after']['available'] and last['available']
assert disposal['after']['observation']==last['observation']
obs=last['observation'];assert obs['monitor']['calls']==obs['transport']['calls']==1 and not obs['monitor']['additionalCalls'] and not obs['transport']['additionalCalls']
assert obs['monitor']['first']['settlement']['state']==obs['transport']['first']['detachSettlement']['state']==obs['transport']['first']['waitSettlement']['state']=='fulfilled'
assert obs['protocol']['failures']==[]

O.mkdir();put('.gitattributes',b'** -text\n');put('.gitignore',b'!*.log\n')
for name in ['before-launch.json','identities.jsonl','wrapper-result.json']:copy(S/name,'run/'+name)
phases=sorted((S/'driver').glob('phase-*.json'));assert len(phases)==len(r['phases'])==46
for p in phases:copy(p,'run/driver/'+p.name)
for name in ['result.json','result.json.incomplete.json','package-provenance.json','image-provenance.json','cold-exited-before-restart.png','cold-restarted-nonzero-ended.png']:copy(S/'driver'/name,'run/driver/'+name)
for p in sorted((S/'driver/accepted-harness').rglob('*')):
    if p.is_file():copy(p,'sources/accepted-harness/'+str(p.relative_to(S/'driver/accepted-harness')))
for name,key in [('launch.mjs','launcherSHA256'),('scenario.mjs','scenarioSHA256')]:assert sha(read(S/'driver/accepted-harness'/name))==r[key]
for row in r['helperBindings']:
    raw=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+row['path']],timeout=20)
    assert meta(raw)=={k:row[k] for k in ['sha256','bytes']}
    assert read(S/'driver/accepted-harness'/row['path'].removeprefix('scripts/acceptance/'))==raw
outer=Path('/workspace/.luci-tools/cloud073-teardown-route-v2/cold/outer-run.py')
assert meta(read(outer))==w['sourceBefore'][str(outer)];copy(outer,'sources/outer-run.py')
cp=Path('/workspace/.luci-tools/cloud073-installed-78665270/independent-cold-closure.py')
assert sha(read(cp))==cl['scriptSHA256'];copy(cp,'sources/independent-cold-closure.py')
copy(S/'independent-closure.json','closure/independent-closure.json')
parity=Path('/workspace/.luci-tools/c073-78665270-image-source-parity.json')
assert sha(read(parity))=='85928dc798516da01d6b6373ab85bf39f222689c88643805ee79aa785f90d985'
ip=json.loads(read(parity));assert ip['selectedProductSource']==C and ip['matchingImageInputCount']==106 and ip['passed']
copy(parity,'runtime-source-bindings.json')
package=get('driver/package-provenance.json');im=get('driver/image-provenance.json')
assert package['sourceCommit']==C and not package['sourceDirty'] and r['packageIntegrity']=={'verifiedMembers':964,'allMembersMatch':True}
for key in ['binarySHA256','webIndexSHA256','archiveSHA256']:assert r['package'][key]==package[key]
assert im['candidateImage']==r['daemon']['imageID']==I and im['sourceCommit']=='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
assert im['sourcesBefore']==im['sourcesAfter'] and len(im['sourcesAfter'])==106

ph=lambda n:get(f'driver/phase-{n:03d}.json')
terminal=[]
for n,code,key,seq in [(11,0,'first',[75,76,77]),(42,7,'second',[78,79,80])]:
    row=ph(n)['wire']['browser']['rows'][0]
    assert len(row['exit'])==1 and row['exit'][0]['reason']=={'kind':'exit-status','code':code}
    assert row['closed']['code']==1000 and row['closed']['wasClean'] and not row['error']
    actual=[row['markers'][key]['sequence'],row['exit'][0]['sequence'],next(x['sequence'] for x in row['events'] if x['kind']=='close')];assert actual==seq
    terminal.append({'phase':n,'exitCode':code,'outputExitCloseSequence':actual,'browserClose':row['closed']})
for n,view in [(22,ph(22)['last']),(44,ph(44))]:
    assert view['containers']==[] and all(view['ledger'][key]==[] for key in ['leases','checks','fences','checkFences'])
assert ph(23)['session']['executionGenerationOmitted']
assert not ph(32)['identicalParsedSessionJSON'] and ph(32)['wire']['browser']['rows']==[]
assert ph(32)['gate']['settlement']=={'attempted':None,'acknowledged':False}
assert ph(32)['response']['session']['status']=='running' and ph(32)['response']['session']['executionGeneration']==r['scenario']['restartedExecutionGeneration']
assert ph(34)['observation']['settlement']=={'attempted':'Fetch.continueResponse','acknowledged':True} and ph(35)['status']==200
assert ph(34)['observedAt']<ph(39)['wire']['browser']['rows'][0]['created']['wall']
assert len([x for x in r['browserMutations'] if x['path'].endswith('/restart')])==1
assert not [x for x in r['apiActions'] if x['path'].endswith('/restart')]
assert not [x for x in r['apiActions']+r['browserMutations'] if x['method']=='DELETE' or x['path'].endswith('/stop')]
assert cl['sameEngine'] and cl['containers']==cl['volumes']==[] and cl['ledger']['counts']=={'leases':0,'checks':0,'fences':0,'checkFences':0}
assert cl['fixtureFilesCount']==31 and cl['originalFilesUnchanged'] and cl['fixtureFilesUnchanged']
assert sum(x['now']['status']=='absent' for x in cl['identities'])==3
assert sum(x['now'].get('state')=='Z' and x['now'].get('parentPID')==1 for x in cl['identities'])==2
launch=read(S/'driver/accepted-harness/launch.mjs').decode()
assert "receipt.passed=!failure&&receipt.scenario?.passed===true&&receipt.cleanup.length===0&&receipt.cleanupPhases.every(x=>x.status==='completed');" in launch
monitor=obs['monitor']['first'];transport=obs['transport']['first']
analysis={'productSource':C,'imageID':I,'launcherSHA256':r['launcherSHA256'],'scenarioSHA256':r['scenarioSHA256'],'scenarioPassed':True,'overallPassed':True,'launcherExitCode':0,'recordedMainCatchFailure':None,'recordedFailedStage':None,'failureObservation':f,'cleanupErrors':[],'pendingMutations':[],'closingReadObservations':r['closingReadObservations'],'failedCallbackMethodPath':'no callback failure recorded in this run; earlier failed callbacks remain unclassified','monitorDisposal':disposal,'monitorDisposalFinal':last,'disposalDurationsMs':{'monitorFirstCallToSettlement':monitor['settlement']['at']['monotonicMs']-monitor['entry']['monotonicMs'],'outerOperationEnteredToWaitSettlement':disposal['wait']['at']['monotonicMs']-disposal['operationEntered']['monotonicMs'],'detachInvocationToUnderlyingSettlement':transport['detachSettlement']['at']['monotonicMs']-transport['detachInvoked']['monotonicMs']},'terminal':terminal,'firstNaturalRelease':{'phase':22,'elapsedMs':ph(22)['elapsedMs'],'polls':ph(22)['polls'],'observedAt':ph(22)['observedAt']},'secondNaturalRelease':{'phase':44,'observedAt':ph(44)['observedAt']},'browserRestartCount':1,'apiRestartCount':0,'stopOrDeleteCount':0,'coldExecutionFingerprintOmitted':True,'realNewGenerationPollBeforeAcknowledgement':True,'identicalParsedSessionJSON':False,'noReplacementAttachBeforeAcknowledgement':True,'acknowledgementAt':ph(34)['observedAt'],'replacementSocketCreatedAt':ph(39)['wire']['browser']['rows'][0]['created']['wall'],'installedRestartStatus':200,'installed409DiagnosticExercised':False,'originalResult':meta(read(S/'driver/result.json')),'originalWrapper':meta(read(S/'wrapper-result.json')),'closure':{'originalSHA256':sha(read(S/'independent-closure.json')),'absentIdentities':3,'pid1Zombies':2,'containers':0,'volumes':0,'ledgerCounts':cl['ledger']['counts'],'fixtureFilesUnchanged':31},'scope':'Retained Linux shell/browser/Docker evidence, not a new run. No retrospective method/path inference for earlier failed callbacks; successful disposal here does not explain the earlier timeout. No model, performance, Windows, raw Close-frame direction/count or full-product acceptance claim.'}
put('analysis.json',enc(analysis))
copy(B/'README.prepared.md','README.md');copy(Path(__file__),'assemble.py')
excluded=[{'path':'driver/private-first-failure.json','scope':'No first failure recorded in public receipt. No raw private error read or exported; no absence census claim.'},{'path':'driver/private-fixture-path','scope':'Metadata pointer read only for exact root substitution; pointer excluded.'},{'path':'driver/private-controller.stderr','bytes':r['controllerLog']['retainedBytes'],'sha256':r['controllerLog']['retainedPrefixSHA256'],'scope':'Existing writer metadata only; raw log not read.'},{'path':'driver/private-transport-errors.jsonl','bytes':r['privateTransportErrors']['bytes'],'sha256':r['privateTransportErrors']['sha256'],'scope':'Existing writer metadata only; raw log not read.'},{'path':'launcher output; fixture/auth/config files; binary/archive/image payloads','scope':'Not read or exported; original verification receipts retain their scopes.'}]
put('original-bindings.json',enc({'originals':originals,'excluded':excluded,'redactionPolicy':{'replacement':replacement,'rootSHA256':sha(private.encode()),'rootBytes':len(private.encode()),'onlyExactRootSubstitution':True}}))
for row in originals:assert meta(read(Path(row['source'])))==row['original']
put('original-immutability.json',enc({'checked':len(originals),'allUnchanged':True,'scope':'Selected public originals before/after. Existing independent closure separately attests31 unchanged private fixture files; no new fixture census.'}))
patterns={'private-key':rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----','github-token':rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})','openai-token':rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}','bearer-value':rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}','url-userinfo':rb'https?://[^/@\s:]+:[^/@\s]+@'}
findings=[]
for p in O.rglob('*'):
    if not p.is_file():continue
    raw=p.read_bytes();assert private.encode() not in raw
    if p.suffix=='.png':continue
    for kind,pattern in patterns.items():
        if re.search(pattern,raw):findings.append({'path':str(p.relative_to(O)),'pattern':kind})
assert not findings,findings
put('privacy-scan.json',enc({'exactPrivateRootAbsent':True,'boundedPatternFindings':findings,'screenshotsManuallyReviewed':['cold-exited-before-restart.png','cold-restarted-nonzero-ended.png'],'scope':'Common credential scan and visual screenshot review, not exhaustive detection. No raw private error/log/auth/fixture content read. Fixed allowlist matches C072 capsule categories with additive safe C073/C074 metadata.'}))
files={str(p.relative_to(O)):meta(p.read_bytes()) for p in sorted(O.rglob('*')) if p.is_file()}
put('publication.json',enc({'version':1,'productSource':C,'overallPassed':True,'scenarioPassed':True,'scope':'Original cold786 attempt2: scenario and driver/wrapper passed; one accepted closing GET and fulfilled disposal. Literal known-owned closure retains2 PID1 zombies; no full-product acceptance.','files':files}))
put('SHA256SUMS',(''.join(v['sha256']+'  '+k+'\n' for k,v in files.items())+sha((O/'publication.json').read_bytes())+'  publication.json\n').encode())
for line in (O/'SHA256SUMS').read_text().splitlines():
    digest,name=line.split('  ',1);assert sha((O/name).read_bytes())==digest
result={'capsule':str(O),'files':len(files)+2,'publicationSHA256':sha((O/'publication.json').read_bytes()),'checksumsSHA256':sha((O/'SHA256SUMS').read_bytes()),'bytes':sum(p.stat().st_size for p in O.rglob('*') if p.is_file())}
(B/'assembly-result.json').write_bytes(enc(result));print(json.dumps(result,indent=2))
