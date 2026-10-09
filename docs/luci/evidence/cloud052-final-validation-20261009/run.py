from pathlib import Path
import collections, datetime, hashlib, json, os, shutil, subprocess, time

root = Path('/workspace/luci')
out = Path('/workspace/.luci-tools/cloud052-final-validation')
out.mkdir(exist_ok=False)
env = dict(os.environ, GOMAXPROCS='2')
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
def save(name, value):
    (out/name).write_text(json.dumps(value, indent=2)+'\n')

goenv = json.loads(subprocess.check_output(['go','env','-json','GOVERSION','GOOS','GOARCH','GOROOT','GOMODCACHE','GOWORK','CGO_ENABLED','GOFLAGS','GOTOOLCHAIN'],cwd=root,env=env))
save('toolchain.json', {**goenv,'goExecutableSHA256':sha(Path(shutil.which('go')))})
def snapshot():
    paths = set()
    for module in ('control-plane','sandboxd'):
        paths.update(p for p in (root/module).rglob('*') if p.is_file() and p.suffix in ('.go','.mod','.sum'))
    paths.update(p for p in root.glob('go.work*') if p.is_file())
    return {str(p.relative_to(root)): {'sha256':sha(p),'bytes':p.stat().st_size} for p in sorted(paths)}

# Hash effective compiler-selected files, embedded files and module manifests.
# External module/toolchain hashes are provenance, not independently fetched evidence.
raw = subprocess.check_output(['go','list','-deps','-test','-json','./...'], cwd=root/'control-plane',env=env)
decoder = json.JSONDecoder(); pos=0; objects=[]; listed=raw.decode()
while pos < len(listed):
    while pos < len(listed) and listed[pos].isspace(): pos += 1
    if pos >= len(listed): break
    value, pos = decoder.raw_decode(listed,pos); objects.append(value)
fields = ['GoFiles','CgoFiles','CFiles','CXXFiles','MFiles','HFiles','FFiles','SFiles','SwigFiles','SwigCXXFiles','SysoFiles','EmbedFiles','TestGoFiles','XTestGoFiles','TestEmbedFiles','XTestEmbedFiles']
inputs = set(); modules = set(); packages = []
for obj in objects:
    base=Path(obj['Dir']); packages.append(obj['ImportPath'])
    for field in fields:
        for name in obj.get(field,[]): inputs.add(base/name)
    module=obj.get('Module',{})
    if module.get('GoMod'): modules.add(Path(module['GoMod']))
    if module.get('Replace',{}).get('GoMod'): modules.add(Path(module['Replace']['GoMod']))
for path in modules:
    inputs.add(path)
    if path.with_suffix('.sum').is_file(): inputs.add(path.with_suffix('.sum'))
inputs.update(p for p in root.glob('go.work*') if p.is_file())
workspace=Path(goenv['GOWORK'])
if workspace.is_file():
    inputs.add(workspace)
    if Path(str(workspace)+'.sum').is_file(): inputs.add(Path(str(workspace)+'.sum'))
def label(p):
    for prefix,tag in ((root,'repository'),(Path(goenv['GOROOT']),'toolchain'),(Path(goenv['GOMODCACHE']),'module-cache')):
        if p.is_relative_to(prefix): return tag+'/'+str(p.relative_to(prefix))
    return 'external/'+str(p)
def compiled_snapshot():
    return {label(p):{'sha256':sha(p),'bytes':p.stat().st_size} for p in sorted(inputs)}

before=snapshot(); compiled_before=compiled_snapshot()
save('repository-before.json',before); save('compiled-before.json',compiled_before)
save('compiled-scope.json',{'command':['go','list','-deps','-test','-json','./...'],'cwd':'control-plane','packages':packages,'fileFields':fields,'scope':'Compiler-selected Linux source/embedded files and module/workspace manifests. Paths rewritten into repository/toolchain/module-cache namespaces; no claim to independent external-byte retrieval.'})
commands=[
 ['go','test','-race','-p','1','-count=1','-timeout=5m','-json','-run','^(TestProjectOverview|TestOverview|TestSwarmSchedulerDisplay|TestSwarmSchedulerCancelledResponse|TestSwarmSchedulerSharedHostPolicy|TestExecutionReservation)','./internal/product','./internal/api'],
 ['go','test','-p','1','-count=1','-timeout=5m','-json','-run','^$','./...'],
]
results=[]
for index,command in enumerate(commands):
    started=now(); begin=time.monotonic(); path=out/f'command-{index+1}.jsonl'
    with path.open('w') as log:
        result=subprocess.run(command,cwd=root/'control-plane',env=env,stdout=log,stderr=subprocess.STDOUT,timeout=420)
    counts={'top':collections.Counter(),'sub':collections.Counter(),'packages':collections.Counter()}
    for line in path.read_text().splitlines():
        try: event=json.loads(line)
        except ValueError: continue
        if event.get('Action') not in ('pass','fail','skip'): continue
        key='packages' if not event.get('Test') else ('sub' if '/' in event['Test'] else 'top')
        counts[key][event['Action']]+=1
    results.append({'command':command,'startedAt':started,'finishedAt':now(),'elapsedSeconds':time.monotonic()-begin,'exitCode':result.returncode,'counts':counts,'logSHA256':sha(path)})
    save('progress.json',results)
    if result.returncode: break
after=snapshot(); compiled_after=compiled_snapshot()
save('repository-after.json',after);save('compiled-after.json',compiled_after)
changes=lambda a,b:sorted(k for k in a.keys()|b.keys() if a.get(k)!=b.get(k))
report={'at':now(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'commands':results,'repositoryInputs':len(before),'compiledInputs':len(compiled_before),'repositoryChanged':changes(before,after),'compiledChanged':changes(compiled_before,compiled_after),'scope':'Affected Linux race checks plus all control-plane package compile/no-test run. Not full test-suite execution, Windows, installed, latency or live-provider acceptance.'}
report['passed']=len(results)==2 and all(r['exitCode']==0 for r in results) and not report['repositoryChanged'] and not report['compiledChanged']
save('report.json',report);print(json.dumps(report),flush=True)
raise SystemExit(0 if report['passed'] else 1)
