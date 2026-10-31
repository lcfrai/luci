import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { runtimePrerequisitesApi, validRuntimePreparation, type RuntimePreparation } from './runtimePrerequisites';
const registry={id:'registry-'+'a'.repeat(32),projectId:'p-one',name:'Private build registry',repository:'registry.example/team/app',revision:1,fingerprint:'b'.repeat(64)};
const fixture=():RuntimePreparation=>({plan:{id:'prep-'+'c'.repeat(32),projectId:'p-one',actor:{userId:'alice',projectId:'p-one'},blueprintRevision:2,blueprintDigest:'d'.repeat(64),contextDigest:'e'.repeat(64),registry,builds:[{selection:{serviceId:'app',dockerfile:'Dockerfile',memoryMiB:512,cpuMillis:1000},port:8080,artifacts:[]}],monthlyBudgetUsd:50,createdAt:'2026-10-31T01:00:00Z',expiresAt:'2026-11-01T01:00:00Z',digest:'f'.repeat(64)},revision:1,builds:[],suggestedTasks:[]});
afterEach(()=>vi.restoreAllMocks());
it('correlates exact saved template registry and reviewed build inputs without an effect endpoint',async()=>{
 const row=fixture(),request=vi.spyOn(productApi,'request').mockResolvedValue(row),signal=new AbortController().signal;
 const input={requestKey:'explicit-original-review',blueprintRevision:2,blueprintDigest:row.plan.blueprintDigest,registryId:registry.id,registryRevision:1,builds:[{cpuMillis:1000,memoryMiB:512,dockerfile:'Dockerfile',serviceId:'app'}],monthlyBudgetUsd:50,acknowledgeBuildAndPush:true};
 expect((await runtimePrerequisitesApi.review('p-one',input,signal)).plan.id).toBe(row.plan.id);
 expect(request).toHaveBeenCalledWith('/v1/product/projects/p-one/runtime-preparations',input,'POST',30000,signal);
 request.mockResolvedValue({...row,plan:{...row.plan,registry:{...registry,revision:2}}});await expect(runtimePrerequisitesApi.review('p-one',input)).rejects.toThrow('exact selected inputs');
});
it('rejects cross-project descriptors malformed state and mismatched private actor scope',()=>{
 const row=fixture();expect(validRuntimePreparation(row,'p-one')).toBe(true);
 expect(validRuntimePreparation({...row,plan:{...row.plan,projectId:'p-other'}},'p-one')).toBe(false);
 expect(validRuntimePreparation({...row,plan:{...row.plan,registry:{...registry,password:'must-not-appear'}}},'p-one')).toBe(false);
 expect(validRuntimePreparation({...row,plan:{...row.plan,actor:{...row.plan.actor,projectId:'p-other'}}},'p-one')).toBe(false);
 expect(validRuntimePreparation({...row,builds:[{serviceId:'app',runId:'run-one',checkId:'check-one',commit:'a'.repeat(40),sourceDigest:'b'.repeat(64),runInputDigest:'c'.repeat(64),checkDigest:'d'.repeat(64),attempted:true,runtime:{phase:['published'],resourcesHeld:false,cleanupConfirmed:true}}]},'p-one')).toBe(false);
});
it('verifies generated task content hashes before exposing a preparation for plan save',async()=>{
 const {webcrypto}=await vi.importActual<{webcrypto:Crypto}>('node:crypto');vi.stubGlobal('crypto',webcrypto);
 try{const row=fixture();row.plan.builds[0].artifacts=[{path:'sdk.mjs',content:'hello',sha256:'2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824'}];const request=vi.spyOn(productApi,'request').mockResolvedValue(row);await expect(runtimePrerequisitesApi.get('p-one',row.plan.id)).resolves.toMatchObject({revision:1});request.mockResolvedValue({...row,plan:{...row.plan,builds:[{...row.plan.builds[0],artifacts:[{...row.plan.builds[0].artifacts[0],content:'tampered'}]}]}});await expect(runtimePrerequisitesApi.get('p-one',row.plan.id)).rejects.toThrow('original hash');}finally{vi.unstubAllGlobals();}
});

it('does not replace an original preparation with a different recorded ID',async()=>{
 const row=fixture();vi.spyOn(productApi,'request').mockResolvedValue({...row,plan:{...row.plan,id:'prep-'+'e'.repeat(32)}});
 await expect(runtimePrerequisitesApi.get('p-one',row.plan.id)).rejects.toThrow(/original preparation/);
});
it('validates the exact persisted batch source and final blueprint before claiming completion',()=>{
 const row=fixture(),source={serviceId:'app',runId:'run-one',checkId:'check-one',commit:'a'.repeat(40),sourceDigest:'b'.repeat(64),runInputDigest:'c'.repeat(64),checkDigest:'d'.repeat(64),attempted:true as const};
 row.builds=[source];row.execution={runId:source.runId,checkId:source.checkId,commit:source.commit,sourceDigest:source.sourceDigest,runInputDigest:source.runInputDigest,checkDigest:source.checkDigest,startedAt:'2026-10-31T01:03:00Z',state:'running'};
 expect(validRuntimePreparation(row,'p-one')).toBe(true);
 row.builds[0].commit='e'.repeat(40);expect(validRuntimePreparation(row,'p-one')).toBe(false);row.builds[0].commit=row.execution.commit;
 row.execution.state='complete';row.execution.finishedAt='2026-10-31T01:04:00Z';expect(validRuntimePreparation(row,'p-one')).toBe(false);
 row.savedBlueprintRevision=3;expect(validRuntimePreparation(row,'p-one')).toBe(true);
});
