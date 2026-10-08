// Pure, bounded diagnostics for one explicitly authorized queue identity.
import assert from 'node:assert/strict';

export function selectAuthorizedTask(items, authorization, seenChildIds) {
  assert(authorization && typeof authorization.id === 'string');
  const matches=items.filter(item=>item.id===authorization.id);
  assert(matches.length<=1,'Duplicate authorized queue identity');
  const item=matches[0];if(!item)return undefined;
  for(const key of ['projectId','runId','taskId'])assert.equal(item[key],authorization[key]);
  if(!item.childId||!item.operationId||seenChildIds.includes(item.childId))return undefined;
  return ['running','failed'].includes(item.state)?item:undefined;
}

export function taskFailureDiagnostic({variant,authorization,queueItem,session,run,expectedAttempt}) {
  const truncatedFields=[];
  function fields(value,names,prefix){
    if(!value)return null;
    return Object.fromEntries(names.filter(name=>value[name]!==undefined).map(name=>{
      const v=value[name];if(typeof v==='string'){
        const limit=['message','reason','exitReason'].includes(name)?2048:256;
        if(v.length>limit)truncatedFields.push(prefix+'.'+name);
        return [name,v.slice(0,limit)];
      }
      return [name,Number.isSafeInteger(v)?v:null];
    }));
  }
  const operation=run?.operations?.find(op=>op.id===queueItem.operationId);
  const failure=(value,prefix)=>fields(value?.startFailure,['stage','reasonCode','message'],prefix+'.startFailure');
  return {
    at:new Date().toISOString(),classification:'unverified-terminal-queue-response',variant,expectedAttempt,
    authorization:fields(authorization,['id','projectId','runId','taskId'],'authorization'),
    queue:fields(queueItem,['id','revision','projectId','runId','taskId','state','childId','operationId','reasonCode','reason'],'queue'),
    session:session?{...fields(session,['id','projectId','projectRunId','projectRunOperationId','worktreeId','status','exitReason'],'session'),startFailure:failure(session,'session')}:null,
    run:run?{...fields(run,['id','projectId','revision','state'],'run'),...fields({currentTaskOperationId:run.swarm?.tasks?.find(task=>task.id===authorization.taskId)?.operationId??null},['currentTaskOperationId'],'run')}:null,
    operation:operation?{...fields(operation,['id','taskId','childId','worktreeId','status','allocation','attempt','message'],'operation'),startFailure:failure(operation,'operation')}:null,
    truncatedFields,
  };
}

export function assertExactTaskFailure(diagnostic) {
  const {authorization:a,queue:q,session:s,run:r,operation:o}=diagnostic;
  assert.deepEqual(diagnostic.truncatedFields,[],'Truncated identity/diagnostics cannot establish the exact failure');
  assert(s&&r&&o,'The failed queue entry needs its exact public session and current operation');
  assert.equal(q.state,'failed');assert.equal(s.status,'exited');assert.equal(o.status,'failed');
  for(const key of ['id','projectId','runId','taskId'])assert.equal(q[key],a[key]);
  assert.equal(r.id,a.runId);assert.equal(r.projectId,a.projectId);assert.equal(r.currentTaskOperationId,o.id);
  assert.equal(q.operationId,o.id);assert.equal(q.childId,o.childId);assert.equal(o.taskId,a.taskId);
  assert.equal(o.attempt,diagnostic.expectedAttempt);assert.equal(s.id,o.childId);
  assert.equal(s.projectId,a.projectId);assert.equal(s.projectRunId,r.id);assert.equal(s.projectRunOperationId,o.id);assert.equal(s.worktreeId,o.worktreeId);
  assert.deepEqual(s.startFailure,o.startFailure,'Session and operation start-failure records must agree');
  if(o.startFailure)assert.equal(q.reasonCode,o.startFailure.reasonCode);
  else assert.equal(q.reasonCode,'worker_failed','Unknown launch outcomes cannot be treated as an acknowledged worker failure');
}
