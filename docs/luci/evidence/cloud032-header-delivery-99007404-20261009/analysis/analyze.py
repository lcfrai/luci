from pathlib import Path
import collections, hashlib, json, math, statistics
ROOT=Path('/workspace/.luci-tools/header-delivery-990-v2-analysis')
OUT=ROOT/'capsule'
ORIG=Path('/workspace/luci/artifacts/quality-swarm/header-delivery-99007404-v2-attempt1')
EXPECTED='876c638211523c48f1657cc723169c0ce35b9dd24d065269b95a179488ad5bdd'
def sha(b):return hashlib.sha256(b).hexdigest()
def write(p,v):p.write_text(json.dumps(v,indent=2,allow_nan=False)+'\n')
def pick(x,keys):return {k:x[k] for k in keys if k in x}
def safe_numbers(x):
 if isinstance(x,dict):
  for v in x.values():safe_numbers(v)
 elif isinstance(x,list):
  for v in x:safe_numbers(v)
 elif isinstance(x,(float,int)):assert math.isfinite(x)
def stats(a):
 a=sorted(a)
 return {'count':len(a),'p50Ms':a[math.ceil(.50*len(a))-1] if a else None,'p95Ms':a[math.ceil(.95*len(a))-1] if a else None,'p99Ms':a[math.ceil(.99*len(a))-1] if a else None,'maxMs':max(a) if a else None,'meanMs':statistics.mean(a) if a else None}
