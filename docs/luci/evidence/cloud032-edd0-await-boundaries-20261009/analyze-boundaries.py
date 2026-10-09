#!/usr/bin/env python3
"""Join already allowlisted projections; original full spans remain untouched."""
from pathlib import Path
import collections,json,math
P=Path(__file__).resolve().parent
PRIMARY='idle-interactive-calibration';TRACE='idle-actionability-trace-followup'
def q(values):
 values=sorted(values);n=len(values)
 return {'count':n,**{label:values[max(0,math.ceil(n*r)-1)] if n else None for label,r in [('p50Ms',.5),('p95Ms',.95),('p99Ms',.99),('maxMs',1)]}}
def main():
 full=json.loads((P/'actual/interactions.projection.json').read_text());boundary=json.loads((P/'actual/boundaries.projection.json').read_text());readiness=json.loads((P/'actual/readiness.projection.json').read_text());assert not boundary['issues']
 joins={r['index']:r for r in readiness['switches']};assert len(full)==len(boundary['rows'])==432
 rows=[]
 for index,(a,b) in enumerate(zip(full,boundary['rows'])):
  assert a['index']==b['index']==index and a['elapsedMs']==b['elapsedMs'] and a['success']==b['success'] and a['nodeStages']==b['nodeStages'] and not b['issues']
  row={k:a[k] for k in ['index','phase','kind','round','projectOrdinal','success','elapsedMs','nodeClockReads']};row['nodeSubspansMs']={**a['nodeSubspansMs'],**b['segments']}
  if a['kind']=='project-switch':
   segments=b['segments'];assert len(segments)==4 and len(a['nodeStages'])==a['nodeClockReads']==12
   assert abs(sum(segments[k] for k in ['fieldWait','contextEvaluate','inputValueAndAssert'])-a['nodeSubspansMs']['verification'])<1e-6
   by={r['stage']:r['offsetMs'] for r in a['nodeStages']};row['nodeSubspansMs']['headingReadyToOpenStart']=by['open-project-click-start']-by['overview-heading-ready']
   if index in joins:
    j=joins[index];assert j['availability']=='joined';row['browserObservedIntervalsMs']={k:j[k] for k in ['homeClickToObservedMs','homeObservedToOpenClickMs','defineClickToObservedMs','defineObservedToDriverMs','trustedOpenClickToDriverDOMMs']};row['observerMaxSampleGapMs']={s:j[s]['maxSampleGapMs'] for s in ['home','define']}
   candidates={k:v for k,v in row['nodeSubspansMs'].items() if k not in ['verification','openProjectCommand','headingReadyToOpenStart']};row['largestRecordedNodeRegion']=max(candidates,key=candidates.get)
  else:assert len(a['nodeStages'])==a['nodeClockReads']==6
  rows.append(row)
 summary={}
 for phase in [PRIMARY,TRACE]:
  selected=[r for r in rows if r['phase']==phase and r['kind']=='project-switch']
  summary[phase]={'switchCount':len(selected),'regions':{k:q([r['nodeSubspansMs'][k] for r in selected]) for k in selected[0]['nodeSubspansMs']},'largestRecordedRegionCounts':dict(collections.Counter(r['largestRecordedNodeRegion'] for r in selected)),'largestRecordedRegionCountsOver300ms':dict(collections.Counter(r['largestRecordedNodeRegion'] for r in selected if r['elapsedMs']>300)),'over300ms':sum(r['elapsedMs']>300 for r in selected),'topFullSpanIndices':[r['index'] for r in sorted(selected,key=lambda x:x['elapsedMs'],reverse=True)[:12]]}
 out={'scope':'All 432 joined full-span/Node-region rows, separate same-browser-clock observed readiness intervals where available. Regions are inclusive waits; neither causal attribution nor subtracted full metrics.','identity':{'productSource':boundary['productSource'],'helperSource':boundary['helperSource'],'originalResultSHA256':boundary['inputSHA256']},'actualMarkCounts':dict(collections.Counter(str(r['nodeClockReads']) for r in rows)),'exactExpectedLabelSequences':True,'verificationComponentsReconcileFor108Switches':True,'summary':summary,'rows':rows}
 (P/'actual/stage-analysis.json').write_text(json.dumps(out,indent=2)+'\n');print(json.dumps({'rows':len(rows),'summary':summary},indent=2))
if __name__=='__main__':main()
