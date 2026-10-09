from pathlib import Path
from collections import defaultdict,Counter
import json,math,hashlib,datetime
BASE=Path('<PRIVATE_RUN_EVIDENCE_DIRECTORY>');OUT=Path('<PRIVATE_TOOLING_DIRECTORY>/publish-sustained7/analysis-prep');OUT.mkdir(parents=True,exist_ok=True)
P=BASE/'result/result.json';J=json.loads(P.read_text());events=defaultdict(list);docs=defaultdict(dict)
for w in J['browserEventWindows']:
 events[w['timeOrigin']]+=w['events']
 for r in w['latency']['rows']:docs[w['timeOrigin']][r['sequence']]=r
stats=lambda v:{'count':len(v),'p50Ms':sorted(v)[math.ceil(len(v)*.5)-1],'p95Ms':sorted(v)[math.ceil(len(v)*.95)-1],'p99Ms':sorted(v)[math.ceil(len(v)*.99)-1],'maxMs':max(v)}if v else {'count':0}
groups=defaultdict(list);rows=[]
for i,r in enumerate(J['interactions']):
 out={k:r[k]for k in ['phase','kind','at','success','elapsedMs']};out['originalIndex']=i
 out['diagnostics']=r['diagnostics']
 marks={m['stage']:m['offsetMs']for m in r['diagnostics']['nodeStages']}
 parts={}
 for label,start,end in [('action','action-start','action-end'),('verify','verify-start','verify-end'),('framesRPC','frames-start','frames-end'),('allProjectsClick','action-start','all-projects-click-end'),('overviewHeading','all-projects-click-end','overview-heading-ready'),('openProjectClick','overview-heading-ready','open-project-click-end')]:
  if start in marks and end in marks:parts[label]=marks[end]-marks[start]
 out['nodeSpansMs']=parts
 if r['kind']=='project-switch':
  refs=r['diagnostics']['browserFrames']['references'];doc=docs[refs['timeOrigin']];op=refs['pointer'];pointers=sorted((q for q in doc.values()if q['kind']=='pointer-dispatch'and q['sequence']<=op['sequence']),key=lambda q:q['sequence']);first=pointers[-2];ex=refs['expectedDOM']['at']
  out['allToOpenBrowserMs']=op['at']-first['at'];out['openToExpectedDOMMs']=ex-op['at']
  for name,point in [('all',first),('open',op)]:
   hits=[e for e in events[refs['timeOrigin']]if e['name']=='pointerdown'and e['processingStart']-.2<=point['at']<=e['processingEnd']+.2]
   out[name+'PointerMatches']=len(hits)
   if len(hits)==1:
    e=hits[0];clicks=[v for v in events[refs['timeOrigin']]if v['name']=='click'and v['interactionId']==e['interactionId']and e['interactionId']and abs(v['startTime']-e['startTime'])<100]
    out[name+'Pointer']={'queueMs':e['processingStart']-e['startTime'],'handlerMs':e['processingEnd']-e['processingStart'],'durationMs':e['duration'],'clickMatches':len(clicks)}
    if len(clicks)==1:
     c=clicks[0];out[name+'Click']={'queueMs':c['processingStart']-c['startTime'],'handlerMs':c['processingEnd']-c['processingStart'],'durationMs':c['duration']}
 rows.append(out);groups[r['phase']+':'+r['kind']].append(out)
summaries={}
for key,items in groups.items():
 summary={'summary':stats([r['elapsedMs']for r in items]),'successful':sum(r['success']for r in items),'failed':sum(not r['success']for r in items),'nodeSpans':{stage:stats([r['nodeSpansMs'][stage]for r in items if stage in r['nodeSpansMs']])for stage in sorted({stage for r in items for stage in r['nodeSpansMs']})},'largest':[{'index':r['originalIndex'],'elapsedMs':r['elapsedMs'],'nodeSpansMs':r['nodeSpansMs']}for r in sorted(items,key=lambda r:r['elapsedMs'],reverse=True)[:5]]}
 assert all(summary['summary'][k]==J['interactionSummary'][key][k]for k in ['count','p50Ms','p95Ms','p99Ms','maxMs'])
 if key.endswith(':project-switch'):
  summary['above300ms']=sum(r['elapsedMs']>300 for r in items)
  summary['browserSpans']={k:stats([r[k]for r in items])for k in ['allToOpenBrowserMs','openToExpectedDOMMs']}
  for side in ['all','open']:
   for kind in ['Pointer','Click']:
    for metric in ['queueMs','handlerMs','durationMs']:summary[side+kind+metric]=stats([r[side+kind][metric]for r in items if side+kind in r])
 summaries[key]=summary
out={'version':1,'sourceResultSHA256':hashlib.sha256(P.read_bytes()).hexdigest(),'analysisProducedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'analyzerSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'scope':'All2128 measured action rows retained with exact numeric diagnostics; private project identity omitted. Offline analysis, not a new run or full sustained acceptance.','summaries':summaries,'rows':rows,'limits':['All original durations including failed gate observations retained. No cost subtraction or physical display latency claim.','Node stages include RPC/scheduler waits; browser EventTiming is temporal matching with0.2ms quantization tolerance. Missing thresholded events are not zero.','Loaded321switches/642inputs meet unchanged200/400minimum counts; all three primary cohorts and the separate amendment cohort are retained. A later phone navigation action failed outside these measured rows; no full functional/performance pass.','No primary actionability or React CPU trace was recorded. Cannot assign causal component/GC/host scheduling bottleneck.']}
(OUT/'interaction-analysis.json').write_text(json.dumps(out,indent=2)+'\n')
print(json.dumps({'sourceResultSHA256':out['sourceResultSHA256'],'groups':{k:{'summary':v['summary'],'above300':v.get('above300ms')}for k,v in summaries.items()}},indent=2))
