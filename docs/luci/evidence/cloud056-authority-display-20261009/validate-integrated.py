#!/usr/bin/env python3
"""Root-invoked bounded validation after integrating C056. Does not edit repository files."""
import argparse,hashlib,json,os,pathlib,signal,subprocess,time
p=argparse.ArgumentParser();p.add_argument('--repo',default='/workspace/luci');p.add_argument('--output',required=True);p.add_argument('--go-overlay',action='store_true');args=p.parse_args()
repo=pathlib.Path(args.repo).resolve();desktop=repo/'desktop';stage=pathlib.Path(__file__).resolve().parent;out=pathlib.Path(args.output).resolve()
if out.exists():raise SystemExit('Refusing an existing evidence destination')
out.mkdir(parents=True)
sha=lambda b:hashlib.sha256(b).hexdigest()
source_paths=[x for x in (desktop/'src').rglob('*') if x.is_file() and x.suffix in ('.ts','.tsx','.css','.json','.svg')]
source_paths += [x for x in desktop.iterdir() if x.is_file() and (x.name in ('package.json','package-lock.json','index.html') or x.name.startswith(('tsconfig','vite.config','vitest.config')))]
source_paths=sorted(set(source_paths))
def snapshot():return [{'path':str(x.relative_to(repo)),'bytes':x.stat().st_size,'sha256':sha(x.read_bytes())} for x in source_paths]
def emit(name,value): (out/name).write_text(json.dumps(value,indent=2)+'\n')
before=snapshot();emit('repository-inputs-before.json',before)
for name in ['desktop/src/api/client.ts','desktop/src/api/teamScope.ts','desktop/src/components/teams/TeamWorkspaceProvider.tsx']:
 expected=stage/'after'/name
 if (repo/name).read_bytes()!=expected.read_bytes():raise SystemExit('Integrated source mismatch: '+name)
target='desktop/src/components/teams/TeamWorkspaceAuthority.test.tsx'
if (repo/target).read_bytes()!=(stage/'repo-ready-final'/target).read_bytes():raise SystemExit('Integrated regression mismatch')
files=['src/components/teams/TeamWorkspaceAuthority.test.tsx','src/components/teams/TeamWorkspaceProvider.test.tsx','src/api/client.test.ts','src/api/authTransport.test.ts','src/api/teamScope.test.ts','src/api/poller.test.ts','src/api/poller.generation.test.ts','src/api/attachRegistry.test.ts','src/api/attachRegistry.exit.test.ts','src/api/attachRegistry.overload-generation.test.ts','src/components/Terminal.test.tsx','src/components/develop/SessionPane.test.tsx','src/components/develop/SessionPermissions.test.tsx','src/components/auth/AuthProvider.test.tsx']
emit('selected-tests.json',files)
emit('tool-versions.json',{k:subprocess.check_output(cmd,text=True).strip() for k,cmd in {'node':['node','--version'],'npm':['npm','--version'],'go':['go','version']}.items()})
emit('git-before.json',{'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'status':subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True)})
lock=desktop/'node_modules/.package-lock.json'
emit('existing-install.json',{'path':'desktop/node_modules/.package-lock.json','sha256':sha(lock.read_bytes()) if lock.exists() else None,'scope':'Observed existing install lock; no npm ci/reinstall performed by this validation'})
def run(label,command,timeout,cwd=desktop,env=None):
 start=time.time();timed=False;forced=False
 with (out/(label+'.stdout.log')).open('wb') as a,(out/(label+'.stderr.log')).open('wb') as b:
  child=subprocess.Popen(command,cwd=cwd,stdout=a,stderr=b,start_new_session=True,env=env)
  try:code=child.wait(timeout=timeout)
  except subprocess.TimeoutExpired:
   timed=True;os.killpg(child.pid,signal.SIGTERM)
   try:code=child.wait(timeout=5)
   except subprocess.TimeoutExpired:forced=True;os.killpg(child.pid,signal.SIGKILL);code=child.wait(timeout=5)
 receipt={'command':command,'cwd':str(cwd),'exitCode':code,'timedOut':timed,'forced':forced,'elapsedSeconds':time.time()-start,'ownedRootPID':child.pid,'rootWaited':True,'processTreeClosure':'Process-group signals only on timeout; no whole-machine/process census claim'}
 emit(label+'.command.json',receipt);return receipt
results={}
results['frontend']=run('frontend',['node','node_modules/vitest/vitest.mjs','run',*files,'--maxWorkers=1','--reporter=json','--outputFile='+str(out/'frontend-report.json')],240)
results['typescript']=run('typescript',['node','node_modules/typescript/bin/tsc','--noEmit','--pretty','false','--listFiles'],120)
# Capture actual TypeScript listed inputs separately. Repository snapshot above
# fences all selected frontend sources pre/post. External declarations are a
# post-compiler observation, not a claim they were fetched or prehashed here.
ts=[]
for line in (out/'typescript.stdout.log').read_text(errors='replace').splitlines():
 candidate=pathlib.Path(line.strip())
 if candidate.is_absolute() and candidate.is_file():
  data=candidate.read_bytes();ts.append({'path':str(candidate),'bytes':len(data),'sha256':sha(data)})
emit('typescript-listed-inputs-after.json',ts)
if args.go_overlay:
 test=stage/'team_session_denial_contract_test.go';virt=repo/'control-plane/internal/management/cloud056_external_denial_contract_test.go';overlay=out/'go-overlay.json';emit('go-overlay.json',{'Replace':{str(virt):str(test)}})
 env=os.environ.copy();env['GOMAXPROCS']='2'
 results['go-control']=run('go-control',['go','test','-overlay='+str(overlay),'-p=1','-count=1','-timeout=90s','-run=^TestCloud056FreshMembershipRemovalReturnsAuthoritativeSessionListDenial$','-json','./internal/management'],100,repo/'control-plane',env)
after=snapshot();emit('repository-inputs-after.json',after)
summary={'sourceStable':before==after,'repositoryInputCount':len(before),'typescriptListedInputCount':len(ts),'results':results,'allCommandsExitZero':all(x['exitCode']==0 for x in results.values()),'scope':'Selected controlled frontend suites plus TypeScript; optional controlled registrar fixture, no installed/native/browser/Docker/provider claim'}
emit('result.json',summary);print(json.dumps(summary));raise SystemExit(0 if summary['sourceStable'] and summary['allCommandsExitZero'] else 1)
