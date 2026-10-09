#!/usr/bin/env python3
from pathlib import Path
import hashlib,json,os,subprocess,re
B=Path(__file__).parent;O=B/'capsule';assert not O.exists();os.umask(0o077);O.mkdir();C='29931afc5b31ae9b9bd7b8973a7e3fdb840609a3';originals=[]
def md(b):return {'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)}
def put(n,b):p=O/n;p.parent.mkdir(parents=True,exist_ok=True);assert not p.exists();p.write_bytes(b)
def enc(v):return(json.dumps(v,indent=2)+'\n').encode()
def copy(p,n):
 p=Path(p);assert p.is_file()and not p.is_symlink()and p.stat().st_size<4*1024*1024;b=p.read_bytes();put(n,b);originals.append({'source':str(p),'publishedPath':n,**md(b)});assert p.read_bytes()==b
P=Path('/workspace/luci/artifacts/quality-swarm/candidate-29931afc/package');r=json.loads((P/'build-result.json').read_bytes());v=json.loads((P/'package-integrity.json').read_bytes());q=json.loads((P/'package-provenance.json').read_bytes())
assert r['sourceCommit']==v['sourceCommit']==q['sourceCommit']==C and r['exitCode']==0 and r['cleanBefore']and r['cleanAfter']and r['sameHead']and not r['timedOut']
assert v['allMembersMatched']is True and v['verifiedExtractedFiles']==962 and v['licenseComponents']==347 and v['missingLicenses']==0
for n in ['build-result.json','build-invocation.json','build.log','previous-release-preservation.json','package-integrity.json','package-provenance.json']:copy(P/n,'receipts/'+n)
for name in ['build-package-next-v1','relocate-historical425-v1']:
 d=Path('/workspace/.luci-tools')/name;m=json.loads((d/'manifest.json').read_bytes())
 for row in m['files']:
  p=d/row['path'];assert md(p.read_bytes())=={k:row[k]for k in ['sha256','bytes']};copy(p,'helpers/'+name+'/'+row['path'])
 copy(d/'manifest.json','helpers/'+name+'/manifest.json')
copy('/workspace/.luci-tools/build-package-next-v1/first-candidate-refusal.json','receipts/first-candidate-refusal.json')
copy('/workspace/.luci-tools/post-29931-package-cache-clean.json','storage/post-package-cache-clean.json')
D=Path('/workspace/.luci-tools/historical425-relocation-operation-20261009-root1')
for f in sorted(D.glob('*.json')):copy(f,'storage/older425-operation/'+f.name)
f=json.loads((D/'007-final.json').read_bytes());assert f['passed']and f['sha256']=='c593f37eaa3795358aa1b76dd355828ddaadfcf503b9a5e3a1aea6976c947c91'and f['measuredRootFreeDelta']==1013334016
assert f['freeBefore']=={'/workspace':6627237888,'/tmp':2414612480}and f['freeAfter']=={'/workspace':7640571904,'/tmp':1401249792}
for name in ['runtime-archive-7e1b1f608cfa','runtime-archive-1f845cfd2b45']:copy(Path('/workspace/.luci-tools')/name/'receipt.json','storage/historical-archive-references/'+name+'.json')
for name in ['release-manifest.json','SHA256SUMS']:copy(Path('/workspace/luci/artifacts/releases/0.5.0')/name,'release/'+name)
copy('/workspace/.luci-tools/verify-local-package.py','helpers/verify-local-package.py')
inputs=[]
for name in ['scripts/build-release.sh','scripts/package.py','scripts/stage-release-docs.py','scripts/collect-licenses.py','scripts/launch-unix.sh']:
 raw=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+name],timeout=30);assert raw==(Path('/workspace/luci')/name).read_bytes();put('selected-source/'+name,raw);inputs.append({'path':name,**md(raw)})
put('source-bindings.json',enc({'commit':C,'inputs':inputs,'scope':'Selected tracked packaging inputs match immutable source and current working files; not full compiled-dependency manifest or independent binary/image verification.'}))
put('scope.json',enc({'sourceCommit':C,'buildSeconds':r['elapsedSeconds'],'packageFiles':962,'licenses':347,'portableArchive':q['subjects'][1],'cloudArchive':q['subjects'][0],'binarySHA256':q['binarySHA256'],'webIndexSHA256':q['webIndexSHA256'],'knownLimits':['Original portable/cloud binary archives and extracted payload files not copied or reread by this publication; verifier receipts remain publisher evidence.','No Windows/native/provider/performance or installed terminal workflow acceptance is asserted.','Runtime760 is not rebuilt or exported here; its later archive/save refresh remains separately pending.','Historical425 byte-preserving placement changes original hardlink topology and inode; TMP snapshot retention is not guaranteed.','Current333 protected metadata parity is not a fresh333 byte-hash pass.','Storage observations are sequential; later runtime allocation may change both volumes. Both-filesystem3,905,250,841B export floor is unchanged and not satisfied by reportedTMP1,401,249,792B.']}))
copy(B/'README.prepared.md','README.md');copy(B/'REPRODUCE.prepared.md','REPRODUCE.md');copy(Path(__file__),'assemble.py')
put('original-bindings.json',enc({'originals':originals,'representation':'Exact retained source/report bytes, without private fixture/state/controller logs or archive payloads','largeObjectsNotReadOrCopied':['portable/cloud binary archives','old425/333 runtime archives','extracted package payload','Docker/image state'],'historicalMetadata':'Absolute task-local artifact paths remain in source/receipts as provenance; no raw credential/env dump included.'}))
for x in originals:assert md(Path(x['source']).read_bytes())=={k:x[k]for k in ['sha256','bytes']}
put('original-immutability.json',enc({'checked':len(originals),'allUnchanged':True,'scope':'Small publication input files only; no new archive/image/fixture invariance check.'}))
files={str(p.relative_to(O)):md(p.read_bytes())for p in sorted(O.rglob('*'))if p.is_file()};put('publication.json',enc({'sourceCommit':C,'files':files}));put('SHA256SUMS',(''.join(v['sha256']+'  '+k+'\n'for k,v in files.items())+md((O/'publication.json').read_bytes())['sha256']+'  publication.json\n').encode());print(json.dumps({'members':len(files),'originals':len(originals),'bytes':sum(p.stat().st_size for p in O.rglob('*')if p.is_file()),'publication':md((O/'publication.json').read_bytes())},indent=2))
