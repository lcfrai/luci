#!/usr/bin/env python3
"""Root-invoked only: build a separate diagnostic web from frozen tracked frontend inputs."""
from pathlib import Path
import hashlib,json,os,signal,subprocess,time,shutil,sys
B=Path(__file__).resolve().parent;R=Path('/workspace/luci');PRODUCT='577f7a0fdb039b7f875901a6a567d62fdfee88c8'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def put(p,v):p.write_text(json.dumps(v,indent=2)+'\n');p.chmod(0o600)
def frozen_files(root):return [{'path':str(p.relative_to(root)),'bytes':p.stat().st_size,'sha256':sha(p)} for p in sorted(root.rglob('*')) if p.is_file() and not p.is_symlink()]
prepared=json.loads((B/'prepared-files.json').read_text())
for row in prepared['files']:
 p=B/row['path'];assert p.is_file() and p.stat().st_size==row['bytes'] and sha(p)==row['sha256'],row['path']+' prepared input changed'
meta=json.loads((B/'overlay-binding.json').read_text());source=B/'frontend';logs=B/'build';web=B/'web'
assert not source.exists() and not logs.exists() and not web.exists(),'Fresh build only; preserve failed attempts'
version=subprocess.check_output(['node','--version'],text=True).strip();assert version=='v24.19.0',version
assert os.statvfs('/').f_bavail*os.statvfs('/').f_frsize>=6*1024**3,'Unchanged six GiB admission floor'
source.mkdir(mode=0o700);logs.mkdir(mode=0o700)
# Freeze all tracked frontend files except the unrelated native crate. No current working bytes are used.
paths=subprocess.check_output(['git','-C',str(R),'ls-tree','-r','--name-only',PRODUCT,'desktop'],text=True).splitlines()
inputs=[]
for name in paths:
 rel=Path(name).relative_to('desktop')
 if rel.parts[0] in ('src-tauri','node_modules','dist','.env') or rel.name.startswith('.env'):continue
 raw=subprocess.check_output(['git','-C',str(R),'show',PRODUCT+':'+name])
 p=source/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(raw);inputs.append({'path':name,'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()})
for row in meta['files']:
 rel=Path(row['path']).relative_to('desktop');p=source/rel
 if row['originalSHA256']:assert sha(p)==row['originalSHA256'],'Overlay differs from packaged frontend'
 replacement=B/'overlay'/row['path'];assert sha(replacement)==row['overlaySHA256'];p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(replacement.read_bytes())
# Public generated design assets are bound to the unchanged installed package.
package=Path('/workspace/.luci-tools/package-577f7a0f/installed')
assets=package/'web/design-assets'
if assets.exists():
 target=source/'public/design-assets'
 if target.exists(): assert frozen_files(target)==frozen_files(assets),'Tracked and packaged design assets differ'
 else: shutil.copytree(assets,target)
(source/'node_modules').symlink_to(R/'desktop/node_modules',target_is_directory=True)
# Raw test import/glob declarations are type-only here; tests are not executed.
# Do not link mutable backend/docs/scripts into this frozen frontend.
dependency_paths=['.package-lock.json','typescript/package.json','typescript/lib/_tsc.js','vite/package.json','vite/dist/node/cli.js','react/package.json','react-dom/package.json','zustand/package.json','zustand/esm/middleware.mjs','@vitejs/plugin-react/package.json','@tailwindcss/vite/package.json','playwright/package.json','playwright/index.mjs']
def dependencies():return {name:sha(R/'desktop/node_modules'/name) for name in dependency_paths}
deps_before=dependencies();put(logs/'dependencies-before.json',deps_before)
before=frozen_files(source);put(logs/'inputs-before.json',{'baseCommit':PRODUCT,'trackedInputs':inputs,'overlay':meta,'frozenFrontend':before,'installedLockSHA256':sha(R/'desktop/node_modules/.package-lock.json')})
commands=[['node',str(R/'desktop/node_modules/typescript/bin/tsc'),'--noEmit'],['node',str(R/'desktop/node_modules/vite/bin/vite.js'),'build','--outDir',str(web),'--emptyOutDir']]
env=os.environ.copy();env['NODE_OPTIONS']='--max-old-space-size=3072';env['GOMAXPROCS']='2'
results=[]
for index,cmd in enumerate(commands):
 row={'argv':cmd,'cwd':str(source),'wallTimeoutSeconds':180,'startedUTC':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime())};put(logs/f'{index}-invocation.json',row)
 start=time.monotonic();timed=False
 with (logs/f'{index}.stdout.log').open('wb') as out,(logs/f'{index}.stderr.log').open('wb') as err:
  proc=subprocess.Popen(cmd,cwd=source,env=env,stdout=out,stderr=err,start_new_session=True)
  try:code=proc.wait(timeout=180)
  except subprocess.TimeoutExpired:
   timed=True;os.killpg(proc.pid,signal.SIGTERM)
   try:code=proc.wait(timeout=5)
   except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);code=proc.wait()
 row.update(exitCode=code,timedOut=timed,elapsedSeconds=time.monotonic()-start);results.append(row);put(logs/'result.json',results)
 if code or timed:
  put(logs/'inputs-after-failure.json',frozen_files(source));raise SystemExit(code or 124)
deps_after=dependencies();put(logs/'dependencies-after.json',deps_after);assert deps_before==deps_after
after=frozen_files(source);put(logs/'inputs-after.json',after);assert before==after
members=frozen_files(web);assert members and len(members)<4096
binding={'version':1,'diagnosticOnly':True,'baseProductCommit':PRODUCT,'helperSourceCommit':json.loads((B/'actual/external-source-binding.json').read_text())['helperSourceCommit'],'binaryPackageUnmodified':True,'instrumentedWebIsNotPackagedProduct':True,'webRoot':str(web),'webIndexSHA256':sha(web/'index.html'),'overlayBindingSHA256':sha(B/'overlay-binding.json'),'webMembers':members,'inputBindingsSHA256':sha(logs/'inputs-before.json'),'buildResults':results,'dependenciesSHA256':sha(logs/'dependencies-before.json')}
put(B/'diagnostic-web-binding.json',binding);print(json.dumps({'prepared':True,'web':str(web),'bindingSHA256':sha(B/'diagnostic-web-binding.json')}))
