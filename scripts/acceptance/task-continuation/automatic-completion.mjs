// Read-only acceptance oracle. Process exit is a prerequisite supplied by the
// driver, never a substitute for the product's recorded result and attention.
import assert from 'node:assert/strict';

export function automaticCompletion({run,handoff,overview,expected}) {
 const {projectId,runId,operation:original,sourceCommit,contextDigest,report,sourceDirty,attentionKind}=expected;
 assert.equal(run.projectId,projectId);assert.equal(run.id,runId);
 const matches=run.operations.filter(op=>op.id===original.id);assert.equal(matches.length,1);
 const operation=matches[0],task=run.swarm.tasks.find(value=>value.id===original.taskId);
 assert.equal(task?.operationId,original.id);assert.equal(task.integration??null,null,'Natural completion must not integrate output');
 for(const field of ['id','childId','taskId','attempt','worktreeId','baseCommit','requestDigest','implementation','checkpoint'])assert.deepEqual(operation[field],original[field],'Observation substituted '+field);
 assert.equal(operation.allocation,'ready');
 if(['running','dispatching'].includes(operation.status))return null;
 assert.equal(operation.status,'review-required','An unknown, failed, cancelled or legacy success state is not this exact clean-exit witness');
 if(operation.handoff?.status!=='captured')return null;
 assert.equal(handoff?.projectId,projectId);assert.equal(handoff.runId,runId);assert.equal(handoff.taskId,original.taskId);assert.equal(handoff.operationId,original.id);
 if(handoff.status!=='captured')return null;
 assert.match(handoff.digest,/^[a-f0-9]{64}$/);assert.equal(handoff.digest,operation.handoff.digest);
 assert.equal(handoff.integration,null);assert.deepEqual(handoff.recordedChecks,[],'Reported commands must not be promoted to independent verification');assert.deepEqual(handoff.answers,[]);
 assert.equal(handoff.questionIds.length,report.questions.length);assert.equal(new Set(handoff.questionIds).size,handoff.questionIds.length);
 assert.equal(operation.handoff.questions,report.questions.length);assert.equal(operation.handoff.unanswered,report.questions.length);
 const value=handoff.receipt;assert(value);assert.equal(value.version,1);assert.equal(value.status,'captured');
 assert.equal(value.projectId,projectId);assert.equal(value.runId,runId);assert.equal(value.taskId,original.taskId);assert.equal(value.operationId,original.id);assert.equal(value.childId,original.childId);assert.equal(value.worktreeId,original.worktreeId);assert.equal(value.requestDigest,original.requestDigest);
 assert.match(value.implementationDigest,/^[a-f0-9]{64}$/);assert.match(value.sourceDigest,/^[a-f0-9]{64}$/);assert.equal(value.contextDigest,contextDigest);assert.equal(value.sourceCommit,sourceCommit);assert.equal(value.sourceDirty,sourceDirty);assert.deepEqual(value.reported,report);
 assert(Number.isFinite(Date.parse(value.capturedAt)));assert.equal(value.capturedAt,operation.handoff.capturedAt);
 assert.equal(overview?.projectId,projectId);
 if(overview.attention?.status!=='recorded')return null;
 const attention=overview.attention.items.filter(item=>item.id===runId);
 if(attention.length===0)return null;
 assert.equal(attention.length,1);assert.equal(attention[0].kind,attentionKind);assert.equal(attention[0].contextStatus,'current');assert.equal(attention[0].state,run.state);
 assert.equal(overview.latestRun.status,'recorded');assert.equal(overview.latestRun.id,runId);assert.equal(overview.latestRun.state,run.state);assert.equal(overview.latestRun.contextStatus,'current');
 assert.equal(overview.attention.count,1);assert.equal(overview.attention.omitted,0);
 return {operation,handoff,attention:attention[0]};
}

export function assertReadOnlyCompletionRequests(actions,projectRoute) {
 for(const action of actions){
  if(action.path===projectRoute||action.path.startsWith(projectRoute+'/'))assert(['GET','HEAD','OPTIONS'].includes(action.method),'Automatic completion witness must not reconcile, collect, integrate, answer or start through an HTTP mutation');
 }
}

export function inPlaceCompletion({reviewVisible,reviewEnabled,reportVisible,reportEnabled,statusVisible,questionVisible,scopeText},expected) {
 if(!statusVisible||!reviewVisible||!reviewEnabled||!reportVisible||!reportEnabled)return null;
 if(expected.report.questions.length&&!questionVisible)return null;
 assert.equal(typeof scopeText,'string');
 assert(scopeText.includes('Attempt '+expected.operation.attempt+' · workspace '+expected.operation.worktreeId+' · session '+expected.operation.childId),'The visible review must belong to the exact current child and workspace');
 return {recordedStatus:'Awaiting review',reviewEnabled:true,reportEnabled:true,questionWarningVisible:questionVisible,operationID:expected.operation.id,childID:expected.operation.childId,worktreeID:expected.operation.worktreeId};
}
