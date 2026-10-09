from pathlib import Path
import hashlib,json
p=Path(__file__).resolve().parent
v=json.loads((p/'publication.json').read_text())
expected={row['path'] for row in v['members']}
actual={str(f.relative_to(p)) for f in p.rglob('*') if f.is_file()}-{'publication.json','SHA256SUMS'}
assert actual==expected,(actual-expected,expected-actual)
for row in v['members']:
 b=(p/row['path']).read_bytes();assert len(b)==row['bytes'] and hashlib.sha256(b).hexdigest()==row['sha256'],row['path']
for line in (p/'SHA256SUMS').read_text().splitlines():
 h,name=line.split('  ',1);assert hashlib.sha256((p/name).read_bytes()).hexdigest()==h,name
print(json.dumps({'verifiedMembers':len(expected),'checksumEntries':len((p/'SHA256SUMS').read_text().splitlines()),'scope':'byte integrity only; no fixture/app execution'}))
