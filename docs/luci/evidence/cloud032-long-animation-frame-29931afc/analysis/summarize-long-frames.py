"""Bounded allowlisted derivation; never modifies original measurements or raw inputs."""
import collections, hashlib, json, math, statistics
from pathlib import Path
ROOT=Path('/workspace/.luci-tools/cloud032-loaf-299-analysis')
RAW=Path('/workspace/luci/artifacts/quality-swarm/idle-29931afc-long-frame-attempt1/driver/result.json')
EXPECTED='d71194560d8c6852b2561062a236ef2af92d32feb917f624a507e28d0977e73f'
assert RAW.stat().st_size < 64*1024*1024
raw=RAW.read_bytes(); assert hashlib.sha256(raw).hexdigest()==EXPECTED
r=json.loads(raw)
def read(n):return json.loads((ROOT/n).read_text())
def write(n,v): (ROOT/n).write_text(json.dumps(v,indent=2)+'\n')
def stats(v):
 v=sorted(v)
 return {'n':len(v),'min':v[0], 'p50':v[math.ceil(.5*len(v))-1],'p95':v[math.ceil(.95*len(v))-1],'max':v[-1]} if v else {'n':0}
j=read('long-frame-joins.json'); e=read('event-timing-click-joins.json')['rows']; s=read('switch-callback-joins.json')
assert j['rawResultSHA256']==EXPECTED and len(e)==200 and len(s)==108
primary={x['index']:x for x in s if x['phase']=='idle-interactive-calibration'}; assert len(primary)==100
allrows=[]
for index,x in primary.items():
 clicks=[z for z in e if z['switchIndex']==index]; joins=[z for z in j['joins'] if z['switchIndex']==index]
 assert len(clicks)==len(joins)==2
 intervals={name:sorted({f for z in joins for f in z['intervals'].get(name,{}).get('frameIDs',[])}) for name in ['reportedClick','clickToObservedReady','homeObservedReadyToOpenClick']}
 allrows.append({'index':index,'projectOrdinal':x['projectOrdinal'],'fullActionMs':x['fullActionMs'],'over300':x['fullActionMs']>300,'nodeRegionsMs':x['nodeRegionsMs'],'largestNodeRegion':x['largestNodeRegion'],'exactClickEvents':clicks,'longFrameIDsByInterval':intervals,'anyQualifyingFrameOverlap':any(intervals.values())})
cohorts={}
for name,rows in [('primaryAll',allrows),('primaryOver300',[x for x in allrows if x['over300']])]:
 cohorts[name]={'switches':len(rows),'withAnyIntervalOverlap':sum(x['anyQualifyingFrameOverlap'] for x in rows),'withoutAnyIntervalOverlap':sum(not x['anyQualifyingFrameOverlap'] for x in rows),'bothReportedClickDurationsAtMost100':sum(all(c['reportedEventDurationMs']<=100 for c in x['exactClickEvents']) for x in rows),'intervals':{k:{'switches':sum(bool(x['longFrameIDsByInterval'][k]) for x in rows),'distinctFrames':len({v for x in rows for v in x['longFrameIDsByInterval'][k]})} for k in allrows[0]['longFrameIDsByInterval']},'largestNodeRegions':dict(collections.Counter(x['largestNodeRegion'] for x in rows))}
# Preserve every resource and diagnostic row, with enums/numbers only. No URLs/headers/bodies.
allowed={'input-dispatch':{'sequence','kind','at'},'input-frame-1':{'sequence','kind','at','inputSequence'},'input-frame-2':{'sequence','kind','at','inputSequence'},'pointer-dispatch':{'sequence','kind','at'},'resource':{'sequence','kind','route','at','duration','responseStatus','requestStart','responseStart','responseEnd','transferSize','encodedBodySize','decodedBodySize'},'expected-dom':{'sequence','kind','at','actionKind','input','pointer'}}
routes={'project-orchestrator','project-overview','project-runs','project-specification','project-swarm-queue','swarm-scheduler'}
windows=[]; resourceRows=[]
for i,w in enumerate(r['browserEventWindows']):
 l=w['latency']
 for x in l['rows']:
  assert set(x)<=allowed[x['kind']]
  if x['kind']=='resource':assert x['route'] in routes; resourceRows.append({'windowIndex':i,'timeOrigin':w['timeOrigin'],**x})
 windows.append({'index':i,'timeOrigin':w['timeOrigin'],'reason':w['reason'],'at':w['at'],'rows':l['rows'],'callbacks':l['callbacks'],'callbackMs':l['callbackMs'],'dropped':l['dropped'],'overflow':l['overflow'],'visibility':l['visibility'],'resourceUnsupported':l['resourceUnsupported']})
