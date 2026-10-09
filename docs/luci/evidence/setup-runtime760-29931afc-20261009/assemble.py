from pathlib import Path
import hashlib,json,shutil
ROOT=Path('/workspace/.luci-tools');OUT=ROOT/'publish-setup-runtime760-29931afc';CAP=OUT/'tree/docs/luci/evidence/setup-runtime760-29931afc-20261009';CAP.mkdir(parents=True,exist_ok=False);bindings=[]
def sha(b):return hashlib.sha256(b).hexdigest()
def add(src,dest):
 src=Path(src);assert src.is_file()and not src.is_symlink()and src.stat().st_size<=1<<20,(src,'must be small original regular file');data=src.read_bytes();p=CAP/dest;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data);bindings.append({'path':dest,'originalPath':str(src),'sha256':sha(data),'bytes':len(data),'representation':'exact original bytes; no redaction or private fixture/state payload included'})
prep=ROOT/'runtime760-bootstrap-v1';pm=json.loads((prep/'preparation-manifest-v2.json').read_bytes())
for x in pm['members']:
 p=prep/x['path'];b=p.read_bytes();assert sha(b)==x['sha256']and len(b)==x['bytes'];add(p,'preparation/'+x['path'])
add(prep/'manifest.json','preparation/initial-manifest-including-runtime-sink.json');add(prep/'preparation-manifest-v2.json','preparation/source-only-manifest-v2.json')
for name in ['runtime760-bootstrap-v1-source-review-initial.json','runtime760-bootstrap-v1-source-review-followup.json']:
 add(ROOT/name,'reviews/'+name)
for p in sorted(prep.iterdir()):
 if p.is_dir()and p.name.startswith(('watchdog-','place-watchdog-')):
  for f in sorted(p.iterdir()):
   if f.is_file()and not f.is_symlink():add(f,'operations/watchdogs/'+p.name+'/'+f.name)
for name in ['runtime760-placement-425-to-root-root1','runtime760-placement-425-to-tmp-root1','runtime760-placement-333-to-tmp-root1']:
 for p in sorted((ROOT/name).glob('*.json')):add(p,'operations/'+name+'/'+p.name)
tools=ROOT/'cloud054-runtime-operations-v2'
for name in ['operation-powershell-7.5.5-to-root-305790159368609','operation-powershell-7.5.5-return-to-tmp-306235930410029','operation-rust-1.99.0-to-root-305770151139833','operation-rust-1.99.0-return-to-tmp-306294319515429']:
 for p in sorted((tools/name).glob('*.json')):add(p,'operations/tools/'+name+'/'+p.name)
for suffix in ['powershell-7.5.5-to-root-305790159368609','powershell-7.5.5-return-to-tmp-306235930410029','rust-1.99.0-to-root-305770151139833','rust-1.99.0-return-to-tmp-306294319515429']:
 for p in sorted((tools/('root-'+suffix)).iterdir()):
  if p.is_file()and not p.is_symlink():add(p,'operations/tool-watchdogs/root-'+suffix+'/'+p.name)
for n in ['operation_common.py','relocate_tools.py']:add(tools/n,'sources/tool-placement/'+n)
add(ROOT/'run-cloud054-tool-relocation.py','sources/tool-placement/run.py')
add(ROOT/'go-tool-restore-v1/operation_common.py','sources/archive-process-observer.py')
add(ROOT/'relocate-historical425-v1/relocate.py','sources/original-425-placement.py')
add(ROOT/'refresh-runtime-archive-333eff.py','sources/original-archive-primitives.py')
add(ROOT/'cloud054-runtime-helper/prepare-runtime.py','sources/verified-runtime-helper.py')
add(ROOT/'sandbox-candidate-20261009T134118Z-4a347241/receipt.json','subjects/runtime-build-and-export-receipt.json')
arc=ROOT/'runtime-archive-760-9a42e556fa22'
for p in sorted(arc.iterdir()):
 if p.is_file()and not p.is_symlink()and p.stat().st_size<=1<<20:add(p,'archive/'+p.name)
for p in sorted((ROOT/'runtime760-restore-check').iterdir()):
 if p.is_file()and not p.is_symlink():add(p,'restore-check/'+p.name)
for n in ['install.sh','start.md','representation-note.json']:add(ROOT/'onboarding299-draft-source'/n,'saved-field-export/'+n)
add(ROOT/'runtime760-config-save-public.json','configuration/save-public.json')
add(Path(__file__),'assemble.py')
(CAP/'original-bindings.json').write_text(json.dumps({'version':1,'scope':'Small original source/receipt bytes only. Large archives, tool trees, packages, private fixtures/credentials and Docker client directories are excluded. Original absolute task-tool paths are references, not promises of reader access.','bindings':bindings},indent=2)+'\n')
print(json.dumps({'capsule':str(CAP),'originalBindings':len(bindings),'bytes':sum(x['bytes']for x in bindings)}))
