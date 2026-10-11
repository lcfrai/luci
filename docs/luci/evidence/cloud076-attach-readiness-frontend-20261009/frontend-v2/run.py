"""Capture one explicit scoped frontend test run; no product/runtime execution."""
from pathlib import Path
import argparse, datetime, hashlib, json, os, subprocess, time
p = argparse.ArgumentParser(); p.add_argument('variant', choices=['before', 'fixed', 'pre-unmount']); p.add_argument('scope', choices=['regression', 'affected']); p.add_argument('--output', required=True, type=Path); args = p.parse_args()
stage = Path(__file__).resolve().parent; repo = Path('/workspace/luci'); desktop = repo / 'desktop'
out = args.output.resolve(); out.mkdir(mode=0o700)
def sha(path): return hashlib.sha256(path.read_bytes()).hexdigest()
def snapshot():
    paths = [p for p in (desktop/'src').rglob('*') if p.is_file() and p.suffix in ['.ts', '.tsx', '.css', '.json']]
    paths += [desktop/name for name in ['package.json','package-lock.json','vitest.config.ts','tsconfig.json','tsconfig.node.json']]
    paths += [p for root in [stage/'before',stage/'fixed',stage/'pre-unmount',stage/'tests'] for p in root.rglob('*') if p.is_file()]
    paths += [stage/'run.py',stage/'vitest.config.mjs']
    return {str(p):sha(p) for p in sorted(set(paths))}
before = snapshot(); (out/'before.json').write_text(json.dumps(before,indent=2)+'\n')
node = '/opt/codex/runtimes/codex-primary-runtime/dependencies/node/bin/node'
cmd = [node,str(desktop/'node_modules/vitest/vitest.mjs'),'run','--config',str(stage/'vitest.config.mjs'),'--reporter=json','--outputFile='+str(out/'test-results.json')]
if args.scope == 'regression': cmd.append(str(stage/'tests/desktop/src/api/attachRegistry.readiness.test.ts'))
env = dict(os.environ); env['C076_VARIANT'] = args.variant
start = time.monotonic(); metadata = {'variant':args.variant,'scope':args.scope,'command':cmd,'cwd':str(desktop),'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'baseFunctionalCommit':'786652704fa8bebd1ba1af41cfa1d414d7def100','environmentOverrides':{'C076_VARIANT':args.variant},'scopeLimit':'Single-worker Vitest/jsdom controlled checks; no installed app, browser, Docker, provider or compiler run.'}
(out/'command.json').write_text(json.dumps(metadata,indent=2)+'\n')
with (out/'stdout.log').open('w') as stdout, (out/'stderr.log').open('w') as stderr:
    result = subprocess.run(cmd,cwd=desktop,env=env,stdout=stdout,stderr=stderr,timeout=180)
after = snapshot(); (out/'after.json').write_text(json.dumps(after,indent=2)+'\n')
summary = {'exitCode':result.returncode,'elapsedSeconds':time.monotonic()-start,'inputsUnchanged':before==after,'inputCount':len(before),'stdoutSHA256':sha(out/'stdout.log'),'stderrSHA256':sha(out/'stderr.log')}
if (out/'test-results.json').exists():
    tests = json.loads((out/'test-results.json').read_text()); summary['counts'] = {k:tests.get(k) for k in ['numPassedTests','numFailedTests','numPendingTests','numTotalTests']}
(out/'result.json').write_text(json.dumps(summary,indent=2)+'\n'); print(json.dumps(summary),flush=True)
raise SystemExit(result.returncode if before==after else 2)
