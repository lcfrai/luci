from pathlib import Path
import json,hashlib,datetime
base=Path('<PRIVATE_RUN_EVIDENCE_DIRECTORY>');out=Path('<PRIVATE_TOOLING_DIRECTORY>/publish-sustained7/analysis-prep')
p=base/'result/result.json';r=json.loads(p.read_text());sha=lambda b:hashlib.sha256(b).hexdigest()
assert r['passed'] is False and 'functionalPassed' not in r and r['latencyTargetsMet'] is False
assert r['actualTaskCount']==len(r['actualTaskResults'])==len(r['naturalCompletions'])==len(r['integrations'])==5
assert r['externalModelCalls']==0 and r['providerCalls']==110
iso=lambda t:datetime.datetime.fromisoformat(t.replace('Z','+00:00'))
rows=[]
for x in r['naturalCompletions']:
 assert x['cleanNativeExitFrame'] and x['codingDispatches']==x['finalResponseRequests']==1 and not x['finalResponseAborts'] and not x['finalResponseRetries']
 row={k:v for k,v in x.items()}
 row['observedExitToContainerAbsenceMs']=(iso(x['stoppedAndAbsentObservedAt'])-iso(x['exitFrameObservedAt'])).total_seconds()*1000
 row['observedExitToLeaseAbsenceMs']=(iso(x['leaseObservation']['firstAbsentAt'])-iso(x['exitFrameObservedAt'])).total_seconds()*1000
 rows.append(row)
artifacts={}
for task,value in r['actualTaskResults'].items():
 a=value['artifact']
 if not isinstance(a,dict): raise AssertionError((task,type(a)))
 artifacts[task]={'task':task,'operation':value['operation'],'outputSHA256':value['outputSHA256'],'base':value['base'],'commit':value['commit'],'changed':value['changed'],'artifact':a}
summary={'scope':'Exact receipt projections plus observed timestamp differences. Same five tasks contribute receipt, exit and integration observations; counts overlap, not fifteen tasks. No new execution and no whole-build or live-provider quality claim.','originalResultSHA256':sha(p.read_bytes()),'productSource':r['sourceCommit'],'helperSource':r['harnessCheckout']['commit'],'originalPassed':r['passed'],'functionalPassedPropertyPresent':'functionalPassed' in r,'latencyTargetsMet':r['latencyTargetsMet'],'failedStage':r['failedStage'],'originalFailure':r['failure'],'measuredActionCount':len(r['interactions']),'successfulMeasuredActionCount':sum(x['success'] is True for x in r['interactions']),'laterPhoneFailureOutsideMeasuredRows':True,'actualTasks':r['actualTaskCount'],'providerCalls':r['providerCalls'],'externalModelCalls':r['externalModelCalls'],'actualFuzzSeconds':r['actualFuzzSeconds'],'activeFuzzSecondsTotal':sum(r['actualFuzzSeconds']),'oneWorkerAtATime':True,'checks':r['checks'],'phases':r['phases'],'naturalCompletions':rows,'taskArtifacts':artifacts,'integrationIdentities':[{'task':i['task'],'integratedCommit':i['commit'],'sourceCommit':i['review']['sourceCommit'],'targetCommit':i['review']['targetCommit'],'reviewDigest':i['review']['reviewDigest'],'handoff':i['review'].get('handoff')} for i in r['integrations']],'taskReceiptSettlements':r['taskReceiptSettlements'],'activeRestarts':r['activeRestarts'],'limits':['Observed timestamp differences include observation/tick delays, not instantaneous event latencies.','Scripted provider chooses known implementations; actual Go/Docker/Codex execution does not establish real-model planning/quality.','Worker handoff reports remain missing; no fabricated report or no-questions conclusion.','All five tasks integrated but independent product whole-build acceptance remains separate.','Original workflowOperationUnsettled remains true although controller/proxy/runtime cleanup and independent ledger closure are recorded separately.']}
(out/'workflow-analysis.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps({'tasks':len(rows),'commandReceiptCount':len(r['taskReceiptSettlements']),'observedExitToLeaseAbsenceMs':{x['task']:x['observedExitToLeaseAbsenceMs'] for x in rows},'sourceSHA256':summary['originalResultSHA256']},indent=2))
