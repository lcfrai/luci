import pathlib, os, json, hashlib, shutil, subprocess, datetime, sys
root=pathlib.Path('/workspace/luci'); out=root/'artifacts/microvm-capture-contract'
cache=pathlib.Path('/dev/shm/luci-go-cache-20261011-recovery')
free=shutil.disk_usage(root).free
maximum=pathlib.Path('/sys/fs/cgroup/memory.max').read_text().strip(); current=int(pathlib.Path('/sys/fs/cgroup/memory.current').read_text())
remaining=(int(maximum)-current) if maximum!='max' else None
cache_bytes=sum(p.stat().st_size for p in cache.rglob('*') if p.is_file())
guard={'rootFreeBytes':free,'cgroupRemainingBytes':remaining,'cacheBytes':cache_bytes,'minimumRootFreeBytes':512<<20,'minimumCgroupRemainingBytes':4<<30,'maximumCacheBytes':2<<30}
(out/'guard.json').write_text(json.dumps(guard,indent=2)+'\n')
if free<512<<20 or remaining is None or remaining<4<<30 or cache_bytes>2<<30:
 print(json.dumps({'guard':'refused','values':guard}));sys.exit(2)
def sources():
 paths=set()
 for base in ['control-plane/internal/backend/e2bbe','control-plane/internal/microvmguest','control-plane/cmd/microvm-bootstrap','control-plane/cmd/microvm-collect','control-plane/internal/statelease']:
  d=root/base
  if d.exists(): paths.update(d.rglob('*.go'))
 paths.update((root/'control-plane/internal/backend').glob('*.go'))
 paths.update(root/p for p in ['go.work','go.work.sum','control-plane/go.mod','control-plane/go.sum'] if (root/p).exists())
 return {str(p.relative_to(root)):{'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'bytes':p.stat().st_size} for p in sorted(paths)}
before=sources();(out/'source-before.json').write_text(json.dumps(before,indent=2)+'\n')
cmd=['go','test','-race','-json','-count=1','-p','1','-timeout','2m','./internal/backend/e2bbe','./internal/microvmguest','./cmd/microvm-bootstrap']
if (root/'control-plane/cmd/microvm-collect').is_dir():cmd.append('./cmd/microvm-collect')
env=dict(os.environ,GOMAXPROCS='2',GOCACHE=str(cache)); start=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (out/'fixed.jsonl').open('wb') as stdout,(out/'fixed.stderr.log').open('wb') as stderr:
 result=subprocess.run(cmd,cwd=root/'control-plane',env=env,stdout=stdout,stderr=stderr,timeout=240)
after=sources();(out/'source-after.json').write_text(json.dumps(after,indent=2)+'\n')
counts={}
for line in (out/'fixed.jsonl').read_text().splitlines():
 try:e=json.loads(line)
 except json.JSONDecodeError:continue
 if e.get('Action') in ['pass','fail','skip'] and e.get('Test'):
  c=counts.setdefault(e['Package'],{}); name=('sub-' if '/' in e['Test'] else 'top-')+e['Action'];c[name]=c.get(name,0)+1
report={'startedAt':start,'finishedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'command':cmd,'exitCode':result.returncode,'selectedSourceStable':before==after,'selectedInputCount':len(before),'changedPaths':[p for p in before.keys()|after.keys() if before.get(p)!=after.get(p)],'counts':counts,'scope':'Selected repository source bindings, controlled TLS/Connect and guest filesystem/kernel fixtures. Not full compiler/dependency provenance or actual KVM/cloud/template/provider acceptance.'}
(out/'fixed-result.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report));sys.exit(result.returncode if before==after else 2)
