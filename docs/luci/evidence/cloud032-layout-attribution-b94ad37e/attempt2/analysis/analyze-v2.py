"""Bounded read-only V2 derivation. No raw URLs/args/private fixture strings exported."""
from pathlib import Path
from urllib.parse import urlparse
import json,hashlib,collections,re
R=Path(__file__).resolve().parent;D=Path('/workspace/luci/artifacts/quality-swarm/layout-b94ad37e-attempt2/driver');H=Path('/workspace/.luci-tools/cloud032-layout-b94-v2');PACKAGE=Path('/tmp/luci-retained-tools/package-0.5.0-b94ad37e/web')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
assert sha(D/'result.json')=='fead13b770f2351282fc4c9a1bd9d0ab34248b087311e9b4ccdb651ed424fe26'
r=json.loads((D/'result.json').read_bytes());capture=json.loads((D/'layout-trace-capture.json').read_bytes());projection=json.loads((D/'layout-trace-projection.json').read_bytes());assert capture['complete'] and not capture['dataLossOccurred'] and capture['completedRounds']==8 and projection['clock']['anchorCount']==16
raw=(D/'layout-trace.raw.json').read_bytes();assert len(raw)==28383461 and hashlib.sha256(raw).hexdigest()=='f55eb46a4fc0d6ece3a2393135fb3636aaf5b5548f7db41eea3931b50659aa3c'
events=json.loads(raw)['traceEvents'];assert len(events)==70958
bundles=json.loads((H/'actual/long-frame-bundles.json').read_text());byPath={x['path']:x['sha256'] for x in bundles};origins=set()
for e in events:
 d=e.get('args',{}).get('data',{});u=urlparse(d.get('url',''))
 if u.path in byPath:origins.add(u.scheme+'://'+u.netloc)
assert len(origins)==1 and next(iter(origins)).startswith('http://127.0.0.1:');origin=next(iter(origins))
for path,digest in byPath.items():assert sha(PACKAGE/path.lstrip('/'))==digest
clock=projection['clock'];pid,tid=clock['renderer']['pid'],clock['renderer']['tid'];offset=clock['offsetUs'];timeOrigin=clock['documentTimeOriginMs'];allBrowserEvents=[{'windowIndex':i,'timeOrigin':w['timeOrigin'],**e} for i,w in enumerate(r['browserEventWindows']) for e in w['events']]
ready=[x for w in r['autonomousReadiness']['windows'] for x in w['rows'] if x.get('stage') in ['home','define']];assert len(ready)==16
projectIDs=list(dict.fromkeys(x['projectID'] for x in r['interactions']));assert len(projectIDs)==4;ordinal={p:i for i,p in enumerate(projectIDs)}
interactions=[{**{k:v for k,v in x.items() if k!='projectID'},'index':i,'projectOrdinal':ordinal[x['projectID']]} for i,x in enumerate(r['interactions'])];assert len(interactions)==32 and all(x['success'] for x in interactions)
regions={'allProjectsClick':('action-start','all-projects-click-end'),'overviewHeadingWait':('all-projects-click-end','overview-heading-ready'),'openProjectClick':('open-project-click-start','open-project-click-end'),'ideaFieldWait':('verify-start','idea-field-wait-end'),'contextEvaluate':('idea-field-wait-end','idea-context-evaluate-end'),'inputValueAssertion':('idea-context-evaluate-end','verify-end'),'finalFrames':('frames-start','frames-end')}
def safeFunction(d):
 u=urlparse(d.get('url',''));digest=byPath.get(u.path)
 if digest and u.scheme+'://'+u.netloc==origin:return {'source':'app-bundle','bundleSHA256':digest,'assetPath':u.path,'lineNumber':d.get('lineNumber'),'columnNumber':d.get('columnNumber')}
 return {'source':'unattributed'}
functions=[];dispatch=[];cats=collections.Counter();names=collections.Counter()
for i,e in enumerate(events):
 cats[e['cat']]+=1
 if e['pid']!=pid or e['tid']!=tid:continue
 if e['name']=='FunctionCall':functions.append({'rawEventIndex':i,'tsUs':e['ts'],'durUs':e.get('dur',0),**safeFunction(e.get('args',{}).get('data',{}))})
 if e['name']=='EventDispatch':
  kind=e.get('args',{}).get('data',{}).get('type');dispatch.append({'rawEventIndex':i,'tsUs':e['ts'],'durUs':e.get('dur',0),'type':kind if kind in ['click','pointerdown','pointerup','mousedown','mouseup','focus','blur','input','keydown','keyup','change','focusin','focusout'] else 'other'})
