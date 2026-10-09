"""Independent one-row-at-a-time arithmetic audit; no imports from workload/sampler."""
import collections as C
import datetime as D
import hashlib
import json
import math
from pathlib import Path

BASE=Path('<PRIVATE_RUN_EVIDENCE_DIRECTORY>')
OUT=Path('<PRIVATE_TOOLING_DIRECTORY>/sustained8-resource-analysis/independent-resource-audit.json')
MAX_ROWS=10000; MAX_LINE=1048576; MAX_IDENTITIES=4096

def binding(path):
 h=hashlib.sha256(); size=0
 with path.open('rb') as f:
  for b in iter(lambda:f.read(65536),b''):h.update(b);size+=len(b)
 return {'bytes':size,'sha256':h.hexdigest()}
def stamp(s):return D.datetime.fromisoformat(s.replace('Z','+00:00'))
def stats(a):
 a=sorted(a)
 return {'count':len(a),'min':a[0] if a else None,'p50':a[math.ceil(len(a)*.5)-1] if a else None,'p95':a[math.ceil(len(a)*.95)-1] if a else None,'max':a[-1] if a else None}
def numeric(v):return type(v) in (int,float) and math.isfinite(v)

files=['observer/metadata.json','observer/summary.json','observer/accepted-sampler.py','observer-observer-self-cost.json']
bindings={n:binding(BASE/n) for n in files}
meta=json.loads((BASE/files[0]).read_text()); summary=json.loads((BASE/files[1]).read_text()); own=json.loads((BASE/files[3]).read_text())
errors=[]; checks=0

def check(ok,code,**extra):
 global checks
 checks+=1
 if not ok:
  if len(errors)>=100:raise RuntimeError('too many discrepancies')
  errors.append({'code':code,**extra})

