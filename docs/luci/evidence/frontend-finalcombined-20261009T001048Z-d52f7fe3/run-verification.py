#!/usr/bin/env python3
from pathlib import Path
import datetime,hashlib,json,subprocess,time
ROOT=Path('/workspace/luci');DESKTOP=ROOT/'desktop';BASE=Path('/workspace/.luci-tools/frontend-finalcombined-d52');STAMP=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ');OUT=BASE/STAMP;OUT.mkdir()
sha=lambda data:hashlib.sha256(data).hexdigest()
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def write(name,j):(OUT/name).write_text(json.dumps(j,indent=2)+'\n')
def snapshot(name):
    paths=set()
    for pattern in ['desktop/src/**/*','desktop/e2e/**/*','desktop/scripts/**/*','desktop/*.json','desktop/*config.*','control-plane/**/*.go','docs/superpowers/plans/phase-2-parts/task-*.md']:
        for p in ROOT.glob(pattern):
            if p.is_file()and 'control-plane/artifacts/'not in str(p.relative_to(ROOT)):paths.add(p)
    for value in ['AGENTS.md','docs/SEAMS.md','docs/verification/2026-09-27-phase-2-results.md','docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md','docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md','images/sandbox/Dockerfile','scripts/native-installed-eval.mjs','control-plane/internal/product/testdata/project-check-http.json','control-plane/internal/management/testdata/execution-storage-http.json','control-plane/internal/product/testdata/task-handoff-http.json']:
        paths.add(ROOT/value)
    files={str(p.relative_to(ROOT)):{'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())}for p in sorted(paths)}
    result={'at':now(),'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'statusPorcelain':subprocess.check_output(['git','status','--porcelain=v1'],cwd=ROOT,text=True).splitlines(),'scope':'Desktop source, test, scripts, configs and lockfile; exact outside files/globs imported by frontend raw-source contract tests. Generated control-plane artifacts excluded like seams.test.ts. Dependencies separately bound by lock and installed tool versions.','files':files};write(name,result);return result
commands=[['npm','test','--','--maxWorkers=2','--reporter=default','--reporter=json','--outputFile.json='+str(OUT/'vitest.json')],['npm','run','typecheck']]
versions={name:subprocess.check_output(command,cwd=DESKTOP,text=True).strip()for name,command in [('node',['node','--version']),('npm',['npm','--version'])]}
for name in ['vitest','typescript','vite','react','react-dom','jsdom']:
    versions[name]=json.loads((DESKTOP/'node_modules'/name/'package.json').read_text())['version']
write('invocation.json',{'createdAt':now(),'cwd':str(DESKTOP),'commands':commands,'toolchain':versions,'activation':'source /workspace/.luci-tools/activate.sh','maxWorkers':2,'scope':'One full Vitest frontend suite and one TypeScript check; no browser/performance/runtime run.'})
print(json.dumps({'output':str(OUT),'startedAt':now(),'commands':commands}),flush=True)
results=[]
for index,command in enumerate(commands):
    label='frontend'if index==0 else'typecheck';before=snapshot(label+'-source-before.json');start=now();monotonic=time.monotonic()
    with (OUT/(label+'.stdout')).open('wb')as stdout,(OUT/(label+'.stderr')).open('wb')as stderr:
        proc=subprocess.run(command,cwd=DESKTOP,stdout=stdout,stderr=stderr,check=False)
    after=snapshot(label+'-source-after.json');changed=[p for p in sorted(set(before['files'])|set(after['files']))if before['files'].get(p)!=after['files'].get(p)]
    result={'label':label,'command':command,'cwd':str(DESKTOP),'startedAt':start,'finishedAt':now(),'elapsedSeconds':time.monotonic()-monotonic,'exitCode':proc.returncode,'sourceChangedPaths':changed,'frontendInputsUnchanged':not any(p.startswith('desktop/')for p in changed),'allCapturedInputsUnchanged':not changed,'stdout':{'file':label+'.stdout','bytes':(OUT/(label+'.stdout')).stat().st_size,'sha256':sha((OUT/(label+'.stdout')).read_bytes())},'stderr':{'file':label+'.stderr','bytes':(OUT/(label+'.stderr')).stat().st_size,'sha256':sha((OUT/(label+'.stderr')).read_bytes())}};results.append(result);write('commands.json',results);print(json.dumps(result),flush=True)
write('completed.json',{'finishedAt':now(),'commandsCompleted':len(results),'exitCodes':[r['exitCode']for r in results],'browserOrPerformanceRun':False})
print(json.dumps({'completed':str(OUT)}),flush=True)
