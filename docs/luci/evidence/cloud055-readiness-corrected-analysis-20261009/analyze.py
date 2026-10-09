#!/usr/bin/env python3
"""Bounded offline allowlisted analysis; no app, network or process actions."""
import argparse, collections, hashlib, json, math, re
from pathlib import Path
PRIMARY='idle-interactive-calibration'; TRACE='idle-actionability-trace-followup'
PHASES={PRIMARY,TRACE}; KINDS={'project-switch','input','draft-reset'}
STAGES={'action-start','action-end','verify-start','verify-end','frames-start','frames-end','all-projects-click-end','overview-heading-ready','open-project-click-end'}
STATUS={'ready','censored','cancelled','closed'}
REASONS={'predicate-observed','sample-or-time-cap','wall-time-cap','node-cap','control-cap','ancestor-cap','ambiguous-root','missing-root','root-replaced','observed-disconnection','missing-editor','ambiguous-editor','retained-editor','editor-replaced','definition-replaced','read-error','superseded','home-not-observed','cohort-end','action-failed','authority-changed','source-changed','document-leaving','driver-cancelled'}
NUMERIC={'generation','round','projectOrdinal','clickAt','observedAt','samples','firstSampleAt','lastSampleAt','maxSampleGapMs','maxNodesVisited','maxControlsVisited','sequence','at','authorityGeneration'}
FROZEN=Path('/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/capsule')
def finite(v):return isinstance(v,(int,float)) and not isinstance(v,bool) and math.isfinite(v)
def number(v):return v if finite(v) else None
def enum(v,allowed):return v if isinstance(v,str) and v in allowed else 'unknown'
def quantiles(values):
 values=sorted(v for v in values if finite(v));n=len(values)
 return {'count':n,**{label:values[max(0,math.ceil(n*q)-1)] if n else None for label,q in [('p50Ms',.5),('p95Ms',.95),('p99Ms',.99),('maxMs',1)]}}
def digest(data):return hashlib.sha256(data).hexdigest()
def load(path,maximum=64*1024**2):
 assert path.is_file() and not path.is_symlink() and path.stat().st_size<=maximum,'Bounded ordinary input file required'
 data=path.read_bytes();return json.loads(data),{'sha256':digest(data),'bytes':len(data)}
def reference(value):
 if not isinstance(value,dict):return None
 return {**{k:number(value.get(k)) for k in ['sequence','at']},**({'actionKind':enum(value.get('actionKind'),{'input','project-switch'})} if 'actionKind'in value else {})}
def frames(value):
 if not isinstance(value,dict):return None
 refs=value.get('references') or {}
 return {**{k:number(value.get(k)) for k in ['requestedAt','firstFrameAt','secondFrameAt']},'references':{'timeOrigin':number(refs.get('timeOrigin')),**{k:reference(refs.get(k)) for k in ['input','pointer','expectedDOM']}}}