check(meta['helperSHA256']==bindings['observer/accepted-sampler.py']['sha256'],'sampler hash')
hz=own['clockTicksPerSecond'];assert numeric(hz) and hz>0
counts=C.Counter(); phase=C.Counter(); statuses=C.Counter(); obsstatuses=C.Counter(); issues=C.Counter(); node_statuses=C.Counter(); missing=C.Counter()
values=C.defaultdict(list); peaks=C.Counter(); totals=C.Counter(); roles=C.Counter(); proc={}; prev={}; containers={}; gaps=[]; containergaps=[]; first=None; last=None; prior=None; memory_formula_checked=0
sha=hashlib.sha256(); size=0; largest=0; previous_host_cpu=None
with (BASE/'observer/samples.jsonl').open('rb') as f:
 while True:
  raw=f.readline(MAX_LINE+1)
  if not raw:break
  assert len(raw)<=MAX_LINE and counts['samples']<MAX_ROWS
  largest=max(largest,len(raw));sha.update(raw);size+=len(raw)
  row=json.loads(raw); i=counts['samples']; counts['samples']+=1; phase[row['phase']]+=1
  at=row['observedAt']; t=row['elapsedSeconds']
  if first is None:first={'index':i,'observedAt':at,'elapsedSeconds':t}
  last={'index':i,'observedAt':at,'elapsedSeconds':t}
  if prior:
   delta=t-prior['elapsedSeconds']; wall=(stamp(at)-stamp(prior['observedAt'])).total_seconds()
   check(delta>0,'non-increasing sample time',index=i)
   values['sampleGapSeconds'].append(delta);values['wallGapSeconds'].append(wall)
   if delta>meta['intervalSeconds']*1.5:gaps.append({'previousIndex':i-1,'index':i,'elapsedGapSeconds':delta,'wallGapSeconds':wall})
  prior=last
  values['sampleDurationSeconds'].append(row['sampleDurationSeconds'])
  if row['sampleDurationSeconds']>meta['intervalSeconds']:counts['sampleDurationAboveNominalInterval']+=1
  issues.update(row.get('containerIssues',[])); counts['unreadableOrRacedProcessStatRows']+=row.get('processStatUnreadableOrRacedCount',0)
  if row.get('processStatUnreadableOrRacedCount',0)>0:counts['samplesWithUnreadableOrRacedProcessStat']+=1
  mem=row['host'].get('memory') or {};counts['physicalCountersValidated:'+str(mem.get('physicalCountersValidated'))]+=1
  for k in ['MemTotal','MemAvailable','SwapTotal','SwapFree','CommitLimit','Committed_AS']:
   if numeric(mem.get(k)):values['hostMemory:'+k].append(mem[k])
   else:missing['hostMemory:'+k]+=1
  for k,v in (row['host'].get('pressure') or {}).items():missing['pressure:'+k+(' unavailable' if v is None else ' observed')]+=1
  for k,v in (row['host'].get('disks') or {}).items():
   if v is None:missing['disk:'+k]+=1
   else:values['diskAvailableBytes:'+k].append(v['availableBytes'])
  hostcpu=row['host'].get('cpuTicks')
  if hostcpu and previous_host_cpu:
   d=[x-y for x,y in zip(hostcpu,previous_host_cpu)]
   if min(d)>=0 and sum(d)>0:values['hostCPUPercentBetweenSamples'].append(100*(sum(d)-d[3]-d[4])/sum(d))
   else:missing['hostCPUUnusableDelta']+=1
  previous_host_cpu=hostcpu
  rss=C.Counter();identities=set()
  for p in row['processes']:
   key=(p['pid'],p['startTicks']);check(key not in identities,'duplicate process identity',index=i);identities.add(key)
   assert len(proc)<MAX_IDENTITIES or key in proc
   rec=proc.setdefault(key,{'pid':key[0],'startTicks':key[1],'role':p['role'],'firstIndex':i,'firstObservedAt':at,'observations':0,'peakRSSBytes':0,'cpuTicksDelta':0,'ioCounterDeltas':C.Counter(),'stateCounts':C.Counter(),'missingIOSamples':0,'observationGaps':0})
   check(rec['role']==p['role'],'identity role changed',index=i,pid=key[0]);rec['observations']+=1;rec['lastIndex']=i;rec['lastObservedAt']=at;rec['lastState']=p['state'];rec['lastParentPid']=p['parentPid'];rec['stateCounts'][p['state']]+=1;rec['peakRSSBytes']=max(rec['peakRSSBytes'],p['residentBytes'])
   if p.get('io') is None:rec['missingIOSamples']+=1
   if key in prev:
    old=prev[key];d=p['cpuTicks']-old['cpuTicks'];check(d>=0,'process cpu counter reset',index=i,pid=key[0]);d=max(0,d);totals['cpuTicks']+=d;roles[p['role']]+=d;rec['cpuTicksDelta']+=d
    if old['index']!=i-1:rec['observationGaps']+=1
    if p.get('io') is not None and old.get('io') is not None:
     expected={k:max(0,p['io'][k]-old['io'][k]) for k in ('read_bytes','write_bytes','rchar','wchar')}
     check(expected==p.get('ioDelta'),'process IO delta mismatch',index=i,pid=key[0]);totals.update(expected);rec['ioCounterDeltas'].update(expected)
     for k in expected:
      if p['io'][k]<old['io'][k]:counts['processIOCounterDecreases:'+k]+=1
    else:check('ioDelta' not in p,'unexpected IO delta with missing observations',index=i,pid=key[0])
   else:check('ioDelta' not in p and 'cpuCoreEquivalent' not in p,'first process counter already has delta',index=i,pid=key[0])
   prev[key]={'index':i,'cpuTicks':p['cpuTicks'],'io':p.get('io')};rss[p['role']]+=p['residentBytes']
  check(dict(rss)==row['summedProcessRSSBytesByRole'],'per-role RSS mismatch',index=i);check(sum(rss.values())==row['summedProcessRSSBytes'],'total RSS mismatch',index=i)
  for role,n in rss.items():peaks['summedRSS_'+role]=max(peaks['summedRSS_'+role],n)
  peaks['summedRSS_allOwnedProcesses']=max(peaks['summedRSS_allOwnedProcesses'],sum(rss.values()));values['ownedSummedRSSBytes'].append(sum(rss.values()))
  check(row['driverAlive']==any(p['pid']==meta['driver']['pid'] and p['startTicks']==meta['driver']['startTicks'] and p['state']!='Z' for p in row['processes']),'driver state mismatch',index=i)
  if not row['driverAlive']:counts['driverAbsentSamples']+=1
  cg=row.get('controlPlaneCgroupMemory',{});statuses[cg.get('status','missing')]+=1;issues.update('cgroup:'+x for x in cg.get('issues',[]))
  for k in ('durationSeconds','readCount','readBytes'):
   if numeric(cg.get(k)):values['cgroup:'+k].append(cg[k])
  for ob in cg.get('observations',[]):
   counts['cgroupObservationRows']+=1;obsstatuses[ob.get('status','missing')]+=1;issues.update('cgroup:'+x for x in ob.get('issues',[]))
   check((ob['pid'],ob['startTicks']) in identities,'cgroup observation outside exact retained process identities',index=i)
   effective=ob.get('productComparableEffectiveAvailableBytes'); total=ob.get('productComparableEffectiveTotalBytes')
   if numeric(effective):values['effectiveAvailableBytes'].append(effective)
   if numeric(total):values['effectiveTotalBytes'].append(total)
   finite=[]
   for n in ob.get('nodes',[]):
    node_statuses[n['limitStatus']+'/'+n['currentStatus']]+=1
    if n['limitStatus']=='finite':
     finite.append(n)
     if type(n.get('currentBytes')) is int:check(n.get('limitMinusCurrentBytes')==max(0,n['limitBytes']-n['currentBytes']),'node headroom mismatch',index=i)
   if ob.get('status')=='observed':
    check(ob.get('identityAndTopologyStable') is True,'observed cgroup identity not stable',index=i)
    usable=type(ob.get('physicalTotalBytes')) is int and type(ob.get('physicalAvailableBytes')) is int and all(type(n.get('limitBytes')) is int and type(n.get('currentBytes')) is int for n in finite)
    check(usable,'observed formula operands unavailable',index=i)
    if usable:
     expectedTotal=min([ob['physicalTotalBytes']]+[n['limitBytes'] for n in finite]);expectedAvailable=min([ob['physicalAvailableBytes']]+[max(0,n['limitBytes']-n['currentBytes']) for n in finite]);memory_formula_checked+=1
     check(expectedTotal==total and expectedAvailable==effective,'effective cgroup formula mismatch',index=i)
     check(ob['physicalTotalBytes']==mem.get('MemTotal') and ob['physicalAvailableBytes']==mem.get('MemAvailable'),'physical cgroup/host mismatch',index=i)
   else:
    counts['nonObservedCgroupRows']+=1
    if ob.get('identityAndTopologyStable') is not True:check(total is None and effective is None,'unstable cgroup advertised effective memory',index=i)
  seen=set()
  for c in row['containers']:
   cid=c['id'];check(cid not in seen,'duplicate container identity',index=i);seen.add(cid)
   assert len(containers)<MAX_IDENTITIES or cid in containers
   r=containers.setdefault(cid,{'id':cid,'firstIndex':i,'firstObservedAt':at,'identitySamples':0,'usageSamples':0,'cpuSamples':0,'peakUsageBytes':None,'lastCPU':None,'cpuNanosecondsDelta':0,'cpuCounterDecreases':0,'pidIdentities':set(),'blockIOObservedSamples':0,'lastBlockIO':None,'blockIODeltas':C.Counter(),'blockIOCounterDecreases':C.Counter(),'observationGaps':0})
   if r['identitySamples'] and r['lastIndex']!=i-1:r['observationGaps']+=1
   r['identitySamples']+=1;r['lastIndex']=i;r['lastObservedAt']=at;r['lastRunning']=c['running'];r['lastPID']=c.get('pid');r['lastPIDStartTicks']=c.get('pidStartTicks')
   if c.get('pid'):r['pidIdentities'].add((c['pid'],c.get('pidStartTicks')))
   if type(c.get('memoryUsageBytes')) is int:
    r['usageSamples']+=1;r['peakUsageBytes']=max(r['peakUsageBytes'] or 0,c['memoryUsageBytes']);peaks['singleContainerCgroupUsageBytes']=max(peaks['singleContainerCgroupUsageBytes'],c['memoryUsageBytes'])
   if type(c.get('cpuUsageNanoseconds')) is int:
    r['cpuSamples']+=1
    if r['lastCPU'] is not None:
     d=c['cpuUsageNanoseconds']-r['lastCPU']
     if d<0:r['cpuCounterDecreases']+=1
     else:r['cpuNanosecondsDelta']+=d
    r['lastCPU']=c['cpuUsageNanoseconds']
   if isinstance(c.get('blockIOBytes'),dict):
    r['blockIOObservedSamples']+=1
    if r['lastBlockIO'] is not None:
     for k,v in c['blockIOBytes'].items():
      d=v-r['lastBlockIO'][k]
      if d<0:r['blockIOCounterDecreases'][k]+=1
      else:r['blockIODeltas'][k]+=d
    r['lastBlockIO']=c['blockIOBytes']
  if not row['containers']:counts['samplesWithoutObservedContainers']+=1
  finalprocs=[{'pid':p['pid'],'startTicks':p['startTicks'],'role':p['role'],'state':p['state'],'parentPid':p['parentPid'],'residentBytes':p['residentBytes']} for p in row['processes']]
  finalcontainers=row['containers']

