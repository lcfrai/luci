import hashlib,json,os,pathlib,shutil,signal,subprocess,sys,time

stage=pathlib.Path(__file__).parent
root=pathlib.Path('/workspace/luci')
out=stage/sys.argv[1]
out.mkdir()
env=os.environ.copy()
env.update(GOMAXPROCS='2',GOMODCACHE='/tmp/luci-retained-tools/go-mod',GOPATH='/tmp/luci-gopath',GOCACHE='/tmp/luci-go-build-cache',GOTMPDIR='/workspace/.t',TMPDIR='/workspace/.t',GOTOOLCHAIN='local',GOWORK='/workspace/.luci-tools/go.work',GOFLAGS='-mod=readonly')
env['PATH']='/workspace/.luci-tools/bin:/workspace/.luci-tools/go1.26.3/bin:'+env['PATH']
go='/workspace/.luci-tools/go1.26.3/bin/go'
command=[go,'test','-race','-p','1','-count=1','-timeout=90s','-json','-run','^TestPreviewHTTPCurrentCancellationReachesAPILockAndRetainsUnknownAcrossReopen$','./internal/api']
def digest(p):return hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
def git(*args):return subprocess.check_output(['git',*args],cwd=root,text=True)
def sources(phase):
    files={}
    for rel in set(git('ls-files','-c','-o','--exclude-standard','-z').split('\0')):
        p=root/rel
        if not rel or not rel.startswith(('control-plane/','sandboxd/')):continue
        if p.suffix not in ('.go','.s','.S','.c','.h') and p.name not in ('go.mod','go.sum') and not rel.startswith('control-plane/internal/platform/supabase_assets/'):continue
        files[rel]=digest(p)
    record={'scope':'Conservative repository Go/native/module/embed inventory from both active modules, including untracked new files; external dependencies bound by module manifests, not individual cache files. This inventory includes files outside the selected compiled dependency closure.','HEAD':git('rev-parse','HEAD').strip(),'status':git('status','--porcelain'),'recordedAt':time.time(),'goWorkSHA256':digest(env['GOWORK']),'goToolchainSHA256':digest(go),'files':dict(sorted(files.items()))}
    (out/(phase+'-sources.json')).write_text(json.dumps(record,indent=2)+'\n')
    return record
def free():return {'root':shutil.disk_usage('/workspace').free,'tmp':shutil.disk_usage('/tmp').free}
def below(v):return v['root']<int(1.25*(1<<30)) or v['tmp']<256*(1<<20)
initial=free()
if below(initial):raise SystemExit('storage floor refuses compilation')
before=sources('before')
selected=['api/api.go','api/preview_execution.go','api/execution_preview_leases.go','api/preview_execution_test.go','api/preview_cancellation_http_test.go','api/sessions_test.go','api/implementation_context_test.go','product/preview_cancel.go','product/preview_runtime.go','product/releases.go','product/browser_preview.go','product/connections.go','product/server.go','product/releases_observation.go']
for rel in selected:
    src=root/'control-plane/internal'/rel
    dst=out/'selected-source'/'control-plane/internal'/rel
    dst.parent.mkdir(parents=True,exist_ok=True)
    shutil.copyfile(src,dst)
receipt={'argv':command,'cwd':str(root/'control-plane'),'startedAt':time.time(),'environment':{k:env[k] for k in ('GOMAXPROCS','GOMODCACHE','GOPATH','GOCACHE','GOTMPDIR','TMPDIR','GOTOOLCHAIN','GOWORK','GOFLAGS')},'freeBefore':initial,'runnerSHA256':digest(__file__)}
(out/'command.json').write_text(json.dumps(receipt,indent=2)+'\n')
minimum=initial.copy();stopped=None;binaries={}
def binary(parent):
    candidates=[parent]
    for pid in candidates:
        try:
            for thread in pathlib.Path(f'/proc/{pid}/task').iterdir():
                for child in thread.joinpath('children').read_text().split():
                    if int(child) not in candidates:candidates.append(int(child))
            path=os.readlink(f'/proc/{pid}/exe')
            if path.endswith('.test') and path not in binaries:
                binaries[path]={'sha256':digest(f'/proc/{pid}/exe'),'observedAt':time.time(),'pid':pid}
        except (OSError,ValueError):pass
with (out/'events.jsonl').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
    p=subprocess.Popen(command,cwd=receipt['cwd'],env=env,stdout=stdout,stderr=stderr,start_new_session=True)
    while p.poll() is None:
        binary(p.pid)
        observed=free()
        for k,v in observed.items():minimum[k]=min(minimum[k],v)
        if below(observed) or time.time()-receipt['startedAt']>300:
            stopped='storage-floor' if below(observed) else 'outer-timeout'
            os.killpg(p.pid,signal.SIGTERM)
            try:p.wait(timeout=10)
            except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL)
            break
        time.sleep(.5)
    code=p.wait()
after=sources('after')
counts={};failed=[]
for line in (out/'events.jsonl').read_text().splitlines():
    try:item=json.loads(line)
    except ValueError:continue
    if item.get('Test') and item.get('Action') in ('pass','fail','skip'):
        action=item['Action'];counts[action]=counts.get(action,0)+1
        if action=='fail':failed.append(item['Test'])
receipt.update(exitCode=code,endedAt=time.time(),minimumFreeBytes=minimum,stopped=stopped,testActions=counts,failedTests=failed,repositoryInputCount=len(before['files']),repositoryInputsStable=before['files']==after['files'],goWorkStable=before['goWorkSHA256']==after['goWorkSHA256'],eventsSHA256=digest(out/'events.jsonl'),stderrSHA256=digest(out/'stderr.log'),observedExecutedBinaries=binaries)
(out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps(receipt))
sys.exit(code or (0 if receipt['repositoryInputsStable'] else 2))
