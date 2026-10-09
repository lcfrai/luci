#!/usr/bin/env python3
from pathlib import Path
import json,hashlib
p=Path(__file__).resolve().parent
m=json.loads((p/'publication.json').read_bytes())
for name,w in m['files'].items():
 q=p/name;assert q.resolve().is_relative_to(p.resolve());b=q.read_bytes();assert len(b)==w['bytes'] and hashlib.sha256(b).hexdigest()==w['sha256'],name
print(json.dumps({'bindings':len(m['files']),'runtimeStarted':False}))
