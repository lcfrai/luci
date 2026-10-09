from pathlib import Path
from collections import defaultdict
import gzip,json,math
p=Path(__file__).resolve().parent
result=json.loads(gzip.decompress((p/'result.projection.json.gz').read_bytes()));expected=json.loads((p/'quantiles.recomputed.json').read_text())['groups'];groups=defaultdict(list)
for row in result['interactions']:groups[row['phase']+':'+row['kind']].append(row)
assert len(result['interactions'])==400
for key,rows in groups.items():
 values=sorted(row['elapsedMs']for row in rows)
 actual={'count':len(values),'successful':sum(row['success']for row in rows),'failed':sum(not row['success']for row in rows),'p50Ms':values[math.ceil(len(values)*.5)-1],'p95Ms':values[math.ceil(len(values)*.95)-1],'p99Ms':values[math.ceil(len(values)*.99)-1],'maxMs':values[-1]}
 assert actual==expected[key],key
print('All400 instrumented rows and three quantile groups match; not an acceptance or complete-profile claim.')
