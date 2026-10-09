from pathlib import Path
import subprocess,json,time,datetime,hashlib,gzip,os
repo=Path('/workspace/luci');desktop=repo/'desktop';out=Path('/workspace/.luci-tools/cloud046047-combined-validation');out.mkdir(exist_ok=False)
sha=lambda b:hashlib.sha256(b).hexdigest()
def snapshot():
 files={f for f in (desktop/'src').rglob('*') if f.is_file()};files|={f for f in desktop.glob('*') if f.is_file() and f.suffix in ['.json','.ts','.mjs','.js','.html']}
 files|={f for f in (repo/'control-plane').rglob('*.go') if 'artifacts' not in f.relative_to(repo/'control-plane').parts}
 files|=set((desktop/'e2e').rglob('*.mjs'))
 files|={repo/x for x in ['scripts/native-installed-eval.mjs','control-plane/internal/product/testdata/project-check-http.json','control-plane/internal/management/testdata/execution-storage-http.json','control-plane/internal/product/testdata/task-handoff-http.json','docs/SEAMS.md','docs/verification/2026-09-27-phase-2-results.md','docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md','docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md','images/sandbox/Dockerfile']}
 files|=set((repo/'docs/superpowers/plans/phase-2-parts').glob('task-*.md'))
 dependencies={f for f in (desktop/'node_modules').rglob('*') if f.is_file() and '.vite' not in f.parts and '.vite-temp' not in f.parts}
 def record(f):
  row={'path':str(f.relative_to(repo)),'bytes':f.stat().st_size,'sha256':sha(f.read_bytes())}
  if f.is_symlink():row['symlinkTarget']=os.readlink(f)
  return row
 return {'repository':[record(f) for f in sorted(files)],'dependencies':[record(f) for f in sorted(dependencies)]}
before=snapshot()
for key,rows in before.items():
 data=(json.dumps(rows,indent=2)+'\n').encode();(out/(key+'-before.json.gz')).write_bytes(gzip.compress(data,mtime=0))
command=['node','node_modules/vitest/vitest.mjs','run','--maxWorkers=2','--reporter=json','--outputFile='+str(out/'full.json')]
start=time.monotonic();stamp=datetime.datetime.now(datetime.timezone.utc).isoformat()
timed_out=False
with (out/'full.stdout.log').open('wb') as stdout,(out/'full.stderr.log').open('wb') as stderr:
 try: result=subprocess.run(command,cwd=desktop,stdout=stdout,stderr=stderr,timeout=900)
 except subprocess.TimeoutExpired:
  timed_out=True
  result=subprocess.CompletedProcess(command,124)
(out/'timeout.json').write_text(json.dumps({'timedOut':timed_out,'budgetSeconds':900})+'\n')
(out/'command.json').write_text(json.dumps({'command':command,'startedAt':stamp,'elapsedSeconds':time.monotonic()-start,'exitCode':result.returncode,'nodeVersion':subprocess.check_output(['node','--version'],text=True).strip(),'repoHEADAtEnd':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()},indent=2)+'\n')
after=snapshot()
for key,rows in after.items():
 data=(json.dumps(rows,indent=2)+'\n').encode();(out/(key+'-after.json.gz')).write_bytes(gzip.compress(data,mtime=0))
summary={'exitCode':result.returncode,'inputCounts':{key:len(rows) for key,rows in before.items()},'stableInputs':before==after,'scope':'All desktop src/config, every configured external raw seam/contract/glob file and installed node_modules regular/symlink files, excluding generated .vite/.vite-temp cache directories; no product/performance runtime'}
if (out/'full.json').exists():
 report=json.loads((out/'full.json').read_text());summary['report']={key:report[key] for key in ['numTotalTests','numPassedTests','numFailedTests','numPendingTests','numTotalTestSuites','numPassedTestSuites','numFailedTestSuites']};summary['failures']=[{'file':f['name'],'title':t['title'],'failureMessages':t['failureMessages']} for f in report['testResults'] for t in f['assertionResults'] if t['status']=='failed']
(out/'result.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary),flush=True)

command=['npm','run','typecheck']
start=time.monotonic();stamp=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (out/'typecheck.stdout.log').open('wb') as stdout,(out/'typecheck.stderr.log').open('wb') as stderr:
 try: typed=subprocess.run(command,cwd=desktop,stdout=stdout,stderr=stderr,timeout=300)
 except subprocess.TimeoutExpired: typed=subprocess.CompletedProcess(command,124)
typed_after=snapshot()
for key,rows in typed_after.items():
 data=(json.dumps(rows,indent=2)+'\n').encode();(out/(key+'-after-typecheck.json.gz')).write_bytes(gzip.compress(data,mtime=0))
typed_summary={'command':command,'startedAt':stamp,'elapsedSeconds':time.monotonic()-start,'exitCode':typed.returncode,'stableInputs':after==typed_after,'scope':'Current combined candidate, same input scope as full frontend run'}
(out/'typecheck-command.json').write_text(json.dumps(typed_summary,indent=2)+'\n');print(json.dumps(typed_summary),flush=True)
raise SystemExit(0 if result.returncode==0 and typed.returncode==0 and before==after==typed_after else 1)
