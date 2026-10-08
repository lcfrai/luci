import sharedHTTP from '../../../control-plane/internal/product/testdata/project-check-http.json?raw';
import {expect,it,vi} from 'vitest';
import {createProductClient} from './product';
import {checkCanRecover,checkRunResult,checkRecoveryResult} from './projectCheckContracts';
import {checkRecoveryFixture,checkRunFixture} from '../test/checkFixtures';

it('keeps passed command output separate from an uncertain cleanup outcome',()=>{
  const value=checkRunResult('p-1',checkRunFixture(),'check-a');
  expect(value.status).toBe('unknown');expect(value.commands[0].status).toBe('passed');expect(value.commands[0].output).toBe('Retained test output');expect(value.resourcesHeld).toBe(true);
});
it.each([{resourcesHeld:undefined},{revision:-1},{revision:1.5},{status:'future-state'},{status:'passed'}])('refuses a malformed modern record or invented passed cleanup: %j',patch=>{
  expect(()=>checkRunResult('p-1',checkRunFixture(patch))).toThrow('could not be verified');
});
it('reads legacy host history without adding recovery or capacity authority',()=>{
  const value=checkRunFixture({revision:undefined,resourcesHeld:undefined,execution:'host'});delete value.commands[0].allocationId;delete value.commands[0].cleanupConfirmed;
  const saved=checkRunResult('p-1',value);expect(saved.execution).toBe('host');expect(saved.revision).toBeUndefined();expect(saved.resourcesHeld).toBeUndefined();
});
it.each([{projectId:'other'},{id:'other'}])('refuses a foreign selected check: %j',patch=>expect(()=>checkRunResult('p-1',checkRunFixture(patch),'check-a')).toThrow('could not be verified'));
it.each([{method:'future-method'},{state:'future-state'},{commandIndex:8},{revision:0},{recoveryDigest:null},{recoveryDigest:'a'.repeat(63)},{recoveryDigest:'A'.repeat(64)},{recoveryDigest:'z'.repeat(64)}])('refuses an unavailable recovery authority shape: %j',patch=>expect(()=>checkRecoveryResult('p-1','check-a',{...checkRecoveryFixture(),...patch})).toThrow('could not be verified'));
it('keeps pending recovery read-only until the exact method and digest are available',()=>{
  expect(checkCanRecover(checkRecoveryResult('p-1','check-a',checkRecoveryFixture({state:'pending',method:null,commandIndex:null,recoveryDigest:null})))).toBe(false);
  expect(checkCanRecover(checkRecoveryFixture({state:'pending'}))).toBe(true);
});
it('uses read-only review and exactly one revision-bound recovery POST without replaying errors',async()=>{
  const review=checkRecoveryFixture();const fetcher=vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(review),{status:200})).mockResolvedValueOnce(new Response(JSON.stringify({error:'check_recovery_conflict',message:'The check changed.'}),{status:409}));
  const client=createProductClient('http://localhost',fetcher);
  await expect(client.checkRecovery('p-1','check-a')).resolves.toEqual(review);
  expect(fetcher.mock.calls[0][0]).toBe('http://localhost/v1/product/projects/p-1/checks/check-a/recovery');expect(fetcher.mock.calls[0][1].method).toBe('GET');
  await expect(client.recoverCheck('p-1','check-a',review)).rejects.toThrow('The check changed');
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({expectedRevision:4,recoveryDigest:review.recoveryDigest,acknowledgeStop:true});expect(fetcher).toHaveBeenCalledTimes(2);
});
it('does not send recovery for a different check or read-only pending review',async()=>{
  const fetcher=vi.fn(),client=createProductClient('http://localhost',fetcher);
  await expect(client.recoverCheck('p-1','other',checkRecoveryFixture())).rejects.toThrow('Refresh this exact check');
  await expect(client.recoverCheck('p-1','check-a',checkRecoveryFixture({state:'pending',method:null}))).rejects.toThrow('Refresh this exact check');expect(fetcher).not.toHaveBeenCalled();
});
it('propagates cancellation to a scoped read without sending a write',async()=>{
  const fetcher=vi.fn(),controller=new AbortController();controller.abort();
  await expect(createProductClient('http://localhost',fetcher).checkRecovery('p-1','check-a',controller.signal)).rejects.toBeDefined();expect(fetcher).not.toHaveBeenCalled();
});

// This exact expected wire fixture is shared with the Go HTTP-handler comparison.
const httpFixtures=JSON.parse(sharedHTTP);
it.each(['blocked','unknownCleanup','recovered'])('decodes the shared %s check HTTP payload without inventing cleanup success',async name=>{
  const fixture=httpFixtures[name];
  const fetcher=vi.fn().mockResolvedValue(new Response(JSON.stringify(fixture),{status:200}));
  const saved=await createProductClient('http://localhost',fetcher).checkRun(fixture.projectId,fixture.id);
  expect(saved).toEqual({...fixture,commands:fixture.commands.map((command:Record<string,unknown>)=>({...command,args:command.args??[]}))});
  expect(saved.resourcesHeld).toBe(fixture.resourcesHeld);expect(saved.status).toBe(fixture.status);
});
it.each(['available','pending','complete'])('decodes the shared %s recovery HTTP payload with exact nullable authority',async name=>{
  const fixture=httpFixtures[name];
  const fetcher=vi.fn().mockResolvedValue(new Response(JSON.stringify(fixture),{status:200}));
  const saved=await createProductClient('http://localhost',fetcher).checkRecovery(fixture.projectId,fixture.checkId);
  expect(saved).toEqual(fixture);expect(checkCanRecover(saved)).toBe(name!=='complete');
});
it('reads the shared exact recovery POST response while retaining its prior command output',async()=>{
  const review=httpFixtures.available,fixture=httpFixtures.recovered;
  const fetcher=vi.fn().mockResolvedValue(new Response(JSON.stringify(fixture),{status:200}));
  const saved=await createProductClient('http://localhost',fetcher).recoverCheck(review.projectId,review.checkId,review);
  expect(saved.commands[0].output).toBe(fixture.commands[0].output);expect(saved.commands[0].cleanupConfirmed).toBe(true);expect(saved.resourcesHeld).toBe(false);
  expect(fetcher).toHaveBeenCalledOnce();expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({expectedRevision:review.revision,recoveryDigest:review.recoveryDigest,acknowledgeStop:true});
});
