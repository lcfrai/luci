import {beforeEach, expect, it, vi} from 'vitest';
import {productApi} from './product';
import {swarmCheckpointApi, type SwarmCheckpointReview} from './swarmCheckpoint';
import type {ProjectRun, RunOperation} from './projectRuns';
import {swarmRunFixture} from '../test/swarmFixtures';

function fixture() {
  const run: ProjectRun = swarmRunFixture();
  const source: RunOperation = {id:'old-op', childId:'old-child', taskId:'a', worktreeId:'old-tree', kind:'session', attempt:1, status:'failed', allocation:'ready', baseCommit:'a'.repeat(40), requestDigest:'r', createdAt:run.createdAt, updatedAt:run.updatedAt};
  run.operations=[source]; run.swarm!.tasks[0].operationId=source.id;
  const review: SwarmCheckpointReview = {projectId:run.projectId,runId:run.id,operationId:source.id,childId:source.childId,taskId:'a',expectedRevision:run.revision,sourceCommit:'b'.repeat(40),targetCommit:'a'.repeat(40),reviewDigest:'d'.repeat(64),canContinue:true,reason:'Ready for review.',totalBytes:3,files:[{path:'src/a.ts',status:'added',before:'',after:'ok\n',beforeMode:'',afterMode:'100644'}]};
  const operation: RunOperation = {...source,id:'new-op',childId:'new-child',worktreeId:'new-tree',attempt:2,status:'reserved',implementation:{...run.input.implementation,sourceCommit:'c'.repeat(40)},checkpoint:{sourceOperationId:source.id,sourceChildId:source.childId,sourceCommit:review.sourceCommit,sourceBaseCommit:source.baseCommit!,targetCommit:review.targetCommit,seedCommit:'c'.repeat(40),reviewDigest:review.reviewDigest,contextDigest:'e'.repeat(64),createdAt:run.createdAt}};
  const result={operation,run:{...run,revision:run.revision+1,operations:[source,operation],swarm:{...run.swarm!,tasks:run.swarm!.tasks.map(task=>task.id==='a'?{...task,operationId:operation.id}:task)}}};
  return {run,source,review,result};
}
beforeEach(()=>vi.restoreAllMocks());
it('reads an exact project/run/attempt review without a request body or mutation',async()=>{
  const f=fixture(),request=vi.spyOn(productApi,'request').mockResolvedValue({review:f.review});
  const signal=new AbortController().signal;
  expect(await swarmCheckpointApi.review(f.run,f.source,signal)).toEqual(f.review);
  expect(request).toHaveBeenCalledWith('/v1/product/projects/p-1/runs/run-1/swarm/operations/old-op/checkpoint',undefined,'GET',30_000,signal);
});
it.each(['projectId','runId','operationId','childId','taskId'] as const)('refuses a foreign %s before showing source text',async key=>{
  const f=fixture();vi.spyOn(productApi,'request').mockResolvedValue({review:{...f.review,[key]:'foreign'}});
  await expect(swarmCheckpointApi.review(f.run,f.source)).rejects.toThrow(/does not match/);
});
it.each(['../outside','/absolute','src\\wrong','src//gap','src/./file','src/file\u0000'])('refuses unsafe returned path %j',async path=>{
  const f=fixture();f.review.files[0].path=path;vi.spyOn(productApi,'request').mockResolvedValue({review:f.review});
  await expect(swarmCheckpointApi.review(f.run,f.source)).rejects.toThrow(/does not match/);
});
it('rejects inconsistent byte totals, oversize text, duplicate paths and unsupported modes',async()=>{
  for(const variant of ['bytes','oversize','duplicate','mode','revision']) {
    const f=fixture();
    if(variant==='bytes')f.review.totalBytes=0;
    if(variant==='oversize'){f.review.files[0].after='x'.repeat(128*1024+1);f.review.totalBytes=f.review.files[0].after.length;}
    if(variant==='duplicate'){f.review.files.push({...f.review.files[0],path:'SRC/A.TS'});f.review.totalBytes*=2;}
    if(variant==='mode')f.review.files[0].afterMode='120000';
    if(variant==='revision')f.review.expectedRevision++;
    vi.spyOn(productApi,'request').mockResolvedValue({review:f.review});
    await expect(swarmCheckpointApi.review(f.run,f.source)).rejects.toThrow(/does not match/);
  }
});
it('retains an explicit refusal with no partial content',async()=>{
  const f=fixture();Object.assign(f.review,{canContinue:false,reviewDigest:'',sourceCommit:'',targetCommit:'',files:[],totalBytes:0,reason:'The task is still active.'});
  vi.spyOn(productApi,'request').mockResolvedValue({review:f.review});
  expect(await swarmCheckpointApi.review(f.run,f.source)).toEqual(f.review);
});
it('reserves only with the exact reviewed identity and validates a distinct seeded attempt',async()=>{
  const f=fixture(),request=vi.spyOn(productApi,'request').mockResolvedValue(f.result);
  expect(await swarmCheckpointApi.reserve(f.run,f.source,f.review,'same-request')).toEqual(f.result);
  expect(request).toHaveBeenCalledWith('/v1/product/projects/p-1/runs/run-1/swarm/operations/old-op/checkpoint',{expectedRevision:f.run.revision,idempotencyKey:'same-request',reviewDigest:f.review.reviewDigest,childId:f.source.childId});
});
it('refuses stale or refused consent before issuing a reservation',async()=>{
  const f=fixture(),request=vi.spyOn(productApi,'request');
  await expect(swarmCheckpointApi.reserve(f.run,f.source,{...f.review,canContinue:false},'k')).rejects.toThrow();
  await expect(swarmCheckpointApi.reserve(f.run,f.source,{...f.review,expectedRevision:f.run.revision+1},'k')).rejects.toThrow();
  expect(request).not.toHaveBeenCalled();
});
it.each(['foreign','same-child','same-tree','wrong-seed','wrong-review'] as const)('refuses mismatched reservation %s',async variant=>{
  const f=fixture();
  if(variant==='foreign')f.result.run.projectId='foreign';
  if(variant==='same-child')f.result.operation.childId=f.source.childId;
  if(variant==='same-tree')f.result.operation.worktreeId=f.source.worktreeId;
  if(variant==='wrong-seed')f.result.operation.implementation!.sourceCommit='e'.repeat(40);
  if(variant==='wrong-review')f.result.operation.checkpoint!.reviewDigest='f'.repeat(64);
  vi.spyOn(productApi,'request').mockResolvedValue(f.result);
  await expect(swarmCheckpointApi.reserve(f.run,f.source,f.review,'k')).rejects.toThrow(/could not be verified/);
});
it('preserves an explicitly uncertain allocation without inventing a ready seed',async()=>{
  const f=fixture();f.result.operation.allocation='unknown';delete f.result.operation.implementation;delete f.result.operation.checkpoint!.seedCommit;
  vi.spyOn(productApi,'request').mockResolvedValue(f.result);
  const value=await swarmCheckpointApi.reserve(f.run,f.source,f.review,'k');
  expect(value.operation.allocation).toBe('unknown');expect(value.operation.checkpoint?.seedCommit).toBeUndefined();
});
