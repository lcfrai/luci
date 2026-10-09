from pathlib import Path
import collections,gzip,hashlib,json,math,zipfile
BASE=Path('/workspace/luci/artifacts/quality-swarm/idle-67a5b7c7-attempt1')
OUT=Path(__file__).resolve().parent
sha=lambda b:hashlib.sha256(b).hexdigest()
def save(name,data):
 (OUT/name).write_text(json.dumps(data,indent=2)+'\n')
raw=(BASE/'driver/result.json').read_bytes();d=json.loads(raw);trace=(BASE/'driver/action-only-trace.zip').read_bytes()
def summary(values):
 a=sorted(values);q=lambda p:a[math.ceil(len(a)*p)-1]
 return {'count':len(a),'p50Ms':q(.5),'p95Ms':q(.95),'p99Ms':q(.99),'maxMs':max(a)}
rows=[{'index':i,**{k:v for k,v in r.items() if k!='projectID'}} for i,r in enumerate(d['interactions'])]
assert len(rows)==432 and all(r['success'] is True for r in rows)
assert sum(r['phase']=='idle-interactive-calibration' for r in rows)==400
for key,record in d['interactionSummary'].items():
 group=[r for r in rows if r['phase']+':'+r['kind']==key]
 actual=summary([r['elapsedMs'] for r in group]);assert all(record[k]==v for k,v in actual.items())
allrows=[]
for window in d['browserEventWindows']:
 for row in window['latency']['rows']:allrows.append({'timeOrigin':window['timeOrigin'],**row})
assert len(allrows)==d['latencyDiagnostics']['rows']
assert len({(r['timeOrigin'],r['sequence']) for r in allrows})==len(allrows)
lookup={(r['timeOrigin'],r['sequence']):r for r in allrows}
labels=[('allProjectsCommand','action-start','all-projects-click-end'),('overviewHeadingWait','all-projects-click-end','overview-heading-ready'),('openProjectCommand','overview-heading-ready','open-project-click-end'),('verifyExpectedContext','verify-start','verify-end'),('twoFrameCommand','frames-start','frames-end')]
spans=[]
for row in rows:
 if row['kind']!='project-switch':continue
 stages={x['stage']:x['offsetMs'] for x in row['diagnostics']['nodeStages']};f=row['diagnostics']['browserFrames'];refs=f['references'];origin=refs['timeOrigin'];p=lookup[(origin,refs['pointer']['sequence'])];e=lookup[(origin,refs['expectedDOM']['sequence'])]
 assert p['kind']=='pointer-dispatch' and p['at']==refs['pointer']['at'];assert e['kind']=='expected-dom' and e['actionKind']=='project-switch' and e['at']==refs['expectedDOM']['at']
 node={name:stages[b]-stages[a] for name,a,b in labels};browser={'lastPointerToDriverExpectedDOMWitnessMs':e['at']-p['at'],'driverExpectedDOMWitnessToFrameRequestMs':f['requestedAt']-e['at'],'frameRequestToSecondCallbackMs':f['secondFrameAt']-f['requestedAt']}
 assert all(v>=0 for v in node.values()) and all(v>=0 for v in browser.values())
 resources=[r for r in allrows if r['timeOrigin']==origin and r['kind']=='resource' and r['at']<=e['at'] and r['responseEnd']>=p['at']]
 spans.append({'index':row['index'],'phase':row['phase'],'elapsedMs':row['elapsedMs'],'nodeSpans':node,'browserClockSpans':browser,'browserRefs':{'pointer':p,'expectedDOM':e,'frames':f},'temporallyOverlappingResources':resources})
