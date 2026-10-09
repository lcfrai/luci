#!/usr/bin/env python3
from pathlib import Path
import hashlib,json,datetime
P=Path('/workspace/.luci-tools/publish-cloud054-5f4-attempt8/capsule');checks=[]
def h(b):return hashlib.sha256(b).hexdigest()
def m(b):return {'sha256':h(b),'bytes':len(b)}
def check(n,v):
 checks.append({'check':n,'passed':bool(v)})
 if not v:raise AssertionError(n)
x=json.loads((P/'publication.json').read_bytes())
for name,meta in x['files'].items():check('member:'+name,m((P/name).read_bytes())==meta)
for line in(P/'SHA256SUMS').read_text().splitlines():
 want,name=line.split('  ',1);check('checksum:'+name,h((P/name).read_bytes())==want)
check('exact-inventory',{str(f.relative_to(P))for f in P.rglob('*')if f.is_file()}==set(x['files'])|{'publication.json','SHA256SUMS'})
o=json.loads((P/'original-bindings.json').read_bytes())
private=Path('/workspace/.luci-tools/cloud054-terminal-close/actual-5f4a966f/evidence/attempt8/driver/private-fixture-path').read_text().strip().encode()
check('private-root-binding',h(private)==o['redactionPolicy']['rootSHA256']and len(private)==o['redactionPolicy']['rootBytes'])
for v in o['originals']:
 source=Path(v['source']);raw=source.read_bytes();pub=(P/v['publishedPath']).read_bytes()
 check('original:'+v['publishedPath'],m(raw)==v['original'])
 check('published:'+v['publishedPath'],m(pub)==v['published'])
 check('representation:'+v['publishedPath'],raw.replace(private,o['redactionPolicy']['replacement'].encode())==pub)
check('redacted-all-public-files',all(private not in f.read_bytes()for f in P.rglob('*')if f.is_file()))
r=json.loads((P/'run/driver/result.json').read_bytes());w=json.loads((P/'run/wrapper-result.json').read_bytes());c=json.loads((P/'closure/safe-projection.json').read_bytes());a=json.loads((P/'analysis.json').read_bytes())
check('product-source',r['sourceCommit']==r['helperSourceCommit']==x['productSource']=='5f4a966f6d70264c92ee2176dde912e1b43fc3c7')
check('source-stable',w['sourceBefore']==w['sourceAfter'])
check('original-results-joined',c['driverResultSHA256']==a['originalResult']['sha256']=='d4a8892d6cf0065f7ed87cab9abb48ccc57429360726f4298a0d69eff993145e'and c['wrapperResultSHA256']==a['originalWrapper']['sha256']=='7f1d84180c8f4021337f71d4c41e7125a5c3cffb78b6b631d43197bbc91ee73a')
check('original-failure-retained',r['passed']is False and w['passed']is False and a['failure']['step']=='first-poll-release')
check('all-28-phase-files',len(list((P/'run/driver').glob('phase-*.json')))==28)
for n in ['primaryObservation','finalPassiveObservation']:
 rows=a[n]['wire']['browser']['rows'];row=rows[0]
 check(n+':single-clean-exit-socket',len(rows)==1 and row['error']is False and row['closed']['code']==1000 and row['closed']['wasClean']is True and len(row['exit'])==1 and row['exit'][0]['reason']=={'kind':'exit-status','code':0})
 check(n+':wire-order',row['markers']['first']['sequence']==72 and row['exit'][0]['sequence']==73 and next(z['sequence']for z in row['events']if z['kind']=='close')==74)
 check(n+':running-ui-no-restart',a[n]['ui']['restartCount']==0 and a[n]['ui']['label'].endswith(' — running'))
check('no-restart-mutation',not any(v['path'].endswith('/restart')for v in r['browserMutations']))
check('supported-delete',any(v['method']=='DELETE'and v['path'].endswith('/s-05496235')and v['status']==200 for v in r['apiActions']))
check('empty-final-reservations',all(not r['final']['ledger'][k]for k in ['leases','checks','fences','checkFences']))
check('empty-final-inventories',r['final']['containers']==[]and r['final']['volumes']==''and r['pendingMutations']==[])
check('two-zombies-retained',c['closure']['knownLiveProcessCount']==0 and c['closure']['knownZombieCount']==2 and sorted(v['pid']for v in c['closure']['identities']if v['now'].get('state')=='Z')==[2442558,2442561])
check('excluded-private-artifacts',not any(f.name in ('private-controller.stderr','private-transport-errors.jsonl','private-fixture-path','independent-closure.json')for f in P.rglob('*')if f.is_file()))
check('no-reread-private-sinks',all(v['publishedPath']not in('private-controller.stderr','private-transport-errors.jsonl','independent-closure.json')for v in o['originals']))
check('current-runtime-count',json.loads((P/'runtime-source-bindings.json').read_bytes())['count']==106)
check('controller-classification',len(r['controllerLog']['relayRows'])==1 and r['controllerLog']['relayRows'][0]['flags']['websocketStatus']==1000)
check('private-error-capture-writer-scope',r['privateTransportErrors']['rows']==0 and r['privateTransportErrors']['passed']is True)
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'checks':len(checks),'failed':0,'members':len(x['files']),'originals':len(o['originals']),'publicationSHA256':h((P/'publication.json').read_bytes()),'sumsSHA256':h((P/'SHA256SUMS').read_bytes()),'scope':'Publication bindings, safe projection and retained observation joins; no runtime rerun, private raw log/state/closure read or binary/image verification. Original fixture invariance remains run-owner evidence.','results':checks}
q=P.parent/'audit-final.json';q.write_text(json.dumps(out,indent=2)+'\n');print(json.dumps({k:v for k,v in out.items()if k!='results'},indent=2));print('auditSHA256',h(q.read_bytes()))
