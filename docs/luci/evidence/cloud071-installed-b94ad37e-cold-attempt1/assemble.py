#!/usr/bin/env python3
"""Publish retained safe cold receipts; no runtime actions or private log reads."""
from pathlib import Path
import hashlib,json,os,re,subprocess
S=Path('/workspace/.luci-tools/cloud071-installed-b94ad37e/evidence/cold-attempt1')
O=Path(__file__).parent/'cold-attempt1'
C='b94ad37e53181e360ac8910ee980570de622728c'
I='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
def sha(b):return hashlib.sha256(b).hexdigest()
def meta(b):return dict(bytes=len(b),sha256=sha(b))
def enc(x):return (json.dumps(x,indent=2)+'\n').encode()
def read(p):
 a=p.stat();assert p.is_file() and not p.is_symlink() and a.st_size<4*1024*1024
 b=p.read_bytes();z=p.stat();assert (a.st_ino,a.st_size,a.st_mtime_ns)==(z.st_ino,z.st_size,z.st_mtime_ns);return b
def put(n,b):
 p=O/n;p.parent.mkdir(parents=True,exist_ok=True);assert not p.exists();p.write_bytes(b)
def get(n):return json.loads(read(S/n))
assert __debug__;os.umask(0o077);assert not O.exists()
private=read(S/'driver/private-fixture-path').decode().strip()
assert private.startswith('/workspace/.luci-tools/cloud071-installed-b94ad37e/fixtures/')
replacement='[PRIVATE_FIXTURE_COLD_ATTEMPT_1]';originals=[]
def copy(p,n):
 b=read(p);v=b.replace(private.encode(),replacement.encode());assert p.suffix!='.png' or b==v
 put(n,v);originals.append(dict(source=str(p),publishedPath=n,original=meta(b),published=meta(v),representation='exact-original' if b==v else 'exact-private-root-substitution',substitutions=b.count(private.encode())))
r=get('driver/result.json');w=get('wrapper-result.json');cl=get('independent-closure.json')
assert sha(read(S/'independent-closure.json'))=='93977f92fb3168bd6e634cffb8daedbbeccd429d01f290c5560885fc603369e9'
assert r['sourceCommit']==r['helperSourceCommit']==C
assert r['passed'] is False and r['scenario']['passed'] is True and w['passed'] is False and w['launcherExitCode']==1
assert r.get('failure') is None and r.get('failedStage') is None and w['failure'] is None
assert r['cleanup']==[] and all(x['status']=='completed' for x in r['cleanupPhases'])
assert r['pendingMutations']==[] and w['sourceBefore']==w['sourceAfter']
assert w['driverReport']['sha256']==cl['originalResultSHA256']==sha(read(S/'driver/result.json'))
assert cl['originalWrapperSHA256']==sha(read(S/'wrapper-result.json'))
for path,m in w['sourceBefore'].items():assert meta(read(Path(path)))==m
O.mkdir()
put('.gitattributes',b'# Keep evidence bytes identical across platforms.\n** -text\n')
for n in ['before-launch.json','identities.jsonl','wrapper-result.json']:copy(S/n,'run/'+n)
for p in sorted((S/'driver').glob('phase-*.json')):copy(p,'run/driver/'+p.name)
assert len(list((S/'driver').glob('phase-*.json')))==len(r['phases'])==46
for n in ['result.json','result.json.incomplete.json','package-provenance.json','image-provenance.json','cold-exited-before-restart.png','cold-restarted-nonzero-ended.png']:copy(S/'driver'/n,'run/driver/'+n)
for p in sorted((S/'driver/accepted-harness').rglob('*')):
 if p.is_file():copy(p,'sources/accepted-harness/'+str(p.relative_to(S/'driver/accepted-harness')))
