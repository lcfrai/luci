from pathlib import Path
import os,json,hashlib,importlib.util,subprocess,stat,traceback
BASE=Path('<PRIVATE_TOOL_ROOT>'); repo=Path('<CHECKOUT>'); checker=BASE/'terminal-retirement-preparation/preflight.py'
sha=lambda b:hashlib.sha256(b).hexdigest()
assert sha(checker.read_bytes())=='f0357901cdfa808e3693b6ee508627b5d3330c421cc87f8aab031ecad5b68e3c'
spec=importlib.util.spec_from_file_location('checks',checker);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
run=repo/'artifacts/quality-swarm/check-recovery-2da1a821-helper-2da1a821-attempt8';r=json.loads((run/'result/report.json').read_text());original=json.loads((BASE/'check8-closure-private/independent-closure.json').read_text());root=Path(original['fixture']);dest=BASE/'terminal-retired-check8-4b9b07e9';out=BASE/'check8-terminal-retirement';out.mkdir(mode=0o700,exist_ok=False)
assert root.is_dir() and root.resolve()==root and not dest.exists();assert original['fixtureUnchanged'] and original['originalArtifactInputsUnchanged'];assert sha((run/'result/report.json').read_bytes())==original['originalArtifactInputs'][str(run/'result/report.json')]
fence=original['fence'];marker='4b9b07e96a0f5afdd7e5aa3118c707544905a993212948c4064a360f874e8373';assert fence['proof']['fenceRef']==marker
config={'productCommit':r['sourceCommit'],'helperCommit':r['helperSourceCommit'],'targetCheckId':'check-5a719faeb22b400c1678b25e','allocationId':'checkexec-1050be441a70586a2442be85','originalContainerId':'d8d410410a198ff579f6b75eebd11701db1209a9f6f0643c1ce2d499ae5f6171','markerId':marker,'engineId':fence['identity']['engineId'],'dockerHost':'unix:///var/run/docker.sock','inputs':{'guardSource':{'path':str(repo/'scripts/acceptance/check-create-recovery/fixture-guards.mjs')}}}
assert config['productCommit']==config['helperCommit']=='2da1a821b8456c20a858fb86ef87649ea917d9de'
wrapper=json.loads((run/'wrapper-exit.json').read_text());m.check_receipts(config,{'recoveryReceipt':r,'wrapperExit':wrapper})
inputs={str(p):sha(p.read_bytes()) for p in [checker,Path(__file__),run/'result/report.json',run/'wrapper-exit.json',BASE/'check8-closure-private/independent-closure.json',Path(config['inputs']['guardSource']['path'])]};m.write_new(out/'input-bindings.json',inputs)
rec={'passed':False,'at':m.now(),'originalAcceptancePassed':True,'scope':'Separate terminal fixture retirement after passed installed recovery. Immutable product receipt remains marker-retained; this is not product automatic cleanup. Archived state is forensic-only and must never resume.','fixture':str(root),'retiredFixture':str(dest),'markerId':marker,'steps':[]}
def stage(name,**data):
 row={'stage':name,'at':m.now(),**data};rec['steps'].append(row);m.write_new(out/(str(len(rec['steps']))+'-'+name+'.json'),row)
def observe():
 known=[]
 for x in original['recordedIdentities']:
  expected=x['recorded'];actual=m.identity(expected['pid']);same=actual and str(actual['startTicks'])==str(expected['startTicks']);state='absent' if actual is None else 'reused' if not same else 'exited-zombie' if actual['state']=='Z' else 'live';assert state!='live'
  if state=='exited-zombie':assert expected['role']=='browser' and actual['parentPid']==1
  known.append({'expected':expected,'observation':actual,'result':state})
 wrapper_id=next(x['recorded'] for x in original['recordedIdentities'] if x['recorded']['role']=='wrapper');owned=[];unreadable=[];otheruid=0
 for p in Path('/proc').iterdir():
  if not p.name.isdigit():continue
  try:
   actual=m.identity(int(p.name))
   if not actual or actual['state']=='Z':continue
   if p.stat().st_uid!=os.getuid():otheruid+=1;continue
   try:
    env=(p/'environ').read_bytes();cmd=(p/'cmdline').read_bytes()
    if any(str(x).encode() in env or str(x).encode() in cmd for x in (root,dest)):owned.append(actual)
   except PermissionError:
    facts=m.minimal_facts(int(p.name));assert facts['pid']==260700 and facts['startTicks']=='2331192' and facts['uid']==os.getuid()==1000 and facts['comm']=='gpg-agent' and facts['parentPid']==1 and int(facts['startTicks'])<wrapper_id['startTicks']
    assert not any(x['recorded']['pid']==facts['pid'] for x in original['recordedIdentities']);unreadable.append({'facts':facts,'scope':'Exact live preexisting process; environment unreadable; predates this wrapper and is not an owned child. Not absence.'})
  except (FileNotFoundError,ProcessLookupError):continue
 assert not owned
 return {'at':m.now(),'known':known,'ownedActive':owned,'preexistingUnreadable':unreadable,'otherUidUninspectedCount':otheruid,'scope':'Exact known identities and readable same-UID fixture ownership only; no whole-machine absence claim.'}
