from pathlib import Path
import hashlib,json
r=Path(__file__).resolve().parent;m=json.loads((r/'publication.json').read_bytes());count=0
for name,v in m['files'].items():
 p=r/name;b=p.read_bytes();assert p.resolve().is_relative_to(r) and len(b)==v['bytes'] and hashlib.sha256(b).hexdigest()==v['sha256'];count+=1
print(json.dumps({'verifiedMembers':count,'runtimeStarted':False}))