for n,k in [('launch.mjs','launcherSHA256'),('scenario.mjs','scenarioSHA256')]:assert sha(read(S/'driver/accepted-harness'/n))==r[k]
for row in r['helperBindings']:
 b=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+row['path']],timeout=30)
 assert meta(b)=={k:row[k] for k in ['sha256','bytes']}
 assert read(S/'driver/accepted-harness'/row['path'].removeprefix('scripts/acceptance/'))==b
copy(Path('/workspace/.luci-tools/cloud071-browser-restart-diagnostic/cold/outer-run.py'),'sources/outer-run.py')
copy(S/'independent-closure.json','closure/independent-closure.json')
cp=Path('/workspace/.luci-tools/cloud071-installed-b94ad37e/independent-cold-closure.py')
assert sha(read(cp))==cl['scriptSHA256'];copy(cp,'sources/independent-cold-closure.py')
package=get('driver/package-provenance.json');im=get('driver/image-provenance.json')
assert package['sourceCommit']==C and package['sourceDirty'] is False
assert r['packageIntegrity']=={'verifiedMembers':963,'allMembersMatch':True}
for k in ['binarySHA256','webIndexSHA256','archiveSHA256']:assert r['package'][k]==package[k]
assert im['candidateImage']==r['daemon']['imageID']==I
assert im['sourceCommit']=='5f4a966f6d70264c92ee2176dde912e1b43fc3c7' and im['sourcesBefore']==im['sourcesAfter'] and len(im['sourcesAfter'])==106
runtime=[]
for p,h in im['sourcesAfter'].items():
 b=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+p],timeout=30);assert sha(b)==h;runtime.append(dict(path=p,**meta(b)))
put('runtime-source-bindings.json',enc(dict(selectedProductSource=C,imageBuildSource=im['sourceCommit'],imageID=I,count=106,inputs=runtime,scope='All captured runtime inputs match immutable b94 source. Package integrity (963 members) and artifact hashes are retained writer receipts, not an independent publication-time binary reread or rebuild.')))
ph=lambda n:get('driver/phase-%03d.json'%n)
for n,code,seq in [(11,0,[76,77,78]),(42,7,[77,78,79])]:
 row=ph(n)['wire']['browser']['rows'][0];assert len(row['exit'])==1 and row['exit'][0]['reason']=={'kind':'exit-status','code':code};assert row['closed']['code']==1000 and row['closed']['wasClean'] is True and row['error'] is False
 assert row['exit'][0]['sequence']==seq[1] and next(x['sequence'] for x in row['events'] if x['kind']=='close')==seq[2]
