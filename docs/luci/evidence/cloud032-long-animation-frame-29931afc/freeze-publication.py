import hashlib,json
from pathlib import Path
root=Path(__file__).resolve().parent
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
prep=root/'accepted-preparation'
assert sha(prep/'manifest.json')=='7a3b863bab3a3021d6892bec5e5ef4d1fa47ea0c15b6be022a3283228fbd9ddb'
for line in (prep/'SHA256SUMS').read_text().splitlines():
 expected, name=line.split('  ',1);assert sha(prep/name)==expected,name
assert json.loads((root/'privacy-review.json').read_text())['passed'] is True
files=[{'path':str(p.relative_to(root)),'bytes':p.stat().st_size,'sha256':sha(p)} for p in sorted(root.rglob('*')) if p.is_file() and not (p.parent==root and p.name in {'manifest.json','SHA256SUMS'})]
(root/'manifest.json').write_text(json.dumps({'schema':1,'scope':'Exact prepared helper and sanitized completed LoAF diagnostic; private original result/trace excluded. Source-only next-attribution plan is not executed.','productSource':'29931afc5b31ae9b9bd7b8973a7e3fdb840609a3','rawResultSHA256':'d71194560d8c6852b2561062a236ef2af92d32feb917f624a507e28d0977e73f','files':files},indent=2)+'\n')
files.append({'path':'manifest.json','sha256':sha(root/'manifest.json')})
(root/'SHA256SUMS').write_text(''.join(x['sha256']+'  '+x['path']+'\n' for x in sorted(files,key=lambda x:x['path'])))
for x in files:assert sha(root/x['path'])==x['sha256']
print(json.dumps({'filesIncludingChecksums':len(files)+1,'manifestSHA256':sha(root/'manifest.json'),'SHA256SUMSSHA256':sha(root/'SHA256SUMS'),'byteTotal':sum(p.stat().st_size for p in root.rglob('*') if p.is_file()),'passed':True}))
