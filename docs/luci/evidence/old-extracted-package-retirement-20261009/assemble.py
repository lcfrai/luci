#!/usr/bin/env python3
"""Assemble retained public evidence only after the exclusive measurement ends."""
from pathlib import Path
import gzip,hashlib,json,os

B=Path(__file__).parent
P=Path('/workspace/.luci-tools/old-extracted-packages-retirement-plan')
A=Path('/workspace/.luci-tools/old-extracted-package-retirement-operation-root1')
O=B/'capsule'
def md(b):return {'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)}
def enc(x):return (json.dumps(x,indent=2)+'\n').encode()
def main():
 assert os.environ.get('LUCI_PUBLICATION_WINDOW_OPEN')=='1','Root measurement window must be closed before assembly'
 os.umask(0o077);assert not O.exists();O.mkdir();originals=[];representations=[]
 def put(n,b):
  p=O/n;p.parent.mkdir(parents=True,exist_ok=True);assert not p.exists();p.write_bytes(b)
 def copy(p,n):
  p=Path(p);assert p.is_file() and not p.is_symlink() and p.stat().st_size<2*1024*1024
  raw=p.read_bytes();put(n,raw);originals.append({'source':str(p),'publishedPath':n,**md(raw)});assert p.read_bytes()==raw
 final=json.loads((A/'015-final.json').read_bytes())
 assert md((A/'015-final.json').read_bytes())['sha256']=='e0898724b9854af115272f9c302cc25a5b45f79d85b9ff1ce2d3525614d2b6e8'
 assert final['status']=='passed' and final['removedFiles']==8362 and final['originalArchivesUnchanged'] and final['keptRootMetadataUnchanged']
 assert final['freeAfter']['/tmp']-final['freeBefore']['/tmp']==859254784
 for p in sorted(A.glob('*.json')):copy(p,'operation/'+p.name)
 copy('/workspace/.luci-tools/old-extracted-package-retirement-root1-command.json','operation/root-command-result.json')
 for n in ['plan.json','verify.py','verify-v2.py','verification.json','verification-v2-correction.json','retire.py','restore.py','check_active_usage_v2.py','active-usage-summary.json','operation-preparation.json','README.md','RETIRE-COMMAND.md']:
  copy(P/n,'prepared/'+n)
 for p in sorted((P/'results').glob('*.json')):copy(p,'prepared/results/'+p.name)
 for p in sorted((P/'verification-v2').glob('*.json')):copy(p,'prepared/verification-v2/'+p.name)
 for p in sorted((P/'verification-v2/results').glob('*.json')):copy(p,'prepared/verification-v2/results/'+p.name)
 plan=json.loads((P/'plan.json').read_bytes())
 assert md((P/'plan.json').read_bytes())['sha256']==final['planSHA256']
 for row in plan['packages']:
  src=Path(row['manifest']['path']);raw=src.read_bytes();assert md(raw)=={k:row['manifest'][k] for k in ('sha256','bytes')}
  name='member-manifests/'+row['name']+'.json.gz';packed=gzip.compress(raw,compresslevel=6,mtime=0);assert gzip.decompress(packed)==raw;put(name,packed)
  representations.append({'source':str(src),'publishedPath':name,'encoding':'gzip','original':md(raw),'stored':md(packed),'decodedPath':'prepared/verification-v2/manifests/'+row['name']+'.json','transformation':'Lossless gzip only; exact original JSON bytes recoverable. No field redaction or content rewrite.'})
  assert src.read_bytes()==raw
 # Summarize repeated retained scans without republishing thousands of unrelated PID identities.
 scans=[]
 for p in sorted(A.glob('*.json')):
  r=json.loads(p.read_bytes())
  if 'usage' in r:
   scans.append({'operationReceipt':p.name,**r['usage'],'sourceKind':'Exact small retained operation receipt; full process snapshot excluded and not reread by this publication.'})
 assert len(scans)==7 and all(x['unknownCount']==4 and x['literalZombieCount']==6636 for x in scans)
 put('process-observation-scope.json',enc({'runtimeOperationScans':scans,'unknownPolicy':'Exact accepted PID/start identities203/1521,254/1534,260700/2331192 have unreadable exe/cwd/maps/fd. Inspector disappearing fd may also remain. No universal idle assertion.','preparedScan':'prepared/active-usage-summary.json','fullProcessSnapshotsPublished':False,'omittedOriginals':'Thousands of literal unrelated zombie/process identities per snapshot are excluded. Their original hashes remain in exact operation receipts. These exclusions do not mean zero processes.'}))
 summary=json.loads((P/'active-usage-summary.json').read_bytes())
 put('prior-incomplete-scan.json',enc({'source':'prepared/active-usage-summary.json','priorIncompleteReceipt':summary['priorIncompleteReceipt'],'scope':'Retained original first scan reached its4096-process cap. This incomplete preparation attempt is not an idle/pass claim; the corrected scan is separately bound. Original full snapshot remains retained privately and excluded from this compact capsule.'}))
 put('archive-and-member-summary.json',enc({'packages':[{'name':x['name'],'sourceCommit':x['sourceCommit'],'archive':x['archive'],'files':x['files'],'directories':x['directories'],'logicalBytes':x['logicalBytes'],'allocatedFileBytes':x['allocatedFileBytes'],'manifest':x['manifest']} for x in plan['packages']],'scope':'Read-only full compressed-archive hash plus every extracted member proof preceded the completed retirement; this publication reuses those retained receipts, not new archive/payload reads.'}))
 put('analysis.json',enc({'status':'retirement-passed','filesRemoved':final['removedFiles'],'elapsedSeconds':final['elapsedSeconds'],'tmpFreeBefore':final['freeBefore']['/tmp'],'tmpFreeAfter':final['freeAfter']['/tmp'],'tmpDelta':final['freeAfter']['/tmp']-final['freeBefore']['/tmp'],'rootFreeBefore':final['freeBefore']['/workspace'],'rootFreeAfter':final['freeAfter']['/workspace'],'rootDelta':final['freeAfter']['/workspace']-final['freeBefore']['/workspace'],'rootCostScope':'The operation emitted retained member/process journals; root-free delta is a sequential observation, not an exclusive allocation attribution.','restoration':'Source prepared and syntax checked; no restoration execution or disposable restore control result exists.','protectedCurrentPackages':['b94ad37e','29931afc','5f4a966f'],'protectionProof':'Operation allowlist excludes these trees; final receipt verifies their root metadata only, not a new full byte census.','archives':'All six retained original archives and aliases unchanged according to operation checks; package payloads are intentionally absent.','limits':['No product memory/admission or causal responsiveness success follows from these storage/memory observations.','No fixture, ledger, source, module/model or Docker mutation.','No global idle or Windows claim.','Prepared failure receipts are preserved; no new control suite or runtime was executed for publication.']}))
 for n in ['README.md','REPRODUCE.md']:
  copy(B/(n.replace('.md','.prepared.md')),n)
 copy(B/'verify-publication.py','verify-publication.py');copy(Path(__file__),'assemble.py')
 put('representations.json',enc({'members':representations,'scope':'Only the six generated member metadata manifests are losslessly compressed. Original/stored hashes are distinct. Other copied receipt/source bytes are exact.'}))
 put('original-bindings.json',enc({'originals':originals,'scope':'Exact small retained originals. Generated package/tool paths are provenance; private fixture/state/configuration paths and argv are not imported.','excluded':['Large process snapshots','All package/runtime binary archives and payloads','Private fixture state and ledgers','Private configuration, authentication and raw controller logs']}))
 for r in originals:assert md(Path(r['source']).read_bytes())=={k:r[k] for k in ('sha256','bytes')}
 put('original-immutability.json',enc({'smallInputsChecked':len(originals),'unchanged':True,'manifestInputsChecked':len(representations),'scope':'Read-only retained small evidence and generated member-manifest inputs. No current package/archive/fixture scan.'}))
 files={str(p.relative_to(O)):md(p.read_bytes()) for p in sorted(O.rglob('*')) if p.is_file()}
 put('publication.json',enc({'scope':'Exact generated old-package retirement, not product/runtime acceptance.','files':files}))
 put('SHA256SUMS',(''.join(r['sha256']+'  '+n+'\n' for n,r in files.items())+md((O/'publication.json').read_bytes())['sha256']+'  publication.json\n').encode())
 tree=B/'tree/docs/luci/evidence';dest=tree/'old-extracted-package-retirement-20261009';dest.mkdir(parents=True);copyplan=[]
 for p in sorted(O.rglob('*')):
  if p.is_file():
   q=dest/p.relative_to(O);q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(p.read_bytes());copyplan.append({'source':str(q),'destination':str(q.relative_to(B/'tree')),**md(q.read_bytes())})
 report=B/'2026-10-09-old-extracted-package-retirement.md';q=tree/report.name;q.write_bytes(report.read_bytes());copyplan.append({'source':str(q),'destination':str(q.relative_to(B/'tree')),**md(q.read_bytes())})
 (B/'copy-plan.json').write_bytes(enc({'files':copyplan,'scope':'Only explicit public capsule members and companion; no parent/private directories.'}))
 (B/'overlay-SHA256SUMS').write_text(''.join(r['sha256']+'  '+r['destination']+'\n' for r in copyplan))
 result={'members':len(files),'files':len(copyplan),'bytes':sum(r['bytes'] for r in copyplan),'originals':len(originals),'compressedManifests':len(representations),'publicationSHA256':md((O/'publication.json').read_bytes())['sha256'],'copyPlanSHA256':md((B/'copy-plan.json').read_bytes())['sha256']}
 (B/'assembly-result.json').write_bytes(enc(result));print(json.dumps(result,indent=2))
if __name__=='__main__':main()
