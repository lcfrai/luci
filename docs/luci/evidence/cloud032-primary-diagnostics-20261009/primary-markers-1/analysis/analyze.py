#!/usr/bin/env python3
"""Bounded, read-only, allowlisted secondary analysis; never an acceptance result."""
from pathlib import Path
import argparse,json,hashlib,math,statistics

def bounded_json(path,cap):
 raw=path.read_bytes()
 if len(raw)>cap:raise ValueError('Declared analysis input bound exceeded: '+path.name)
 return json.loads(raw),{'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
def rank(rows,q):return sorted(rows)[max(0,math.ceil(len(rows)*q)-1)] if rows else None
def union_ms(spans):
 total=0;end=None
 for a,b in sorted(spans):
  if end is None or a>end:total+=b-a;end=b
  elif b>end:total+=b-end;end=b
 return total

def main():
 p=argparse.ArgumentParser();p.add_argument('--receipt',type=Path,required=True);p.add_argument('--profile-dir',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 receipt,rb=bounded_json(a.receipt,64*1024**2)
 trace,tb=bounded_json(a.profile_dir/'browser-timeline.json',32*1024**2)
 cpu,cb=bounded_json(a.profile_dir/'browser-cpu.cpuprofile',12*1024**2)
 status,sb=bounded_json(a.profile_dir/'profiling-status.json',2*1024**2)
 events=trace.get('traceEvents',[])
 if not isinstance(events,list) or len(events)>1000000:raise ValueError('Trace event count exceeds bound')
 rows=[x for x in receipt['interactions'] if x['phase']=='idle-interactive-calibration']
 if len(rows)!=400:raise ValueError('No complete original 400-action primary cohort')
 markers={}
 for e in events:
  name=e.get('name','')
  if isinstance(name,str) and name.startswith(('luci-primary:','luci-latency:')) and isinstance(e.get('ts'),(float,int)):
   markers.setdefault(name,[]).append(e)
 def marker(name):
  found=markers.get(name,[])
  if len(found)!=1:raise ValueError('Expected one marker: '+name+' count='+str(len(found)))
  return found[0]
 browser_windows=[x.get('latency',{}) for x in receipt.get('browserEventWindows',[])]
 passive_rows=[{**x,'_timeOrigin':w.get('timeOrigin')} for w in browser_windows for x in w.get('rows',[]) if x.get('kind') in ('long-task','long-animation-frame','resource')]
 anchors=[];thread=None;result=[]
 # Use BOTH exact frame markers as a measured same-browser mapping witness.
 # Node wall/performance clocks are never used to align these browser intervals.
 for i,row in enumerate(rows,1):
  f=row['diagnostics']['browserFrames'];ref=f['primaryProfile']
  if ref['actionID']!=i or ref['markerOverflow']:raise ValueError('Primary correlation ID or marker bound mismatch')
  first=marker(f'luci-primary:{i}:requested');last=marker(f'luci-primary:{i}:frame-2')
  if (first.get('pid'),first.get('tid'))!=(last.get('pid'),last.get('tid')):raise ValueError('Frame markers differ in renderer thread')
  if thread is None:thread=(first.get('pid'),first.get('tid'))
  if thread!=(first.get('pid'),first.get('tid')):raise ValueError('Renderer changed during primary cohort')
  left=first['ts']/1000-f['requestedAt'];right=last['ts']/1000-f['secondFrameAt'];anchors.extend([left,right])
  r={'ordinal':i,'kind':row['kind'],'elapsedMs':row['elapsedMs'],'success':row['success'],'nodeStages':row['diagnostics']['nodeStages'],'browserFrameMs':f['secondFrameAt']-f['requestedAt'],'traceFrameMarkerSpanMs':(last['ts']-first['ts'])/1000,'browserTraceAnchorOffsetDriftMs':right-left}
  # Existing references choose the exact expected-DOM/input/pointer witness;
  # this is post-dispatch browser work, NOT the whole Node action duration.
  expected=f.get('references',{}).get('expectedDOM')
  if row['kind'] in ('input','project-switch') and expected and expected.get('actionKind')==row['kind']:
   beginref=f['references'].get('input' if row['kind']=='input' else 'pointer')
   if beginref:
    start=marker('luci-latency:'+('input-dispatch' if row['kind']=='input' else 'pointer-dispatch')+':'+str(beginref['sequence']))
    expect=marker('luci-latency:expected-dom:'+str(expected['sequence']))
    lo,hi=start['ts'],last['ts']
    if lo>expect['ts'] or expect['ts']>hi:raise ValueError('Browser witness sequence is not ordered')
    r['postDispatchBrowserMs']=(hi-lo)/1000;r['dispatchToDriverObservedExpectedDOMMs']=(expect['ts']-lo)/1000
    spans={'long-task':[],'long-animation-frame':[]};resource_spans=[];frame_summaries=[]
    # All values below are from the same browser performance clock. Do not
    # subtract Node's elapsed time or use this as a physical-input witness.
    start_ms,end_ms=beginref['at'],f['secondFrameAt']
    for e in passive_rows:
     if e.get('_timeOrigin')!=f['references'].get('timeOrigin'):continue
     ts,dur=e.get('at'),e.get('duration')
     if not isinstance(ts,(int,float)) or not isinstance(dur,(int,float)):continue
     left,right=max(start_ms,ts),min(end_ms,ts+dur)
     if right<=left:continue
     if e['kind'] in spans:spans[e['kind']].append((left,right))
     if e['kind']=='resource':resource_spans.append({'routeClass':e.get('route'),'overlapMs':right-left,'durationMs':dur,'responseStatus':e.get('responseStatus')})
     if e['kind']=='long-animation-frame':frame_summaries.append({k:e.get(k) for k in ['duration','blockingDuration','renderStart','styleAndLayoutStart','scriptCount','observedScripts','scriptObservationTruncated','scriptDurationMs','forcedStyleAndLayoutDurationMs','scriptPauseDurationMs']})
    r['passiveLongObservationUnionMs']={key:union_ms(value) for key,value in spans.items()}
    r['overlappingLongFrameSummaries']=frame_summaries
    r['overlappingKnownResourceClasses']=resource_spans
  result.append(r)
 groups={kind:[x for x in result if x['kind']==kind] for kind in ('project-switch','input','draft-reset')}
 summary={kind:{'count':len(values),'p50Ms':rank([x['elapsedMs'] for x in values],.5),'p95Ms':rank([x['elapsedMs'] for x in values],.95),'p99Ms':rank([x['elapsedMs'] for x in values],.99)} for kind,values in groups.items()}
 selected=sorted({x['ordinal'] for kind in ('project-switch','input','draft-reset') for x in sorted(groups[kind],key=lambda x:x['elapsedMs'],reverse=True)[:max(1,math.ceil(len(groups[kind])*.05))]})
 # No URLs, function names, source strings, requests or private app contents are
 # exported. The exact CPU file stays private for bundle-offset inspection.
 cpu_nodes={x['id']:x for x in cpu.get('nodes',[])};samples=cpu.get('samples',[]);deltas=cpu.get('timeDeltas',[])
 if len(samples)!=len(deltas):raise ValueError('CPU sample/delta count mismatch')
 if not all(isinstance(x,(float,int)) and not isinstance(x,bool) and math.isfinite(x) for x in deltas):raise ValueError('CPU delta shape invalid')
 negative_deltas=[x for x in deltas if x<0]
 missing_nodes=sum(sid not in cpu_nodes for sid in samples)
 weighted_cpu_allowed=not negative_deltas and missing_nodes==0
 counts={}
 for sid,delta in zip(samples,deltas) if weighted_cpu_allowed else []:
  frame=cpu_nodes.get(sid,{}).get('callFrame',{});key=(hashlib.sha256(str(frame.get('url','')).encode()).hexdigest(),frame.get('lineNumber'),frame.get('columnNumber'))
  counts[key]=counts.get(key,0)+delta
 out={'version':3,'scope':'Instrumented primary400 diagnosis only; no acceptance or causal attribution','inputs':{'receipt':rb,'timeline':tb,'cpu':cb,'status':sb},'captureComplete':status.get('complete') is True,'primarySummary':summary,'browserTraceAnchors':{'count':len(anchors),'minOffsetMs':min(anchors),'maxOffsetMs':max(anchors),'spreadMs':max(anchors)-min(anchors),'medianOffsetMs':statistics.median(anchors),'note':'User timing marker witnesses only. No Node/browser cross-clock subtraction. Offset drift is exposed, never silently assumed zero.'},'slowCohortOrdinals':selected,'slowCohorts':[r for r in result if r['ordinal'] in selected],'passiveSupport':{'windows':len(browser_windows),'longTaskUnsupportedWindows':sum(w.get('longTaskUnsupported',True) for w in browser_windows),'longAnimationFrameUnsupportedWindows':sum(w.get('longAnimationFrameUnsupported',True) for w in browser_windows),'timelineStyleLayoutPaintEvents':'intentionally not captured; zero work cannot be inferred'},'cpuWindow':{'samples':len(samples),'sumDeltaUs':sum(deltas),'deltaMinUs':min(deltas) if deltas else None,'deltaMedianUs':statistics.median(deltas) if deltas else None,'deltaP95Us':rank(deltas,.95),'requestedIntervalUs':status.get('limits',{}).get('cpuIntervalUs'),'startTime':cpu.get('startTime'),'endTime':cpu.get('endTime'),'shapeAssessment':{'negativeDeltaCount':len(negative_deltas),'negativeDeltaSumUs':sum(negative_deltas),'samplesReferencingMissingNode':missing_nodes,'weightedAttributionAllowed':weighted_cpu_allowed,'policy':'Preserve irregular deltas unchanged. Withhold weighted CPU and bundle attribution if any delta is negative or sample identity is missing; never clamp, reorder or take absolute values without an independently supported contract.'},'topBundleLocations':([{'urlSHA256':k[0],'lineNumber':k[1],'columnNumber':k[2],'sampleWeightUs':v} for k,v in sorted(counts.items(),key=lambda kv:kv[1],reverse=True)[:20]] if weighted_cpu_allowed else None),'scope':'Whole primary window, not mapped into individual actions. When permitted by shape validation, sample weight and CPU attribution are statistical; (program) means unknown/native, not app CPU; minified bundle locations are not React component proof.'},'limitations':['Expected-DOM marker is the driver observation after locator.waitFor, not the first app-ready or painted instant','CPU duration attribution is withheld when raw sample deltas are negative; raw profile preserved','Passive >50ms observations omit shorter tasks/frames; intervals can overlap and must not be summed across kinds','Frame script summaries include at most64 entries; truncation is explicit, and script durations can overlap','Post-dispatch browser interval omits preceding transport/actionability waits and the first overview click for switches','Node stages retain transport and scheduler overhead','Resource timings remain browser-clock evidence; they are not backend function timing','No Playwright actionability retry logs in CDP-only arm; unresolved gaps require separately instrumented action trace','Broad timeline/style/layout/GC categories deliberately excluded; absence is not evidence of zero work','All original primary actions and thresholds preserved; instrumentation can change their costs']}
 raw=json.dumps(out,indent=2)+'\n'
 if len(raw.encode())>2*1024**2:raise ValueError('Analysis output exceeds 2 MiB bound')
 with a.output.open('x') as f:f.write(raw)
 a.output.chmod(0o600)

if __name__=='__main__':main()
