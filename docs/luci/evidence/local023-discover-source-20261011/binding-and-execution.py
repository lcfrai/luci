from pathlib import Path
import datetime, hashlib, json, os, re, signal, subprocess, time
R=Path('/workspace/luci');D=R/'desktop';B=Path(__file__).resolve().parent
sha=lambda b:hashlib.sha256(b).hexdigest()
ACTIVE=None
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()

def write(p,obj):
 raw=(json.dumps(obj,indent=2)+'\n').encode();tmp=p.with_suffix(p.suffix+'.tmp');tmp.write_bytes(raw);os.replace(tmp,p)

def git(*args):return subprocess.check_output(['git','-C',str(R),*args],timeout=20).decode().strip()

def filemeta(p):
 a=p.stat();raw=p.read_bytes();z=p.stat();assert (a.st_ino,a.st_size,a.st_mtime_ns)==(z.st_ino,z.st_size,z.st_mtime_ns)
 return {'bytes':len(raw),'sha256':sha(raw)}

def inputs():
 paths=set();reasons={}
 def add(p,why):
  p=Path(p);assert p.is_file() and not p.is_symlink(),str(p)
  assert p.resolve().is_relative_to(R),str(p)
  rel=p.relative_to(R).as_posix();assert '/artifacts/' not in rel
  paths.add(p);reasons.setdefault(rel,set()).add(why)
 for root in [D/'src',D/'e2e',D/'scripts']:
  for p in root.rglob('*'):
   if p.is_file():add(p,'desktop source/test/config support tree')
 for p in D.iterdir():
  if p.is_file() and p.suffix in ['.ts','.tsx','.json','.js','.mjs','.cjs','.html','.css']:add(p,'desktop root source/config/lock')
 for p in (R/'control-plane').rglob('*.go'):
  if 'artifacts' not in p.relative_to(R/'control-plane').parts:add(p,'seams.test.ts actual external Go glob')
 for rel in ['docs/SEAMS.md','docs/verification/2026-09-27-phase-2-results.md','docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md','docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md','images/sandbox/Dockerfile']:
  add(R/rel,'seams.test.ts exact external raw input')
 for p in (R/'docs/superpowers/plans/phase-2-parts').glob('task-*.md'):add(p,'seams.test.ts task plan glob')
 # Literal raw imports outside the already captured trees (goldens and native driver).
 raw_specs=[]
 for p in sorted(paths):
  if not p.is_relative_to(D) or p.suffix not in ['.ts','.tsx','.js','.mjs','.mts','.cts','.jsx','.cjs']:continue
  source=p.read_text()
  for spec in re.findall(r'''["']([^"'\n]+\?raw)["']''',source):
   if not spec.startswith(('.', '/')):continue
   target=(D/spec[1:].split('?')[0]) if spec.startswith('/') else (p.parent/spec.split('?')[0])
   target=Path(os.path.normpath(target));add(target,'literal raw import from '+p.relative_to(R).as_posix());raw_specs.append({'from':p.relative_to(R).as_posix(),'specifier':spec,'target':target.relative_to(R).as_posix()})
 # Vite env autoload is refused instead of reading potentially private contents.
 envnames=[str(p.relative_to(R)) for root in [R,D] for p in root.glob('.env*') if p.is_file()]
 assert not envnames,'Unexpected environment file names: '+repr(envnames)
 rows={p.relative_to(R).as_posix():{**filemeta(p),'reasons':sorted(reasons[p.relative_to(R).as_posix()])} for p in sorted(paths)}
 return {'files':rows,'count':len(rows),'rawImports':raw_specs,'envFiles':envnames,'scope':'All desktop src/e2e/scripts and root source/config/lock files, actual seams external Go/docs/task-plan/Dockerfile globs, literal raw imports. Includes dirty and untracked source. Excludes private artifacts and generated node_modules caches. Dependency identity is separately bounded, not a full transitive dependency byte census.'}

def deps(node):
 selected=[D/'node_modules/.package-lock.json',D/'node_modules/vitest/vitest.mjs',D/'node_modules/typescript/bin/tsc',D/'node_modules/typescript/lib/tsc.js',D/'node_modules/typescript/lib/_tsc.js']
 for name in ['vitest','vite','typescript','react','react-dom','jsdom','zustand','@vitejs/plugin-react','@testing-library/react']:
  selected.append(D/'node_modules'/name/'package.json')
 return {'node':{'path':node,'resolved':str(Path(node).resolve()),**filemeta(Path(node).resolve())},'selectedDependencies':{str(p.relative_to(D)):filemeta(p) for p in selected},'activation':{'path':'/workspace/.luci-tools/activate.sh',**filemeta(Path('/workspace/.luci-tools/activate.sh'))},'runner':filemeta(Path(__file__)),'scope':'Actual executable, installed lock and selected runner/compiler/package entries; no dependency installation or claim that every transitive installed file was independently hashed.'}

def execute(label,cmd,seconds,out,env):
 global ACTIVE
 row={'label':label,'command':cmd,'cwd':str(D),'timeoutSeconds':seconds,'startedAt':now(),'cleanupBoundSeconds':20,'maxLogBytesPerStream':32*1024*1024};start=time.monotonic()
 with (out/(label+'.stdout.log')).open('xb') as so,(out/(label+'.stderr.log')).open('xb') as se:
  proc=subprocess.Popen(cmd,cwd=D,env=env,stdout=so,stderr=se,start_new_session=True);ACTIVE=proc;row['pid']=proc.pid;row['processGroup']=proc.pid
  write(out/(label+'.invocation.json'),row);issue=None
  while proc.poll() is None:
   elapsed=time.monotonic()-start
   if elapsed>seconds:issue='timeout'
   if any((out/(label+'.'+n+'.log')).stat().st_size>row['maxLogBytesPerStream'] for n in ['stdout','stderr']):issue='log-bound'
   if issue:break
   time.sleep(.5)
  if issue:
   row['failure']=issue
   try:os.killpg(proc.pid,signal.SIGTERM)
   except ProcessLookupError:pass
   try:proc.wait(timeout=10)
   except subprocess.TimeoutExpired:
    try:os.killpg(proc.pid,signal.SIGKILL)
    except ProcessLookupError:pass
    proc.wait(timeout=10)
  else:proc.wait()
  row.update({'exitCode':proc.returncode,'elapsedSeconds':time.monotonic()-start,'completedAt':now()})
 row['logs']={n:filemeta(out/(label+'.'+n+'.log')) for n in ['stdout','stderr']};write(out/(label+'.result.json'),row);print(json.dumps({k:row[k] for k in ['label','exitCode','elapsedSeconds']}),flush=True);return row
