from pathlib import Path
import os,json,hashlib,subprocess,datetime,stat,re,base64
repo=Path('/workspace/luci'); out=repo/'artifacts/quality-swarm/worker-recovery-d348f012-attempt3'
result=json.loads((out/'result/result.json').read_text());wrapper=json.loads((out/'wrapper-result.json').read_text());summary=json.loads((out/'observer/summary.json').read_text())
fixture=Path(result['fixtureRetained']['path']);assert fixture.is_absolute() and fixture.resolve()==fixture and str(fixture).startswith('/workspace/.ls-')
instance=Path((out/'instance-path.txt').read_text().strip());assert instance.is_relative_to(fixture);state=instance.parent.parent;runtime=state.parent
sha=lambda b:hashlib.sha256(b).hexdigest()
def inventory(root):
 rows=[];total=0
 for base,dirs,files in os.walk(root,followlinks=False):
  for name in sorted(dirs+files):
   p=Path(base)/name;s=p.lstat();relative=str(p.relative_to(root))
   if stat.S_ISREG(s.st_mode):
    assert s.st_size<=32*1024**2;data=p.read_bytes();total+=len(data);rows.append([relative,len(data),sha(data)])
   elif stat.S_ISLNK(s.st_mode):rows.append([relative,'symlink',sha(os.readlink(p).encode())])
 assert len(rows)<20000 and total<512*1024**2
 return {'files':len(rows),'bytes':total,'sha256':sha(json.dumps(sorted(rows),separators=(',',':')).encode()),'rows':sorted(rows)}
before=inventory(fixture);originals={str(p):sha(p.read_bytes()) for p in out.rglob('*') if p.is_file()}
def ident(pid):
 try:
  fields=Path(f'/proc/{pid}/stat').read_text().rsplit(')',1)[1].split();return {'pid':pid,'startTicks':int(fields[19]),'state':fields[0]}
 except (FileNotFoundError,ProcessLookupError):return None
identities={int(x['pid']):x for x in summary['processIdentities']}
for key in ['driverIdentity','observerIdentity']:
 v=wrapper[key];identities[v['pid']]={**v,'role':key}
identityChecks=[]
for pid,known in identities.items():
 current=ident(pid);live=bool(current and current['startTicks']==known['startTicks'] and current['state']!='Z');assert not live
 identityChecks.append({'recorded':known,'current':current,'exactIdentityLive':live})
owned=[];unreadable=0
for p in Path('/proc').iterdir():
 if not p.name.isdigit():continue
 i=ident(int(p.name))
 if not i or i['state']=='Z':continue
 try:
  values=dict(part.split(b'=',1) for part in (p/'environ').read_bytes().split(b'\0') if b'=' in part)
  stateValue=values.get(b'LUCI_STATE_DIR',b'').decode(errors='replace')
  if stateValue==str(fixture/'state') or stateValue.startswith(str(fixture/'state')+'/'):owned.append(i)
 except (FileNotFoundError,ProcessLookupError):pass
 except PermissionError:unreadable+=1