primary=[r for r in spans if r['phase']=='idle-interactive-calibration'];slow=[r for r in primary if r['elapsedMs']>300]
analysis={'originalResultSHA256':sha(raw),'originalTraceSHA256':sha(trace),'sourceCommit':d['sourceCommit'],'cohorts':d['interactionSummary'],'thresholdComparison':d['targetComparison'],'primarySlowSwitches':len(slow),'primarySwitches':len(primary),'nodeSpanSummaries':{cohort:{name:summary([r['nodeSpans'][name] for r in group]) for name,_,_ in labels} for cohort,group in [('all',primary),('over300',slow),('atMost300',[r for r in primary if r['elapsedMs']<=300])]},'browserSpanSummaries':{name:summary([r['browserClockSpans'][name] for r in slow]) for name in slow[0]['browserClockSpans']},'slowSwitches':slow,'limits':['Original end-to-end rows and thresholds are unchanged; no timing subtraction, excluded outliers or additive p95 calculation.','Node command spans include Playwright transport and scheduling/actionability. They do not isolate product functions.','Expected-DOM is sampled by a driver-triggered evaluate after its locator wait; its browser-clock interval also includes intervening driver/transport waits and is not autonomous React-commit or physical-paint timing.','Resource entries are browser-observed timing and are associated by clock overlap only. Route categories lack exact project identity; overlap is not causality or exclusive request-to-action mapping.','Primary slow actions were not traced. The trace is the separately instrumented 32-action follow-up, which must not explain earlier failures by inference.','No CPU profile or weighted CPU attribution is used. No allocation, workload throughput, live-model or Windows claim.']}
save('analysis.json',analysis)
# Trace publication: do not expose text values, fixture paths, selectors, auth or requests.
z=zipfile.ZipFile(BASE/'driver/action-only-trace.zip');original=[json.loads(r) for r in z.read('trace.trace').splitlines()];calls={};logs=[]
for r in original:
 if r['type']=='before':calls[r['callId']]={'callId':r['callId'],'class':r['class'],'method':r['method'],'startTime':r['startTime']}
 elif r['type']=='after':
  c=calls.get(r['callId']);assert c is not None;c.update(endTime=r['endTime'],durationMs=r['endTime']-c['startTime'],hadError='error' in r)
 elif r['type']=='log':
  message=r.get('message','');known=['attempting click action','  waiting for element to be visible, enabled and stable','  element is visible, enabled and stable','  scrolling into view if needed','  done scrolling','  performing click action','  click action done','  waiting for scheduled navigations to finish','  navigations have finished']
  if message in known:category=message.strip()
  elif message.startswith('waiting for getByRole('):category='waiting for role locator'
  elif 'locator resolved to' in message:category='locator resolved (contents omitted)'
  else:category='other log (contents omitted)'
  logs.append({'callId':r['callId'],'time':r['time'],'category':category,'originalMessageSHA256':sha(message.encode()),'originalMessageBytes':len(message.encode())})
clicks=[c for c in calls.values() if c['method']=='click'];assert len(clicks)==40
clickchains=[]
for c in clicks:
 chain=[l for l in logs if l['callId']==c['callId']];counts=collections.Counter(x['category'] for x in chain)
 assert counts['attempting click action']==1 and counts['element is visible, enabled and stable']==1 and counts['click action done']==1
 clickchains.append({'callId':c['callId'],'durationMs':c['durationMs'],'actionabilityLogCounts':dict(counts)})
traceProjection={'originalTraceSHA256':sha(trace),'originalTraceBytes':len(trace),'originalTraceRows':len(original),'scope':'Separate eight-round follow-up only, not the original 100 primary rounds. Params/selectors/body data omitted; exact original archive remains private.','calls':list(calls.values()),'logs':logs,'clicks':clickchains,'all40ClicksReportedOneAttemptAndOneStableWitness':True,'methodSummaries':{method:summary([c['durationMs'] for c in calls.values() if c['method']==method]) for method in sorted({c['method'] for c in calls.values()})}}
save('trace.projection.json',traceProjection)
projection={'originalResultSHA256':sha(raw),'originalResultBytes':len(raw),'representation':'Allowlisted projection; exact numeric action/timing data, excluding projectID only from interactions; no whole original result reconstruction claim.','interactions':rows,'interactionSummary':d['interactionSummary'],'latencyTargets':d['latencyTargets'],'targetComparison':d['targetComparison'],'browserLatencyRows':allrows,'browserDiagnosticStatus':d['latencyDiagnostics'],'browserTimingLedgerStatus':d['browserTimingLedger'],'scope':analysis['limits']}
(OUT/'timings.projection.json.gz').write_bytes(gzip.compress((json.dumps(projection,indent=2)+'\n').encode(),mtime=0))
print(json.dumps({'primarySlowSwitches':len(slow),'rawActions':len(rows),'latencyRows':len(allrows),'traceCalls':len(calls),'traceClicks':len(clicks),'switchP95':d['targetComparison']['project-switch']['p95Ms'],'inputP95':d['targetComparison']['input']['p95Ms']}))
