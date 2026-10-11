#!/usr/bin/env python3
from pathlib import Path
import gzip,hashlib,json
P=Path(__file__).resolve().parent
sha=lambda raw:hashlib.sha256(raw).hexdigest()
def read(rel):
 raw=(P/rel).read_bytes()
 return gzip.decompress(raw) if rel.endswith('.gz') else raw
def jsonread(rel):return json.loads(read(rel))
listed={line[66:]:line[:64] for line in (P/'SHA256SUMS').read_text().splitlines()}
actual={p.relative_to(P).as_posix():sha(p.read_bytes()) for p in P.rglob('*') if p.is_file() and p.name!='SHA256SUMS'}
assert listed==actual,'Checksum inventory or byte mismatch'
publication=jsonread('publication.json')
for rel,row in publication['bindings'].items():
 raw=read(rel)
 assert len(raw)==row['originalBytes'] and sha(raw)==row['originalSHA256'],rel
historical=jsonread('interrupted-attempt-001/vitest.json.gz')
assert (historical['numTotalTests'],historical['numPassedTests'],historical['numFailedTests'])==(2940,2938,2)
assert historical['success'] is False
assert len(historical['testResults'])==168
assert sum(len(r['assertionResults']) for r in historical['testResults'])==2940
assert sum(a['status']=='passed' for r in historical['testResults'] for a in r['assertionResults'])==2938
assert sum(a['status']=='failed' for r in historical['testResults'] for a in r['assertionResults'])==2
for label,counts,exitcode in [('before',(24,23,1),1),('after',(24,24,0),0)]:
 report=jsonread('runs/'+label+'/vitest.json');result=jsonread('runs/'+label+'/result.json')
 assert (report['numTotalTests'],report['numPassedTests'],report['numFailedTests'])==counts
 assert result['exitCode']==exitcode and result['inputsStable'] is True
 assert len(report['testResults'])==1 and report['testResults'][0]['name'].endswith('/src/seams.test.ts')
 assert result['command'][3:7]==['src/seams.test.ts','--maxWorkers=1','--no-file-parallelism','--reporter=json']
 before=jsonread('runs/'+label+'/source-before.json.gz');after=jsonread('runs/'+label+'/source-after.json.gz')
 assert before==after and len(before)==1229
 for name,row in result['artifacts'].items():
  rel='runs/'+label+'/'+name
  if not (P/rel).exists():rel+='.gz'
  raw=read(rel)
  assert len(raw)==row['bytes'] and sha(raw)==row['sha256'],rel
first=jsonread('runs/before/source-before.json.gz');second=jsonread('runs/after/source-before.json.gz')
contract='docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md'
assert [p for p in first if first[p]!=second[p]]==[contract]
for variant,manifest in [('original',first),('fixed',second)]:
 assert sha(read('seam/'+variant+'/'+contract))==manifest[contract]
for path in (P/'seam/unchanged').rglob('*'):
 if path.is_file():
  rel=path.relative_to(P/'seam/unchanged').as_posix()
  assert sha(read('seam/unchanged/'+rel))==first[rel.removesuffix('.gz')]
fixed=read('seam/fixed/'+contract).decode();heading=fixed.index('## 2. The domain model');start=fixed.index('```ts\n',heading)+6;end=fixed.index('\n```',start)
assert fixed[start:end].rstrip('\n')==read('seam/unchanged/desktop/src/store/types.ts').decode().rstrip('\n')
review=jsonread('provenance-review.json')
assert review['sourceComparison']['matchingCount']==1229
assert review['wrapperEvidence']['vitestExitCodeObserved'] is False
assert review['wrapperEvidence']['typecheckStartObserved'] is False
assert review['wrapperEvidence']['historicalAfterSnapshotObserved'] is False
print(json.dumps({'verifiedFiles':len(actual),'bytes':sum(p.stat().st_size for p in P.rglob('*') if p.is_file()),'historicalAssertions':2940,'historicalFailed':2,'narrowBefore':'23 passed, 1 failed, exit 1','narrowAfter':'24 passed, 0 failed, exit 0','historicalExitObserved':False,'historicalTypecheckObserved':False},indent=2))
