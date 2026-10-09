from pathlib import Path
import json,hashlib,math,statistics,csv,collections
HERE=Path(__file__).resolve().parent;ROOT=Path('/workspace/luci/artifacts/quality-swarm/idle-5f4a966f-navigation-callback-attempt1');RAW=ROOT/'driver/result.json';digest=hashlib.sha256(RAW.read_bytes()).hexdigest();assert digest=='61bd37dfd0267e679344cc0d9a1e276dc92d87a5e1606b35bb10718e2bbfc2d0';r=json.loads(RAW.read_text());assert len(r['interactions'])==432
project_ids=list(dict.fromkeys(x['projectID'] for x in r['interactions']));assert len(project_ids)==4
pmap={p:i for i,p in enumerate(project_ids)};full=[];switches=[];issues=[]
def dist(v):
 if not v:return {'n':0}
 s=sorted(v);return {'n':len(s),'min':s[0],'median':statistics.median(s),'p50':s[math.ceil(len(s)*.5)-1],'p95':s[math.ceil(len(s)*.95)-1],'p99':s[math.ceil(len(s)*.99)-1],'max':s[-1]}
readiness=[]
for window in r['autonomousReadiness']['windows']:
 for row in window['rows']:readiness.append({'timeOrigin':window['timeOrigin'],**row})
assert len(readiness)==201
indexed={}
for row in readiness:
 if row.get('stage') in ['home','define']:
  key=(row['round'],row['stage']);assert key not in indexed;indexed[key]=row
assert len(indexed)==200
switch_labels=['action-start','all-projects-click-end','overview-heading-ready','open-project-click-start','open-project-click-end','action-end','verify-start','idea-field-wait-end','idea-context-evaluate-end','verify-end','frames-start','frames-end']
other_labels=['action-start','action-end','verify-start','verify-end','frames-start','frames-end']
regions={'allProjectsClick':('action-start','all-projects-click-end'),'overviewHeadingWait':('all-projects-click-end','overview-heading-ready'),'openProjectClick':('open-project-click-start','open-project-click-end'),'ideaFieldWait':('verify-start','idea-field-wait-end'),'contextEvaluate':('idea-field-wait-end','idea-context-evaluate-end'),'inputValueAssertion':('idea-context-evaluate-end','verify-end'),'finalFrames':('frames-start','frames-end')}
primary_round=0
for i,x in enumerate(r['interactions']):
 assert x['success'];stages=x['diagnostics']['nodeStages'];labels=[s['stage'] for s in stages];assert labels==(switch_labels if x['kind']=='project-switch' else other_labels);s={v['stage']:v['offsetMs'] for v in stages};assert all(b['offsetMs']>=a['offsetMs'] for a,b in zip(stages,stages[1:]))
 row={k:x[k] for k in ['phase','kind','at','success','elapsedMs']};row.update(index=i,projectOrdinal=pmap[x['projectID']],diagnostics=x['diagnostics'])
 if x['kind']=='project-switch':
  record=x['navigationVerification'];assert record['status']=='returned' and record['reason'] is None;w=record['witness'];assert set(w)==set(['version','status','timeOrigin','enteredAt','firstSampleAt','firstMatchedAt','resolveRequestedAt','samples','failedSamples','failedDisconnected','failedValueMismatch']);assert w['version']==1 and w['status']=='matched';assert w['samples']==w['failedSamples']+1 and w['failedSamples']==w['failedDisconnected']+w['failedValueMismatch'];assert w['enteredAt']<=w['firstSampleAt']<=w['firstMatchedAt']<=w['resolveRequestedAt'];assert all(math.isfinite(v) for v in w.values() if isinstance(v,(int,float)))
  row['navigationVerification']=record;node={name:s[end]-s[start] for name,(start,end) in regions.items()};refs=x['diagnostics']['browserFrames']['references'];assert refs['timeOrigin']==w['timeOrigin'];mark=refs['expectedDOM'];assert mark['actionKind']=='project-switch' and w['firstMatchedAt']<=mark['at']<=w['resolveRequestedAt']
  item={'index':i,'phase':x['phase'],'projectOrdinal':pmap[x['projectID']],'fullActionMs':x['elapsedMs'],'nodeRegionsMs':node,'callback':w,'callbackEntryToFirstSampleMs':w['firstSampleAt']-w['enteredAt'],'callbackEntryToMatchMs':w['firstMatchedAt']-w['enteredAt'],'callbackEntryToResolveRequestMs':w['resolveRequestedAt']-w['enteredAt'],'expectedDOMMarkAt':mark['at'],'sameDocumentExpectedDOMMarkJoin':True,'largestNodeRegion':max(node,key=node.get)}
  if x['phase']=='idle-interactive-calibration':
   home=indexed[(primary_round,'home')];define=indexed[(primary_round,'define')];assert home['timeOrigin']==define['timeOrigin']==w['timeOrigin'];assert home['status']==define['status']=='ready';assert home['projectOrdinal']==define['projectOrdinal']==pmap[x['projectID']]
   item.update(readinessJoinStatus='joined',round=primary_round,home=home,define=define,homeClickToObservedMs=home['observedAt']-home['clickAt'],defineClickToObservedMs=define['observedAt']-define['clickAt'],defineObservedToCallbackEntryMs=w['enteredAt']-define['observedAt'],defineObservedToCallbackMatchMs=w['firstMatchedAt']-define['observedAt'],defineClickToCallbackEntryMs=w['enteredAt']-define['clickAt']);primary_round+=1
  else:item.update(readinessJoinStatus='not-armed-in-original-trace-cohort',round=None)
  switches.append(item)
 full.append(row)
