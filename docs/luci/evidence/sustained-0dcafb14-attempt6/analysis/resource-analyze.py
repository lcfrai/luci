import json,pathlib,datetime,collections,hashlib,base64,re,statistics
ROOT=pathlib.Path('/workspace/luci');BASE=ROOT/'artifacts/quality-swarm/sustained-0dcafb14-attempt6';OUT=pathlib.Path('<PRIVATE_TOOLING_DIRECTORY>/publish-sustained6/analysis-prep')
def h(b):return hashlib.sha256(b).hexdigest()
def dt(s):return datetime.datetime.fromisoformat(s.replace('Z','+00:00'))
d=json.loads((BASE/'result/result.json').read_text());samples=[json.loads(x) for x in (BASE/'observer/samples.jsonl').read_text().splitlines()]; rows=[]
for i,s in enumerate(samples):
 c=s.get('controlPlaneCgroupMemory',{});obs=c.get('observations',[])
 rows.append({'sampleIndex':i,'observedAt':s['observedAt'],'status':c.get('status'),'durationSeconds':c.get('durationSeconds'),'physicalAvailableBytes':s['host']['memory'].get('MemAvailable'),'summedProcessRSSBytes':s.get('summedProcessRSSBytes'),'controlPlaneRSSBytes':s.get('summedProcessRSSBytesByRole',{}).get('control-plane'),'observations':[{k:o.get(k) for k in ['pid','startTicks','status','identityAndTopologyStable','productComparableEffectiveTotalBytes','productComparableEffectiveAvailableBytes','issues']} for o in obs]})
def summarise(ss):
 vals=[o['productComparableEffectiveAvailableBytes'] for r in ss for o in r['observations'] if o.get('status')=='observed' and isinstance(o.get('productComparableEffectiveAvailableBytes'),int)]
 physical=[r['physicalAvailableBytes'] for r in ss if isinstance(r['physicalAvailableBytes'],int)]
 return {'samples':len(ss),'statusCounts':dict(collections.Counter(r['status'] for r in ss)),'validObservationCount':len(vals),'effectiveAvailableMinBytes':min(vals) if vals else None,'effectiveAvailableMaxBytes':max(vals) if vals else None,'physicalAvailableMinBytes':min(physical) if physical else None,'physicalAvailableMaxBytes':max(physical) if physical else None,'firstObservedAt':ss[0]['observedAt'] if ss else None,'lastObservedAt':ss[-1]['observedAt'] if ss else None}
phases=[]
for p in d['phases']:
 a=dt(p['startedAt']);end=p.get('finishedAt') or next(json.loads(line)['at'] for line in (BASE/'result/diagnostic-journal.jsonl').read_text().splitlines() if json.loads(line).get('event')=='driver-caught-failure');z=dt(end)
 ss=[r for r in rows if a<=dt(r['observedAt'])<z];phases.append({'name':p['name'],'intervalStart':p['startedAt'],'intervalEnd':end,'endBasis':'Recorded phase end' if p.get('finishedAt') else 'Driver-caught-failure journal event; incomplete loaded phase',**summarise(ss)})
def bracket(at):
 t=dt(at);bef=[r for r in rows if dt(r['observedAt'])<=t and any(o.get('status')=='observed' for o in r['observations'])];aft=[r for r in rows if dt(r['observedAt'])>t and any(o.get('status')=='observed' for o in r['observations'])]
 def v(r):return {**r,'deltaMsFromEvent':(dt(r['observedAt'])-t).total_seconds()*1000} if r else None
 return {'before':v(bef[-1] if bef else None),'after':v(aft[0] if aft else None)}
events=[]
for x in d['dockerTransportObservations']['requests']:
 if x['method']=='POST':
  for k in ['requestedAt','respondedAt','deliveredAt']:
   if k in x:events.append({'name':f"{x['path'].split('/')[-1]}-{k}",'at':x[k],'requestSequence':x['requestSequence'],'bracket':bracket(x[k])})
for k in ['startedAt','finishedAt']:
 at=d['activeRestarts'][0][k];events.append({'name':'active-restart-'+k,'at':at,'bracket':bracket(at)})
