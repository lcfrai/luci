from pathlib import Path
import hashlib,json,os,signal,subprocess,sys,time
repo=Path('/workspace/luci');base=Path('/workspace/.luci-tools/cloud080-product-preview');task=base/'label-followup'
mode=sys.argv[1]
if mode not in ['before-label','fixed-label']:raise SystemExit('invalid run')
out=task/mode
if out.exists():raise SystemExit('refuse attempt reuse')
out.mkdir()
source=base/'validation/affected-001/overlay-files'
files={str(p.relative_to(source)):p for p in source.rglob('*.go')}
backend_source=Path('/workspace/.luci-tools/cloud080-docker/terminal-triage/fixed-preview/frozen')
for p in backend_source.rglob('*.go'):files[str(p.relative_to(backend_source))]=p
product='control-plane/internal/product/'
files[product+'releases.go']=task/('before' if mode=='before-label' else 'tree')/(product+'releases.go')
files[product+'preview_runtime_fixture_test.go']=task/'tree'/(product+'preview_runtime_fixture_test.go')
mapping={}
for rel,p in sorted(files.items()):
 q=out/'frozen'/rel;q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(p.read_bytes());mapping[str(repo/rel)]=str(q)
(out/'overlay.json').write_text(json.dumps({'Replace':mapping},indent=2)+'\n')
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def inputs():
 tracked=subprocess.check_output(['git','ls-files','-z','control-plane','sandboxd','go.work','go.work.sum'],cwd=repo).decode().split('\0')
 paths={f:repo/f for f in tracked if f and (f.endswith('.go') or f.endswith('go.mod') or f.endswith('go.sum') or f.endswith('go.work') or f.endswith('go.work.sum') or '/supabase_assets/' in f)}
 for rel in files:paths[rel]=out/'frozen'/rel
 for name in ['go.work','go.work.sum']:
  p=Path('/workspace/.luci-tools')/name
  if p.exists():paths['external-tools/'+name]=p
 return {'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'trackedDiffSha256':hashlib.sha256(subprocess.check_output(['git','diff','--binary','HEAD'],cwd=repo)).hexdigest(),'files':[{'path':name,'sha256':sha(p),'bytes':p.stat().st_size} for name,p in sorted(paths.items())]}
before=inputs();(out/'inputs-before.json').write_text(json.dumps(before,indent=2)+'\n')
env=os.environ.copy();env.update(GOMAXPROCS='2',GOTOOLCHAIN='local',GOPROXY='off',GOTMPDIR='/workspace/.t')
free={p:os.statvfs(p).f_bavail*os.statvfs(p).f_frsize for p in ['/tmp','/workspace']}
if free['/workspace']<(1<<30):raise SystemExit('insufficient compilation scratch')
version=subprocess.check_output(['go','version'],env=env,text=True).strip()
if 'go1.26.3 ' not in version:raise SystemExit(version)
pattern='^TestLocalReleaseBuildsCommittedSafeContextThenVerifiesHealthAndOwnership$'
cmd=['go','test','-json','-mod=readonly','-overlay='+str(out/'overlay.json'),'-count=1','-p=1','-timeout=120s','-run',pattern,'./internal/product']
(out/'invocation.json').write_text(json.dumps({'argv':cmd,'cwd':str(repo/'control-plane'),'version':version,'environment':{k:env[k] for k in ['GOMAXPROCS','GOTOOLCHAIN','GOPROXY','GOTMPDIR','GOCACHE','GOMODCACHE','GOWORK','GOFLAGS'] if k in env},'freeBytesBefore':free,'wallTimeoutSeconds':180,'startedUTC':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime())},indent=2)+'\n')
start=time.monotonic();timedout=False
with (out/'stdout.jsonl').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 proc=subprocess.Popen(cmd,cwd=repo/'control-plane',env=env,stdout=stdout,stderr=stderr,start_new_session=True)
 try:code=proc.wait(timeout=180)
 except subprocess.TimeoutExpired:
  timedout=True;os.killpg(proc.pid,signal.SIGTERM)
  try:code=proc.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);code=proc.wait()
after=inputs();(out/'inputs-after.json').write_text(json.dumps(after,indent=2)+'\n')
counts={};fails=[];packages=[]
for line in (out/'stdout.jsonl').read_text(errors='replace').splitlines():
 try:event=json.loads(line)
 except ValueError:continue
 if event.get('Action') not in ['pass','fail','skip']:continue
 if event.get('Test'):
  key=event['Action'];part='subtests' if '/' in event['Test'] else 'topLevel';v=counts.setdefault(key,{'topLevel':0,'subtests':0});v[part]+=1
  if key=='fail':fails.append(event['Test'])
 else:packages.append({k:event[k] for k in ['Package','Action','Elapsed'] if k in event})
result={'underlyingExitCode':code,'timedOut':timedout,'elapsedSeconds':time.monotonic()-start,'inputsUnchanged':before==after,'inputCount':len(before['files']),'counts':counts,'failedTests':fails,'packages':packages,'logSha256':{p.name:sha(p) for p in [out/'stdout.jsonl',out/'stderr.log']}}
(out/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
