#!/usr/bin/env python3
import argparse,datetime,hashlib,json,os,pathlib,subprocess,time
p=argparse.ArgumentParser();p.add_argument('kind',choices=['go','frontend']);a=p.parse_args()
repo=pathlib.Path('/workspace/luci');out=pathlib.Path('/workspace/.luci-tools/cloud054-combined-validation')/a.kind
out.mkdir(parents=True,exist_ok=False)
def stamp():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def binding():
    roots=['control-plane','shared','desktop/src','desktop/e2e','desktop/scripts','desktop/package.json','desktop/package-lock.json','desktop/tsconfig.json','desktop/vitest.config.ts','desktop/vite.config.ts']
    result={}
    for name in roots:
        root=repo/name
        paths=root.rglob('*') if root.is_dir() else [root]
        for f in paths:
            if not f.is_file() or f.is_symlink() or any(x in f.parts for x in ['node_modules','target','.git','dist','artifacts']):continue
            b=f.read_bytes();result[str(f.relative_to(repo))]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
    for ext in [pathlib.Path('/workspace/.luci-tools/go.work')]:
        b=ext.read_bytes();result[str(ext)]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
    return dict(sorted(result.items()))
def save(n,v):(out/n).write_text(json.dumps(v,indent=2)+'\n')
before=binding();save('inputs-before.json',before)
commands=[(['go','test','-race','-p','1','-count=1','-timeout=5m','-json','./internal/proxy','./internal/api','./internal/management','-run','^(TestRelay|TestAttach|TestADroppedSocket|TestNewActuallyRoutesTheAttachRoute|TestIsWebSocketUpgrade|TestSessionExecutionGeneration|TestTeamOpaqueExecutionGeneration|TestRestartKeepsTheIDAndMintsANewSandboxAndANewToken|TestSessionJSON)'],repo/'control-plane')] if a.kind=='go' else [(['npm','test','--','--maxWorkers=2','--reporter=json','--outputFile='+str(out/'vitest.json')],repo/'desktop'),(['npm','run','typecheck'],repo/'desktop')]
env=dict(os.environ,GOMAXPROCS='2');rows=[]
for i,(cmd,cwd) in enumerate(commands):
    row={'command':cmd,'cwd':str(cwd),'startedAt':stamp()};save('running.json',row);start=time.monotonic()
    with (out/f'{i}.stdout.log').open('w') as stdout,(out/f'{i}.stderr.log').open('w') as stderr:
        try:r=subprocess.run(cmd,cwd=cwd,env=env,stdout=stdout,stderr=stderr,timeout=600);row['exitCode']=r.returncode
        except subprocess.TimeoutExpired:row['exitCode']=None;row['timeout']=True
    row.update(finishedAt=stamp(),elapsedSeconds=time.monotonic()-start);rows.append(row);save('commands.json',rows)
after=binding();save('inputs-after.json',after)
result={'kind':a.kind,'commands':rows,'inputsCount':len(before),'inputsStable':before==after,'scope':'Integrated source validation; not installed/Windows/provider/performance acceptance.'}
if a.kind=='go':
    outcomes=[]
    for line in (out/'0.stdout.log').read_text().splitlines():
        try:e=json.loads(line)
        except ValueError:continue
        if e.get('Test') and e.get('Action') in ['pass','fail','skip']:outcomes.append(e)
    result['outcomes']={k:sum(x['Action']==k for x in outcomes) for k in ['pass','fail','skip']};result['topLevel']=sum('/' not in x['Test'] for x in outcomes)
else:
    if (out/'vitest.json').exists():
        v=json.loads((out/'vitest.json').read_text());result['vitest']={k:v.get(k) for k in ['numTotalTests','numPassedTests','numFailedTests','numPendingTests','numTotalTestSuites','success']};result['configuredFiles']=len(v.get('testResults',[]))
save('result.json',result);print(json.dumps(result),flush=True)
raise SystemExit(0 if before==after and all(x.get('exitCode')==0 for x in rows) else 1)
