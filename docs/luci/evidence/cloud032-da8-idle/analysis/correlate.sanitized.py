from pathlib import Path
from collections import defaultdict,Counter
import json,math,hashlib
P=Path('<PRIVATE_DA8_RUN_DIRECTORY>/result.json');J=json.loads(P.read_text());events=defaultdict(list);docs=defaultdict(dict)
for w in J['browserEventWindows']:
 events[w['timeOrigin']]+=w['events']
 for r in w['latency']['rows']:docs[w['timeOrigin']][r['sequence']]=r
stats=lambda v:{'count':len(v),'p50':sorted(v)[math.ceil(len(v)*.5)-1],'p95':sorted(v)[math.ceil(len(v)*.95)-1],'max':max(v)}if v else {'count':0}
rows=[]
for i,r in enumerate(r for r in J['interactions']if r['phase']=='idle-interactive-calibration'and r['kind']=='project-switch'):
 refs=r['diagnostics']['browserFrames']['references'];doc=docs[refs['timeOrigin']];op=refs['pointer'];pointers=sorted((q for q in doc.values()if q['kind']=='pointer-dispatch'and q['sequence']<=op['sequence']),key=lambda q:q['sequence']);first=pointers[-2];ex=refs['expectedDOM']['at'];out={'index':i,'elapsedMs':r['elapsedMs'],'allToOpenBrowserMs':op['at']-first['at'],'openToExpectedDOMMs':ex-op['at'],'nodeMarks':r['diagnostics']['nodeStages']}
 for name,point in [('all',first),('open',op)]:
  hits=[e for e in events[refs['timeOrigin']]if e['name']=='pointerdown'and e['processingStart']-.2<=point['at']<=e['processingEnd']+.2]
  out[name+'PointerMatches']=len(hits)
  if len(hits)==1:
   e=hits[0]; clicks=[v for v in events[refs['timeOrigin']]if v['name']=='click'and v['interactionId']==e['interactionId']and e['interactionId']and abs(v['startTime']-e['startTime'])<100]
   out[name+'Pointer']={'queueMs':e['processingStart']-e['startTime'],'handlerMs':e['processingEnd']-e['processingStart'],'durationMs':e['duration'],'clickMatches':len(clicks)}
   if len(clicks)==1:
    c=clicks[0];out[name+'Click']={'queueMs':c['processingStart']-c['startTime'],'handlerMs':c['processingEnd']-c['processingStart'],'durationMs':c['duration']}
 rows.append(out)
groups={}
for label,selection in [('all',rows),('over300',[r for r in rows if r['elapsedMs']>300]),('within300',[r for r in rows if r['elapsedMs']<=300])]:
 groups[label]={'count':len(selection),'elapsedMs':stats([r['elapsedMs']for r in selection]),'allToOpenBrowserMs':stats([r['allToOpenBrowserMs']for r in selection]),'openToExpectedDOMMs':stats([r['openToExpectedDOMMs']for r in selection])}
 for side in ['all','open']:
  for kind in ['Pointer','Click']:
   for metric in ['queueMs','handlerMs','durationMs']:
    groups[label][side+kind+metric]=stats([r[side+kind][metric]for r in selection if side+kind in r])
base=json.loads(Path('<PRIVATE_PRIOR_RUN_DIRECTORY>/result.json').read_text());out={'scope':'All100 switch rows retained; temporal browser EventTiming matching only, not React/function causal attribution.0.2ms quantization tolerance; click uses same nonzero interactionID and start within100ms of pointerdown. Thresholded events absent are not zero.','resultSHA256':hashlib.sha256(P.read_bytes()).hexdigest(),'rows':rows,'groups':groups,'thresholdComparison':{'d348':base['targetComparison'],'da8':J['targetComparison']},'limits':['Sequential single arms; changes cannot be causally attributed.','No clock-domain subtraction or filtering of primary metric rows.','Separate8-round trace does not contain primary slow events; zero retries there does not prove primary has no retries.']};Path('<PRIVATE_ANALYSIS_OUTPUT_DIRECTORY>/passive-correlation.json').write_text(json.dumps(out,indent=2)+'\n');print(json.dumps({'groups':groups,'top5':sorted(rows,key=lambda r:r['elapsedMs'],reverse=True)[:5]},indent=2))
