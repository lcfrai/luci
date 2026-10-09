#!/usr/bin/env python3
"""Publish one failed installed shell run; private error/state bytes excluded."""
from pathlib import Path
import json,hashlib,datetime,subprocess,os
B=Path('/workspace/.luci-tools/cloud054-terminal-close')
S=B/'actual-5f4a966f/evidence/attempt8'
O=Path(__file__).parent/'capsule'
C='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
def sha(b):return hashlib.sha256(b).hexdigest()
def meta(b):return {'sha256':sha(b),'bytes':len(b)}
def enc(x):return (json.dumps(x,indent=2)+'\n').encode()
def read(p):return json.loads(p.read_bytes())
def put(p,b):p.parent.mkdir(parents=True,exist_ok=True,mode=0o700);assert not p.exists();p.write_bytes(b);p.chmod(0o600)
os.umask(0o077);assert not O.exists();O.mkdir(mode=0o700)
# Only the pointer is read to remove that exact private fixture root. No state/log bytes.
private=(S/'driver/private-fixture-path').read_text().strip();assert private.startswith(str(B/'actual-5f4a966f/fixtures')+'/')
replacement='[PRIVATE_FIXTURE_ATTEMPT_8]';orig=[]
def copy(p,name):
 before=p.stat();assert p.is_file()and not p.is_symlink()and before.st_size<=4*1024*1024
 raw=p.read_bytes();after=p.stat();assert (before.st_ino,before.st_size,before.st_mtime_ns)==(after.st_ino,after.st_size,after.st_mtime_ns)
 out=raw.replace(private.encode(),replacement.encode());assert p.suffix!='.png'or out==raw
 put(O/name,out);orig.append({'source':str(p),'publishedPath':name,'original':meta(raw),'published':meta(out),'representation':'exact-original'if raw==out else'exact-private-root-substitution','substitutions':raw.count(private.encode())})

r=read(S/'driver/result.json');w=read(S/'wrapper-result.json');safe=read(B/'closure-attempt8/safe-projection.json')
assert r['sourceCommit']==r['helperSourceCommit']==C and not r['passed']and not w['passed']
assert w['driverReport']['sha256']==safe['driverResultSHA256']==sha((S/'driver/result.json').read_bytes())
assert safe['wrapperResultSHA256']==sha((S/'wrapper-result.json').read_bytes())
assert safe['workflowPassed']is False and safe['failedStep']=='first-poll-release'
assert w['sourceBefore']==w['sourceAfter']
for p,m in w['sourceBefore'].items():assert meta(Path(p).read_bytes())==m
for name in ['before-launch.json','identities.jsonl','launcher.stderr','launcher.stdout','wrapper-result.json']:
 copy(S/name,'run/'+name)
for p in sorted((S/'driver').iterdir()):
 if p.is_file()and(p.name.startswith('phase-')or p.name in ['result.json','result.json.incomplete.json','package-provenance.json','image-provenance.json','failure.png','first-ended-poll-held.png']):copy(p,'run/driver/'+p.name)
for p in sorted((S/'driver/accepted-harness').rglob('*')):
 if p.is_file():copy(p,'sources/accepted-harness/'+str(p.relative_to(S/'driver/accepted-harness')))
for name,key in [('launch.mjs','launcherSHA256'),('scenario.mjs','scenarioSHA256')]:
 assert sha((S/'driver/accepted-harness'/name).read_bytes())==r[key]==sha((B/'installed-v8'/name).read_bytes())
for row in r['helperBindings']:
 raw=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+row['path']],timeout=30)
 assert meta(raw)=={k:row[k]for k in ['sha256','bytes']}
 assert(S/'driver/accepted-harness'/row['path'].removeprefix('scripts/acceptance/')).read_bytes()==raw
copy(B/'outer-v2/run.py','sources/outer-run.py')
for name in ['safe-projection.json','project-safe-result.py','independent-closure.py','prepared.json']:
 copy(B/'closure-attempt8'/name,'closure/'+name)
image=read(S/'driver/image-provenance.json');assert image['sourceCommit']==C and image['candidateImage']==safe['imageID']=='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
assert image['status']=='export-verified-runtime-pending'and image['sourcesBefore']==image['sourcesAfter']and len(image['sourcesAfter'])==106
bindings=[]
for p,h in image['sourcesAfter'].items():
 raw=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+p],timeout=30);assert sha(raw)==h;bindings.append({'path':p,'sha256':h,'bytes':len(raw)})
put(O/'runtime-source-bindings.json',enc({'commit':C,'imageID':image['candidateImage'],'count':106,'inputs':bindings,'scope':'Current runtime source hashes checked against immutable Git, not old104 inputs or independent image download.'}))
typed=next(x for x in r['phases']if x['kind']=='typed-exit-before-poll');failure=next(x for x in r['phases']if x['kind']=='scenario-failure');last=next(x for x in r['phases']if x['kind']=='scenario-failure-observation')
for phase in [typed,last]:
 rows=phase['wire']['browser']['rows'];assert len(rows)==1;row=rows[0]
 assert not row['error']and row['closed']['code']==1000 and row['closed']['wasClean']is True
 assert[row['exit'][0]['reason']]==[{'kind':'exit-status','code':0}]and len(row['exit'])==1
 assert row['markers']['first']['sequence']<row['exit'][0]['sequence']<next(e['sequence']for e in row['events']if e['kind']=='close')
 assert row['exit'][0]['payload']['sha256']==sha(b'exit status 0')
 assert phase['ui']['restartCount']==0 and phase['ui']['label'].endswith(' — running')