assert not owned
def docker(*args):return subprocess.check_output(['docker',*args],text=True,timeout=30).strip().splitlines()
containers=docker('ps','-aq','--no-trunc');volumes=docker('volume','ls','-q');assert not containers and not volumes
ledgerPath=fixture/'state/control-plane/execution/leases.json';ledgerBytes=ledgerPath.read_bytes();ledger=json.loads(ledgerBytes);assert ledger['version']==1 and ledger['leases']==[];assert not ledger.get('checkFences',[])
sessionsPath=state/'sessions/sessions.json';sessions=json.loads(sessionsPath.read_text())['sessions'];assert len(sessions)==2
schedulerPath=state/'sessions/swarm-scheduler.json';scheduler=json.loads(schedulerPath.read_text())
exports=['.luci/design-handoff.json','.luci/design-handoff.md','.luci/design.json','.luci/plan.json','.luci/specification.json','.luci/specification.md','LUCI.md']
expectedNames=sorted(exports+['.luci/brief.json','.luci/task-handoffs.json','manifest.json']);retained=[]
env={**os.environ,'GIT_OPTIONAL_LOCKS':'0','GIT_CONFIG_GLOBAL':'/dev/null','GIT_CONFIG_SYSTEM':'/dev/null','GIT_CONFIG_NOSYSTEM':'1','GIT_TERMINAL_PROMPT':'0'}
def git(cwd,*args):return subprocess.check_output(['git','--no-optional-locks','-c','core.fsmonitor=false',*args],cwd=cwd,env=env,text=True,timeout=15).strip()
for session in sessions:
 sid=session['id'];project=session['projectId'];runPath=state/'sessions/project-runs'/project/(session['projectRunId']+'.json');run=json.loads(runPath.read_text())['run'];ops=[op for op in run['operations'] if op['id']==session['projectRunOperationId']];assert len(ops)==1;op=ops[0];task=op['taskId'];actual=result['actualTaskResults'][task];assert op['childId']==sid and actual['operation']==op['id']
 checkout=runtime/'projects/.luci-swarms'/project/run['id']/op['id'];assert checkout.resolve().is_relative_to(fixture)
 primaries=[]
 for p in (runtime/'projects').iterdir():
  if p.name.startswith('.') or not p.is_dir():continue
  saved=p/'.luci/state.json'
  if saved.is_file() and json.loads(saved.read_text())['brief']['projectId']==project:primaries.append(p)
 assert len(primaries)==1;primary=primaries[0];capture=state/'conversations/local/.reviewed-context'/sid
 files={str(p.relative_to(capture)):p.read_bytes() for p in capture.rglob('*') if p.is_file()};assert sorted(files)==expectedNames
 captured={name:{'bytes':len(data),'sha256':sha(data)} for name,data in files.items()};assert captured==actual['files'];assert sha(files['manifest.json'])==session['reviewedContext']['digest']
 canonical={name:sha((primary/name).read_bytes()) for name in exports};assert not (primary/'.luci/brief.json').exists()
 for name in exports:
  if name=='LUCI.md':assert files[name].startswith((primary/name).read_bytes())
  else:assert canonical[name]==sha(files[name])
 head=git(checkout,'rev-parse','HEAD');primaryHead=git(primary,'rev-parse','HEAD');checkoutStatus=git(checkout,'status','--porcelain');primaryStatus=git(primary,'status','--porcelain');assert head==actual['commit'] and primaryHead==actual['base'] and checkoutStatus==primaryStatus==''
 changed=git(checkout,'diff','--name-only',actual['base'],'HEAD').splitlines();assert changed==actual['changed']==[task+'-result.txt'];output=(checkout/(task+'-result.txt')).read_bytes();assert sha(output)==actual['outputSHA256'];assert not (primary/(task+'-result.txt')).exists()
 native=state/'conversations/native/local'/session['worktreeId']/'codex';transcripts=[];occurrences=0
 for p in native.glob('*/sessions/**/*.jsonl'):
  data=p.read_bytes();assert 0<len(data)<=4*1024**2;count=0
  for encoded in re.findall(rb'LUCI_SWARM_OBSERVATION=([A-Za-z0-9+/=]+)',data):
   try:count+=json.loads(base64.b64decode(encoded))==actual
   except (ValueError,KeyError):pass
  occurrences+=count;transcripts.append({'pathSHA256':sha(str(p.relative_to(native)).encode()),'bytes':len(data),'sha256':sha(data),'matchingRecordedObservationOccurrences':count})
 assert transcripts and occurrences>0
 queue=[row for row in scheduler['items'] if row.get('childId')==sid];assert len(queue)==1;queue=queue[0];assert queue['state']=='completed' and queue.get('memoryMiB',0)==0 and queue.get('cpuMillis',0)==0
 assert session['status']=='exited' and session['exitReason']==('killed' if task=='build-a' else 'process exited') and op['status']=='review-required'
 retained.append({'task':task,'childId':sid,'projectId':project,'runId':run['id'],'operationId':op['id'],'sessionStatus':session['status'],'exitReason':session['exitReason'],'operationStatus':op['status'],'runState':run['state'],'queueId':queue['id'],'queueState':queue['state'],'queueMemoryMiB':queue.get('memoryMiB',0),'queueCPUMillis':queue.get('cpuMillis',0),'checkoutHead':head,'primaryHead':primaryHead,'checkoutStatus':checkoutStatus,'primaryStatus':primaryStatus,'changed':changed,'outputSHA256':sha(output),'outputBytes':len(output),'outputAbsentFromPrimary':True,'canonical':canonical,'capturedFiles':captured,'capturedFilesMatchActualCommand':True,'reviewedContext':session['reviewedContext'],'transcripts':transcripts,'runStoreSHA256':sha(runPath.read_bytes())})
after=inventory(fixture);assert before==after
assert all(sha(Path(p).read_bytes())==digest for p,digest in originals.items())
receipt={'version':1,'observedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'mode':'independent read-only exact fixture closure and retained artifact verification','fixture':str(fixture),'instanceFile':str(instance),'recordedIdentities':identityChecks,'ownedProcesses':owned,'unreadableProcessEnvironmentCount':unreadable,'dockerContainers':containers,'dockerVolumes':volumes,'ledger':{'path':str(ledgerPath),'sha256':sha(ledgerBytes),'version':ledger['version'],'leaseCount':len(ledger['leases']),'checkFenceCount':len(ledger.get('checkFences',[]))},'retained':retained,'fixtureInventoryBefore':before,'fixtureInventoryAfter':after,'fixtureUnchanged':True,'originalArtifactInputsUnchanged':True,'noSignalsOrMutationsPerformed':True,'sessionsStoreSHA256':sha(sessionsPath.read_bytes()),'schedulerStoreSHA256':sha(schedulerPath.read_bytes())}
private=Path('/workspace/.luci-tools/worker3-closure-private.json');assert not private.exists();private.write_text(json.dumps(receipt,indent=2)+'\n');private.chmod(0o600)
print(json.dumps({'observedAt':receipt['observedAt'],'recordedIdentities':len(identityChecks),'exactIdentitiesLive':0,'ownedProcesses':0,'dockerContainers':0,'dockerVolumes':0,'leases':0,'retainedTasks':len(retained),'fixtureFiles':before['files'],'fixtureUnchanged':True,'originalArtifactInputsUnchanged':True,'privateReceiptSHA256':sha(private.read_bytes())}))
