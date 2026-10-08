import assert from 'node:assert/strict';
import {test} from 'node:test';
import {automaticCompletion,assertReadOnlyCompletionRequests,inPlaceCompletion} from './automatic-completion.mjs';
import {taskHandoff} from './continuation-fixture.mjs';

function fixture(key='partial'){
 const operation={id:'op',childId:'s-12345678',taskId:'task-work',attempt:key==='partial'?1:2,worktreeId:'work',baseCommit:'a'.repeat(40),requestDigest:'b'.repeat(64),implementation:{projectId:'project',worktreeId:'work',taskIds:['task-work']},status:'running',allocation:'ready'};
 const report=taskHandoff(key),capturedAt='2026-10-08T15:00:00Z',digest='c'.repeat(64);
 const expected={projectId:'project',runId:'run',operation,sourceCommit:'d'.repeat(40),contextDigest:'e'.repeat(64),report,sourceDirty:key==='partial',attentionKind:key==='partial'?'task-question':'task-review'};
 const run={projectId:'project',id:'run',revision:9,state:'awaiting-checks',operations:[{...structuredClone(operation),status:'review-required',handoff:{digest,status:'captured',questions:report.questions.length,unanswered:report.questions.length,capturedAt}}],swarm:{tasks:[{id:operation.taskId,operationId:operation.id}]}};
 const handoff={projectId:'project',runId:'run',taskId:operation.taskId,operationId:operation.id,status:'captured',digest,integration:null,recordedChecks:[],answers:[],questionIds:report.questions.map((_,i)=>'q-'+i),receipt:{version:1,projectId:'project',runId:'run',taskId:operation.taskId,operationId:operation.id,childId:operation.childId,worktreeId:operation.worktreeId,requestDigest:operation.requestDigest,status:'captured',implementationDigest:'f'.repeat(64),sourceDigest:'1'.repeat(64),contextDigest:expected.contextDigest,sourceCommit:expected.sourceCommit,sourceDirty:expected.sourceDirty,capturedAt,reported:report}};
 const overview={projectId:'project',latestRun:{status:'recorded',id:'run',state:run.state,contextStatus:'current'},attention:{status:'recorded',count:1,omitted:0,items:[{id:'run',kind:expected.attentionKind,state:run.state,contextStatus:'current'}]}};
 return {run,handoff,overview,expected};
}
test('both natural completions need exact captured report and distinct Home review attention',()=>{
 for(const key of ['partial','finish']){const f=fixture(key),before=structuredClone(f),v=automaticCompletion(f);assert.equal(v.attention.kind,key==='partial'?'task-question':'task-review');assert.deepEqual(f,before,'Read-only oracle must not repair input');}
});
test('exit alone, pending capture and unavailable or empty attention remain incomplete',()=>{
 for(const change of [f=>f.run.operations[0].status='running',f=>delete f.run.operations[0].handoff,f=>f.handoff.status='pending',f=>f.overview.attention.status='unavailable',f=>f.overview.attention.items=[]]){const f=fixture();change(f);assert.equal(automaticCompletion(f),null);}
});
test('failed, uncertain, cancelled and legacy-success records do not stand in for exact terminal observation',()=>{
 for(const status of ['failed','unknown','cancelled','succeeded','reserved']){const f=fixture();f.run.operations[0].status=status;assert.throws(()=>automaticCompletion(f));}
});
test('a different attempt, child, frozen context, source or report is refused',()=>{
 for(const change of [f=>f.run.swarm.tasks[0].operationId='another',f=>f.run.operations[0].childId='s-ffffffff',f=>f.handoff.operationId='another',f=>f.handoff.receipt.childId='s-ffffffff',f=>f.handoff.receipt.contextDigest='0'.repeat(64),f=>f.handoff.receipt.sourceCommit='0'.repeat(40),f=>f.handoff.receipt.sourceDirty=false,f=>f.handoff.receipt.reported={...f.handoff.receipt.reported,summary:'other'},f=>f.handoff.digest='0'.repeat(64)]){const f=fixture();change(f);assert.throws(()=>automaticCompletion(f));}
});
test('integration, invented verification, wrong Home scope/kind and answered questions cannot silently pass',()=>{
 for(const change of [f=>f.run.swarm.tasks[0].integration={state:'integrated'},f=>f.handoff.integration={state:'integrated'},f=>f.handoff.recordedChecks=[{status:'passed'}],f=>f.handoff.answers=[{answer:'invented'}],f=>f.overview.projectId='other',f=>f.overview.attention.items[0].kind='verification',f=>f.overview.latestRun.id='other',f=>f.run.operations[0].handoff.unanswered=0]){const f=fixture();change(f);assert.throws(()=>automaticCompletion(f));}
});
test('manual mutation cannot manufacture the positive automatic-observation witness',()=>{
 const p='/v1/product/projects/project';assertReadOnlyCompletionRequests([{method:'GET',path:p+'/runs/run'},{method:'GET',path:p+'/overview'},{method:'POST',path:'/v1/auth/login'}],p);
 for(const path of ['/runs/run/swarm/reconcile','/runs/run/swarm/operations/op/handoff/collect','/runs/run/swarm/tasks/task/integrate','/runs/run/swarm/tasks/task/reserve'])assert.throws(()=>assertReadOnlyCompletionRequests([{method:'POST',path:p+path}],p));
});
test('in-place review waits for visible enabled controls and the question without navigation',()=>{
 const {expected}=fixture();const view={statusVisible:true,reviewVisible:true,reviewEnabled:true,reportVisible:true,reportEnabled:true,questionVisible:true,scopeText:'Attempt 1 · workspace work · session s-12345678'};
 assert.equal(inPlaceCompletion(view,expected).childID,'s-12345678');
 for(const key of ['statusVisible','reviewVisible','reviewEnabled','reportVisible','reportEnabled','questionVisible'])assert.equal(inPlaceCompletion({...view,[key]:false},expected),null);
 assert.throws(()=>inPlaceCompletion({...view,scopeText:'Attempt 1 · workspace other · session s-ffffffff'},expected));
 const finished=fixture('finish').expected;assert(inPlaceCompletion({...view,questionVisible:false,scopeText:'Attempt 2 · workspace work · session s-12345678'},finished));
});
