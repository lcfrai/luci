import hashlib,json,os,pathlib,subprocess,time,shutil,signal
root=pathlib.Path('/workspace/luci');out=pathlib.Path('/workspace/.luci-tools/local026-http-validation'); phase=os.environ['LOCAL026_PHASE']; target=out/phase;target.mkdir(exist_ok=True)
cache=pathlib.Path('/dev/shm/luci-go-cache-20261011-recovery');cache.mkdir(mode=0o700,exist_ok=True)
def resources():
 disk=shutil.disk_usage('/').free
 memory=int(pathlib.Path('/sys/fs/cgroup/memory.max').read_text())-int(pathlib.Path('/sys/fs/cgroup/memory.current').read_text())
 cachebytes=sum(p.stat().st_size for p in cache.rglob('*') if p.is_file())
 return {'rootFreeBytes':disk,'cgroupRemainingBytes':memory,'cacheBytes':cachebytes}
def guarded(r): return r['rootFreeBytes']>=512*1024*1024 and r['cgroupRemainingBytes']>=4*1024**3 and r['cacheBytes']<=2*1024**3
initial=resources()
if not guarded(initial):raise SystemExit('test environment admission refused: '+json.dumps(initial))
files=list((root/'control-plane').rglob('*.go'))+[root/'control-plane/go.mod',root/'control-plane/go.sum',root/'go.work',root/'go.work.sum']
files=[p for p in files if p.exists() and not any(x in p.parts for x in ['vendor','node_modules','.git'])]
overlay=json.loads((out/(phase+'-head-overlay.json')).read_text())
def manifest():
 rows=[]
 for p in sorted(files):
  effective=pathlib.Path(overlay['Replace'].get(str(p),str(p)));data=effective.read_bytes()
  rows.append({'path':str(p.relative_to(root)),'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data)})
 return rows
before=manifest();(target/'source-before.json').write_text(json.dumps(before,separators=(',',':'))+'\n')
cmd=['go','test','-p','1','-count=1','-timeout','90s','-json','./internal/product','-run','^(TestCoordinationMissingTurnReturnsBadRequestWithoutMutation|TestOrchestratorProblemKeepsConflictAndStorageFailureDistinct)$']
(target/'overlay.json').write_text(json.dumps(overlay)+'\n');cmd[2:2]=['-overlay',str(target/'overlay.json')]
(target/'command.json').write_text(json.dumps({'cwd':str(root/'control-plane'),'argv':cmd,'environment':{'GOMAXPROCS':'2','GOCACHE':str(cache)},'resourcePolicy':{'rationale':'Root-authorized test-only cache relocation following ENOSPC; product runtime admission unchanged','rootFloorMiB':512,'cgroupRemainingFloorGiB':4,'cacheLimitGiB':2,'initial':initial},'scope':'Product package restored to committed 544ae842 via exact overlays, same new regression, '+phase+' LOCAL026 mapping; dependency source/modules current, not full clean checkout'},indent=2)+'\n')
start=time.time()
with (target/'stdout.jsonl').open('wb') as stdout,(target/'stderr.log').open('wb') as stderr:
 child=subprocess.Popen(cmd,cwd=root/'control-plane',env={**os.environ,'GOMAXPROCS':'2','GOCACHE':str(cache)},stdout=stdout,stderr=stderr,start_new_session=True)
 samples=[];stopReason=None
 while child.poll() is None:
  sample=resources();sample['elapsedSeconds']=time.time()-start;samples.append(sample)
  if not guarded(sample) or sample['elapsedSeconds']>300:
   stopReason='resource guard' if not guarded(sample) else 'outer 300s deadline'
   os.killpg(child.pid,signal.SIGTERM)
   try:child.wait(timeout=5)
   except subprocess.TimeoutExpired:os.killpg(child.pid,signal.SIGKILL);child.wait()
   break
  time.sleep(1)
 code=child.wait()
(target/'resource-samples.json').write_text(json.dumps(samples,indent=2)+'\n')
end=time.time();after=manifest();(target/'source-after.json').write_text(json.dumps(after,separators=(',',':'))+'\n')
events=[]
for line in (target/'stdout.jsonl').read_text().splitlines():
 try:
  row=json.loads(line)
  if row.get('Test') and row.get('Action') in ('pass','fail','skip'):events.append({'name':row['Test'],'action':row['Action']})
 except ValueError: pass
summary={'phase':phase,'exitCode':code,'stopReason':stopReason,'seconds':end-start,'sourceStable':before==after,'repositoryGoModuleInputs':len(before),'outcomes':events,'counts':{status:len([e for e in events if e['action']==status]) for status in ['pass','fail','skip']}}
(target/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary));raise SystemExit(code)