assert len(switches)==108 and primary_round==100
cohorts={}
for phase in sorted(set(x['phase'] for x in full)):
 cohorts[phase]={kind:dist([x['elapsedMs'] for x in full if x['phase']==phase and x['kind']==kind]) for kind in ['project-switch','input','draft-reset']}
primary=[x for x in switches if x['phase']=='idle-interactive-calibration'];trace=[x for x in switches if x['phase']!='idle-interactive-calibration'];tails=[x for x in primary if x['fullActionMs']>300]
summary={'rawResultSHA256':digest,'fullRows':432,'switchCallbacks':108,'sameDocumentExpectedDOMJoins':108,'autonomousPrimaryJoins':100,'traceObserverNotArmedByDesign':8,'autonomousRows':201,'cohorts':cohorts,'callbackSamplesHistogram':dict(collections.Counter(x['callback']['samples'] for x in switches)),'failedPredicateSamples':sum(x['callback']['failedSamples'] for x in switches),'unavailableCallbackReceipts':0,'callbackClocks':{key:dist([x[key] for x in switches]) for key in ['callbackEntryToFirstSampleMs','callbackEntryToMatchMs','callbackEntryToResolveRequestMs']},'primaryNodeRegions':{k:dist([x['nodeRegionsMs'][k] for x in primary]) for k in regions},'primaryBrowserIntervals':{k:dist([x[k] for x in primary]) for k in ['homeClickToObservedMs','defineClickToObservedMs','defineObservedToCallbackEntryMs','defineObservedToCallbackMatchMs','defineClickToCallbackEntryMs']},'negativeObservedToEntryCount':sum(x['defineObservedToCallbackEntryMs']<0 for x in primary),'primaryOver300Count':len(tails),'largestNodeRegionAllPrimary':dict(collections.Counter(x['largestNodeRegion'] for x in primary)),'largestNodeRegionOver300':dict(collections.Counter(x['largestNodeRegion'] for x in tails)),'primaryByProject':{str(i):{'action':dist([x['fullActionMs'] for x in primary if x['projectOrdinal']==i]),'evaluate':dist([x['nodeRegionsMs']['contextEvaluate'] for x in primary if x['projectOrdinal']==i]),'observedToEntry':dist([x['defineObservedToCallbackEntryMs'] for x in primary if x['projectOrdinal']==i])} for i in range(4)},'topFullSwitches':sorted(primary,key=lambda x:x['fullActionMs'],reverse=True)[:10],'topEvaluateSwitches':sorted(primary,key=lambda x:x['nodeRegionsMs']['contextEvaluate'],reverse=True)[:10],'limits':['All full original action spans and thresholds retained; no subtraction or latency acceptance replacement.','Browser intervals use same-document timeOrigin only. No browser clock is subtracted from Node clock.','Autonomous witness is sampled structural readiness, not first possible readiness or physical paint.','First-sample callback match rules out sampled predicate waiting only; no attribution among lookup, scheduling, transport or handle lifecycle.','Trace8 switches have callback/expectedDOM joins; autonomous observer was closed by original design before trace.','Different product/run from prior settled control; no cross-version improvement attribution.']}
projection={'schema':1,'rawResultSHA256':digest,'productSource':r['sourceCommit'],'helperCheckout':{k:r['harnessCheckout'][k] for k in ['commit','expectedSource','productSource','dirty']},'projectIdentityMapping':'Four original project identities mapped by first appearance to ordinals0..3; private original retains exact IDs.','interactions':full,'autonomousReadiness':r['autonomousReadiness'],'runtimeGuardAssertions':{k:r.get(k) for k in ['providerCalls','externalModelCalls','fixtureRemoved','cleanup','runtimeCleanup','dockerRequestsDrained','dockerMutationStateCertain']}}
for name,value in [('result-projection.json',projection),('switch-callback-joins.json',switches),('analysis.json',summary)]:
 data=json.dumps(value,indent=2)+'\n';assert len(data.encode())<=2*1024**2,name+' exceeds publication cap';(HERE/name).write_text(data)
with (HERE/'all432rows.csv').open('w') as f:
 fields=['index','phase','kind','projectOrdinal','success','elapsedMs'];w=csv.DictWriter(f,fields);w.writeheader();w.writerows({k:x[k] for k in fields} for x in full)
print(json.dumps({k:summary[k] for k in ['fullRows','switchCallbacks','callbackSamplesHistogram','failedPredicateSamples','callbackClocks','primaryNodeRegions','primaryBrowserIntervals','primaryOver300Count','largestNodeRegionOver300']}))
