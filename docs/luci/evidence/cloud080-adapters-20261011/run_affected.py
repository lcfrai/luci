from pathlib import Path
import hashlib,json,os,signal,subprocess,time
repo=Path('/workspace/luci'); base=Path('/workspace/.luci-tools/cloud080-product-preview'); dest=base/'validation'/'affected-001'
if dest.exists():raise SystemExit('refuse repeated attempt path')
dest.mkdir(parents=True)
sources=[Path('/workspace/.luci-tools/cloud080-docker/tree'),Path('/workspace/.luci-tools/cloud080-preview-admission/frozen-c080'),base/'tree']
merged={}
for source in sources:
 for p in source.rglob('*.go'):merged[str(p.relative_to(source))]=p
for name in ['execution_capacity.go','execution_capacity_typed_test.go']:
 merged['control-plane/internal/api/'+name]=Path('/workspace/.luci-tools/cloud081-runtime-install-20261011/tree/control-plane/internal/api')/name
# API owner merges product/platform option wire and deployment authority wire.
main=Path('/workspace/.luci-tools/cloud080-preview-admission/frozen-c080/control-plane/cmd/api/main.go')
if main.exists():merged['control-plane/cmd/api/main.go']=main
frozen=dest/'overlay-files'; mapping={}
for rel,p in sorted(merged.items()):
 q=frozen/rel;q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(p.read_bytes());mapping[str(repo/rel)]=str(q)
(dest/'overlay.json').write_text(json.dumps({'Replace':mapping},indent=2)+'\n')
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def inputs():
 tracked=subprocess.check_output(['git','ls-files','-z','control-plane'],cwd=repo).decode().split('\0')
 files={f:repo/f for f in tracked if f and (f.endswith('.go') or f.endswith('/go.mod') or f.endswith('/go.sum') or '/supabase_assets/' in f)}
 for rel in merged:files[rel]=frozen/rel
 return {'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'dirtyTrackedDiffSha256':hashlib.sha256(subprocess.check_output(['git','diff','--binary','HEAD'],cwd=repo)).hexdigest(),'files':[{'path':f,'sha256':sha(p),'bytes':p.stat().st_size} for f,p in sorted(files.items())]}
before=inputs();(dest/'inputs-before.json').write_text(json.dumps(before,indent=2)+'\n')
regex='Test(LocalRelease|LocalPreview|PreviewReceipt|PreviewHealth|PreviewArchive|PreviewConfiguration|StopRelease|FailedReleaseCleanup|ReleaseCleanup|PreviewProxy|ReleaseBrowser|PreviewBridge|PublicPreview|PrivateApproval(AllRelease|Authority)|AdmittedDeployment|RetainedCrashed|BrowserDeployment|DeploymentPersists|ForeignContainer|InvalidSources|PlatformCapabilities|ArchiveUses|CloudBrowserDataOrigin|NativeProjectRun|ApplicationPrototype|DatabaseRestoredSelection|DatabaseRestartBinding|DatabaseRestoreAdmission)'
cmd=['go','test','-json','-mod=readonly','-overlay='+str(dest/'overlay.json'),'-count=1','-p=1','-timeout=180s','-run',regex,'./internal/product','./internal/platform']
env=os.environ.copy();env.update(GOMAXPROCS='2',GOTOOLCHAIN='local',GOPROXY='off',GOTMPDIR='/workspace/.t')
freeBytes={p:os.statvfs(p).f_bavail*os.statvfs(p).f_frsize for p in ['/workspace','/workspace/.t','/tmp']}
if freeBytes['/workspace'] < (1<<30):raise SystemExit('insufficient workspace compile storage '+str(freeBytes))
version=subprocess.check_output(['go','version'],env=env,text=True).strip()
if 'go1.26.3 ' not in version:raise SystemExit('wrong Go version '+version)
invocation={'cwd':str(repo/'control-plane'),'argv':cmd,'toolchain':version,'environment':{k:env[k] for k in ['GOMAXPROCS','GOTOOLCHAIN','GOPROXY','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR'] if k in env},'freeBytesBefore':freeBytes,'wallTimeoutSeconds':240,'frozenOverlay':[{'path':str(Path(k).relative_to(repo)),'sha256':sha(Path(v))} for k,v in sorted(mapping.items())]}
(dest/'invocation.json').write_text(json.dumps(invocation,indent=2)+'\n')
start=time.monotonic();timeout=False
with (dest/'stdout.jsonl').open('wb') as stdout,(dest/'stderr.log').open('wb') as stderr:
 proc=subprocess.Popen(cmd,cwd=repo/'control-plane',env=env,stdout=stdout,stderr=stderr,start_new_session=True)
 try:code=proc.wait(timeout=240)
 except subprocess.TimeoutExpired:
  timeout=True;os.killpg(proc.pid,signal.SIGTERM)
  try:code=proc.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);code=proc.wait()
after=inputs();(dest/'inputs-after.json').write_text(json.dumps(after,indent=2)+'\n')
counts={};fails=[];packages=[]
for line in (dest/'stdout.jsonl').read_text(errors='replace').splitlines():
 try:v=json.loads(line)
 except ValueError:continue
 if v.get('Action') in ['pass','fail','skip']:
  if v.get('Test'):
   k=v['Action'];counts[k]=counts.get(k,0)+1
   if k=='fail':fails.append({'package':v.get('Package'),'test':v['Test']})
  else:packages.append({k:v[k] for k in ['Package','Action','Elapsed'] if k in v})
result={'underlyingExitCode':code,'wallTimeout':timeout,'elapsedSeconds':time.monotonic()-start,'inputBindingUnchanged':before['files']==after['files'],'repositoryMetadataUnchanged':before['head']==after['head'] and before['dirtyTrackedDiffSha256']==after['dirtyTrackedDiffSha256'],'countsIncludingSubtests':counts,'failedTests':fails,'packages':packages,'logSha256':{p.name:sha(p) for p in [dest/'stdout.jsonl',dest/'stderr.log']}}
(dest/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