write('all-latency-windows.json',{'rawResultSHA256':EXPECTED,'windows':windows})
# Finite API snapshot totals are cumulative per document, never sum repeated cumulative snapshots.
last={w['timeOrigin']:w for w in r['longAnimationFrameCapture']['windows']}
overhead={'documents':len(last),'callbackCount':sum(w['callbacks'] for w in last.values()),'reportedCallbackSelfMs':sum(w['callbackMs'] for w in last.values()),'maxDocumentCallbackSelfMs':max(w['callbackMs'] for w in last.values()),'scope':'Partial observer callback self time only, excluding native observation, serialization, resource/EventTiming/autonomous observers and command overhead; no matched no-observer control, not total observer cost.'}
source={'bundlePath':'web/assets/index-oRjgUib4.js','bundleSHA256':'526b6f92ddac31da933f04978f13f5bd5a3e2e5312422a04d89b8c53051c9c3e','sourceCharPosition':211123,'offsetUnit':'UTF-16 code units per Long Animation Frame API','attribution':'React discrete-event dispatch wrapper _h; changes update priority to 2 then calls yh/Kf. This identifies the outer dispatcher, not the specific component or layout-reading caller.'}
b=Path('/tmp/luci-retained-tools/package-0.5.0-29931afc')/source['bundlePath']; bs=b.read_bytes();assert hashlib.sha256(bs).hexdigest()==source['bundleSHA256']; text=bs.decode(); source['excerptStartUTF16']=211090;source['excerpt']=text.encode('utf-16-le')[211090*2:211290*2].decode('utf-16-le')
framePhaseCounts=collections.Counter(); scriptSummary=[]
for f in j['allFrames']:
 epoch=f['timeOrigin']+f['startTime']; phase='setup-or-outside-declared-phases'
 from datetime import datetime
 for p in r['phases']:
  start=datetime.fromisoformat(p['startedAt'].replace('Z','+00:00')).timestamp()*1000;end=datetime.fromisoformat(p['finishedAt'].replace('Z','+00:00')).timestamp()*1000
  if start<=epoch<end: phase=p['name'];break
 framePhaseCounts[phase]+=1
 for script in f['scripts']:scriptSummary.append({'frameID':f['id'],'frameDuration':f['duration'],'phaseByBrowserEpochAndPhaseWallClock':phase,**script})
limits=['Every original action and threshold preserved; no subtraction, filtering or synthetic replacement.','LoAF eligibility is selective and snapshot coverage is not a continuous rendering watermark. Absence of qualifying overlap does not establish idle rendering or transport causality.','Reported frame/script/forced-layout durations are wall intervals, not sampled CPU or proof of a specific React component.','EventTiming duration is quantized and includes processing and presentation delay. Inclusive Node commands mix transport, scheduler, actionability and browser work.','Per-stage quantiles are not additive. Browser clocks are not subtracted from Node clocks.','Phase grouping uses browser timeOrigin+start and declared wall phase times, not exact action attribution; exact primary joins use original same-document trusted click events.','Separate trace follow-up does not trace primary actions and cannot retroactively explain their transport costs.']
write('long-frame-analysis.json',{'rawResultSHA256':EXPECTED,'capture':{k:v for k,v in r['longAnimationFrameCapture'].items() if k!='windows'},'cohorts':cohorts,'snapshotCoverageCounts':dict(collections.Counter(v['coverage'] for z in j['joins'] for v in z['intervals'].values())),'phaseFrameCounts':dict(framePhaseCounts),'partialObserverCost':overhead,'knownBundleAttribution':source,'all100Switches':allrows,'all50Scripts':scriptSummary,'resourceSummary':{route:stats([x['duration'] for x in resourceRows if x['route']==route]) for route in sorted(routes)},'limits':limits})
write('long-frame-capture.json',{'rawResultSHA256':EXPECTED,**r['longAnimationFrameCapture']})
receiptKeys=['startedAt','finishedAt','sourceCommit','sourceDirty','harnessCheckout','binarySHA256','webIndexSHA256','driverSHA256','helperSHA256','reusedHelperSHA256','resourceScope','externalModelCalls','cloudResourcesCreated','scope','limitations','diagnosticOnly','fullSustainedAcceptance','phases','browserVersion','engineIdentitySHA256','interactionSummary','latencyTargets','targetComparison','traceScope','runtimeCleanup','fixtureRemoved','providerCalls','auxiliaryCalls','actualTaskCount','elapsedMs','totalElapsedMs','workflowOperationUnsettled','originalReceiptComplete','signal','idleDiagnosticCompleted','dockerMutationStateCertain','dockerTransportNotStarted']
write('run-receipt.json',{'rawResultSHA256':EXPECTED,'rawResultPrivate':True,'rawTracePrivate':True,'rawTraceSHA256':read('trace-analysis.json')['traceArchiveSHA256'],**{k:r[k] for k in receiptKeys}})
print(json.dumps({'cohorts':cohorts,'partialObserverCost':overhead,'framePhases':dict(framePhaseCounts),'resourceCounts':len(resourceRows),'bundleOffsetAttribution':'React dispatcher only'}))
