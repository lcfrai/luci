import pathlib, json, hashlib, subprocess, time, sys, datetime
root=pathlib.Path('/workspace/luci')
base=pathlib.Path('/workspace/.luci-tools/cloud080-preview-cancel/management')
label=sys.argv[1]
receipt=base/label
receipt.mkdir()
inputs=sorted((root/'control-plane/internal/management').glob('*.go'))+[root/'control-plane/go.mod',root/'control-plane/go.sum',root/'sandboxd/go.mod',root/'sandboxd/go.sum',pathlib.Path('/workspace/.luci-tools/activate.sh'),pathlib.Path('/workspace/.luci-tools/go.work')]
def bindings():
 return [{'path':str(p.relative_to(root)) if p.is_relative_to(root) else str(p),'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest()} for p in inputs]
before=bindings()
(receipt/'source-before.json').write_text(json.dumps(before,indent=2)+'\n')
command=['bash','-c',"source /workspace/.luci-tools/activate.sh && GOMAXPROCS=2 go test -p 1 -count=1 -timeout=2m -json ./internal/management -run '^TestPreviewCancellation(TeamRouteCapabilities|SuspendedServiceRechecksCurrentGrants)$'"]
(receipt/'invocation.json').write_text(json.dumps({'command':command,'cwd':str(root/'control-plane'),'startedUTC':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceScope':'Selected management package source/tests, module declarations and activation; not complete transitive compile closure.'},indent=2)+'\n')
start=time.monotonic()
with (receipt/'stdout.jsonl').open('wb') as out,(receipt/'stderr.txt').open('wb') as err:
 result=subprocess.run(command,cwd=root/'control-plane',stdout=out,stderr=err,timeout=180)
after=bindings()
(receipt/'source-after.json').write_text(json.dumps(after,indent=2)+'\n')
events=[json.loads(line) for line in (receipt/'stdout.jsonl').read_text().splitlines() if line.startswith('{')]
terminal=[r for r in events if r.get('Test') and r.get('Action') in ['pass','fail','skip']]
summary={'exitCode':result.returncode,'elapsedSeconds':round(time.monotonic()-start,3),'selectedSourceInputs':len(before),'selectedSourceStable':before==after,'outcomes':{k:sum(r['Action']==k for r in terminal) for k in ['pass','fail','skip']},'topLevel':sum('/' not in r['Test'] for r in terminal),'subtests':sum('/' in r['Test'] for r in terminal),'failedTests':[r['Test'] for r in terminal if r['Action']=='fail']}
(receipt/'result.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary))
