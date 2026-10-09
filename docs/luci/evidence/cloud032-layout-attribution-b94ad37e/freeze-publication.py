"""Final finite privacy scan and byte manifest. No runtime/application calls."""
from pathlib import Path
import hashlib, json, re
ROOT=Path(__file__).resolve().parent
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
bindings=json.loads((ROOT/'source-bindings.json').read_text())
secrets=set();identities=set()
def inspect(value):
    if isinstance(value,dict):
        for key,item in value.items():
            if re.fullmatch(r'(?:access_?token|refresh_?token|password|authorization|cookie|set-cookie|secret)',key,re.I) and isinstance(item,str) and len(item)>=8:secrets.add(item)
            inspect(item)
    elif isinstance(value,list):
        for item in value:inspect(item)
for item in bindings['privateInputs']:
    for key in ['privateResult','privateTrace']:
        p=Path(item[key]);assert sha(p)==item[key+'SHA256'];raw=p.read_bytes()
        identities.update(x.decode() for x in re.findall(rb'\b(?:p-[a-f0-9]{8}|run-[a-f0-9]{24}|queue-[a-f0-9]{24}|user-[A-Za-z0-9_-]{22})\b',raw))
        secrets.update(x.decode() for x in re.findall(rb'Bearer ([A-Za-z0-9_./+~-]{16,})',raw))
        inspect(json.loads(raw))
excluded={'manifest.json','SHA256SUMS','final-privacy-review.json'}
files=sorted(p for p in ROOT.rglob('*') if p.is_file() and p.parent==ROOT and p.name not in excluded)
files+=sorted(p for p in ROOT.rglob('*') if p.is_file() and p.parent!=ROOT)
matches=[]
for p in files:
    raw=p.read_bytes()
    for kind,values in [('credential',secrets),('fixtureIdentity',identities)]:
        if any(x.encode() in raw for x in values):matches.append({'path':str(p.relative_to(ROOT)),'kind':kind})
assert not matches,matches
review={'scope':'Final exact-byte finite scan; explicit field allowlisting remains primary boundary. No universal secret-absence claim. Raw result/trace excluded.','publicFilesScanned':len(files),'capturedCredentialValuesChecked':len(secrets),'capturedFixtureIdentityValuesChecked':len(identities),'matches':matches,'scannedFiles':{str(p.relative_to(ROOT)):sha(p) for p in files}}
(ROOT/'final-privacy-review.json').write_text(json.dumps(review,indent=2)+'\n')
entries=[]
for p in sorted(ROOT.rglob('*')):
    if not p.is_file() or (p.parent==ROOT and p.name in {'manifest.json','SHA256SUMS'}):continue
    assert not p.is_symlink()
    entries.append({'path':str(p.relative_to(ROOT)),'bytes':p.stat().st_size,'sha256':sha(p)})
manifest={'scope':'Safe combined V1 failure/V2 eight-round diagnostic. Capture completion only; original latency failure remains. Private raw evidence hash-bound, not included.','files':entries,'totalBytes':sum(x['bytes'] for x in entries),'capturedRunSource':'b94ad37e53181e360ac8910ee980570de622728c','v2ObservedCheckout':'e780abe20cc6d67096c00e901578716afbb2cb51','repositoryEdits':False,'runtimeReruns':False}
(ROOT/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
checks=entries+[{'path':'manifest.json','sha256':sha(ROOT/'manifest.json')}]
(ROOT/'SHA256SUMS').write_text(''.join(f"{x['sha256']}  {x['path']}\n" for x in checks))
for item in checks:assert sha(ROOT/item['path'])==item['sha256']
# Verify every frozen preparation's original checksum entries after publication.
for v in [1,2]:
    p=ROOT/f'accepted-preparation-v{v}'
    for line in (p/'SHA256SUMS').read_text().splitlines():
        digest,name=line.split('  ',1);assert sha(p/name)==digest
print(json.dumps({'filesWithManifestAndSums':len(entries)+2,'totalBytesWithoutManifestAndSums':manifest['totalBytes'],'manifestSHA256':sha(ROOT/'manifest.json'),'SHA256SUMSSHA256':sha(ROOT/'SHA256SUMS'),'knownPrivateValueMatches':0,'frozenNestedChecksums':'verified'}))