joined=[]
for witness in ready:
 matches=[e for e in allBrowserEvents if e['timeOrigin']==timeOrigin and e['name']=='click' and e['processingStart']<=witness['clickAt']<=e['processingEnd']];assert len(matches)==1
 click=matches[0];start=witness['clickAt'];end=witness['observedAt'];traceStart=start*1000+offset;traceEnd=end*1000+offset
 overlaps=lambda e:e['tsUs']<traceEnd and e['tsUs']+(e.get('durUs') or 0)>traceStart
 layouts=[e for e in projection['events'] if e['sameRendererThread'] and e['name'] in ['Layout','UpdateLayoutTree','RecalculateStyles'] and overlaps(e)]
 fs=[e for e in functions if overlaps(e)];ds=[e for e in dispatch if overlaps(e)]
 joined.append({'round':witness['round'],'projectOrdinal':witness['projectOrdinal'],'stage':witness['stage'],'browserClickAt':start,'browserObservedAt':end,'clickToObservedMs':end-start,'event':click,'eventHandlerMs':click['processingEnd']-click['processingStart'],'eventQueueDelayMs':click['processingStart']-click['startTime'],'reportedEventDurationMs':click['duration'],'layoutStyleEvents':layouts,'functionCalls':fs,'eventDispatch':ds,'limits':'Same anchored renderer clock, overlap only; nested inclusive durations are not additive CPU or causality.'})
switches=[]
for i,x in enumerate(interactions):
 if x['kind']!='project-switch':continue
 marks={m['stage']:m['offsetMs'] for m in x['diagnostics']['nodeStages']};switches.append({'index':i,'round':i//4,'fullActionMs':x['elapsedMs'],'nodeRegionsMs':{k:marks[b]-marks[a] for k,(a,b) in regions.items()},'stageJoins':[j for j in joined if j['round']==i//4]})
# Categorize all emitted Layout/style events, including reset/typing portions outside switch-stage windows.
allLayout=[]
for x in projection['events']:
 if x['name'] not in ['Layout','UpdateLayoutTree','RecalculateStyles']:continue
 overlapsStages=[{'round':j['round'],'stage':j['stage']} for j in joined if any(e['rawEventIndex']==x['rawEventIndex'] for e in j['layoutStyleEvents'])]
 allLayout.append({**x,'overlappingSwitchStages':overlapsStages})
# Preserve already-safe complete projection and capture metadata. Raw trace/result stay private.
(R/'all32-interactions.json').write_text(json.dumps({'rawResultSHA256':sha(D/'result.json'),'projectIdentityMapping':'First-appearance ordinals0..3','interactions':interactions},indent=2)+'\n')
(R/'all16-stage-joins.json').write_text(json.dumps({'clock':clock,'rows':joined},indent=2)+'\n')
(R/'all-layout-style-events.json').write_text(json.dumps({'rows':allLayout},indent=2)+'\n')
(R/'all-renderer-function-calls.json').write_text(json.dumps({'rows':functions},indent=2)+'\n')
summary={'scope':'Eight instrumented diagnostic rounds; no primary acceptance or latency comparison. Every action/stage/layout/function row retained; no subtraction.','rawResultSHA256':sha(D/'result.json'),'privateRawTraceSHA256':sha(D/'layout-trace.raw.json'),'captureComplete':capture['complete'],'dataLossOccurred':capture['dataLossOccurred'],'rawBytes':len(raw),'rawEvents':len(events),'categoryCounts':dict(cats),'maximumReportedBufferFraction':max(x['fraction'] for x in capture['bufferUsage']),'clock':clock,'knownBundleOriginCount':len(origins),'knownBundleOriginSHA256':hashlib.sha256(origin.encode()).hexdigest(),'switches':switches,'allLayoutStyleEventCount':len(allLayout),'layoutStyleEventsOverlappingSwitchStages':sum(bool(x['overlappingSwitchStages']) for x in allLayout),'topLayoutStyleEvents':sorted(allLayout,key=lambda x:x.get('durUs',0),reverse=True)[:15],'limits':['No causal/CPU inference from inclusive wall intervals or selected source positions.','Observed-ready is the original passive DOM witness, not first paint or server authority.','Intervals intentionally do not subtract browser values from Node action spans.','Stage overlap does not include all work in whole round: input and draft reset are separately recorded.','Raw URLs/function names/args/private fixture identity omitted; known exact assets identified by SHA.']}
(R/'v2-analysis.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps({'switches':len(switches),'joins':len(joined),'layoutStyleEvents':len(allLayout),'overlapSwitchStages':summary['layoutStyleEventsOverlappingSwitchStages'],'rendererFunctionCalls':len(functions),'maxBufferFraction':summary['maximumReportedBufferFraction'],'anchorSpreadUs':clock['spreadUs']}))
