#!/usr/bin/env python3
"""Copy completed, bounded receipts only. Never reads package/image binaries."""
from pathlib import Path
import datetime, hashlib, json, os, shutil, stat, subprocess

TOOLS = Path('/workspace/.luci-tools')
ROOT = Path('/workspace/luci')
BASE = TOOLS/'publish-package-runtime-5f4'
OUT = BASE/'capsule'
COMMIT = '5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
RUNTIME = TOOLS/'sandbox-candidate-20261009T134118Z-4a347241'
EXPORT = RUNTIME/'export-20261009T135357Z-01f4c63c'
COPIED = []

def sha(b): return hashlib.sha256(b).hexdigest()
def read(p):
    s=p.lstat(); assert stat.S_ISREG(s.st_mode) and s.st_size <= 16*1024*1024, p
    b=p.read_bytes(); t=p.lstat()
    assert (s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns)==(t.st_dev,t.st_ino,t.st_size,t.st_mtime_ns) and len(b)==s.st_size, p
    return b
def put(rel,b):
    p=OUT/rel; assert not p.exists(); p.parent.mkdir(parents=True,mode=0o700,exist_ok=True);p.write_bytes(b);p.chmod(0o600)
def copy(p,rel):
    b=read(p);put(rel,b);COPIED.append({'source':str(p),'publicationPath':rel,'sha256':sha(b),'bytes':len(b),'representation':'exact original bytes'})
def json_put(rel,value):put(rel,(json.dumps(value,indent=2)+'\n').encode())
def frozen_bundle(name,dest):
    p=TOOLS/name;m=json.loads(read(p/'manifest.json'))
    rows=m.get('members') or [dict(path=k,**v) for k,v in m['files'].items()]
    for x in rows:
        r=x['path'];assert not Path(r).is_absolute() and '..'not in Path(r).parts
        b=read(p/r);assert sha(b)==x['sha256'] and len(b)==x['bytes'],r
        copy(p/r,dest+'/'+r)
    copy(p/'manifest.json',dest+'/manifest.original.json')
def flat_files(p,dest):
    for f in sorted(p.iterdir()):
        if f.is_file() and not f.is_symlink():copy(f,dest+'/'+f.name)

