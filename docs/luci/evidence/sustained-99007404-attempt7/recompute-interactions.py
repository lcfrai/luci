#!/usr/bin/env python3
from pathlib import Path
import json,gzip,math
root=Path(__file__).resolve().parent
x=json.loads(gzip.decompress((root/'analysis/interactions-with-spans.projection.json.gz').read_bytes()))
assert len(x['rows'])==2128
for key,expected in x['recomputedSummaries'].items():
 rows=[r for r in x['rows'] if r['phase']+':'+r['kind']==key]; vals=sorted(r['elapsedMs'] for r in rows);q=lambda p:vals[max(0,math.ceil(len(vals)*p)-1)]
 actual=dict(count=len(vals),successful=sum(r['success'] is True for r in rows),failed=sum(r['success'] is not True for r in rows),p50Ms=q(.5),p95Ms=q(.95),p99Ms=q(.99),maxMs=max(vals))
 assert actual==expected,key
print(json.dumps({'rows':len(x['rows']),'summaryGroups':len(x['recomputedSummaries']),'allOriginalNumericRowsIncluded':True,'sustainedAcceptance':'failed'}))
