#!/usr/bin/env python3
from pathlib import Path
import hashlib,json
r=Path(__file__).resolve().parent
count=0
for manifest in [r/'publication.json',*sorted(r.glob('attempt*/publication.json'))]:
 v=json.loads(manifest.read_bytes());base=manifest.parent
 for name,want in v['files'].items():
  p=base/name;assert p.resolve().is_relative_to(base.resolve());b=p.read_bytes();assert len(b)==want['bytes'] and hashlib.sha256(b).hexdigest()==want['sha256'],str(p);count+=1
print(json.dumps({'verifiedBindings':count,'runtimeStarted':False}))
