from pathlib import Path
import gzip,json,math
root=Path(__file__).resolve().parent
p=json.loads(gzip.decompress((root/'timings.projection.json.gz').read_bytes()));a=json.loads((root/'analysis.json').read_text());rows=p['interactions']
assert len(rows)==432 and [r['index'] for r in rows]==list(range(432))
assert sum(r['phase']=='idle-interactive-calibration' for r in rows)==400
for key,s in p['interactionSummary'].items():
 cohort=[r for r in rows if r['phase']+':'+r['kind']==key];v=sorted(r['elapsedMs'] for r in cohort);q=lambda x:v[math.ceil(len(v)*x)-1]
 actual={'count':len(v),'successful':sum(r['success'] is True for r in cohort),'failed':sum(r['success'] is not True for r in cohort),'p50Ms':q(.5),'p95Ms':q(.95),'p99Ms':q(.99),'maxMs':max(v)}
 assert all(s[k]==val for k,val in actual.items())
slow=[r for r in rows if r['phase']=='idle-interactive-calibration' and r['kind']=='project-switch' and r['elapsedMs']>300];assert len(slow)==18
assert [r['index'] for r in slow]==[r['index'] for r in a['slowSwitches']]
labels=[('allProjectsCommand','action-start','all-projects-click-end'),('overviewHeadingWait','all-projects-click-end','overview-heading-ready'),('openProjectCommand','overview-heading-ready','open-project-click-end'),('verifyExpectedContext','verify-start','verify-end'),('twoFrameCommand','frames-start','frames-end')]
for original,projection in zip(slow,a['slowSwitches']):
 stages={r['stage']:r['offsetMs'] for r in original['diagnostics']['nodeStages']}
 assert projection['nodeSpans']=={name:stages[end]-stages[start] for name,start,end in labels}
 f=original['diagnostics']['browserFrames'];refs=f['references'];assert projection['browserClockSpans']=={'lastPointerToDriverExpectedDOMWitnessMs':refs['expectedDOM']['at']-refs['pointer']['at'],'driverExpectedDOMWitnessToFrameRequestMs':f['requestedAt']-refs['expectedDOM']['at'],'frameRequestToSecondCallbackMs':f['secondFrameAt']-f['requestedAt']}
assert p['targetComparison']['project-switch']['withinReference'] is False and p['targetComparison']['input']['withinReference'] is True
print(json.dumps({'rows':len(rows),'primary':400,'separateTraceFollowup':32,'slowPrimarySwitches':18,'switchTarget':'failed','inputTarget':'passed','noFilteringOrSubtraction':True}))
