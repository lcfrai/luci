#!/usr/bin/env python3
"""All400 original rows, with only same-Node-clock stage differences."""
from pathlib import Path
import gzip,json,math,collections
B=Path(__file__).resolve().parent;p=json.loads(gzip.decompress((B/'timings.all400.projection.json.gz').read_bytes()));rows=[]
for r in p['interactions']:
 s={x['stage']:x['offsetMs'] for x in r['diagnostics']['nodeStages']};spans={name:s[end]-s[start] for name,start,end in [('action','action-start','action-end'),('verification','verify-start','verify-end'),('twoFrames','frames-start','frames-end')]};assert all(v>=0 for v in spans.values());rows.append({'index':r['index'],'roundOrdinal':r['index']//4+1,'kind':r['kind'],'elapsedMs':r['elapsedMs'],'nodeSpansMs':spans,'largestNodeSpan':max(spans,key=spans.get)})
def q(v):
 v=sorted(v);return {'count':len(v),**{k:v[math.ceil(len(v)*p)-1] for k,p in [('p50Ms',.5),('p95Ms',.95),('p99Ms',.99),('maxMs',1)]}}
summary={kind:{'all':q([r['elapsedMs'] for r in rows if r['kind']==kind]),'nodeSpansMs':{phase:q([r['nodeSpansMs'][phase] for r in rows if r['kind']==kind]) for phase in ['action','verification','twoFrames']}} for kind in ['project-switch','input','draft-reset']}
out={'originalResultSHA256':p['originalResultSHA256'],'rows':rows,'summary':summary,'inputsIndividuallyOver100ms':[r for r in rows if r['kind']=='input' and r['elapsedMs']>100],'limits':['All400 rows retained unchanged. Derived Node spans share only the Node clock.','One action spans all three real buttons in draft-reset; individual reset click boundaries were not retained.','Single input durations above100ms do not themselves fail a p95<=100ms gate.','No source-function or physical-paint claim.']};(B/'node-stages.all400.json').write_text(json.dumps(out,indent=2)+'\n');print(json.dumps({'inputOver100':[{'index':r['index'],'round':r['roundOrdinal'],'ms':r['elapsedMs'],'largest':r['largestNodeSpan']} for r in out['inputsIndividuallyOver100ms']],'summary':summary},indent=2))

frames=[]
for r in out['inputsIndividuallyOver100ms']:
 original=p['interactions'][r['index']];f=original['diagnostics']['browserFrames'];frames.append({'index':r['index'],'roundOrdinal':r['roundOrdinal'],'elapsedMs':r['elapsedMs'],'nodeSpansMs':r['nodeSpansMs'],'browserFrameRequestToSecondCallbackMs':f['secondFrameAt']-f['requestedAt'],'browserFrameRequestToFirstCallbackMs':f['firstFrameAt']-f['requestedAt']})
(B/'input-tail-browser-frames.json').write_text(json.dumps({'rows':frames,'limits':'Node two-frame command includes RPC dispatch/return; browser requestedAt to second callback is separately retained browser-clock interval. No clock subtraction or causal attribution.'},indent=2)+'\n')
