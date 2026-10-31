import { expect,it } from 'vitest';
import { resourceSelectionMatches,validRuntimeResourcePlan,validRuntimeResourceOperation,type RuntimeResourcePlan } from './runtimeResources';

const plan=():RuntimeResourcePlan=>({targetId:'cluster-one',targetFingerprint:'a'.repeat(64),namespace:'apps',namespaceUid:'ns-original',volumes:[{serviceId:'model',storageClass:'managed-csi',sizeGiB:10,claimName:'luci-model-original',storageClassUid:'sc-original',storageClassVersion:'12',bindingMode:'WaitForFirstConsumer',reclaimPolicy:'Delete'}],models:[{serviceId:'model',url:'https://models.example/weights.gguf',sha256:'b'.repeat(64),bytes:5}],secrets:[{name:'transfer-key',kind:'transfer-key',keys:['transfer.key'],generationId:'secret-original',serviceId:'model'}],modelsRelativePaths:{model:'weights/model.gguf'},modelSinkBuilderImage:'registry.example/go@sha256:'+'c'.repeat(64),modelSinkSourceSha256:'d'.repeat(64)});
it('requires exact new volume/model inputs while preserving omitted legacy resource selection',()=>{
 expect(resourceSelectionMatches(undefined,undefined)).toBe(true);const p=plan();expect(validRuntimeResourcePlan(p)).toBe(true);
 const input={volumes:[{serviceId:'model',storageClass:'managed-csi',sizeGiB:10}],models:p.models,generateTelemetryTls:false,generateImagePullSecret:false,modelSinkBuilderImage:p.modelSinkBuilderImage,acknowledgeNewResources:true};expect(resourceSelectionMatches(p,input)).toBe(true);
 expect(resourceSelectionMatches(p,{...input,models:[{...p.models[0],sha256:'e'.repeat(64)}]})).toBe(false);
 expect(resourceSelectionMatches(p,{...input,volumes:[{...input.volumes[0],storageClass:'other'}]})).toBe(false);
});
it('rejects credential values malformed enum types and unreviewed telemetry binding',()=>{
 const p=plan();for(const v of [{...p,secrets:[{...p.secrets[0],data:{'transfer.key':'not-public'}}]},{...p,volumes:[{...p.volumes[0],bindingMode:['Immediate']}]},{...p,volumes:[{...p.volumes[0],purpose:'telemetry-wal'}]},{...p,models:[{...p.models[0],url:'https://user:secret@models.example/file'}]},{...p,models:[{...p.models[0],bytes:9*2**30}]}])expect(validRuntimeResourcePlan(v)).toBe(false);
});
it('never labels a receipt or matching file hash as original zero-exit proof',()=>{
 const p=plan(),operation={source:{runId:'run-one',checkId:'check-one',commit:'a'.repeat(40),sourceDigest:'b'.repeat(64),runInputDigest:'c'.repeat(64),checkDigest:'d'.repeat(64),attempted:true},applyAttempted:true,revision:1,state:'pending',helperBuildAttempted:false,resources:[],models:[{serviceId:'model',operationId:'prep-model-original',podName:'model-sink-original',uploadAttempted:true,originalZeroExit:true,state:'stored-original-zero'}],observedAt:'2026-10-31T00:00:00Z'};
 expect(validRuntimeResourceOperation(operation,p,'prep-original')).toBe(false);
 expect(validRuntimeResourceOperation({...operation,models:[{...operation.models[0],originalZeroExit:false}]},p,'prep-original')).toBe(true);
 expect(validRuntimeResourceOperation({...operation,state:['ready']},p,'prep-original')).toBe(false);
});
