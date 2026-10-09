"""Read-only bounded publication scan. Private values stay in memory; report counts only."""
import hashlib,json,re,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parent
RAW=Path('/workspace/luci/artifacts/quality-swarm/idle-29931afc-long-frame-attempt1/driver/result.json')
TRACE=RAW.parent/'action-only-trace.zip'
assert RAW.stat().st_size<64*1024*1024 and TRACE.stat().st_size<64*1024*1024
assert hashlib.sha256(RAW.read_bytes()).hexdigest()=='d71194560d8c6852b2561062a236ef2af92d32feb917f624a507e28d0977e73f'
assert hashlib.sha256(TRACE.read_bytes()).hexdigest()=='1a954cca6e7a4565dd3b6c4a447fd28753ec60915532fedb5b2a26bbc630e5ea'
needles=set(); privateIDs=set(); privateText=[RAW.read_text()]
def walk(x):
 if isinstance(x,dict):
  if str(x.get('name','')).lower() in {'authorization','cookie','set-cookie','x-api-key'}:
   v=x.get('value');
   if isinstance(v,str) and len(v)>=8:
    needles.add(v)
    if v.lower().startswith('bearer '):needles.add(v[7:])
  for k,v in x.items():
   if k.lower() in {'password','access_token','refresh_token','apikey','api_key','sessiontoken'} and isinstance(v,str) and len(v)>=8:needles.add(v)
   walk(v)
 elif isinstance(x,list):
  for v in x:walk(v)
walk(json.loads(privateText[0]))
with zipfile.ZipFile(TRACE) as z:
 assert sum(i.file_size for i in z.infolist())<128*1024*1024
 for i in z.infolist():
  if i.filename.endswith(('.network','.trace')):
   content=z.read(i).decode();privateText.append(content)
   for line in content.splitlines():
    try:walk(json.loads(line))
    except json.JSONDecodeError:pass
for t in privateText:privateIDs.update(re.findall(r'\b(?:project|user|org|workspace|team)-[A-Za-z0-9_-]{16,}\b',t))
# Copy all accepted harness bytes exactly, separately from artifact privacy review.
runHarness=RAW.parent/'accepted-harness';prep=ROOT/'accepted-preparation'/'actual'
for p in runHarness.iterdir():
 if p.is_file():assert (prep/p.name).read_bytes()==p.read_bytes(),p.name
hits=[]; allfiles=[]
for p in ROOT.rglob('*'):
 if not p.is_file() or p.name in {'privacy-review.json','manifest.json','SHA256SUMS'} and p.parent==ROOT:continue
 b=p.read_bytes();assert len(b)<64*1024*1024
 text=b.decode('utf8');allfiles.append(str(p.relative_to(ROOT)))
 if any(n in text for n in needles|privateIDs):hits.append(str(p.relative_to(ROOT)))
forbidden=[str(p.relative_to(ROOT)) for p in ROOT.rglob('*') if p.is_file() and (p.suffix in {'.zip','.db','.sqlite','.har'} or p.name in {'result.json','invocation.json'})]
report={'rawResultSHA256':hashlib.sha256(RAW.read_bytes()).hexdigest(),'privateTraceSHA256':hashlib.sha256(TRACE.read_bytes()).hexdigest(),'filesScanned':len(allfiles),'credentialNeedleCount':len(needles),'privateOpaqueIDNeedleCount':len(privateIDs),'literalNeedleHitFiles':hits,'forbiddenPrivateArtifactFiles':forbidden,'acceptedHarnessMatchesFrozenPreparation':True,'passed':not hits and not forbidden,'scope':'Known credentials/opaque IDs from private retained result/trace searched literally, plus explicit raw-artifact exclusion and allowlisted numeric projections. Negative scan is not a universal secret detector. Runtime source includes credential-handling code, never captured values.'}
(ROOT/'privacy-review.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report));assert report['passed']
