"""Read-only published small-file/source receipt verification. No operation import."""
from pathlib import Path
import hashlib,json,stat
root=Path(__file__).resolve().parent
def sha(b): return hashlib.sha256(b).hexdigest()
def data(p):
    s=p.lstat(); assert stat.S_ISREG(s.st_mode) and s.st_size<=262144, str(p)
    return p.read_bytes()
manifest=json.loads(data(root/'publication.json')); count=0
for m in manifest['members']:
    p=root/m['path']; assert p.resolve().is_relative_to(root)
    b=data(p); assert len(b)==m['bytes'] and sha(b)==m['sha256'],m['path']; count+=1
expected={m['path'] for m in manifest['members']}|{'publication.json','SHA256SUMS'}
assert {p.relative_to(root).as_posix() for p in root.rglob('*') if p.is_file()}==expected
for line in data(root/'SHA256SUMS').decode().splitlines():
    h,n=line.split('  ',1); assert sha(data(root/n))==h,n;count+=1
for b in json.loads(data(root/'original-bindings.json'))['members']:
    raw=data(root/b['publicationPath']); assert len(raw)==b['bytes'] and sha(raw)==b['sha256'];count+=1
prep=json.loads(data(root/'preparation/manifest.json'))
for m in prep['members']:
    raw=data(root/'preparation'/m['path']); assert len(raw)==m['bytes'] and sha(raw)==m['sha256'];count+=1
final=json.loads(data(root/'actual/operation-304739566509590/011-finished.json'))
wrapper=json.loads(data(root/'actual/root-operation-304739566509590/wrapper.json'))
coordbytes=data(root/'actual/root-operation-304739566509590/coordination.json'); coord=json.loads(coordbytes)
assert final['sourceBefore']==final['sourceAfter']
for p,h in final['sourceBefore'].items(): assert sha(data(root/'preparation'/Path(p).name))==h;count+=1
assert coord==final['coordination']
assert coord['operationSources']=={Path(k).name:v for k,v in final['sourceBefore'].items()}
assert wrapper['wrapperSHA256']==sha(data(root/'preparation/run.py'))
assert wrapper['command'][wrapper['command'].index('--quiescence-sha256')+1]==sha(coordbytes)
assert final['passed'] and wrapper['passed'] and wrapper['exitCode']==0 and not wrapper['forced'] and not wrapper['timedOut']
for i,p in enumerate(sorted((root/'actual/operation-304739566509590').glob('*.json')),1):
    j=json.loads(data(p)); assert j['steps']==final['steps'][:i] and j['sourceBefore']==final['sourceBefore'];count+=1
print(json.dumps({'passed':True,'hashAndJournalChecks':count,'scope':'Published small-file and declared receipt/source bindings only; no operation/test or large-payload validation.'}))
