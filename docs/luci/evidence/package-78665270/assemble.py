#!/usr/bin/env python3
"""Publish exact small package receipts externally; no build/runtime/repo writes."""
from pathlib import Path
import ast, hashlib, json, os, re, subprocess

B=Path(__file__).resolve().parent
O=B/'capsule'
REPO=Path('/workspace/luci')
C='786652704fa8bebd1ba1af41cfa1d414d7def100'
OLD='b94ad37e53181e360ac8910ee980570de622728c'
P=REPO/'artifacts/quality-swarm/candidate-78665270/package'
os.umask(0o077)
assert not O.exists(), 'Use an unused external destination; preserve existing capsule'
O.mkdir()
originals=[]
meta=lambda raw:{'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)}
enc=lambda obj:(json.dumps(obj,indent=2)+'\n').encode()
def put(name,raw):
    p=O/name;p.parent.mkdir(parents=True,exist_ok=True);assert not p.exists();p.write_bytes(raw)
def copy(source,name):
    p=Path(source);assert p.is_file() and not p.is_symlink() and p.stat().st_size<4*1024*1024
    raw=p.read_bytes();put(name,raw);originals.append({'source':str(p),'publishedPath':name,**meta(raw)})
    assert p.read_bytes()==raw
def blob(commit,path):
    return subprocess.check_output(['git','-C',str(REPO),'show',commit+':'+path],timeout=20)

r=json.loads((P/'build-result.json').read_bytes())
q=json.loads((P/'package-provenance.json').read_bytes())
v=json.loads((P/'package-integrity.json').read_bytes())
old=json.loads((P/'previous-release-preservation.json').read_bytes())
assert r['sourceCommit']==q['sourceCommit']==v['sourceCommit']==C
assert r['exitCode']==0 and r['cleanBefore'] and r['cleanAfter'] and r['sameHead'] and not r['timedOut']
assert not q['sourceDirty'] and not v['sourceDirty']
assert v['allMembersMatched'] and v['verifiedExtractedFiles']==964 and v['licenseComponents']==347 and v['missingLicenses']==0
assert q['subjects']==v['subjects'] and old['oldSource']==OLD and old['newSource']==C and old['status']=='preserved-verified' and old['originalProvenanceUnchanged']
log=(P/'build.log').read_text()
guide_lines=[line for line in log.splitlines() if line.startswith("{'compact': True")]
assert len(guide_lines)==1
guide=ast.literal_eval(guide_lines[0]);assert guide['sourceCommit']==C and guide['files']==53 and guide['rewrittenLinks']==338
for name in ['build-result.json','build-invocation.json','build.log','previous-release-preservation.json','package-integrity.json','package-provenance.json']:
    copy(P/name,'receipts/'+name)

image_path=Path('/workspace/.luci-tools/c073-78665270-image-source-parity.json')
backend_path=Path('/workspace/.luci-tools/c070-78665270-source-parity.json')
assert meta(image_path.read_bytes())['sha256']=='85928dc798516da01d6b6373ab85bf39f222689c88643805ee79aa785f90d985'
assert meta(backend_path.read_bytes())['sha256']=='9db1fb073cc4e6edb6096da908d2db3baab4cb3574d150642929b87c8cf3e569'
image=json.loads(image_path.read_bytes());backend=json.loads(backend_path.read_bytes())
assert image['selectedProductSource']==C and image['passed'] and image['imageInputCount']==image['matchingImageInputCount']==106
assert backend['sourceCommit']==C and backend['compiledRepositorySourceCount']==944 and backend['allCapturedRepositoryInputsMatch'] and backend['capturedRepositoryPathSetMatches']
copy(image_path,'receipts/runtime-input-parity.json');copy(backend_path,'receipts/c070-controlled-source-parity.json')
image_provenance=Path(image['originalImageProvenance']['path'])
assert meta(image_provenance.read_bytes())['sha256']==image['originalImageProvenance']['sha256']
copy(image_provenance,'receipts/original-runtime-build-provenance.json')

c075_path=Path('/workspace/.luci-tools/c075-integrated-parity.json')
c075=json.loads(c075_path.read_bytes());assert c075['unchangedCapturedInputs']==511
copy(c075_path,'receipts/c075-original-integrated-parity.json')
bindings=[]
for row in c075['changedFinalFiles']:
    raw=blob(C,row['path']);assert meta(raw)=={k:row[k] for k in ['sha256','bytes']}
    bindings.append({'path':row['path'],**meta(raw)})
changed=subprocess.check_output(['git','-C',str(REPO),'diff','--name-only',c075['sourceHead'],C],timeout=20).decode().splitlines()
allowed={x['path'] for x in bindings}
assert all(x in allowed or x.startswith('docs/luci/') for x in changed)
put('receipts/c075-immutable-commit-binding.json',enc({'commit':C,'originalIntegrationReceiptSHA256':meta(c075_path.read_bytes())['sha256'],'originalIntegrationHead':c075['sourceHead'],'exactFinalFiles':bindings,'changedPathsFromOriginalIntegrationHead':changed,'scope':'Four reviewed final files match immutable786. Other changes since the original integration HEAD are documentation only. Original511-input parity is carried from its original receipt, not a new test run.'}))

helper=Path('/workspace/.luci-tools/build-package-next-v1')
manifest=json.loads((helper/'manifest.json').read_bytes())
for row in manifest['files']:
    raw=(helper/row['path']).read_bytes();assert meta(raw)=={k:row[k] for k in ['sha256','bytes']}
    copy(helper/row['path'],'helpers/build-package-next-v1/'+row['path'])
copy(helper/'manifest.json','helpers/build-package-next-v1/manifest.json')
assert meta((helper/'build-package.py').read_bytes())['sha256']==r['helperSHA256']
copy('/workspace/.luci-tools/verify-local-package.py','helpers/verify-local-package.py')
for name in ['release-manifest.json','SHA256SUMS']:
    copy(REPO/'artifacts/releases/0.5.0'/name,'release/'+name)
release=json.loads((O/'release/release-manifest.json').read_bytes());assert release['sourceCommit']==C and not release['sourceDirty'] and not release['nativeRequested']
assert meta((O/'release/release-manifest.json').read_bytes())['sha256']==next(s['sha256'] for s in q['subjects'] if s['name'].endswith('release-manifest.json'))

inputs=[]
for name in ['scripts/build-release.sh','scripts/package.py','scripts/stage-release-docs.py','scripts/collect-licenses.py','scripts/launch-unix.sh']:
    raw=blob(C,name);assert raw==(REPO/name).read_bytes();put('selected-source/'+name,raw);inputs.append({'path':name,**meta(raw)})
put('source-bindings.json',enc({'commit':C,'inputs':inputs,'scope':'Five selected packaging scripts match immutable source/current bytes; not a complete compiler/external dependency inventory.'}))
windows=[]
for name in ['scripts/start-local-acceptance.ps1','scripts/build-release.ps1','scripts/test-installed-windows.ps1','docs/luci/LOCAL-WINDOWS-ACCEPTANCE.md']:
    raw=blob(C,name);assert raw==blob(OLD,name);windows.append({'path':name,**meta(raw),'unchangedFrom':OLD})
put('windows-source-parity.json',enc({'commit':C,'inputs':windows,'scope':'Read-only immutable source equality only. Commands unchanged; no Windows/native/setup execution or configuration publication.'}))
put('scope.json',enc({'sourceCommit':C,'previousSource':OLD,'buildSeconds':r['elapsedSeconds'],'packageFiles':964,'licenses':347,'missingLicenses':0,'guideBuildProjection':guide,'subjects':q['subjects'],'binarySHA256':q['binarySHA256'],'webIndexSHA256':q['webIndexSHA256'],'runtimeImage':image['carriedImageID'],'runtimeInputs':106,'c070CapturedInputs':944,'installed786Acceptance':'pending; separate runtime evidence required','environmentScope':'Only original wrapper-captured GOMAXPROCS/GOFLAGS. Build process exited before attempted additional allowlisted process-environment observation; no cache/temp environment inferred.','limitations':['Original archives/extracted payloads remain publisher-held and were not rehashed by this publication.','Original build/licence/guide verification is carried verbatim; no new guide audit, build, runtime or native execution.','Historical wrapper controls, C070 checks and C075 tests retain original revisions/scopes; no cumulative test total.','No source/runtime image rebuild, admission relaxation, latency change, storage reclamation or fixture/ledger mutation.']}))
for name in ['README.md','REPRODUCE.md']:
    copy(B/name.replace('.md','.prepared.md'),name)
copy(Path(__file__),'assemble.py')
put('.gitattributes',b'* -text\n');put('.gitignore',b'!*.log\n')
put('original-bindings.json',enc({'originals':originals,'representation':'Exact small source/report bytes. No original receipt or build log rewritten.','excluded':['Binary/package/runtime archives and extracted payloads','Private fixture state/ledgers/bootstrap data','Raw controller logs/credentials'],'historicalPaths':'Historical paths are provenance, not a guarantee of fresh-clone availability.'}))
for row in originals:assert meta(Path(row['source']).read_bytes())=={k:row[k] for k in ['sha256','bytes']}
put('original-immutability.json',enc({'checked':len(originals),'allUnchanged':True,'scope':'Small selected original bytes only; no large archive/image/fixture verification.'}))
files={str(p.relative_to(O)):meta(p.read_bytes()) for p in sorted(O.rglob('*')) if p.is_file()}
# Build logs/source contain configuration identifiers; captured credential values
# are excluded. Refuse token-shaped literal headers/private-key material.
findings=[]
for name in files:
    raw=(O/name).read_bytes()
    if re.search(rb'Bearer [A-Za-z0-9_./+~-]{24,}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',raw):findings.append(name)
assert not findings,findings
put('privacy-review.json',enc({'scope':'Selected source and original small public build/parity receipts only. No environment dump, private fixture, credential/config values or controller log. Finite pattern scan supplements selection and is not universal secret-absence proof.','scannedFiles':len(files),'literalCredentialPatternMatches':findings}))
files['privacy-review.json']=meta((O/'privacy-review.json').read_bytes())
put('publication.json',enc({'sourceCommit':C,'files':files}))
put('SHA256SUMS',(''.join(x['sha256']+'  '+n+'\n' for n,x in files.items())+meta((O/'publication.json').read_bytes())['sha256']+'  publication.json\n').encode())
for line in (O/'SHA256SUMS').read_text().splitlines():
    digest,name=line.split('  ',1);assert meta((O/name).read_bytes())['sha256']==digest

tree=B/'tree';assert not tree.exists()
plan=[]
def tree_copy(source,relative):
    target=tree/relative;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(source.read_bytes())
    plan.append({'source':str(target),'destination':relative,**meta(target.read_bytes())})
for p in sorted(O.rglob('*')):
    if p.is_file():tree_copy(p,'docs/luci/evidence/package-78665270/'+str(p.relative_to(O)))
checkpoint=B/'TEST-CHECKPOINT-78665270.prepared.md';assert checkpoint.stat().st_size<=5120
tree_copy(checkpoint,'docs/luci/TEST-CHECKPOINT-78665270.md')
tree_copy(B/'2026-10-09-package-78665270.prepared.md','docs/luci/evidence/2026-10-09-package-78665270.md')
(B/'copy-plan.json').write_bytes(enc({'scope':'Only new package capsule/index/checkpoint paths. No repository writes performed; root publishes after runtime closure.','files':plan}))
(B/'overlay-SHA256SUMS').write_text(''.join(x['sha256']+'  '+x['destination']+'\n' for x in plan))
result={'capsuleFiles':len(files)+2,'originals':len(originals),'copyFiles':len(plan),'copyBytes':sum(x['bytes'] for x in plan),'publicationSHA256':meta((O/'publication.json').read_bytes())['sha256'],'copyPlanSHA256':meta((B/'copy-plan.json').read_bytes())['sha256'],'checkpointBytes':checkpoint.stat().st_size}
(B/'assembly-result.json').write_bytes(enc(result));print(json.dumps(result,indent=2))
