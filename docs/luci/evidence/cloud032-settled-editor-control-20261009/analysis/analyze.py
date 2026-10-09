from pathlib import Path
import json,math,csv,hashlib,statistics
HERE=Path(__file__).resolve().parent;ROOT=Path('/workspace/luci/artifacts/quality-swarm/settled-evaluate-edd0cb12-attempt1');r=json.loads((ROOT/'driver/result.json').read_text());c=r['settledEvaluateControl'];assert len(c['rows'])==32
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
assert sha(ROOT/'driver/result.json')=='b1b04e879105e641702f9ae37652847e912f9515f31ce70c2062b7980223cd18'
rows=[]
for i,x in enumerate(c['rows']):
 assert x['index']==i and x['success'] is True
 expected=['locator','handle','handle','locator'] if x['block']==0 else ['handle','locator','locator','handle']
 assert x['arm']==expected[x['position']] and x['generation']==x['destination']*2+x['block']+1
 marks=x['nodeStages'];assert [m['stage'] for m in marks]==['wait-start','wait-end','evaluate-end','input-value-end'];s={m['stage']:m['offsetMs'] for m in marks};assert 0<=s['wait-start']<=s['wait-end']<=s['evaluate-end']<=s['input-value-end']<=x['elapsedMs']
 cb=x['callback'];assert cb['status']=='observed' and cb['reason'] is None and cb['samples']==1 and cb['call']==x['position']+1 and cb['generation']==x['generation']
 assert all(cb[k] is True for k in ['sameElement','connected','valueMatches','nativeEditable'])
 rows.append({k:x[k] for k in ['index','destination','block','generation','position','arm','success','elapsedMs','nodeStages','callback']}|{'waitMs':s['wait-end']-s['wait-start'],'evaluateMs':s['evaluate-end']-s['wait-end'],'inputValueMs':s['input-value-end']-s['evaluate-end'],'callbackMs':cb['finishedAt']-cb['enteredAt'],'sampleOffsetMs':cb['firstSampleAt']-cb['enteredAt']})
assert c['status']=='completed' and len(c['blocks'])==8 and c['lateCallbacks']==0 and c['failures']==[]
assert all(b['status']=='completed' and b['close']['closed'] and b['close']['reason'] is None and b['close']['calls']==4 for b in c['blocks'])
metrics=['elapsedMs','waitMs','evaluateMs','inputValueMs','callbackMs','sampleOffsetMs']
def dist(values):
 v=sorted(values)
 return {'n':len(v),'min':v[0],'mean':statistics.mean(v),'median':statistics.median(v),'p50NearestRank':v[math.ceil(len(v)*.5)-1],'p95NearestRank':v[math.ceil(len(v)*.95)-1],'p99NearestRank':v[math.ceil(len(v)*.99)-1],'max':v[-1]}
def summary(rs):return {m:dist([x[m] for x in rs]) for m in metrics}
by_arm={a:summary([x for x in rows if x['arm']==a]) for a in ['locator','handle']}
pairs=[]
for start in range(0,32,2):
 pair=rows[start:start+2];a=next(x for x in pair if x['arm']=='locator');b=next(x for x in pair if x['arm']=='handle')
 pairs.append({'pair':start//2,'destination':a['destination'],'block':a['block'],'order':[x['arm'] for x in pair],'locatorIndex':a['index'],'handleIndex':b['index'],'locatorMinusHandleEvaluateMs':a['evaluateMs']-b['evaluateMs'],'locatorMinusHandleFullCallMs':a['elapsedMs']-b['elapsedMs']})
projection={'schema':1,'productSource':r['sourceCommit'],'declaredHelperSource':r['harnessCheckout']['selectedHelperSource'],'observedCheckout':r['harnessCheckout']['observedCheckoutCommit'],'rawResultSHA256':sha(ROOT/'driver/result.json'),'originalFullCohortExecuted':False,'acceptanceReplacement':False,'status':c['status'],'rows':rows,'blocks':c['blocks'],'failures':c['failures'],'lateCallbacks':c['lateCallbacks'],'phaseElapsedMs':c['elapsedMs'],'originalReferenceTargets':r['originalLatencyTargetReference'],'originalInteractionCount':len(r['interactions'])}
assert not r['originalActionCohortExecuted'] and not r['acceptanceReplacement'] and len(r['interactions'])==0
out={'schema':1,'rowCount':32,'arms':by_arm,'byDestination':{str(d):{a:summary([x for x in rows if x['destination']==d and x['arm']==a]) for a in by_arm} for d in range(4)},'byBlockOrder':{str(b):{a:summary([x for x in rows if x['block']==b and x['arm']==a]) for a in by_arm} for b in range(2)},'byPosition':{str(p):{a:summary([x for x in rows if x['position']==p and x['arm']==a]) for a in by_arm} for p in range(4)},'pairs':pairs,'pairedEvaluateDifference':dist([p['locatorMinusHandleEvaluateMs'] for p in pairs]),'pairedLocatorLonger':sum(p['locatorMinusHandleEvaluateMs']>0 for p in pairs),'pairedHandleLonger':sum(p['locatorMinusHandleEvaluateMs']<0 for p in pairs),'quantileMethod':'nearest rank ceil(q*n)-1; median separately arithmetic midpoint for even n. With16 per arm p95/p99 are observed maximum, not stable tail estimates.','limits':['32 settled diagnostic evaluations only; original432 actions unexecuted and targets unmeasured.','Same predicate and same retained DOM identity verified, not original navigation polling/rAF callback.','Node evaluate spans include resolution/protocol/evaluation/disposal as applicable; browser callback duration uses only its own monotonic clock.','No subtraction from original metrics, no cross-clock decomposition, CPU/paint/app optimization or universal transport-floor claim.','Fixed balanced ABBA/BAAB order, four contexts, one short installed run. No randomization or independent repeated run.','Stage markers add observation work; both arms share them. No overhead adjustment.']}
(HERE/'result-projection.json').write_text(json.dumps(projection,indent=2)+'\n');(HERE/'analysis.json').write_text(json.dumps(out,indent=2)+'\n')
with (HERE/'all32rows.csv').open('w') as f:
 fields=['index','destination','block','generation','position','arm','success',*metrics];w=csv.DictWriter(f,fields);w.writeheader();w.writerows({k:x[k] for k in fields} for x in rows)
print(json.dumps({'armSummaries':by_arm,'pairedEvaluateDifference':out['pairedEvaluateDifference'],'locatorLongerPairs':out['pairedLocatorLonger'],'handleLongerPairs':out['pairedHandleLonger'],'controlPhaseMs':c['elapsedMs']}))
