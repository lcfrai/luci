from pathlib import Path
import os,json,hashlib,subprocess,datetime,stat
repo=Path('<CHECKOUT>');out=repo/'artifacts/quality-swarm/check-recovery-2da1a821-helper-2da1a821-attempt8'
r=json.loads((out/'result/report.json').read_text());summary=json.loads((out/'observation/summary.json').read_text());cost=json.loads((out/'observation-observer-self-cost.json').read_text());life=[json.loads(l) for l in (out/'observation-lifecycle.jsonl').read_text().splitlines()]
assert r['passed'] is True and not r.get('failure');assert r['dockerTransportObservations']['failures']==[];assert r['cleanup']==[] and r['controllersStopped'] and r['dockerRequestsDrained'];assert r['proxyClosure']['drained'] and not r['proxyClosure']['uncertainMutation'];assert all(x['state']=='closed' and not x['late'] and not x['closeTimedOut'] for x in r['startupResources'])
instance=Path((out/'observation-instance-path.txt').read_text().strip());fixture=next(p for p in instance.parents if p.name.startswith('luci-check-'));assert fixture.is_absolute() and fixture.resolve()==fixture
sha=lambda b:hashlib.sha256(b).hexdigest()
def inventory():
 rows=[];total=0
 for base,dirs,files in os.walk(fixture,followlinks=False):
  for name in sorted(dirs+files):
   p=Path(base)/name;s=p.lstat();rel=str(p.relative_to(fixture))
   if stat.S_ISREG(s.st_mode):
    assert s.st_size<=32*1024**2;b=p.read_bytes();total+=len(b);rows.append([rel,len(b),sha(b)])
   elif stat.S_ISLNK(s.st_mode):rows.append([rel,'symlink',sha(os.readlink(p).encode())])
 assert len(rows)<20000 and total<512*1024**2
 return {'files':len(rows),'bytes':total,'rows':sorted(rows),'sha256':sha(json.dumps(sorted(rows),separators=(',',':')).encode())}
before=inventory();originals={str(p):sha(p.read_bytes()) for p in out.rglob('*') if p.is_file()}
def ident(pid):
 try:
  f=Path('/proc',str(pid),'stat').read_text().rsplit(') ',1)[1].split();return {'pid':pid,'startTicks':int(f[19]),'state':f[0],'ppid':int(f[1])}
 except (FileNotFoundError,ProcessLookupError):return None
known={x['pid']:x for x in summary['processIdentities']};known[cost['pid']]={'pid':cost['pid'],'startTicks':cost['processStartClockTicks'],'role':'observer'}
w=next(x for x in life if x['event']=='wrapper-start');known[w['pid']]={'pid':w['pid'],'startTicks':w['startTicks'],'role':'wrapper'}
checks=[]
for k in known.values():
 current=ident(k['pid']);same=current and current['startTicks']==k['startTicks'];state='absent' if not current else 'reused' if not same else 'exited-zombie' if current['state']=='Z' else 'live';assert state!='live';checks.append({'recorded':k,'current':current,'observation':state})
owned=[];unreadable=[]
for p in Path('/proc').iterdir():
 if not p.name.isdigit():continue
 i=ident(int(p.name))
 if not i or i['state']=='Z':continue
 try:
  vals=dict(x.split(b'=',1) for x in (p/'environ').read_bytes().split(b'\0') if b'=' in x);cmd=(p/'cmdline').read_bytes()
  if any(str(fixture).encode() in v for k,v in vals.items() if k in [b'LUCI_STATE_DIR',b'LUCI_PROJECTS_DIR',b'HOME']) or str(fixture).encode() in cmd:owned.append(i)
 except (FileNotFoundError,ProcessLookupError):pass
 except PermissionError:unreadable.append(i)
assert not owned
ledgerPath=fixture/'state/control-plane/execution/leases.json';lb=ledgerPath.read_bytes();ledger=json.loads(lb);assert not ledger['leases'] and not ledger.get('checks',[]);assert len(ledger['checkFences'])==1
fence=ledger['checkFences'][0];assert fence['state']=='retired';expected=next(x for x in r['phases'] if x['phase']=='positive-fence-proof');assert fence['proof']==expected['proof'];assert sha(lb)==r['independentClosure']['ledgerSHA256'];marker=fence['proof']['fenceRef'];assert marker=='4b9b07e96a0f5afdd7e5aa3118c707544905a993212948c4064a360f874e8373'
docker=lambda *a:subprocess.check_output(['docker',*a],text=True,timeout=15)
containers=docker('ps','-aq','--no-trunc').split();volumes=docker('volume','ls','-q').split();assert containers==[marker] and not volumes
inspect=json.loads(docker('inspect',marker));assert len(inspect)==1
work=Path('<PRIVATE_TOOL_ROOT>/check8-closure-private');work.mkdir(mode=0o700,exist_ok=False);(work/'fence.json').write_text(json.dumps(fence));(work/'inspect.json').write_text(json.dumps(inspect[0]))
script="import {readFileSync} from 'node:fs';import {validateFence} from '<CHECKOUT>/scripts/acceptance/check-create-recovery/fixture-guards.mjs';const record=JSON.parse(readFileSync(process.argv[1]));validateFence(JSON.parse(readFileSync(process.argv[2])),{record,name:'luci-check-'+record.identity.allocationId,inspected:true});"
subprocess.run(['node','--input-type=module','-e',script,str(work/'fence.json'),str(work/'inspect.json')],check=True,timeout=10,capture_output=True)
assert docker('info','--format','{{.ID}}').strip()==fence['identity']['engineId'];after=inventory();assert before==after;assert all(sha(Path(p).read_bytes())==h for p,h in originals.items())
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Independent read-only closure after overall passed helper; exact retired recovery proof/marker remains, not a whole-machine process absence claim.','fixture':str(fixture),'recordedIdentities':checks,'ownedActiveProcesses':owned,'unreadableProcessEnvironments':unreadable,'dockerContainers':containers,'dockerVolumes':volumes,'fence':fence,'markerShapeValidated':True,'ledgerSHA256':sha(lb),'activeLeaseCount':0,'fixtureInventoryBefore':before,'fixtureInventoryAfter':after,'fixtureUnchanged':True,'originalArtifactInputs':originals,'originalArtifactInputsUnchanged':True,'noSignalsOrMutationsPerformed':True}
p=work/'independent-closure.json';p.write_text(json.dumps(result,indent=2)+'\n');p.chmod(0o600)
print(json.dumps({'at':result['at'],'identities':len(checks),'observations':{s:sum(x['observation']==s for x in checks) for s in ['absent','reused','exited-zombie','live']},'unreadableEnvironments':len(unreadable),'containers':containers,'volumes':volumes,'retiredFences':1,'activeCharges':0,'fixtureFiles':before['files'],'fixtureUnchanged':True,'originalArtifactsUnchanged':True,'privateReceiptSHA256':sha(p.read_bytes())}))
