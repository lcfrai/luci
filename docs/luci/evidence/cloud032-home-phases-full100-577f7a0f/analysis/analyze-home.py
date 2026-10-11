#!/usr/bin/env python3
"""Exact all100 numeric Home-phase projection; offline and raw-hash fenced."""
from pathlib import Path
import argparse,hashlib,json,math,collections
B=Path(__file__).resolve().parent;p=argparse.ArgumentParser();p.add_argument('--result',type=Path,required=True);a=p.parse_args();raw=a.result.read_bytes();sha=lambda b:hashlib.sha256(b).hexdigest();expected='ea7a1722ba0e284ed9e123893b792305b8494579c834b06f842f31e924da4b99';assert sha(raw)==expected;v=json.loads(raw)
kinds={1:'handler',2:'handler_to_home_commit',3:'navigation_state',4:'scoped_navigation_write',5:'navigation_encode',6:'scope_scan',7:'navigation_set_item',8:'cockpit_serialize_write',9:'cockpit_partialize',10:'storage_key_count',11:'other_navigation_scope_count',12:'app_shell_render_entry_to_layout_effect'}
windows=[];rows=[];previous=0;homes=0
for index,w in enumerate(v['browserEventWindows']):
 h=w.get('homePhase')
 if h is None:continue
 assert set(h)=={'version','timeOrigin','homes','observed','dropped','pending','active','rows'}
 assert all(isinstance(h[k],(int,float)) and not isinstance(h[k],bool) and math.isfinite(h[k]) for k in h if k!='rows');assert h['version']==1 and homes<=h['homes']<=100;homes=h['homes'];assert h['dropped']==max(0,homes-16)
 assert len(h['rows'])<=2048
 for r in h['rows']:
  assert set(r)<=set(['sequence','home','kind','start','end','ok','count']);assert all(isinstance(x,(int,float)) and not isinstance(x,bool) and math.isfinite(x) for x in r.values());assert r['sequence']==previous+1;previous=r['sequence'];assert r['kind'] in kinds and 1<=r['home']<=100;assert r['ok']==1 and r['end']>=r['start'];rows.append({'windowIndex':index,'timeOrigin':h['timeOrigin'],**r,'durationMs':r['end']-r['start']})
 assert previous==h['observed'];windows.append({'windowIndex':index,**{k:h[k] for k in h if k!='rows'},'retainedRows':len(h['rows']),'expectedCompletedStartEntryRetirements':max(0,homes-16)})
assert len(rows)==1200 and previous==1200 and homes==100;assert windows[-1]['dropped']==84 and windows[-1]['pending']==0 and windows[-1]['active']==0
for home in range(1,101):
 for kind in kinds:assert sum(r['home']==home and r['kind']==kind for r in rows)==1
switches=json.loads((B/'switches.all100.json').read_text());joined=[]
for s in switches:
 home=s['index']//4+1;phase=[r for r in rows if r['home']==home];joined.append({'home':home,'originalActionIndex':s['index'],'switchElapsedMs':s['elapsedMs'],'over300ms':s['elapsedMs']>300,'largestNodeSpan':s['largestRecordedNodeSpan'],'nodeSpansMs':s['nodeSpans'],'homePhaseMs':{kinds[r['kind']]:r['durationMs'] for r in phase if r['kind'] not in [10,11]},'storageCounts':{kinds[r['kind']]:r['count'] for r in phase if r['kind'] in [10,11]}})
def q(values):
 values=sorted(values);return {'count':len(values),**{k:values[math.ceil(len(values)*p)-1] for k,p in [('p50Ms',.5),('p95Ms',.95),('p99Ms',.99),('maxMs',1)]}}
summary={name:q([r['durationMs'] for r in rows if r['kind']==kind]) for kind,name in kinds.items() if kind not in [10,11]}
projection={'rawPrivateSHA256':expected,'diagnosticOnly':True,'performanceAcceptance':False,'sourceCommit':v['sourceCommit'],'binarySHA256':v['binarySHA256'],'packagedWebIndexSHA256':v['packagedWebIndexSHA256'],'instrumentedWebIndexSHA256':v['instrumentedWebIndexSHA256'],'diagnosticWebBindingSHA256':v['diagnosticWebBindingSHA256'],'kinds':kinds,'windows':windows,'homeRows':rows,'all100JoinedSwitches':joined,'phaseSummary':summary,'complete':True,'phaseRows':1200,'homes':100,'exactCompletedStartEntryRetirements':84,'additionalDropsOrOverlaps':0,'limits':['Raw dropped counters retained unchanged; original16-entry completed-start retirement accounting is independently checked per snapshot.','All100 Homes have all12 phase kinds, contiguous1200 sequence rows and successful operations.','Home ordinal joins sequential switch ordinal; all400 original actions retained in separate projection.','Nested phase durations are not additive; Node/browser clocks are not subtracted.','No cross-run gain or instrumentation-overhead subtraction claim.','Destination DefinitionWorkspace commit and first locator match were not instrumented.']}
(B/'home-phases.all100.projection.json').write_text(json.dumps(projection,indent=2)+'\n');assert a.result.read_bytes()==raw
(B/'home-completeness.json').write_text(json.dumps({'rawPrivateSHA256Before':expected,'rawPrivateSHA256After':sha(a.result.read_bytes()),'scriptSHA256':sha(Path(__file__).read_bytes()),'homes':100,'phaseRows':1200,'eachPhaseKindCount':dict(collections.Counter(r['kind'] for r in rows)),'completedStartEntryRetirements':84,'additionalDropsOrOverlaps':0,'pending':0,'active':0,'rawUnchanged':True,'offlineOnly':True},indent=2)+'\n')
print(json.dumps({'phaseSummary':summary,'slowJoins':[r for r in joined if r['over300ms']],'storageCounts':sorted({tuple(sorted(r['storageCounts'].items())) for r in joined})},indent=2))