bindings['observer/samples.jsonl']={'bytes':size,'sha256':sha.hexdigest()}
check(counts['samples']==summary['samples'],'summary sample count')
check(dict(peaks)==summary['peakObserved'],'summary peaks')
expectedCounters={'observedProcessCPUSeconds':totals['cpuTicks']/hz,**{'observedProcessIO_'+k:totals[k] for k in ('read_bytes','write_bytes','rchar','wchar')}}
for k,v in expectedCounters.items():check(math.isclose(v,summary['observedCounterDeltas'].get(k,float('nan')),rel_tol=1e-12,abs_tol=1e-8),'summary counter',counter=k,expected=v,recorded=summary['observedCounterDeltas'].get(k))
expectedIdentities=sorted((p['pid'],p['startTicks'],p['role']) for p in proc.values()); reportedIdentities=sorted((p['pid'],p['startTicks'],p['role']) for p in summary['processIdentities'])
check(expectedIdentities==reportedIdentities,'summary process identities');check(sorted(containers)==summary['containerIds'],'summary containers');check(bool(counts['driverAbsentSamples'])==summary['driverExitObserved'],'summary driver exit')
check(summary['durationSeconds']>=last['elapsedSeconds'],'summary duration precedes sample');check(stamp(summary['finishedAt'])>=stamp(last['observedAt']),'summary finish precedes sample')
check(own['exitCode']==0 and own['status']=='completed','observer wrapper status')
selfcpu=own['userCPUSeconds']+own['systemCPUSeconds'];check(math.isclose(selfcpu/own['wallSeconds'],own['cpuSecondsPerWallSecond'],rel_tol=1e-12),'self-cost CPU ratio')
check(summary['limitations']==meta['limitations'],'summary limits');check(summary['baseline']==meta['baseline'],'summary baseline')
for k,r in proc.items():r['cpuSecondsDelta']=r['cpuTicksDelta']/hz;r['presentInFinalSample']=any(p['pid']==k[0] and p['startTicks']==k[1] for p in finalprocs)
for r in containers.values():r['pidIdentities']=[{'pid':p,'startTicks':t} for p,t in sorted(r['pidIdentities'])];r['presentInFinalSample']=any(c['id']==r['id'] for c in finalcontainers)
# Ensure the audited inputs were not concurrently rewritten. Rehash is read-only.
post={n:binding(BASE/n) for n in bindings}
for n in bindings:check(post[n]==bindings[n],'input changed during audit',path=n)
report={'schemaVersion':1,'scope':'Independent bounded stream audit of retained Linux sustained8 resource evidence; no new runtime tests, no private fixture read','status':'passed' if not errors else 'failed','createdAt':D.datetime.now(D.timezone.utc).isoformat(),'bindings':bindings,'sourceInputsStable':post==bindings,'analyzer':binding(Path(__file__)),'bounds':{'maxRows':MAX_ROWS,'maxLineBytes':MAX_LINE,'maxIdentities':MAX_IDENTITIES,'largestObservedLineBytes':largest,'policy':'One raw sample retained at once; bounded scalar lists and identity aggregates only'},'arithmeticChecks':checks,'discrepancies':errors,'counts':dict(counts),'phases':dict(phase),'sampleCgroupStatuses':dict(statuses),'cgroupObservationStatuses':dict(obsstatuses),'cgroupNodeStatuses':dict(node_statuses),'formulaObservationsChecked':memory_formula_checked,'unavailable':dict(missing),'issues':dict(issues),'first':first,'last':last,'publishedDurationSeconds':summary['durationSeconds'],'gapsAbove1_5Seconds':gaps,'distributions':{k:stats(v) for k,v in values.items()},'recomputedPeaks':dict(peaks),'recomputedCounters':expectedCounters,'counterCPUSecondsByRole':{k:v/hz for k,v in roles.items()},'processes':sorted(proc.values(),key=lambda p:(p['pid'],p['startTicks'])),'containers':sorted(containers.values(),key=lambda p:p['id']),'finalObservedProcesses':finalprocs,'finalObservedContainers':finalcontainers,'selfCost':{'receipt':own,'recomputedCPUSeconds':selfcpu,'recomputedCPUSecondsPerWallSecond':selfcpu/own['wallSeconds'],'scope':'Observer interpreter only; excludes spawned children and Docker daemon; not subtracted from resource counters'},'limits':['Source product-basis hashes are retained metadata; no compiler/product archive identity proof in this audit.','Process CPU tick and IO deltas recomputed by PID/start identity across observed intervals, excluding first observation lifetime; process core-equivalent rates cannot be reconstructed exactly because per-process monotonic clocks were not serialized.','Container observations are distinct Docker cgroup counters and must not be added to process RSS; working-set calculation cannot be independently reconstructed without raw inactive-file counters.','Container CPU/block IO deltas describe retained cumulative counters per exact container ID; gaps, missing readings and PID identity changes remain explicit.','Summed process RSS can double-count shared pages; it is not unique physical RAM or whole-computer impact.','No atomic admission operands, UI latency, Windows behavior, provider quality or workload output verification is established.','Sequential sampled cgroup effective memory applies independent visible physical/ancestor minima; no hidden ancestors or contemporaneous admission decisions inferred.','Final sample state and absence are retained observations, not current census or successful product resource/lease cleanup.','Observer self-cost excludes children and daemon; host values include unrelated work; no overhead subtraction.','1 Hz sampling may miss short-lived processes and container transitions; post-driver-exit settling is not a pre-run idle baseline.']}
OUT.write_text(json.dumps(report,indent=2)+'\n');OUT.chmod(0o600)
print(json.dumps({'output':str(OUT),'sha256':binding(OUT)['sha256'],'status':report['status'],'samples':counts['samples'],'formulaRows':memory_formula_checked,'cgroupStatuses':dict(statuses),'formulaStatuses':dict(obsstatuses),'errors':errors,'gaps':len(gaps),'counts':dict(counts),'peak':dict(peaks),'cpuSeconds':expectedCounters['observedProcessCPUSeconds'],'effectiveMin':report['distributions'].get('effectiveAvailableBytes',{}).get('min'),'lastProcesses':finalprocs},indent=2))