b=(ORIG/'driver/result.json').read_bytes();assert sha(b)==EXPECTED;d=json.loads(b)
schedule=d['headerDeliverySchedule']['rounds'];assert len(schedule)==80 and len(d['interactions'])==320
assert d['passed'] and d['diagnosticOnly'] and not d['fullSustainedAcceptance']
assert d['sourceCommit']=='99007404a5ddd8592dee376ea00d99c6180d26fe'
assert d['harnessCheckout']['commit']=='a31e1ac485c95bdbcc9ebb011581da5ce3f5c67a'
rows=[];project_map={};condition_names={'baseline','defer'};kinds=['project-switch','input','input','draft-reset']
for i,x in enumerate(d['interactions']):
 s=schedule[i//4];project='ABCD'[s['projectIndex']]
 assert s['round']==i//4 and s['condition'] in condition_names and x['phase']=='diagnostic-header-'+s['condition'] and x['kind']==kinds[i%4] and x['success']
 if project in project_map:assert project_map[project]==x['projectID']
 project_map[project]=x['projectID']
 dg=x['diagnostics'];frames=dg['browserFrames'];refs=frames['references']
 row={'index':i,'round':s['round'],'block':s['block'],'project':project,'condition':s['condition'],**pick(x,['kind','at','success','elapsedMs']), 'diagnostics':{'nodeStages':[pick(y,['stage','offsetMs']) for y in dg['nodeStages']],'nodeClockReads':dg['nodeClockReads'],'browserFrames':{**pick(frames,['requestedAt','firstFrameAt','secondFrameAt']),'references':{'timeOrigin':refs['timeOrigin'],**{k:pick(refs[k],['sequence','at','actionKind']) if refs[k] else None for k in ['input','pointer','expectedDOM']}}}}}
 assert set(y['stage'] for y in row['diagnostics']['nodeStages'])<={'action-start','action-end','verify-start','verify-end','frames-start','frames-end','all-projects-click-end','overview-heading-ready','open-project-click-end'}
 rows.append(row)
assert len(set(project_map.values()))==4
# Passive collectors already remove URLs/values; strict field projection keeps only documented fields.
windows=[]
event_names={'beforeinput','click','input','keydown','keypress','keyup','mousedown','mouseout','mouseover','mouseup','pointerdown','pointerenter','pointerleave','pointerout','pointerover','pointerup'}
latency_kinds={'expected-dom','input-dispatch','input-frame-1','input-frame-2','pointer-dispatch','resource'}
routes={'project-overview','project-specification','project-orchestrator','project-swarm-queue','project-runs','swarm-scheduler'}
for w in d['browserEventWindows']:
 ew=pick(w,['at','version','timeOrigin','observedEvents','droppedEvents','overflow','unsupported','visibility','visibilityNow']);ew['index']=len(windows);ew['events']=[]
 for e in w['events']:
  assert e['name'] in event_names
  ew['events'].append(pick(e,['sequence','name','startTime','duration','processingStart','processingEnd','interactionId']))
 lt=w['latency'];ew['latency']=pick(lt,['version','timeOrigin','observed','dropped','overflow','resourceUnsupported','callbacks','callbackMs','visibility']);ew['latency']['rows']=[]
 for e in lt['rows']:
  assert e['kind'] in latency_kinds and (e.get('route') is None or e['route'] in routes) and (e.get('actionKind') is None or e['actionKind'] in ['input','project-switch'])
  q=pick(e,['sequence','kind','at','inputSequence','actionKind','route','duration','responseStatus','requestStart','responseStart','responseEnd','transferSize','encodedBodySize','decodedBodySize'])
  for k in ['input','pointer']:
   if k in e:q[k]=pick(e[k],['sequence','at']) if e[k] else None
  ew['latency']['rows'].append(q)
 windows.append(ew)
h=d['headerDelivery'];header=pick(h,['version','diagnosticOnly','status','valid','limits','metadataBytes','pausedIDsAtClose']);header['requests']=[];header['responses']=[];header['events']=[];header['longTaskWindows']=[]
for x in h['requests']:
 assert x['route'] in {'home-overview','home-capacity'} and x['condition'] in condition_names and x['outcome'] in {'natural-browser-abort','network-loading-finished'}
 header['requests'].append(pick(x,['sequence','route','projectIndex','round','condition','requestAt','terminal','failedAt','finishedAt','cancelled','outcome']))
for x in h['responses']:
 assert x['route'] in {'home-overview','home-capacity'} and x['requestCondition'] in condition_names and x['responseCondition'] in condition_names and x['outcome'] in {'continued-original-headers','original-network-cancelled-terminal-ID'} and x['releaseReason'] in {'round-complete','baseline-immediate'}
 header['responses'].append(pick(x,['sequence','requestSequence','route','projectIndex','status','requestRound','requestCondition','responseRound','responseCondition','pausedAt','headerBytes','held','outcome','releaseReason','releasedAt','pauseMs']))
for x in h['events']:
 assert x['kind'] in {'round-start','round-end','headers-held','response-resolved'}
 header['events'].append(pick(x,['kind','nodeAt','condition','round','sequence','route','outcome','reason','pauseMs']))
for x in h['longTaskWindows']:
 assert not x['unsupported'] and x['dropped']==0
 header['longTaskWindows'].append({'afterRound':int(x['reason'].removeprefix('after-round-')),**pick(x,['version','timeOrigin','observed','dropped','unsupported']),'rows':[pick(r,['sequence','startTime','duration']) for r in x['rows']]})
assert len(header['requests'])==len(header['responses'])==400 and header['status']=='closed' and header['valid'] and header['pausedIDsAtClose']==[]
projection={'originalResult':{'relativePath':'driver/result.json','sha256':EXPECTED,'bytes':len(b),'projection':'Explicit allowlist. Private project IDs replaced by A/B/C/D; private URL/path/context/body fields omitted. Original is preserved outside this capsule.'},**pick(d,['passed','diagnosticOnly','fullSustainedAcceptance','headerDeliveryDiagnosticCompleted','startedAt','finishedAt','sourceCommit','sourceDirty','harnessCheckout','finalHarnessCheckout','packageIntegrity','binarySHA256','webIndexSHA256','driverSHA256','helperSHA256','reusedHelperSHA256','image','latencyTargets','targetComparison','interactionSummary','externalModelCalls','cloudResourcesCreated','providerCalls','auxiliaryCalls','actualTaskCount','controllersStopped','dockerRequestsDrained','dockerMutationStateCertain','fixtureRemoved','workflowOperationUnsettled','originalReceiptComplete','elapsedMs','totalElapsedMs']), 'interactions':rows,'browserEventWindows':windows,'headerDelivery':header,'headerDeliverySchedule':d['headerDeliverySchedule'],'cleanup':d['cleanup'],'browserErrors':d['browserErrors'],'cleanupPhases':d['cleanupPhases'],'runtimeCleanupDriverAssertions':d['runtimeCleanup'],'dockerTransportCounts':d['dockerTransportObservations']['counts'],'browserMutationCountsByStage':dict(collections.Counter(x['stage'] for x in d['browserMutations'])),'latencyDiagnostics':d['latencyDiagnostics'],'browserTimingLedger':d['browserTimingLedger']}
safe_numbers(projection)
# Ensure the projection cannot accidentally retain original opaque project identities or private fixture directories.
encoded=json.dumps(projection)
for value in project_map.values():assert value not in encoded
for prefix in ['/workspace/.c32h','luci-sustained-','Bearer ','api_key','Authorization'] : assert prefix not in encoded
write(OUT/'result-projection.json',projection)
# Verify the original nearest-rank summaries from all320 rows, including every successful action.
groups={}
for c in ['baseline','defer']:
 for kind in ['project-switch','input','draft-reset']:
  selected=[r for r in rows if r['condition']==c and r['kind']==kind];s=stats([r['elapsedMs'] for r in selected]);groups[c+':'+kind]=s
  existing=d['interactionSummary']['diagnostic-header-'+c+':'+kind]
  for key in ['count','p50Ms','p95Ms','p99Ms','maxMs']:assert abs(s[key]-existing[key])<1e-7
perproject={p:{c:{k:stats([r['elapsedMs'] for r in rows if r['project']==p and r['condition']==c and r['kind']==k]) for k in ['project-switch','input','draft-reset']} for c in ['baseline','defer']} for p in 'ABCD'}
perblock=[{'block':b,'schedule':['ABCD'[s['projectIndex']]+':'+s['condition'] for s in schedule if s['block']==b], 'conditions':{c:{k:stats([r['elapsedMs'] for r in rows if r['block']==b and r['condition']==c and r['kind']==k]) for k in ['project-switch','input','draft-reset']} for c in ['baseline','defer']}} for b in range(10)]
# Exact within-block/per-project comparisons: no significance or independent-sample claim.
pairs=[]
for block in range(10):
 for p in 'ABCD':
  pair={c:next(r for r in rows if r['block']==block and r['project']==p and r['condition']==c and r['kind']=='project-switch') for c in ['baseline','defer']}
  pairs.append({'block':block,'project':p,'baselineMs':pair['baseline']['elapsedMs'],'deferMs':pair['defer']['elapsedMs'],'deferMinusBaselineMs':pair['defer']['elapsedMs']-pair['baseline']['elapsedMs'],'baselineRound':pair['baseline']['round'],'deferRound':pair['defer']['round']})
# Request durations use only one original CDP Network clock (seconds). No Node/browser clock subtraction.
exposure=[]
for s in schedule:
 req=[x for x in header['requests'] if x['round']==s['round']];res=[x for x in header['responses'] if x['requestRound']==s['round']]
 assert len(req)==len(res)==5 and {x['sequence'] for x in req}=={x['requestSequence'] for x in res}
 assert all(x['responseRound']==s['round'] and x['status']==200 and x['responseCondition']==s['condition'] for x in res)
 assert collections.Counter(x['route'] for x in res)=={'home-overview':4,'home-capacity':1}
 assert all(x.get('held',False)==(s['condition']=='defer') for x in res)
 exposure.append({**s,'requestCount':len(req),'responseCount':len(res),'held':sum(bool(x.get('held')) for x in res),'continued':sum(x['outcome']=='continued-original-headers' for x in res),'cancelledTerminalIDs':sum(x['outcome']=='original-network-cancelled-terminal-ID' for x in res),'pauseMs':stats([x['pauseMs'] for x in res]),'originalRequestUntilNetworkTerminalMs':stats([(x.get('finishedAt',x.get('failedAt'))-x['requestAt'])*1000 for x in req])})
# Long tasks are joined using the same document/browser clock only. The first pointer through second-frame interval excludes pre-pointer automation waits.
longtasks=[{'timeOrigin':w['timeOrigin'],**x} for w in header['longTaskWindows'] for x in w['rows']]
assert len(longtasks)==28 and [x['sequence'] for x in longtasks]==list(range(1,29))
latency=[{'timeOrigin':w['latency']['timeOrigin'],**x} for w in windows for x in w['latency']['rows']]
intervals=[]
previous_end=None
for r in rows:
 f=r['diagnostics']['browserFrames'];refs=f['references'];origin=refs['timeOrigin'];kind=r['kind']
 if kind=='input':
  start=refs['input']['at'];sequences=[refs['input']['sequence']];basis='trusted input dispatch to second rAF'
 else:
  pointer=[x for x in latency if x['timeOrigin']==origin and x['kind']=='pointer-dispatch' and x['sequence']<=refs['pointer']['sequence']]
  chosen=pointer[-(2 if kind=='project-switch' else 3):];assert len(chosen)==(2 if kind=='project-switch' else 3)
  assert chosen[-1]['sequence']==refs['pointer']['sequence'];start=chosen[0]['at'];sequences=[x['sequence'] for x in chosen];basis='first of exact scripted '+str(len(chosen))+' trusted pointer dispatches to second rAF'
  if previous_end is not None:assert all(x['at']>=previous_end for x in chosen)
 end=f['secondFrameAt'];assert start<=end and (previous_end is None or start>=previous_end)
 overlaps=[{'sequence':x['sequence'],'overlapMs':max(0,min(end,x['startTime']+x['duration'])-max(start,x['startTime']))} for x in longtasks if x['timeOrigin']==origin and x['startTime']<end and x['startTime']+x['duration']>start]
 intervals.append({'index':r['index'],'round':r['round'],'block':r['block'],'project':r['project'],'condition':r['condition'],'kind':kind,'timeOrigin':origin,'startTime':start,'endTime':end,'durationMs':end-start,'basis':basis,'dispatchSequences':sequences,'longTaskOverlaps':overlaps,'longTaskOverlapMs':sum(x['overlapMs'] for x in overlaps)})
 previous_end=end
lt_summary={}
for c in ['baseline','defer']:
 lt_summary[c]={}
 for kind in ['project-switch','input','draft-reset']:
  selected=[x for x in intervals if x['condition']==c and x['kind']==kind]
  lt_summary[c][kind]={'observedActionIntervals':len(selected),'intervalsWithLongTaskOverlap':sum(bool(x['longTaskOverlaps']) for x in selected),'uniqueOverlappingLongTasks':len({y['sequence'] for x in selected for y in x['longTaskOverlaps']}),'overlapMs':sum(x['longTaskOverlapMs'] for x in selected),'observedBrowserSubspanMs':stats([x['durationMs'] for x in selected])}
covered={y['sequence'] for x in intervals for y in x['longTaskOverlaps']}
resources=[x for x in latency if x['kind']=='resource']
resource_stats={route:{'count':len([x for x in resources if x['route']==route]),'durationMs':stats([x['duration'] for x in resources if x['route']==route]),'statusCounts':dict(collections.Counter(str(x['responseStatus']) for x in resources if x['route']==route))} for route in sorted(routes)}
node_stages={}
for c in ['baseline','defer']:
 selected=[r for r in rows if r['condition']==c and r['kind']=='project-switch'];m={r['index']:{s['stage']:s['offsetMs'] for s in r['diagnostics']['nodeStages']} for r in selected}
 pairs_stages=[('allProjectsClick','action-start','all-projects-click-end'),('overviewHeadingWait','all-projects-click-end','overview-heading-ready'),('openProjectClick','overview-heading-ready','open-project-click-end'),('savedContextVerification','verify-start','verify-end'),('twoFrameRPC','frames-start','frames-end')]
 node_stages[c]={label:stats([v[end]-v[start] for v in m.values()]) for label,start,end in pairs_stages}
summary={'sourceResultSHA256':EXPECTED,'quantileMethod':'Nearest rank ceil(n*q)-1, unchanged original helper method; per-project n10 and per-block n4 p95/p99 are maxima, not stable population estimates. All320 rows retained.','completedActions':len(rows),'groups':groups,'perProject':perproject,'perBlock':perblock,'matchedProjectBlockSwitchPairs':pairs,'matchedPairDescription':{'count':40,'deferFaster':sum(x['deferMinusBaselineMs']<0 for x in pairs),'deferSlower':sum(x['deferMinusBaselineMs']>0 for x in pairs),'deltaMs':stats([x['deferMinusBaselineMs'] for x in pairs]),'scope':'Descriptive paired order-block observations only. No independence, significance, generalization or causal mechanism claim.'},'exposureByRound':exposure,'exposureByCondition':{c:{'requests':sum(x['condition']==c for x in header['requests']),'responses':sum(x['requestCondition']==c for x in header['responses']),'held':sum(x['requestCondition']==c and bool(x.get('held')) for x in header['responses']),'responseOutcomes':dict(collections.Counter(x['outcome'] for x in header['responses'] if x['requestCondition']==c)),'pauseMs':stats([x['pauseMs'] for x in header['responses'] if x['requestCondition']==c]),'originalRequestUntilNetworkTerminalMs':stats([(x.get('finishedAt',x.get('failedAt'))-x['requestAt'])*1000 for x in header['requests'] if x['condition']==c])} for c in ['baseline','defer']},'longTasks':{'count':len(longtasks),'durationMs':stats([x['duration'] for x in longtasks]),'rows':longtasks,'sameClockObservedActionIntervals':intervals,'conditionSummary':lt_summary,'overlapAnyObservedAction':len(covered),'withoutObservedActionOverlap':[x['sequence'] for x in longtasks if x['sequence'] not in covered],'limits':'Same-browser-document intervals start at trusted input/first scripted pointer dispatch and end at second rAF; they exclude automation waits before dispatch and are not full Node spans, physical paint, React commit or CPU ownership attribution. Rows drained after a round can precede measurement; drain window does not assign causality.'},'browserResourceTiming':{'byRoute':resource_stats,'scope':'All retained browser resource entries, including setup/quiet/drain; not exact CDP request joins or backend function timings. Do not subtract from Node timestamps or infer body completion while held.'},'projectSwitchNodeSubspans':node_stages,'limits':['One controlled run of exact product990/helpera31; not current67a5 performance acceptance.','All200 held responses naturally cancelled; no held response later delivered its body. This is response availability/backpressure plus cancellation, not isolated frontend commit deferral.','Both project-switch p95 values exceed unchanged300ms. Inputs satisfy the100ms reference only under intervention; no overall latency pass.','Repeated actions share one fixture/browser; small tails and block/project summaries are descriptive. No optimization or frontend-only cause justified.','No model/provider/worker runs; explicit setup writes outside measured interval remain in original evidence.','Independent closure proves only exact driver/controller absence and empty exclusive Docker/volume inventory; no independent browser PID census or whole-machine closure.']}
write(OUT/'analysis/summary.json',summary)
closure=json.loads((ROOT/'independent-closure.json').read_text());metadata={'scope':'Private original artifacts remain unchanged outside capsule; metadata only. No original raw result/stdout/journal/private path copied.','originalResultSHA256':EXPECTED,'originals':[{'path':path,'sha256':expected,'bytes':(ORIG/path).stat().st_size} for path,expected in closure['originalArtifacts'].items()],'projection':'result-projection.json','projectionSHA256':sha((OUT/'result-projection.json').read_bytes())};write(OUT/'original-artifacts-metadata.json',metadata)
assert sha((ORIG/'driver/result.json').read_bytes())==EXPECTED
print(json.dumps({'rows':len(rows),'groups':groups,'matchedPairs':summary['matchedPairDescription'],'longTasks':lt_summary,'longTasksOverlappingAny':len(covered),'uncovered':summary['longTasks']['withoutObservedActionOverlap'],'exposure':summary['exposureByCondition'],'projectionSHA256':metadata['projectionSHA256']},indent=2))