def main():
    os.umask(0o077);assert not OUT.exists();OUT.mkdir(mode=0o700)
    # The child fragment is immutable and carries its own original bindings.
    fragment=BASE/'package-fragment'
    for p in sorted(fragment.rglob('*')):
        if p.is_file():copy(p,'package/'+str(p.relative_to(fragment)))
    frozen_bundle('cloud054-runtime-helper','preparation/runtime-helper')
    frozen_bundle('cloud054-runtime-operations','preparation/operations-v1')
    frozen_bundle('cloud054-runtime-operations-v2','preparation/operations-v2')
    frozen_bundle('cloud054-cache-retirement-plan','preparation/cache-review')
    for name in ['cloud054-runtime-helper-independent-review.json','cloud054-runtime-operations-v2-validation-scope.json','cloud054-tool-relocation-readonly-observation.json','cloud054-relocation-wrapper-preflight-refusal.json','cloud054-task-cache-reclamation.json','cloud054-default-go-cache-reclamation.json']:
        copy(TOOLS/name,'operations/'+name)
    for name in ['run-cloud054-tool-relocation-before-final-pin.py','run-cloud054-tool-relocation.py','retire-cloud054-old-image-root.py','retire-cloud054-old-cache.py.preflight-original','retire-cloud054-old-cache.py','clean-task-default-go-cache.py']:
        copy(TOOLS/name,'operations/helpers/'+name)
    for name in ['cloud054-old-image-retirement-5f4a966f','cloud054-old-cache-retirement','cloud054-old-cache-retirement-attempt2']:
        flat_files(TOOLS/name,'operations/'+name)
    operations=TOOLS/'cloud054-runtime-operations-v2'
    moves=[x for x in sorted(operations.iterdir()) if x.is_dir() and x.name.startswith(('root-','operation-'))]
    assert len(moves)==8
    for p in moves:
        flat_files(p,'operations/tool-moves/'+p.name)
        if p.name.startswith('operation-'):
            last=json.loads(read(p/'009-finished.json'));assert last['passed'] is True and last['stage']=='finished'
    for name in ['rust-relocation-20261009/tree-before.json','tool-relocation-20261009T1158/powershell-7.5.5/tree-before.json']:
        copy(TOOLS/name,'operations/original-tool-manifests/'+name.replace('/','-'))
    # Old archive proof is included only as receipt; huge binary stays private.
    for name in ['cloud054-runtime-space-plan/retained-333eff-archive-inspection.json','runtime-archive-1f845cfd2b45/receipt.json','runtime-archive-1f845cfd2b45/diagnostics.json']:
        copy(TOOLS/name,'operations/old-archive-proof/'+name.replace('/','-'))
    built=json.loads(read(EXPORT/'receipt-before.json'));final=json.loads(read(RUNTIME/'receipt.json'))
    assert built['sourceCommit']==COMMIT and built['status']=='built-config-verified-export-and-runtime-pending'
    assert final['sourceCommit']==COMMIT and final['status']=='export-verified-runtime-pending' and final['exportVerification']['status']=='passed'
    assert built['sourcesBefore']==built['sourcesAfter']==final['sourcesAfter'] and len(final['sourcesAfter'])==106
    copy(EXPORT/'receipt-before.json','runtime/build-receipt.original.json')
    copy(RUNTIME/'receipt.json','runtime/export-verified-receipt.original.json')
    for n in ['compile-disk.jsonl','compile.log','docker-build-disk.jsonl','docker-build.log']:
        copy(RUNTIME/n,'runtime/'+n)
    copy(RUNTIME/'context/Dockerfile','runtime/additive-Dockerfile.original')
    copy(RUNTIME/'context/bin/entrypoint.sh','runtime/entrypoint.original.sh')
    flat_files(EXPORT,'runtime/export')
    # Current source hashes are checked against immutable Git, not future HEAD.
    bindings=[]
    for path,digest in final['sourcesAfter'].items():
        b=subprocess.check_output(['git','-C',str(ROOT),'show',COMMIT+':'+path],timeout=30)
        assert sha(b)==digest,path;bindings.append({'path':path,'sha256':digest,'bytes':len(b)})
    json_put('runtime/source-bindings.json',{'commit':COMMIT,'inputCount':106,'inputs':bindings,'scope':'All declared tracked runtime inputs checked against immutable Git; not historical104 inputs or full control-plane compilation inventory'})
    for name in ['sandboxd/go.mod','sandboxd/go.sum','sandboxd/vendor/modules.txt','control-plane/go.mod','control-plane/go.sum']:
        b=subprocess.check_output(['git','-C',str(ROOT),'show',COMMIT+':'+name],timeout=30);put('runtime/module-sources/'+name,b)
    # Preserve the first publication-only path error without duplicating its partial capsule.
    for name in ['assemble.py','README.prepared.md','assembly.stdout','assembly.stderr','failure.json']:
        copy(BASE/'assembly-attempt1-missing-controller-vendor'/name,'publication-preparation-failure/'+name)
    copy(Path(__file__),'repro/assemble.py')
    json_put('original-bindings.json',{'sourceCommit':COMMIT,'originals':COPIED,'scope':'Exact originals only; no binary/archive bytes or installed terminal-run receipts in this capsule. Private fixture/state/ledger contents excluded.'})
    exclusions={
        'runtimeImageAndArchives':'No Docker image/archive bytes published. Original old archive proof and actual new streaming-export evidence are receipts. The stream verifier did not retain a new runtime archive.',
        'packageBinaries':'Package/image archives and executed binaries remain publisher-held, not newly made downloadable by this capsule.',
        'executionScope':'Build/export/storage maintenance only. No model quality, product lease release, Windows acceptance or terminal/WebSocket result is established.',
        'cacheScope':'Exact tool/root coordination and limited observations, not universal process inactivity. No original failed product fixture or charged lease was edited.',
        'provenanceLimits':'Upstream pin/compile/export source evidence does not certify a reproducible complete binary build. Actual matching browser/native runtime remains separately required.'
    }
    json_put('scope-limits.json',exclusions)
    # README is deliberately prepared separately; packaging fragment remains exact.
    copy(BASE/'README.prepared.md','README.md')
    members=[]
    for p in sorted(OUT.rglob('*')):
        if p.is_file():
            b=read(p);members.append({'path':str(p.relative_to(OUT)),'sha256':sha(b),'bytes':len(b)})
    json_put('publication.json',{'version':1,'productCommit':COMMIT,'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'members':members,'scope':'Completed Linux package/runtime build-export and exact root maintenance evidence. Installed terminal and local release gates separate.'})
    entries=members+[{'path':'publication.json','sha256':sha(read(OUT/'publication.json')),'bytes':(OUT/'publication.json').stat().st_size}]
    put('SHA256SUMS', ''.join(x['sha256']+'  '+x['path']+'\n'for x in entries).encode())
    print(json.dumps({'members':len(members),'checksums':len(entries),'originals':len(COPIED),'publicationSHA256':sha(read(OUT/'publication.json')),'sumsSHA256':sha(read(OUT/'SHA256SUMS'))},indent=2))

if __name__=='__main__':main()