spans=[r['durationSeconds'] for r in rows if isinstance(r.get('durationSeconds'),(int,float))]
def pct(v,p):return sorted(v)[max(0,__import__('math').ceil(len(v)*p)-1)] if v else None
out={'source':'Derived read-only analysis of byte-bound raw attempt 6 receipts, not a new run or atomic admission snapshot','observedSampleCount':len(rows),'overall':summarise(rows),'controlPlaneIdentities':sorted({(o['pid'],o['startTicks']) for r in rows for o in r['observations']}),'cgroupObservationDurationSeconds':{'p50':pct(spans,.5),'p95':pct(spans,.95),'max':max(spans)},'phaseSummaries':phases,'eventBrackets':events,'limitations':['Samples are sequential approximately 1 Hz observations; nearest bracketing counters are not the actual admission decision or an atomic measurement.','The cgroup and physical counters measure different scopes. The effective formula clamps physical available against visible cgroup limits minus current; no reclaimable-cache discount is applied.','The host cgroup includes unrelated workload, retained RAM-backed files and observation work. It is not memory attributable solely to Luci.','Process RSS sums include test driver/browser and double-count shared pages. Container memory is separate; do not add overlapping values indiscriminately.','Missing/identity-only rows are explicitly retained. Visible ancestors only; no claim about hidden cgroups.','No matched no-observer run or causal observer overhead estimate is available. No whole-computer or Windows responsiveness is established.'],'bindings':{f:{'bytes':(BASE/f).stat().st_size,'sha256':h((BASE/f).read_bytes())} for f in ['observer/samples.jsonl','observer/metadata.json','observer/summary.json','result/result.json','result/diagnostic-journal.jsonl']}}
formula=[];valid=0
for i,s in enumerate(samples):
 for o in s.get('controlPlaneCgroupMemory',{}).get('observations',[]):
  if o.get('status')!='observed':continue
  valid+=1
  nodes=[n for n in o['nodes']if n.get('limitStatus')=='finite']
  assert all(n.get('currentStatus')=='observed'for n in nodes)
  total=min([o['physicalTotalBytes']]+[n['limitBytes']for n in nodes])
  available=min([o['physicalAvailableBytes']]+[max(0,n['limitBytes']-n['currentBytes'])for n in nodes])
  if total!=o['productComparableEffectiveTotalBytes']or available!=o['productComparableEffectiveAvailableBytes']:formula.append({'sampleIndex':i,'pid':o['pid'],'startTicks':o['startTicks']})
out['formulaRecomputation']={'observations':valid,'mismatches':formula,'method':'Minimum physical and each visible finite ancestor; no cache discount or atomic admission claim.'}
durations=[s['sampleDurationSeconds']for s in samples if isinstance(s.get('sampleDurationSeconds'),(int,float))]
out['wholeSamplerDurationSeconds']={'p50':pct(durations,.5),'p95':pct(durations,.95),'max':max(durations)}
containers=collections.defaultdict(list)
for s in samples:
 for c in s.get('containers',[]):containers[c['id']].append(c)
out['containerSampleCoverage']=[{'containerId':identity,'identityRows':len(values),'usageRows':sum(isinstance(v.get('memoryUsageBytes'),int)for v in values),'peakCgroupUsageBytes':max([v['memoryUsageBytes']for v in values if isinstance(v.get('memoryUsageBytes'),int)],default=None)}for identity,values in containers.items()]
out['observerSelfCost']=json.loads((BASE/'observer-observer-self-cost.json').read_text())
out['bindings']['observer-observer-self-cost.json']={'bytes':(BASE/'observer-observer-self-cost.json').stat().st_size,'sha256':h((BASE/'observer-observer-self-cost.json').read_bytes())}
out['sampledOwnedProcessRSSPeakBytes']=max(s['summedProcessRSSBytes']for s in samples)
out['sampledRSSByRolePeaksBytes']={role:max(s.get('summedProcessRSSBytesByRole',{}).get(role,0)for s in samples)for role in sorted({role for s in samples for role in s.get('summedProcessRSSBytesByRole',{})})}
out['analysisProducedAt']=datetime.datetime.now(datetime.timezone.utc).isoformat()
out['analyzerSHA256']=h(pathlib.Path(__file__).read_bytes())
(OUT/'resource-analysis.json').write_text(json.dumps(out,indent=2)+'\n')
(OUT/'interaction-summary.json').write_text(json.dumps({'source':'Extracted unchanged from final result; targets evaluated separately, partial run remains failed','summary':d['interactionSummary'],'predeclaredTargets':{'switchP95Ms':300,'inputP95Ms':100,'allMeasuredActionP99Ms':1000},'basis':'Automation command through expected DOM content at two animation frames. Includes RPC/DOM/waits; not physical input-to-display latency.','completeSustainedAcceptance':False,'sourceResultSHA256':h((BASE/'result/result.json').read_bytes())},indent=2)+'\n')
print(json.dumps({'overall':out['overall'],'phases':phases,'sampler':out['cgroupObservationDurationSeconds'],'identities':out['controlPlaneIdentities']},indent=2))
