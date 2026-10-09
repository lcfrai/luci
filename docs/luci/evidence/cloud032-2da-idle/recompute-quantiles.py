#!/usr/bin/env python3
"""Read this capsule only; verify all six summaries from all 432 numeric rows."""
from pathlib import Path
import gzip,json,math,hashlib
root=Path(__file__).resolve().parent
payload=gzip.decompress((root/'result.projection.json.gz').read_bytes())
binding=json.loads((root/'binding.json').read_text())
assert hashlib.sha256(payload).hexdigest()==binding['resultProjectionUncompressedSHA256']
data=json.loads(payload); rows=data['interactionDurations']; expected=json.loads((root/'quantiles.recomputed.json').read_text())['groups']
assert len(rows)==432 and [r['index'] for r in rows]==list(range(432))
for key,summary in expected.items():
    cohort=[r for r in rows if r['phase']+':'+r['kind']==key]
    vals=sorted(r['elapsedMs'] for r in cohort)
    q=lambda p:vals[max(0,math.ceil(len(vals)*p)-1)]
    actual=dict(count=len(vals),successful=sum(r['success'] is True for r in cohort),failed=sum(r['success'] is not True for r in cohort),p50Ms=q(.50),p95Ms=q(.95),p99Ms=q(.99),maxMs=max(vals))
    assert actual==summary, key
    assert all(actual[k]==data['result']['interactionSummary'][key][k] for k in actual)
assert sum(r['phase']=='idle-interactive-calibration' for r in rows)==400
assert sum(r['phase']=='idle-actionability-trace-followup' for r in rows)==32
assert data['result']['targetComparison']['project-switch']['withinReference'] is False
assert data['result']['targetComparison']['input']['withinReference'] is True
print(json.dumps({'numericRows':len(rows),'groups':len(expected),'primarySwitchTarget':'failed','primaryInputTarget':'passed','noFilteringOrSubtraction':True}))
