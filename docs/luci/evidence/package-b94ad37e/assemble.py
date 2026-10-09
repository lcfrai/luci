#!/usr/bin/env python3
"""Publish small, exact package receipts only; no build/archive/image execution."""
from pathlib import Path
import hashlib,json,os,subprocess

B=Path(__file__).parent
O=B/'capsule'
C='b94ad37e53181e360ac8910ee980570de622728c'
OLD='29931afc5b31ae9b9bd7b8973a7e3fdb840609a3'
P=Path('/workspace/luci/artifacts/quality-swarm/candidate-b94ad37e/package')
os.umask(0o077)
assert not O.exists()
O.mkdir()
originals=[]

def meta(b): return {'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)}
def enc(x): return (json.dumps(x,indent=2)+'\n').encode()
def put(n,b):
 p=O/n
 p.parent.mkdir(parents=True,exist_ok=True)
 assert not p.exists()
 p.write_bytes(b)
def copy(p,n):
 p=Path(p)
 assert p.is_file() and not p.is_symlink() and p.stat().st_size<4*1024*1024
 b=p.read_bytes()
 put(n,b)
 originals.append({'source':str(p),'publishedPath':n,**meta(b)})
 assert p.read_bytes()==b

r=json.loads((P/'build-result.json').read_bytes())
q=json.loads((P/'package-provenance.json').read_bytes())
v=json.loads((P/'package-integrity.json').read_bytes())
preserved=json.loads((P/'previous-release-preservation.json').read_bytes())
assert r['sourceCommit']==q['sourceCommit']==v['sourceCommit']==C
assert r['exitCode']==0 and r['cleanBefore'] and r['cleanAfter'] and r['sameHead'] and not r['timedOut']
assert not q['sourceDirty'] and not v['sourceDirty']
assert v['allMembersMatched'] and v['verifiedExtractedFiles']==963 and v['licenseComponents']==347 and v['missingLicenses']==0
assert q['subjects']==v['subjects']
assert preserved['oldSource']==OLD and preserved['newSource']==C and preserved['status']=='preserved-verified' and preserved['originalProvenanceUnchanged']

for n in ['build-result.json','build-invocation.json','build.log','previous-release-preservation.json','package-integrity.json','package-provenance.json']:
 copy(P/n,'receipts/'+n)
copy('/workspace/.luci-tools/b94ad37e-package-preflight-refusal.json','receipts/first-preflight-refusal.json')
copy('/workspace/.luci-tools/c070-b94ad37e-source-parity.json','receipts/controlled-source-and-runtime-parity.json')

d=Path('/workspace/.luci-tools/build-package-next-v1')
m=json.loads((d/'manifest.json').read_bytes())
for row in m['files']:
 raw=(d/row['path']).read_bytes()
 assert meta(raw)=={k:row[k] for k in ('sha256','bytes')}
 copy(d/row['path'],'helpers/build-package-next-v1/'+row['path'])
copy(d/'manifest.json','helpers/build-package-next-v1/manifest.json')
assert meta((d/'build-package.py').read_bytes())['sha256']==r['helperSHA256']
copy('/workspace/.luci-tools/verify-local-package.py','helpers/verify-local-package.py')
for n in ['release-manifest.json','SHA256SUMS']:
 copy(Path('/workspace/luci/artifacts/releases/0.5.0')/n,'release/'+n)

put('execution-environment-supplement.json',enc({
 'sourceCommit':C,
 'environment':{'GOCACHE':'/workspace/.luci-tools/c070-go-build-cache','GOTMPDIR':'/workspace/.t'},
 'provenance':'Root supplied these exact execution-environment settings for this successful build. They supplement the original wrapper receipt, which captures only GOMAXPROCS and GOFLAGS. No retrospective process-environment read was performed by this publisher.',
 'originalWrapperMetadataUnchanged':True,
 'scope':'Build cache/temp placement only. This is not a compiled dependency inventory or performance comparison.'
}))

inputs=[]
for n in ['scripts/build-release.sh','scripts/package.py','scripts/stage-release-docs.py','scripts/collect-licenses.py','scripts/launch-unix.sh']:
 raw=subprocess.check_output(['git','-C','/workspace/luci','show',C+':'+n],timeout=30)
 assert raw==(Path('/workspace/luci')/n).read_bytes()
 put('selected-source/'+n,raw)
 inputs.append({'path':n,**meta(raw)})
put('source-bindings.json',enc({'commit':C,'inputs':inputs,'scope':'Five selected packaging sources match immutable Git and the current files. This is not a full compiler-input or external dependency inventory.'}))
put('scope.json',enc({
 'sourceCommit':C,'buildSeconds':r['elapsedSeconds'],'packageFiles':v['verifiedExtractedFiles'],
 'licenses':v['licenseComponents'],'subjects':q['subjects'],'binarySHA256':q['binarySHA256'],'webIndexSHA256':q['webIndexSHA256'],
 'runtimeImage':'sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8',
 'runtimeParityEvidence':'receipts/controlled-source-and-runtime-parity.json',
 'knownLimits':[
  'Original binary archives and extracted package payloads remain publisher-held; only their original verification receipts are included.',
  'The publication does not repeat the build or independently rehash/download package binaries.',
  'The parity receipt reports 106 matching runtime inputs and separately controlled backend source evidence; no new image build or installed runtime pass occurs here.',
  'The ten wrapper controls are retained earlier disposable controls, not newly rerun or cumulative build results.',
  'Linux package verification does not establish Windows/native, live-provider, recovery workflow, responsiveness or whole-computer acceptance.',
  'No storage reclamation, runtime admission snapshot or image/fixture/ledger mutation occurred during this publication.'
 ]
}))
for n in ['README.md','REPRODUCE.md']:
 copy(B/(n.replace('.md','.prepared.md')),n)
copy(Path(__file__),'assemble.py')
put('original-bindings.json',enc({'originals':originals,'representation':'Exact retained small source/report bytes. Supplementary environment metadata is separately labelled; original receipts are not rewritten.','excluded':['Binary/package/image archives and extracted payloads','Private fixture state and ledgers','Credentials, private configuration and raw controller logs'],'historicalPaths':'Task-local historical artifact/helper paths remain as provenance; they do not imply availability in a fresh clone.'}))
for row in originals:
 assert meta(Path(row['source']).read_bytes())=={k:row[k] for k in ('sha256','bytes')}
put('original-immutability.json',enc({'checked':len(originals),'allUnchanged':True,'scope':'Small publication inputs only, without a new large archive/image/fixture verification.'}))
files={str(p.relative_to(O)):meta(p.read_bytes()) for p in sorted(O.rglob('*')) if p.is_file()}
put('publication.json',enc({'sourceCommit':C,'files':files}))
put('SHA256SUMS',(''.join(x['sha256']+'  '+n+'\n' for n,x in files.items())+meta((O/'publication.json').read_bytes())['sha256']+'  publication.json\n').encode())
tree=B/'tree'/'docs/luci/evidence'
target=tree/'package-b94ad37e'
target.mkdir(parents=True)
plan=[]
for p in sorted(O.rglob('*')):
 if not p.is_file():continue
 rel=p.relative_to(O);t=target/rel;t.parent.mkdir(parents=True,exist_ok=True);t.write_bytes(p.read_bytes())
 plan.append({'source':str(t),'destination':str(t.relative_to(B/'tree')),**meta(t.read_bytes())})
companion=B/'2026-10-09-package-b94ad37e.md'
t=tree/companion.name;t.write_bytes(companion.read_bytes())
plan.append({'source':str(t),'destination':str(t.relative_to(B/'tree')),**meta(t.read_bytes())})
(B/'copy-plan.json').write_bytes(enc({'scope':'Only listed public small assets; no repository writes.','files':plan}))
(B/'overlay-SHA256SUMS').write_text(''.join(x['sha256']+'  '+x['destination']+'\n' for x in plan))
result={'members':len(files),'originals':len(originals),'files':len(plan),'bytes':sum(x['bytes'] for x in plan),'publicationSHA256':meta((O/'publication.json').read_bytes())['sha256'],'copyPlanSHA256':meta((B/'copy-plan.json').read_bytes())['sha256']}
(B/'assembly-result.json').write_bytes(enc(result))
print(json.dumps(result,indent=2))
