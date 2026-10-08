import type {ProjectRun, RunOperation} from '../api/projectRuns';
import type {TaskHandoffView} from '../api/swarmHandoff';
import {swarmRunFixture} from './swarmFixtures';

export function handoffFixture() {
  const run = swarmRunFixture();
  const operation: RunOperation = {id:'op-a',kind:'session',childId:'child-a',taskId:'a',worktreeId:'task-a',attempt:1,status:'review-required',allocation:'ready',requestDigest:'d'.repeat(64),createdAt:run.createdAt,updatedAt:run.updatedAt};
  run.operations = [operation];
  run.swarm!.tasks[0].operationId = operation.id;
  const handoff: TaskHandoffView = {
    projectId:run.projectId,runId:run.id,taskId:'a',operationId:operation.id,status:'captured',digest:'a'.repeat(64),
    receipt:{version:1,projectId:run.projectId,runId:run.id,taskId:'a',operationId:operation.id,childId:operation.childId,worktreeId:operation.worktreeId!,requestDigest:operation.requestDigest,implementationDigest:'b'.repeat(64),contextDigest:'c'.repeat(64),sourceCommit:'e'.repeat(40),sourceDigest:'f'.repeat(64),sourceDirty:false,status:'captured',capturedAt:run.createdAt,
      reported:{version:1,summary:'Added the shared settings view.',changedFiles:['src/settings.ts'],decisions:[{decision:'Reuse the project permissions.',rationale:'Keep authorization consistent.'}],reportedChecks:[{command:'npm test',outcome:'passed',detail:'Worker says the suite passed.'}],questions:[{question:'Should exports include archived projects?',impact:'This changes the export scope.',blocking:true}],limitations:['Native Windows behavior remains unverified.']}},
    answers:[],questionIds:['1'.repeat(64)],recordedChecks:[],integration:null,
  };
  return {run, operation, handoff};
}
export function handoffAnswerResult(run: ProjectRun, handoff: TaskHandoffView, answer = 'Exclude archived projects.') {
  return {run:{...run,revision:run.revision+1},handoff:{...handoff,answers:[{questionId:handoff.questionIds[0],answer,answeredAt:run.createdAt,attribution:'Project editor'}]}};
}