assert failure['step']=='first-poll-release'
assert not[x for x in r['browserMutations']if x['path'].endswith('/restart')]
assert any(x['method']=='DELETE'and x['path'].endswith('/s-05496235')and x['status']==200 for x in r['apiActions'])
assert r['final']['ledger']['leases']==[]and r['final']['containers']==[]and r['final']['volumes']==''and not r['pendingMutations']
assert r['privateTransportErrors']['rows']==0 and r['privateTransportErrors']['passed']is True
assert len(r['controllerLog']['relayRows'])==1 and r['controllerLog']['relayRows'][0]['flags']['websocketStatus']==1000
assert safe['closure']['knownZombieCount']==2 and safe['closure']['knownLiveProcessCount']==0
assert sorted(x['pid']for x in safe['closure']['identities']if x['now'].get('state')=='Z')==[2442558,2442561]
excluded=[{'path':'driver/private-fixture-path','reason':'Used only for exact redaction; pointer bytes and fixture root excluded'},
 {'path':'driver/private-controller.stderr','bytes':r['controllerLog']['retainedBytes'],'sha256':r['controllerLog']['retainedPrefixSHA256'],'hashScope':'Writer-reported private retained bytes, not reread by publisher','reason':'Raw potentially private log text excluded'},
 {'path':'driver/private-transport-errors.jsonl','bytes':r['privateTransportErrors']['bytes'],'sha256':r['privateTransportErrors']['sha256'],'hashScope':'Writer-reported private retained bytes, not reread by publisher','reason':'Private diagnostic sink excluded even when empty'},
 {'path':'independent-closure.json','sha256':safe['independentClosureSHA256'],'hashScope':'Bound by provided safe projection; private original not read or copied','reason':'Use safe whitelisted closure projection only'},
 {'path':'launcher-home/','reason':'Private CLI environment directory excluded'},
 {'path':'original fixture contents, binary package/image/archive','reason':'No bytes read/copied; scope remains publisher receipts'}]
summary={'productSource':C,'imageID':image['candidateImage'],'helperVersion':8,'overallPassed':False,'originalResult':meta((S/'driver/result.json').read_bytes()),'originalWrapper':meta((S/'wrapper-result.json').read_bytes()),'failure':failure,'primaryObservation':typed,'finalPassiveObservation':last,'cleanup':r['cleanup'],'closure':safe['closure'],'positiveScope':'One actual interactive shell output/typed-native-exit0/normal browser1000 closure with no socketerror and no duplicate browser socket through recorded failure observation. This is transport evidence only.','openScope':'Fresh exited state and Restart were not reached; no same-ID restart or second exit7. Source C063 liveness correction, Windows/model/performance/recovery acceptance separate.','privacy':'No private stderr/error text/fixture contents/private closure read or published. Original/public hashes distinguish root-substituted result bytes.','sourceChecks':{'utilityGitHelpers':4,'currentRuntimeInputs':106,'acceptedHelpersMatchSourcePins':True,'sourceBeforeAfterEqual':True},'wireLimits':'CDP observed zero opcode8 rows; actual browser CloseEvent1000/wasClean is separately retained, without asserting absence or direction of network close frames.'}
put(O/'analysis.json',enc(summary));copy(Path(__file__).parent/'README.prepared.md','README.md');copy(Path(__file__),'assemble.py')
for n in ['assemble.py','assembly.stdout','assembly.stderr']:
 copy(Path(__file__).parent/'assembly-attempt1-volume-type'/n,'publication-attempt1/'+n)
put(O/'original-bindings.json',enc({'originals':orig,'excluded':excluded,'redactionPolicy':{'replacement':replacement,'rootSHA256':sha(private.encode()),'rootBytes':len(private.encode()),'onlyExactRootSubstitution':True}}))
for x in orig:assert meta(Path(x['source']).read_bytes())==x['original']
put(O/'original-immutability.json',enc({'checked':len(orig),'allUnchanged':True,'scope':'Publication before/after originals only; no repeat runtime or private fixture census'}))
for p in O.rglob('*'):
 if p.is_file():assert private.encode()not in p.read_bytes()
members={str(p.relative_to(O)):meta(p.read_bytes())for p in sorted(O.rglob('*'))if p.is_file()}
put(O/'publication.json',enc({'version':1,'productSource':C,'overallPassed':False,'files':members}))
put(O/'SHA256SUMS',(''.join(v['sha256']+'  '+k+'\n'for k,v in members.items())+sha((O/'publication.json').read_bytes())+'  publication.json\n').encode())
print(json.dumps({'members':len(members),'originals':len(orig),'publicationSHA256':sha((O/'publication.json').read_bytes())},indent=2))