def analyze(raw):
 issues=[]
 def issue(code,index=None):
  if len(issues)<1000:issues.append({'code':code,**({'index':index} if index is not None else {})})
 original=raw.get('interactions',[]);assert isinstance(original,list) and len(original)<=6000,'Interaction bound exceeded'
 first=[r.get('projectID') for r in original if r.get('phase')==PRIMARY and r.get('kind')=='project-switch'][:4]
 mapping={v:i for i,v in enumerate(first)} if len(first)==4 and all(isinstance(v,str) and v for v in first) and len(set(first))==4 else {}
 if len(mapping)!=4:issue('owner-order-unavailable')
 projection=[];invalid_rounds=set();rounds=collections.defaultdict(lambda:-1);position=collections.defaultdict(int)
 for index,row in enumerate(original):
  phase=enum(row.get('phase'),PHASES);kind=enum(row.get('kind'),KINDS)
  if kind=='project-switch':rounds[phase]+=1;position[phase]=0
  pos=position[phase];position[phase]+=1;ordinal=mapping.get(row.get('projectID'));round_no=rounds[phase]
  if phase=='unknown' or kind=='unknown':issue('unknown-action-class',index)
  if pos>=4 or kind!=['project-switch','input','input','draft-reset'][min(pos,3)]:issue('action-order-mismatch',index);invalid_rounds.add((phase,round_no))
  if ordinal is None or round_no<0 or ordinal!=round_no%4:issue('project-order-mismatch',index);invalid_rounds.add((phase,round_no))
  diagnostics=row.get('diagnostics') or {};stages=[]
  for item in diagnostics.get('nodeStages',[]):
   if len(stages)>=12:issue('node-stage-bound',index);break
   stages.append({'stage':enum(item.get('stage'),STAGES),'offsetMs':number(item.get('offsetMs'))})
  if not finite(row.get('elapsedMs')):issue('duration-unavailable',index)
  value={'index':index,'at':row.get('at') if isinstance(row.get('at'),str) and re.fullmatch(r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z',row.get('at')) else None,'nodeClockReads':number(diagnostics.get('nodeClockReads')),'phase':phase,'kind':kind,'round':round_no,'positionInRound':pos,'projectOrdinal':ordinal,'success':row.get('success') is True,'elapsedMs':number(row.get('elapsedMs')),'errorPresent':bool(row.get('error')),'nodeStages':stages,'browserFrames':frames(diagnostics.get('browserFrames'))}
  bystage={x['stage']:x['offsetMs'] for x in stages};segments={}
  for name,a,b in [('allProjectsCommand','action-start','all-projects-click-end'),('headingWait','all-projects-click-end','overview-heading-ready'),('openProjectCommand','overview-heading-ready','open-project-click-end'),('verification','verify-start','verify-end'),('twoFrameCommand','frames-start','frames-end')]:
   if finite(bystage.get(a)) and finite(bystage.get(b)):segments[name]=bystage[b]-bystage[a]
  value['nodeSubspansMs']=segments;projection.append(value)
 # Resolve expected-DOM references from the retained original latency stream.
 marks={};mark_duplicates=set();windows=raw.get('browserEventWindows',[]);assert len(windows)<=2000
 for window in windows:
  latency=window.get('latency') or {};origin=number(latency.get('timeOrigin'));entries=latency.get('rows',[]);assert len(entries)<=2048
  for row in entries:
   if row.get('kind')!='expected-dom':continue
   key=(origin,row.get('sequence'))
   if key in marks:mark_duplicates.add(key)
   marks[key]={'at':number(row.get('at')),'actionKind':enum(row.get('actionKind'),{'project-switch','input'})}
 obs=raw.get('autonomousReadiness') or {};obswindows=obs.get('windows',[]);assert len(obswindows)<=105
 setup=obs.get('setup') or {};origin=number(setup.get('timeOrigin'));authority=number(setup.get('authorityGeneration'));seq=0;observer=[];window_projection=[];stream_valid=True
 for wi,window in enumerate(obswindows):
  if window.get('timeOrigin')!=origin or window.get('authorityGeneration')!=authority:issue('observer-window-clock-or-authority',wi);stream_valid=False
  entries=window.get('rows',[]);assert len(entries)<=1000
  for item in entries:
   seq+=1
   if item.get('sequence')!=seq:issue('observer-sequence-gap',wi);stream_valid=False
   value={key:number(item.get(key)) for key in sorted(NUMERIC)};value.update(stage=item.get('stage') if item.get('stage') in ['home','define',None] else 'unknown',status=enum(item.get('status'),STATUS),reason=enum(item.get('reason'),REASONS),timeOrigin=number(window.get('timeOrigin')),windowIndex=wi)
   if value['status']=='unknown' or value['reason']=='unknown' or value['authorityGeneration']!=authority:issue('observer-row-invalid',seq);stream_valid=False
   observer.append(value)
  if window.get('sequence')!=seq:issue('observer-counter-gap',wi);stream_valid=False
  if window.get('overflow'):issue('observer-overflow',wi);stream_valid=False
  window_projection.append({k:window.get(k) if isinstance(window.get(k),bool) else number(window.get(k)) for k in ['timeOrigin','authorityGeneration','round','stages','totalSamples','sequence','bytes','overflow','lateCallbacks','activeGeneration','closed']})
 # Never sum cumulative window counters.
 last=window_projection[-1] if window_projection else None
 stage_rows=collections.defaultdict(list)
 for row in observer:
  if row['stage'] in ['home','define']:stage_rows[(row['round'],row['stage'])].append(row)
 switches=[r for r in projection if r['phase']==PRIMARY and r['kind']=='project-switch'];joins=[]
 for row in switches:
  r=row['round'];join={'index':row['index'],'round':r,'projectOrdinal':row['projectOrdinal'],'elapsedMs':row['elapsedMs'],'originalSuccess':row['success'],'availability':'unavailable','reasons':[]}
  matches={stage:stage_rows.get((r,stage),[]) for stage in ['home','define']}
  for stage,items in matches.items():
   join[stage+'StageCount']=len(items)
   if len(items)!=1:join['reasons'].append(stage+'-missing' if not items else stage+'-ambiguous')
   elif items[0]['status']!='ready':join['reasons'].append(stage+'-'+items[0]['status'])
  bf=row['browserFrames'] or {};refs=bf.get('references') or {};expected=refs.get('expectedDOM') or {};key=(refs.get('timeOrigin'),expected.get('sequence'));mark=marks.get(key)
  if (PRIMARY,r) in invalid_rounds:join['reasons'].append('round-action-or-owner-order-invalid')
  if not row['success']:join['reasons'].append('original-action-failed')
  if not stream_valid:join['reasons'].append('observer-stream-invalid')
  if row['projectOrdinal']!=r%4:join['reasons'].append('owner-order-invalid')
  if refs.get('timeOrigin')!=origin or origin is None:join['reasons'].append('clock-mismatch')
  if key in mark_duplicates or not mark or mark.get('actionKind')!='project-switch' or expected.get('actionKind')!='project-switch' or mark.get('at')!=expected.get('at'):join['reasons'].append('driver-mark-unresolved')
  chronology=[expected.get('at'),bf.get('requestedAt'),bf.get('firstFrameAt'),bf.get('secondFrameAt')]
  if not all(finite(v) for v in chronology) or chronology!=sorted(chronology):join['reasons'].append('driver-frame-order-invalid')
  if all(len(items)==1 for items in matches.values()):
   home,define=matches['home'][0],matches['define'][0]
   join['home']=home;join['define']=define
   if any(v['projectOrdinal']!=r%4 or v['authorityGeneration']!=authority or v['timeOrigin']!=origin for v in [home,define]) or home['generation']!=r*2+1 or define['generation']!=r*2+2:join['reasons'].append('stage-identity-invalid')
   times=[home['clickAt'],home['observedAt'],define['clickAt'],define['observedAt']]
   if not all(finite(v) for v in times) or times!=sorted(times):join['reasons'].append('stage-order-unavailable')
   if finite(expected.get('at')) and finite(define['clickAt']) and expected['at']<define['clickAt']:join['reasons'].append('driver-mark-before-click')
   if any(v['status']=='closed' and finite(v['at']) and finite(home['clickAt']) and finite(bf.get('secondFrameAt')) and home['clickAt']<=v['at']<=bf['secondFrameAt'] for v in observer):join['reasons'].append('intervening-invalidation')
   home_reasons=[reason for reason in join['reasons'] if reason not in ['define-censored','define-cancelled','stage-order-unavailable']]
   if not home_reasons and home['status']=='ready' and finite(home['observedAt']) and finite(define['clickAt']) and home['clickAt']<=home['observedAt']<=define['clickAt']:
    join.update(homeAvailability='joined',homeObservedToOpenClickMs=define['clickAt']-home['observedAt'],homeClickToObservedMs=home['observedAt']-home['clickAt'],trustedOpenClickToDriverDOMMs=expected['at']-define['clickAt'])
   else:join['homeAvailability']='unavailable'
   if not join['reasons']:
    join.update(availability='joined',driverExpectedDOMAt=expected['at'],defineObservedToDriverMs=expected['at']-define['observedAt'],homeObservedToOpenClickMs=define['clickAt']-home['observedAt'],homeClickToObservedMs=home['observedAt']-home['clickAt'],defineClickToObservedMs=define['observedAt']-define['clickAt'],observedAfterOriginalFrames=define['observedAt']>bf['secondFrameAt'])
  joins.append(join)
 cohorts={}
 for phase in PHASES:
  for kind in KINDS:
   values=[r for r in projection if r['phase']==phase and r['kind']==kind];cohorts[phase+':'+kind]={**quantiles([r['elapsedMs'] for r in values]),'rows':len(values),'successful':sum(r['success'] for r in values),'failed':sum(not r['success'] for r in values)}
 counts=collections.Counter((v['stage'],v['status'],v['reason']) for v in observer if v['stage'] in ['home','define'])
 expected_stage_counts={stage:{'expected':100,'missing':sum(not stage_rows.get((r,stage)) for r in range(100)),'ambiguous':sum(len(stage_rows.get((r,stage),[]))>1 for r in range(100))} for stage in ['home','define']}
 ready=[v for v in joins if v['availability']=='joined'];home_ready=[v for v in joins if v.get('homeAvailability')=='joined'];groups={}
 for key,selector in [('projectOrdinal',lambda x:x['projectOrdinal']),('tenRoundBlock',lambda x:x['round']//10)]:
  groups[key]=[{key:value,'switches':quantiles([r['elapsedMs'] for r in joins if selector(r)==value]),'joinedObservedToDriver':quantiles([r['defineObservedToDriverMs'] for r in ready if selector(r)==value])} for value in sorted(set(selector(r) for r in joins),key=lambda v:-1 if v is None else v)]
 expected_counts={PRIMARY:400,TRACE:32};actual_counts=dict(collections.Counter(r['phase'] for r in projection))
 for phase,count in expected_counts.items():
  if actual_counts.get(phase,0)!=count:issue('incomplete-primary' if phase==PRIMARY else 'incomplete-trace')
 targets={}
 for kind,limit in [('project-switch',300),('input',100)]:
  c=cohorts[PRIMARY+':'+kind];targets[kind]={'p95LimitMs':limit,'p99StrictlyBelowMs':1000,'withinOriginalReference':c['rows']==(100 if kind=='project-switch' else 200) and c['failed']==0 and c['p95Ms'] is not None and c['p95Ms']<=limit and c['p99Ms']<1000,'expectedCount':100 if kind=='project-switch' else 200,'actualCount':c['rows'],'allSuccessful':c['failed']==0}
 summary={'allOriginalRows':len(projection),'expectedRows':432,'actualPhaseCounts':actual_counts,'cohorts':cohorts,'originalTargets':targets,'observerStatus':enum(obs.get('status'),{'not-started','active','closed','overflow','cleanup-unavailable','observer-unavailable','installation-uncertain','installation-uncertain-cleaned'}),'observerLastCumulativeSnapshot':last,'observerStreamValid':stream_valid,'observerRows':len(observer),'stageCounts':[{'stage':k[0],'status':k[1],'reason':k[2],'count':v} for k,v in sorted(counts.items())],'stageCompleteness':expected_stage_counts,'joinedSwitches':len(ready),'unavailableSwitches':len(joins)-len(ready),'signedObservedToDriverMs':quantiles([r['defineObservedToDriverMs'] for r in ready]),'signedLagNegative':sum(r['defineObservedToDriverMs']<0 for r in ready),'signedLagPositive':sum(r['defineObservedToDriverMs']>0 for r in ready),'homeJoinedSwitches':len(home_ready),'homeClickToObservedMs':quantiles([r['homeClickToObservedMs'] for r in home_ready]),'homeObservedToOpenClickMs':quantiles([r['homeObservedToOpenClickMs'] for r in home_ready]),'trustedOpenClickToDriverDOMMs':quantiles([r['trustedOpenClickToDriverDOMMs'] for r in home_ready]),'samplingGaps':{stage:{**quantiles([r['maxSampleGapMs'] for r in observer if r['stage']==stage]),'over50Ms':sum((r['maxSampleGapMs'] or 0)>50 for r in observer if r['stage']==stage),'over100Ms':sum((r['maxSampleGapMs'] or 0)>100 for r in observer if r['stage']==stage)} for stage in ['home','define']},'groups':groups,'slowSwitchIndices':[r['index'] for r in joins if (r['elapsedMs'] or 0)>300],'issues':issues,'limitations':['Original full-span metrics unchanged; no overhead subtraction.','Signed lag compares same-document first observed structural predicate with driver expected-DOM mark, not actual first readiness, physical paint or pure transport.','Home has no equivalent driver browser mark. Reset references are not fresh reset witnesses.','Censoring/late sampling cannot establish late app readiness; trace actions have no observer by design.','Fixed context/authority and helper owner ordering; no standalone server provenance or permission proof.']}
 summary['primarySwitchNodeSubspansMs']={key:quantiles([r['nodeSubspansMs'].get(key) for r in switches]) for key in ['allProjectsCommand','headingWait','openProjectCommand','verification','twoFrameCommand']}
 summary['slowVsOtherObservedToDriver']={label:quantiles([r['defineObservedToDriverMs'] for r in ready if ((r['elapsedMs'] or 0)>300)==slow]) for label,slow in [('over300Ms',True),('atMost300Ms',False)]}
 summary['topSwitchIndices']=[r['index'] for r in sorted(joins,key=lambda r:r['elapsedMs'] or -1,reverse=True)[:20]]
 return {'summary':summary,'interactions':projection,'observerRows':observer,'observerWindows':window_projection,'switchJoins':joins}
def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('result',type=Path);parser.add_argument('output',type=Path);args=parser.parse_args();assert not args.output.exists(),'Fresh output only'
 raw,binding=load(args.result);output=analyze(raw)
 helper_names=['sustained.mjs','interaction-metrics.mjs','latency-diagnostics.mjs','autonomous-readiness.mjs','readiness-ledger.mjs','external-source-binding.json']
 sources={}
 for name in helper_names:
  frozen=(FROZEN/'actual'/name).read_bytes();accepted=args.result.parent/'accepted-harness'/name
  sources[name]={'frozenSHA256':digest(frozen),'acceptedSHA256':digest(accepted.read_bytes()) if accepted.is_file() and accepted.stat().st_size<1024**2 else None}
  sources[name]['matches']=sources[name]['frozenSHA256']==sources[name]['acceptedSHA256']
 binding.update(productSource=raw.get('sourceCommit') if re.fullmatch('[a-f0-9]{40}',raw.get('sourceCommit','')) else None,helperSource=(raw.get('harnessCheckout') or {}).get('commit') if re.fullmatch('[a-f0-9]{40}',(raw.get('harnessCheckout') or {}).get('commit','')) else None,helperSources=sources,analyzerSHA256=digest(Path(__file__).read_bytes()),rawPathPublished=False)
 if not all(v['matches'] for v in sources.values()):output['summary']['issues'].append({'code':'accepted-helper-binding-mismatch'})
 args.output.mkdir(parents=True,mode=0o700)
 for name,value in [('analysis.json',output['summary']),('interactions.projection.json',output['interactions']),('readiness.projection.json',{'rows':output['observerRows'],'windows':output['observerWindows'],'switches':output['switchJoins']}),('original-binding.json',binding)]:
  (args.output/name).write_text(json.dumps(value,indent=2,allow_nan=False)+'\n')
 assert digest(args.result.read_bytes())==binding['sha256'],'Original result changed during analysis'
 print(json.dumps({'output':str(args.output),'rows':len(output['interactions']),'joined':output['summary']['joinedSwitches'],'issues':len(output['summary']['issues']),'originalUnchanged':True}))
if __name__=='__main__':main()