def originals_unchanged():
 assert all(sha(Path(p).read_bytes())==h for p,h in original['originalArtifactInputs'].items())
 assert all(sha(Path(p).read_bytes())==h for p,h in inputs.items())
try:
 originals_unchanged();before=observe();assert not os.path.lexists(root/'docker-proxy.sock') and not list(root.glob('.luci-fence-proxy-*'))
 ledger=m.ordinary(root/'state/control-plane/execution/leases.json');parsed=json.loads(ledger);assert not parsed['leases'] and not parsed.get('checks',[]) and parsed['checkFences']==[fence] and sha(ledger)==original['ledgerSHA256']
 manifest=m.fixture_manifest(root);m.write_new(out/'fixture-manifest.json',manifest);archive=m.archive_fixture(root,out,manifest);rec['archive']=archive;rec['ledgerSHA256']=sha(ledger);stage('archive-verified',archive=archive,closure=before,marker=m.marker(config,fence))
 assert m.fixture_manifest(root)==manifest;originals_unchanged();stage('pre-move-closure',closure=observe(),marker=m.marker(config,fence))
 os.rename(root,dest)
 tomb={'terminallyRetired':True,'doNotResume':True,'receipt':str(out/'retirement-final.json'),'originalRecoverySHA256':inputs[str(run/'result/report.json')],'archivedFixtureForensicOnly':True}
 m.write_new(root,tomb);root.chmod(0o400);tombbytes=root.read_bytes();assert stat.S_ISREG(root.lstat().st_mode) and root.lstat().st_nlink==1 and not root.lstat().st_mode&0o222
 for child in ['state','projects']:
  try:os.stat(root/child);raise AssertionError('old path remains traversable')
  except NotADirectoryError:pass
 assert m.fixture_manifest(dest)==manifest and m.ordinary(dest/'state/control-plane/execution/leases.json')==ledger
 stage('forensic-move-and-nonresumption-barrier',tombstoneSHA256=sha(tombbytes),manifestUnchanged=True,closure=observe(),marker=m.marker(config,fence))
 assert m.file_digest(Path(archive['path']),m.MAX_BYTES+64*1024**2)==archive['sha256'];originals_unchanged()
 cmd=['docker','container','rm',marker];q=subprocess.run(cmd,env={'PATH':os.environ['PATH'],'DOCKER_HOST':config['dockerHost']},capture_output=True,text=True,timeout=15);stage('exact-id-nonforce-removal',command=cmd,exitCode=q.returncode,stdout=q.stdout.strip(),stderr=q.stderr.strip());assert q.returncode==0 and q.stdout.strip()==marker
 q=subprocess.run(['docker','inspect',marker],env={'PATH':os.environ['PATH'],'DOCKER_HOST':config['dockerHost']},capture_output=True,text=True,timeout=10);assert q.returncode==1 and ('No such object: '+marker) in q.stderr
 assert m.docker(config,'info','--format','{{.ID}}')==config['engineId'];assert not m.docker(config,'ps','-aq','--no-trunc') and not m.docker(config,'volume','ls','-q');assert m.fixture_manifest(dest)==manifest and m.ordinary(dest/'state/control-plane/execution/leases.json')==ledger and root.read_bytes()==tombbytes;originals_unchanged()
 stage('final-closure',closure=observe(),containers=[],volumes=[],markerAbsent=True,ledgerAndFixtureBytesUnchanged=True)
 rec.update(passed=True,finishedAt=m.now(),markerAbsent=True,archiveForensicOnlyNeverResume=True,originalReceiptUnchanged=True,tombstoneSHA256=sha(tombbytes));m.write_new(out/'retirement-final.json',rec)
 print(json.dumps({'passed':True,'markerRemoved':marker,'containers':0,'volumes':0,'fixtureAndLedgerBytesPreserved':True,'originalPathTombstoned':True,'originalAcceptanceUnchanged':True,'receiptSHA256':sha((out/'retirement-final.json').read_bytes())}))
except BaseException as e:
 rec['failure']={'type':type(e).__name__,'message':str(e),'stack':traceback.format_exc()};m.write_new(out/'retirement-failure.json',rec);raise
