from pathlib import Path
import shutil,json,hashlib,subprocess
HERE=Path('/workspace/.luci-tools/cloud032-settled-evaluate-publication-v1');assert not HERE.exists();HERE.mkdir()
PREP=Path('/workspace/.luci-tools/cloud032-settled-evaluate-control-v1/capsule-launch-guard-v2');ANALYSIS=Path('/workspace/.luci-tools/cloud032-settled-evaluate-analysis-v1/capsule');sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
assert sha(PREP/'manifest.json')=='d6423f5d170f03169305fcedd27464ce3012563352cce52216cc54dc197dd628';assert sha(ANALYSIS/'manifest.json')=='c3152017ef2064ccc5ca2642aff50c78d4ef356e27c6328114620f4a9369f921'
for root,dest in [(PREP,HERE/'preparation'),(ANALYSIS,HERE/'analysis')]:
 for p in root.rglob('*'):
  n=p.relative_to(root)
  if not p.is_file() or str(n) in ['manifest.json','SHA256SUMS'] or n.parts[:2]==('dependencies','snapshot'):continue
  out=dest/n;out.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(p,out)
licenses=[]
for p in (PREP/'dependencies/snapshot').rglob('*'):
 if p.is_file() and (any(t in p.name.lower() for t in ['license','notice','copying']) or p.name=='package.json'):
  n=p.relative_to(PREP/'dependencies/snapshot');out=HERE/'dependency-metadata'/n;out.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(p,out);licenses.append({'path':str(out.relative_to(HERE)),'capturedPath':str(n),'sha256':sha(p),'bytes':p.stat().st_size})
commit='4d6595072888bfa06fed0070d9c63b4cc1e6d1a9';lock=subprocess.check_output(['git','show',commit+':desktop/package-lock.json'],cwd='/workspace/luci',timeout=10);parsed=json.loads(lock);pins={n:parsed['packages']['node_modules/'+n] for n in ['playwright','playwright-core']}
assert all(v['version']=='1.63.0' and v['integrity'].startswith('sha512-') for v in pins.values())
(HERE/'dependency-pins.json').write_text(json.dumps({'schema':1,'sourceCommit':commit,'sourcePath':'desktop/package-lock.json','sourceBlobSHA256':hashlib.sha256(lock).hexdigest(),'packages':pins,'licencesAndPackageMetadata':licenses,'npmArchiveReproductionExecuted':False},indent=2)+'\n')
(HERE/'external-references.json').write_text(json.dumps({'schema':1,'scope':'External historical references only; these capsules and omitted dependency sources are not members of this publication. Publication checksums cover only files actually present here.','references':[{'kind':'complete-preparation-capsule','externalPath':str(PREP),'manifestSHA256':sha(PREP/'manifest.json'),'payloadMembers':260,'payloadBytes':19467400},{'kind':'complete-analysis-capsule','externalPath':str(ANALYSIS),'manifestSHA256':sha(ANALYSIS/'manifest.json'),'payloadMembers':40,'payloadBytes':608188}],'dependencySnapshot':{'capturedAndVerifiedLocally':True,'ordinaryFiles':176,'bytes':18547881,'directories':39,'inventorySHA256':'c49a27bd78518d76caa1ba4d49414cf3034050e074e7be6301b6c341f3caf863','originalManifestSHA256':'377518398482124b8f74c7619b52ca2d21c023fc851b377ab9bb2cb3eecec785','republished':'Exact inventory, metadata and12 licence/notice files; executable library source omitted.','modeMeaning':'Recorded installed modes are600/700 rather than assuming npm tar modes; reconstruction applies exact recorded file modes after verifying bytes.'}},indent=2)+'\n')
print(str(HERE))
