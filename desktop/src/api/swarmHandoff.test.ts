import {beforeEach, expect, it, vi} from 'vitest';
import {productApi} from './product';
import {swarmHandoffApi, type TaskHandoffView} from './swarmHandoff';
import {handoffAnswerResult, handoffFixture} from '../test/handoffFixtures';
import httpGolden from '../../../control-plane/internal/product/testdata/task-handoff-http.json?raw';

beforeEach(() => vi.restoreAllMocks());
it.each(['captured','missing','notRecorded'])('decodes the shared actual-handler %s contract without recreating its wire shape', async name => {
  const golden=JSON.parse(httpGolden)[name];
  const operation={...handoffFixture().operation,...golden.operation};
  vi.spyOn(productApi,'request').mockResolvedValue(golden.response);
  const value=await swarmHandoffApi.get(golden.response.handoff.projectId,golden.response.handoff.runId,operation);
  expect(value.status).toBe(golden.response.handoff.status);
  expect(value.answers).toEqual(golden.response.handoff.answers);
  expect(value.receipt).toEqual(golden.response.handoff.receipt);
  if(name==='captured'){
    expect(value.recordedChecks[0].commands[0].args).toEqual([]);
    expect(value.recordedChecks[0].checkId).toBe('check-wire-contract');
    expect(value.receipt?.reported?.reportedChecks[0].outcome).toBe('unknown');
  } else expect(value.recordedChecks).toEqual([]);
});
it('reads one exact attempt without capture or execution and preserves separate reported and recorded checks', async () => {
  const f=handoffFixture(), signal=new AbortController().signal;
  const request=vi.spyOn(productApi,'request').mockResolvedValue({handoff:f.handoff});
  const result=await swarmHandoffApi.get(f.run.projectId,f.run.id,f.operation,signal);
  expect(result.receipt?.reported?.reportedChecks[0].outcome).toBe('passed');
  expect(result.recordedChecks).toEqual([]);
  expect(request).toHaveBeenCalledExactlyOnceWith('/v1/product/projects/p-1/runs/run-1/swarm/operations/op-a/handoff',undefined,'GET',30_000,signal);
});
it.each(['projectId','runId','taskId','operationId'])('rejects a foreign %s before rendering private claims', async field => {
  const f=handoffFixture();vi.spyOn(productApi,'request').mockResolvedValue({handoff:{...f.handoff,[field]:'foreign'}});
  await expect(swarmHandoffApi.get(f.run.projectId,f.run.id,f.operation)).rejects.toThrow(/exact attempt/);
});
it.each(['childId','worktreeId','requestDigest','projectId','runId','taskId','operationId'])('rejects receipt identity mismatch %s', async field => {
  const f=handoffFixture();vi.spyOn(productApi,'request').mockResolvedValue({handoff:{...f.handoff,receipt:{...f.handoff.receipt,[field]:'foreign'}}});
  await expect(swarmHandoffApi.get(f.run.projectId,f.run.id,f.operation)).rejects.toThrow(/exact attempt/);
});
it.each(['not-recorded','missing','invalid','unavailable'] as const)('keeps %s honest without inventing a report', async status => {
  const f=handoffFixture();const handoff:TaskHandoffView={...f.handoff,status,questionIds:[],answers:[],receipt:status==='not-recorded'?null:{...f.handoff.receipt!,status,reported:null}};
  if(status==='not-recorded')delete handoff.digest;
  vi.spyOn(productApi,'request').mockResolvedValue({handoff});
  const result=await swarmHandoffApi.get(f.run.projectId,f.run.id,f.operation);
  expect(result.receipt?.reported ?? null).toBeNull();expect(result.recordedChecks).toEqual([]);expect(result.questionIds).toEqual([]);
});
it.each(['unknown-question','duplicate-question','duplicate-answer','wrong-status','unsafe-path','oversize-answer','oversize-question','foreign-check'])('rejects inconsistent or unbounded %s',async variant=>{
  const f=handoffFixture();const h=f.handoff;
  if(variant==='unknown-question')h.answers=[{questionId:'2'.repeat(64),answer:'No',answeredAt:f.run.createdAt,attribution:'Editor'}];
  if(variant==='duplicate-question')h.questionIds.push(h.questionIds[0]);
  if(variant==='duplicate-answer'){h.answers=handoffAnswerResult(f.run,h).handoff.answers;h.answers.push({...h.answers[0]});}
  if(variant==='wrong-status')h.status='missing';
  if(variant==='unsafe-path')h.receipt!.reported!.changedFiles=['../private'];
  if(variant==='oversize-answer')h.answers=handoffAnswerResult(f.run,h,'🙂'.repeat(1001)).handoff.answers;
  if(variant==='oversize-question')h.receipt!.reported!.questions[0].question='x'.repeat(2001);
  if(variant==='foreign-check')h.recordedChecks=[{projectId:'foreign'} as TaskHandoffView['recordedChecks'][number]];
  vi.spyOn(productApi,'request').mockResolvedValue({handoff:h});
  await expect(swarmHandoffApi.get(f.run.projectId,f.run.id,f.operation)).rejects.toThrow(/exact attempt/);
});
it('normalizes a recorded zero-argument command without promoting worker claims to evidence',async()=>{
  const f=handoffFixture();
  const check={checkId:'check-1',projectId:f.run.projectId,commit:f.handoff.receipt!.sourceCommit,sourceDigest:f.handoff.receipt!.sourceDigest,execution:'local',createdAt:f.run.createdAt,joinedAt:f.run.createdAt,originalAvailable:false,commands:[{label:'Version',executable:'version-tool',args:null,status:'passed',exitCode:0}]};
  vi.spyOn(productApi,'request').mockResolvedValue({handoff:{...f.handoff,recordedChecks:[check]}});
  const result=await swarmHandoffApi.get(f.run.projectId,f.run.id,f.operation);
  expect(result.recordedChecks).toHaveLength(1);expect(result.recordedChecks[0].commands[0].args).toEqual([]);expect(result.receipt!.reported!.reportedChecks[0].command).toBe('npm test');
});
it('captures and answers only with explicit revision and exact question receipt',async()=>{
  const f=handoffFixture(),result=handoffAnswerResult(f.run,f.handoff),request=vi.spyOn(productApi,'request').mockResolvedValue(result);
  await swarmHandoffApi.collect(f.run,f.operation);
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/runs/run-1/swarm/operations/op-a/handoff/collect',{expectedRevision:f.run.revision});
  await swarmHandoffApi.answer(f.run,f.operation,f.handoff.digest!,f.handoff.questionIds[0],'Keep archived projects out.');
  expect(request).toHaveBeenLastCalledWith(`/v1/product/projects/p-1/runs/run-1/swarm/operations/op-a/handoff/questions/${f.handoff.questionIds[0]}/answer`,{expectedRevision:f.run.revision,handoffDigest:f.handoff.digest,answer:'Keep archived projects out.'});
  expect(request).toHaveBeenCalledTimes(2);
});
it.each(['','   ','🙂'.repeat(1001),'x\0y'])('refuses an empty or oversized UTF-8 answer before sending',async answer=>{
  const f=handoffFixture(),request=vi.spyOn(productApi,'request');
  await expect(swarmHandoffApi.answer(f.run,f.operation,f.handoff.digest!,f.handoff.questionIds[0],answer)).rejects.toThrow(/4,000 UTF-8 bytes/);
  expect(request).not.toHaveBeenCalled();
});
it.each(['foreign-run','foreign-project','foreign-attempt','older-revision'])('rejects a mismatched mutation %s',async variant=>{
  const f=handoffFixture(),result=handoffAnswerResult(f.run,f.handoff);
  if(variant==='foreign-run')result.run.id='elsewhere';
  if(variant==='foreign-project')result.run.projectId='elsewhere';
  if(variant==='foreign-attempt')result.run.operations=[{...f.operation,childId:'elsewhere'}];
  if(variant==='older-revision')result.run.revision=f.run.revision-1;
  vi.spyOn(productApi,'request').mockResolvedValue(result);
  await expect(swarmHandoffApi.collect(f.run,f.operation)).rejects.toThrow(/exact attempt/);
});
it('does not retry a mutation after a lost response',async()=>{
  const f=handoffFixture(),request=vi.spyOn(productApi,'request').mockRejectedValue(new Error('Lost response'));
  await expect(swarmHandoffApi.collect(f.run,f.operation)).rejects.toThrow('Lost response');expect(request).toHaveBeenCalledOnce();
});
