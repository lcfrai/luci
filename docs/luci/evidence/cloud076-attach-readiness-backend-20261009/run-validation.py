import os,json,pathlib,subprocess,sys,time,hashlib,signal,shutil
root=pathlib.Path(__file__).parent
mode=sys.argv[1]
out=root/'evidence'/mode
if out.exists():raise SystemExit('receipt already exists; use a new mode name')
out.mkdir(parents=True)
env=os.environ.copy()
env.update(GOMAXPROCS='2',GOMODCACHE='/tmp/luci-retained-tools/go-mod',GOPATH='/tmp/luci-gopath',GOCACHE='/tmp/luci-go-build-cache',GOTMPDIR='/workspace/.t',TMPDIR='/workspace/.t',GOTOOLCHAIN='local',GOWORK='/workspace/.luci-tools/go.work',GOFLAGS='-mod=readonly')
for key in ['GOCACHE','GOTMPDIR']:pathlib.Path(env[key]).mkdir(parents=True,exist_ok=True)
args=['/workspace/.luci-tools/go1.26.3/bin/go','test','-race','-p','1','-count=1','-timeout=90s','-json']
if mode.startswith('recovery-before'):args += ['-overlay='+str(root/'unsafe-overlay.json'),'-run','^TestSessionPreparationRecovery','./internal/api']
elif mode.startswith('before'):args += ['-overlay='+str(root/'original-overlay.json'),'-run','^TestSession(AttachReadinessHTTPRestartWaitsForRecordedEndpoint$|PreparationRecovery)','./internal/api']
elif mode.startswith('compile'):args += ['-overlay='+str(root/'overlay.json'),'-run','^$','./...']
elif mode.startswith('related'):args += ['-overlay='+str(root/'overlay.json'),'-run','Test(SessionExecutionGeneration|AttachStartingSandboxIsAllowedThrough|AttachRejects|AttachExitedSessionIs409|AttachBackendVerdictsBeforeTheUpgrade|RestartAdmission|ARestart)' ,'./internal/api']
else:args += ['-overlay='+str(root/'overlay.json'),'-run','^TestSession(AttachReadiness|Preparation)','./internal/api']
def hashes():
 return {str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'overlay.json',root/'original-overlay.json',root/'unsafe-overlay.json',root/'CLOUD-076-backend.patch',*sorted((root/'tree').rglob('*.go'))]}
freeBefore={'root':shutil.disk_usage('/workspace').free,'tmp':shutil.disk_usage('/tmp').free}
if freeBefore['root']<int(1.25*(1<<30)) or freeBefore['tmp']<256*(1<<20):raise SystemExit('storage floor refuses compilation before launch')
receipt={'sourceBaseCommit':'786652704fa8bebd1ba1af41cfa1d414d7def100','argv':args,'cwd':'/workspace/luci/control-plane','sourceCommit':subprocess.check_output(['git','-C','/workspace/luci','rev-parse','HEAD'],text=True).strip(),'startedAt':time.time(),'environment':{k:env[k] for k in ['GOMAXPROCS','GOMODCACHE','GOPATH','GOCACHE','GOTMPDIR','TMPDIR','GOTOOLCHAIN','GOWORK','GOFLAGS']},'beforeSHA256':hashes()}
(out/'command.json').write_text(json.dumps(receipt,indent=2)+'\n')
subprocess.run([sys.executable,str(root/'bind-effective-sources.py'),mode,'before'],check=True)
minimum={'root':None,'tmp':None};stopped=None
binaries={}
def record_binaries(parent):
 candidates=[parent]
 for current in candidates:
  try:
   
   for thread in pathlib.Path(f'/proc/{current}/task').iterdir():
    for child in thread.joinpath('children').read_text().split():
     if int(child) not in candidates:candidates.append(int(child))
   path=os.readlink(f'/proc/{current}/exe')
  except (OSError,ValueError):continue
  if path.endswith('.test') and path not in binaries:
   digest=hashlib.sha256()
   try:
    with open(f'/proc/{current}/exe','rb') as binary:
     for block in iter(lambda:binary.read(1<<20),b''):digest.update(block)
    binaries[path]={'sha256':digest.hexdigest(),'observedAt':time.time(),'pid':current}
   except OSError:continue
 if binaries:(out/'compiled-test-binaries.json').write_text(json.dumps(binaries,indent=2)+'\n')
with (out/'events.jsonl').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 p=subprocess.Popen(args,cwd=receipt['cwd'],env=env,stdout=stdout,stderr=stderr,start_new_session=True)
 while p.poll() is None:
  record_binaries(p.pid)
  free={'root':shutil.disk_usage('/workspace').free,'tmp':shutil.disk_usage('/tmp').free}
  for k,v in free.items():minimum[k]=v if minimum[k] is None else min(minimum[k],v)
  if free['root']<int(1.25*(1<<30)) or free['tmp']<256*(1<<20):
   stopped='headroom_floor';os.killpg(p.pid,signal.SIGTERM)
   try:p.wait(timeout=10)
   except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL)
   break
  time.sleep(1)
 code=p.wait()
subprocess.run([sys.executable,str(root/'bind-effective-sources.py'),mode,'after'],check=True)
receipt.update(exitCode=code,endedAt=time.time(),minimumFreeBytes=minimum,stopped=stopped,afterSHA256=hashes())
counts={};failed=[]
for line in (out/'events.jsonl').read_text().splitlines():
 try:x=json.loads(line)
 except ValueError:continue
 if 'Test' in x and x.get('Action') in ('pass','fail','skip'):
  counts[x['Action']]=counts.get(x['Action'],0)+1
  if x['Action']=='fail':failed.append(x['Test'])
receipt.update(testActions=counts,failedTests=failed,eventsSHA256=hashlib.sha256((out/'events.jsonl').read_bytes()).hexdigest(),stderrSHA256=hashlib.sha256((out/'stderr.log').read_bytes()).hexdigest())
(out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps({'mode':mode,'exitCode':code,'testActions':counts,'failedTests':failed,'stopped':stopped,'minimumFreeBytes':minimum}))
sys.exit(code)