assert ph(22)['last']['containers']==[] and all(ph(22)['last']['ledger'][k]==[] for k in ['leases','checks','fences','checkFences'])
assert ph(23)['session']['executionGenerationOmitted'] is True
assert ph(32)['identicalParsedSessionJSON'] is False and ph(32)['wire']['browser']['rows']==[]
assert ph(34)['observation']['settlement']=={'attempted':'Fetch.continueResponse','acknowledged':True}
assert ph(35)['status']==200
assert ph(44)['containers']==[] and all(ph(44)['ledger'][k]==[] for k in ['leases','checks','fences','checkFences'])
assert len([x for x in r['browserMutations'] if x['path'].endswith('/restart')])==1
assert not [x for x in r['apiActions'] if x['path'].endswith('/restart')]
assert not [x for x in r['apiActions']+r['browserMutations'] if x['method']=='DELETE' or x['path'].endswith('/stop')]
assert cl['sameEngine'] is True and cl['containers']==cl['volumes']==[] and cl['ledger']['counts']==dict(leases=0,checks=0,fences=0,checkFences=0)
assert cl['fixtureFilesCount']==31 and cl['originalFilesUnchanged'] and cl['fixtureFilesUnchanged']
assert len([x for x in cl['identities'] if x['now']['status']=='absent'])==3
assert len([x for x in cl['identities'] if x['now'].get('state')=='Z' and x['now'].get('parentPID')==1])==2
launch=read(S/'driver/accepted-harness/launch.mjs').decode()
assert "receipt.passed=!failure&&receipt.scenario?.passed===true&&receipt.cleanup.length===0&&receipt.cleanupPhases.every(x=>x.status==='completed');" in launch
analysis=dict(productSource=C,imageID=I,scenarioPassed=True,overallPassed=False,launcherExitCode=1,recordedFailure=None,recordedFailedStage=None,overallFailureCause='Unrecorded. Final boolean requires no hidden failure. All other recorded operands pass. Two PID 1 Chromium zombies are observed residue, not a demonstrated cause.',originalResult=meta(read(S/'driver/result.json')),originalWrapper=meta(read(S/'wrapper-result.json')),firstTerminal=dict(exitCode=0,outputExitCloseSequence=[76,77,78],browserCloseCode=1000,clean=True),secondTerminal=dict(exitCode=7,outputExitCloseSequence=[77,78,79],browserCloseCode=1000,clean=True),firstNaturalRelease=dict(phase=22,elapsedMs=ph(22)['elapsedMs'],polls=ph(22)['polls']),secondNaturalRelease=dict(phase=44),browserRestartCount=1,apiRestartCount=0,stopOrDeleteCount=0,pendingMutationCount=0,coldExecutionFingerprintOmitted=True,realNewGenerationPollBeforeAcknowledgement=True,identicalParsedSessionJSON=False,noReplacementAttachBeforeAcknowledgement=True,installedC071Status=200,installed409DiagnosticExercised=False,closure=dict(absentIdentities=3,pid1Zombies=2,containers=0,volumes=0,ledgerCounts=cl['ledger']['counts'],fixtureFilesUnchanged=31),scope='Read-only retained local Linux Docker/browser shell evidence. No runtime launch, cleanup mutation, private raw log read, admission-capacity benchmark, model/provider validation, Windows certification, raw Close-frame direction/count, or overall acceptance.')
put('analysis.json',enc(analysis))
readme='''# Installed b94 cold terminal evidence — overall failed, scenario passed

The original cold browser-reload scenario passed, but the driver and wrapper failed overall (launcher exit 1). The receipt does not retain a final failure or failed stage. All recorded cleanup phases completed and cleanup errors are empty. The pinned final expression also requires the hidden `failure` value to be empty, so a failure occurred outside the retained main-catch projection. Its provenance is unknown. Two PID 1 Chromium zombies are observed residue; the available evidence does not identify them as the cause. CLOUD-072 tracks the missing late-failure diagnostic. Original receipts are unchanged.

Product source is `b94ad37e53181e360ac8910ee980570de622728c`, runtime image `76043c5d…71acec8`. The six accepted helpers are retained; four Git helpers match that immutable product commit, and launch/scenario hashes match the run and the unchanged outer source-before/after receipts. All 106 runtime inputs match b94. The retained package verification reports 963 matching members. Publication does not reread or rebuild the binary/archive/image.

| Observed result | Evidence |
|---|---|
| Native shell exit 0, then browser-reported clean Close1000 | phase011; output/Exit/close sequence 76/77/78 |
| Original exact allocation and manual reservation naturally absent before Restart | phase022; 44 polls, 32.332788356 seconds |
| Cold reopened ended row omits execution fingerprint | phases023–026; ended screenshot |
| New-generation running poll reaches cold view before Restart acknowledgement; no replacement socket yet | phase032 |
| Acknowledgement precedes replacement attachment | phase034 at 16:29:51.206Z; socket creation phase039 at 16:29:51.224Z |
| Native shell exit 7, then browser-reported clean Close1000 | phase042; sequence 77/78/79 |
| Replacement exact allocation and reservation naturally absent | phase044, before cleanup |
| Exactly one browser Restart; no API Restart, Stop or DELETE assistance | result actions/mutations; pending mutations empty |

The held response says `starting` and the early poll says `running`; `identicalParsedSessionJSON=false`. This does not demonstrate an identical-DTO race. The installed Restart response is **200 only**: it does not exercise C071's non-200 diagnostic branch. Browser close events report clean 1000; CDP contains no decoded Close frames, so direction/count of raw Close frames is not established.

Independent read-only closure records three tracked identities absent, two exact-start Chromium zombies parented to PID 1, empty selected-daemon container/volume inventory, empty four-array ledger, and 31 private fixture files unchanged. Historical `knownAllocations` entries are not surviving resources. This is not whole-machine process certification or automatic-cleanup acceptance. The 16.009196-second first retirement belongs to separate warm attempt2, not this cold attempt.

This capsule exports safe structured receipts, accepted source, and two visually reviewed disposable fixture screenshots. The exact private fixture root is replaced consistently; original and public hashes record every substitution. Raw controller/transport logs, fixture contents, auth files and launcher output are excluded. Private sinks are represented only by existing writer hashes/lengths. The source manifest and checksums bind the publication; original immutability checks cover selected published originals. No runtime, Docker mutation, tests, or product edits were performed for this audit.
'''
put('README.md',readme.encode())
copy(Path(__file__),'assemble.py')
excluded=[dict(path='driver/private-fixture-path',reason='Read only for exact root substitution; pointer excluded'),dict(path='driver/private-controller.stderr',bytes=r['controllerLog']['retainedBytes'],sha256=r['controllerLog']['retainedPrefixSHA256'],scope='Writer-reported only; raw file not reread'),dict(path='driver/private-transport-errors.jsonl',bytes=r['privateTransportErrors']['bytes'],sha256=r['privateTransportErrors']['sha256'],scope='Writer-reported only; raw file not reread'),dict(path='launcher.stdout / launcher.stderr / fixture and auth contents',reason='Not read or exported by publisher'),dict(path='binary / archive / image',reason='Retained provenance and writer verification only')]
put('original-bindings.json',enc(dict(originals=originals,excluded=excluded,redactionPolicy=dict(replacement=replacement,rootSHA256=sha(private.encode()),rootBytes=len(private.encode()),onlyExactRootSubstitution=True))))
for x in originals:assert meta(read(Path(x['source'])))==x['original']
put('original-immutability.json',enc(dict(checked=len(originals),allUnchanged=True,scope='Selected publication originals before/after only. Existing independent closure separately attests 31 unchanged private fixture files; no new private fixture census.')))
patterns={'private-key':rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----','github-token':rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})','openai-token':rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}','bearer-value':rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}','url-userinfo':rb'https?://[^/@\s:]+:[^/@\s]+@'}
findings=[]
for p in O.rglob('*'):
 if not p.is_file():continue
 b=p.read_bytes();assert private.encode() not in b
 if p.suffix=='.png':continue
 for k,v in patterns.items():
  if re.search(v,b):findings.append(dict(path=str(p.relative_to(O)),pattern=k))
assert not findings,findings
put('privacy-scan.json',enc(dict(exactPrivateRootAbsent=True,boundedPatternFindings=findings,screenshotsManuallyReviewed=['cold-exited-before-restart.png','cold-restarted-nonzero-ended.png'],scope='Common credential pattern scan and visual screenshot review, not exhaustive detection. No private raw logs or fixture contents read.')))
m={str(p.relative_to(O)):meta(p.read_bytes()) for p in sorted(O.rglob('*')) if p.is_file()}
put('publication.json',enc(dict(version=1,productSource=C,overallPassed=False,scenarioPassed=True,scope='Retained installed cold positive functional scenario with unrecorded overall failure cause',files=m)))
put('SHA256SUMS',(''.join(v['sha256']+'  '+k+'\n' for k,v in m.items())+sha((O/'publication.json').read_bytes())+'  publication.json\n').encode())
print(json.dumps(dict(capsule=str(O),files=len(m)+2,publicationSHA256=sha((O/'publication.json').read_bytes()),checksumsSHA256=sha((O/'SHA256SUMS').read_bytes()),bytes=sum(p.stat().st_size for p in O.rglob('*') if p.is_file())),indent=2))
