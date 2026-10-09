#!/usr/bin/env python3
"""Verify published small members and bounded lossless metadata representations."""
from pathlib import Path
import gzip,hashlib,json
B=Path(__file__).parent
def md(b):return {'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)}
checks=0
for n,r in json.loads((B/'publication.json').read_bytes())['files'].items():
 assert '..' not in Path(n).parts and not Path(n).is_absolute()
 assert md((B/n).read_bytes())==r;checks+=1
for line in (B/'SHA256SUMS').read_text().splitlines():
 sha,n=line.split('  ',1);assert hashlib.sha256((B/n).read_bytes()).hexdigest()==sha;checks+=1
for r in json.loads((B/'representations.json').read_bytes())['members']:
 p=B/r['publishedPath'];assert md(p.read_bytes())==r['stored'];assert r['original']['bytes']<8*1024*1024
 with gzip.open(p,'rb') as f:
  b=f.read(r['original']['bytes']+1);assert not f.read(1)
 assert md(b)==r['original'];checks+=1
print(json.dumps({'passed':True,'checks':checks,'scope':'Public member hashes/lengths and decoded metadata only; no original archive/private-process/fixture reads or runtime.'}))
