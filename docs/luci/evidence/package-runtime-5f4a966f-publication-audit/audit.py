#!/usr/bin/env python3
from pathlib import Path
import hashlib,json,datetime
P=Path('/workspace/.luci-tools/publish-package-runtime-5f4/capsule')
checks=[]
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def check(name,ok):
 checks.append({'check':name,'passed':bool(ok)})
 if not ok:raise AssertionError(name)
m=json.loads((P/'publication.json').read_text());listed=set()
for x in m['members']:
 p=P/x['path'];check('member:'+x['path'],p.is_file()and p.stat().st_size==x['bytes']and digest(p)==x['sha256']);listed.add(x['path'])
for line in(P/'SHA256SUMS').read_text().splitlines():
 value,name=line.split('  ',1);check('sum:'+name,digest(P/name)==value)
check('exact-publication-inventory',set(str(p.relative_to(P))for p in P.rglob('*')if p.is_file())==listed|{'publication.json','SHA256SUMS'})
o=json.loads((P/'original-bindings.json').read_text())
for x in o['originals']:
 p=Path(x['source']);check('original:'+x['publicationPath'],p.is_file()and p.stat().st_size==x['bytes']and digest(p)==x['sha256']and digest(P/x['publicationPath'])==x['sha256'])
runtime=json.loads((P/'runtime/export-verified-receipt.original.json').read_text());build=json.loads((P/'runtime/build-receipt.original.json').read_text());bindings=json.loads((P/'runtime/source-bindings.json').read_text())
check('exact106runtime-inputs',len(runtime['sourcesBefore'])==len(runtime['sourcesAfter'])==bindings['inputCount']==106 and runtime['sourcesBefore']==runtime['sourcesAfter']==build['sourcesAfter']=={x['path']:x['sha256']for x in bindings['inputs']})
check('newruntime-source-image',runtime['sourceCommit']==m['productCommit']=='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'and runtime['candidateImage']=='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8')
check('phase-status-separation',build['status']=='built-config-verified-export-and-runtime-pending'and runtime['status']=='export-verified-runtime-pending'and runtime['exportVerification']['status']=='passed')
check('verified-binary-payload',runtime['inputFiles']=={Path(k).name:v for k,v in runtime['exportVerification']['verifiedFiles'].items()})
check('bothmodulepins-and-sandboxvendor',all('github.com/coder/websocket v1.8.15'in(P/'runtime/module-sources'/n).read_text()for n in ['sandboxd/go.mod','control-plane/go.mod','sandboxd/vendor/modules.txt']))
ops=list((P/'operations/tool-moves').glob('operation-*/009-finished.json'))
check('fourcompleted-move-return-journals',len(ops)==4 and all(json.loads(x.read_text())['passed']is True for x in ops))
ret=json.loads((P/'operations/cloud054-old-cache-retirement-attempt2/005-final.json').read_text());check('cache-final-passed',ret.get('status')=='passed')
check('only-selected-public-cache-row-removed',{k:v for k,v in ret['cacheBefore'].items()if k!='jxury1xmsdvxr24fm76wfizm5'}==ret['cacheAfter'])
check('protected-all-image-inventory-preserved',ret['imagesBefore']==ret['imagesAfter'])
check('physical-free-observation-operands',ret['freeBefore']['/workspace']==3408728064 and ret['freeAfter']['/workspace']==5776240640 and ret['archiveUnchanged']is True)
check('no-binary-archive-copied',not any(p.name.endswith(('.tar.gz','.exe','.zip','.sqlite'))for p in P.rglob('*')if p.is_file()))
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'checks':len(checks),'failed':0,'memberCount':len(m['members']),'originalCount':len(o['originals']),'publicationSHA256':digest(P/'publication.json'),'sumsSHA256':digest(P/'SHA256SUMS'),'scope':'Independent small-file consistency and exact original parity; no package/image binary read or runtime rerun. Original private archive/inventory assertions remain publisher evidence.','results':checks}
Q=P.parent/'audit-final.json';Q.write_text(json.dumps(out,indent=2)+'\n');print(json.dumps({k:v for k,v in out.items()if k!='results'},indent=2));print('auditSHA256',digest(Q))
