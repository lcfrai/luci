from pathlib import Path
import os,json,hashlib,subprocess
p=Path('/home/agent/.cache/go-build');assert p.is_dir() and not p.is_symlink() and p.stat().st_uid==os.getuid()
assert p.joinpath('README').read_text().startswith('This directory holds cached build artifacts from the Go build system.')
active=[];zombies=[]
for x in Path('/proc').iterdir():
 if not x.name.isdigit():continue
 try:
  comm=x.joinpath('comm').read_text().strip()
  if comm not in ['go','compile','link','cgo'] or x.stat().st_uid!=os.getuid():continue
  raw=x.joinpath('stat').read_text();fields=raw[raw.rfind(') ')+2:].split();row={'pid':int(x.name),'comm':comm,'state':fields[0],'startTicks':fields[19]}
  (zombies if fields[0]=='Z' else active).append(row)
 except (OSError,ValueError):pass
assert not active,active
free=lambda:os.statvfs('/workspace').f_bavail*os.statvfs('/workspace').f_frsize
r={'scope':'Task-user Go standard build cache only; no source, module cache, package, evidence, state, fixture or product ledger mutation. No wholehost idle claim. Other agents coordinating read-only preparation only.','path':str(p),'beforeBytes':free(),'ownerUID':p.stat().st_uid,'readmeSHA256':hashlib.sha256(p.joinpath('README').read_bytes()).hexdigest(),'helperSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'command':['go','clean','-cache'],'status':'pending','knownZombieCompilerIdentities':zombies,'priorPreflight':'Names-only compiler scan refused before cache mutation; subsequent exact stat confirmed four zombies. No processes signalled.'}
out=Path('/workspace/.luci-tools/cloud054-default-go-cache-reclamation.json');assert not out.exists();out.write_text(json.dumps(r,indent=2)+'\n')
env=dict(os.environ,GOCACHE=str(p));s=subprocess.run(r['command'],env=env,capture_output=True,text=True,timeout=30);r.update(exitCode=s.returncode,stdout=s.stdout,stderr=s.stderr,afterBytes=free(),status='passed' if s.returncode==0 else 'failed');out.write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r))
raise SystemExit(s.returncode)
