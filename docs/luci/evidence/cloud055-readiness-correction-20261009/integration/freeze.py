from pathlib import Path
import hashlib,json,shutil,datetime
base=Path(__file__).resolve().parent.parent
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
receipt=base/'controlled/receipts/attempt2';reports=list(receipt.rglob('report.json'));assert len(reports)==1,reports
r=json.loads(reports[0].read_text());assert r['passed'] and r['sourceUnchanged']
assert sha(base/'observer/autonomous-readiness.mjs')=='7f534431a37a0bd599c2de48870d3c9e8badfd1e33b2d48e47078782aa74467b'
assert (base/'actual/autonomous-readiness.mjs').read_bytes()==(base/'observer/autonomous-readiness.mjs').read_bytes()
summary={'scope':'Controlled actual d20 source fixture only; no installed v2 run.','checks':r['checks'],'sourceUnchanged':r['sourceUnchanged'],'cleanup':r['cleanup'],'reportSHA256':sha(reports[0]),'observerSHA256':sha(base/'observer/autonomous-readiness.mjs'),'frozenV1ObserverSHA256':sha(base/'controlled/original/autonomous-readiness.mjs'),'fixtureSourceBindingSHA256':sha(base/'controlled/source-binding.json'),'retainedFailure':'attempt1 had exactly32 real settled controls and correctly failed the >32 precondition; no comparison was certified. Attempt2 uses two legitimate recorded review obligations per project to reveal existing show-more controls.','metricLimitation':'No first-ready/paint/cause/performance claim. Same original targets/action spans/caps.','runNotLaunched':True}
(base/'validation-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
capsule=base/'capsule';assert not capsule.exists();capsule.mkdir()
for name in ['README.md','launch.py','template-capture.json','observer.patch','validation-summary.json']:
 shutil.copyfile(base/name,capsule/name)
for directory in ['observer','actual','original-v1','integration']:
 shutil.copytree(base/directory,capsule/directory)
# Include every helper/proof/receipt except reproducible caches, the separately
# hash-bound immutable full Git extraction, archive and ephemeral profile paths.
for p in (base/'controlled').rglob('*'):
 relative=p.relative_to(base/'controlled')
 if not p.is_file() or p.is_symlink() or relative.parts[0]=='source' or any(part in ['vite-cache','node_modules','__pycache__'] for part in relative.parts) or p.suffix in ['.tar','.gz'] or p.name=='owned-resources.json':continue
 target=capsule/'controlled'/relative;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(p,target)
rows=[{'path':str(p.relative_to(capsule)),'bytes':p.stat().st_size,'sha256':sha(p)} for p in sorted(capsule.rglob('*')) if p.is_file()]
manifest={'createdAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'External v2 review capsule; immutable Git source extraction retained externally and bound by source manifest, not recopied. Reproducible browser caches and private owned temporary profile paths omitted. Original failures and all helper/proof bytes retained.','members':rows,'payloadBytes':sum(r['bytes'] for r in rows)}
(capsule/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');(capsule/'SHA256SUMS').write_text(''.join(r['sha256']+'  '+r['path']+'\n' for r in rows)+sha(capsule/'manifest.json')+'  manifest.json\n')
print(json.dumps({'capsule':str(capsule),'payloadMembers':len(rows),'payloadBytes':manifest['payloadBytes'],'manifestSHA256':sha(capsule/'manifest.json'),'checksumsSHA256':sha(capsule/'SHA256SUMS'),'observerSHA256':sha(base/'actual/autonomous-readiness.mjs'),'bindingSHA256':sha(base/'actual/external-source-binding.json'),'launcherSHA256':sha(base/'launch.py'),'checks':len(r['checks'])}))
