import { afterEach, expect, it, vi } from 'vitest';
import { deploymentBlueprintApi, nodeTelemetryPackages, validStoredBlueprint, type DeploymentBlueprint, type DeploymentBlueprintPlan } from './deploymentBlueprint';
import { productApi } from './product';
import { validKubernetesTelemetry, type KubernetesReview, type KubernetesTelemetry } from './kubernetesDeployment';
const { webcrypto } = await vi.importActual<{ webcrypto: Crypto }>('node:crypto');
afterEach(()=>{vi.restoreAllMocks();vi.unstubAllGlobals();});
const blueprint=():DeploymentBlueprint=>({version:1,projectId:'p-one',source:{commit:'b'.repeat(40),configurationRevision:1},infrastructure:{id:'cluster',kind:'kubernetes',reference:'target-one',namespace:'apps'},externalResources:[{id:'analytics',kind:'analytics',reference:'owned-backend'}],services:[{id:'collector',kind:'collector',image:'registry/otel@sha256:'+'a'.repeat(64),port:4318,replicas:1,resources:{requestMilliCpu:100,limitMilliCpu:500,requestMemoryMiB:128,limitMemoryMiB:256},collector:{profile:'otelcol-contrib-0.163.0',destination:{resourceId:'analytics',endpoint:'https://owned.example/otlp'},receiverTLS:{resourceName:'server-tls',caKey:'ca',certKey:'cert',keyKey:'key'},events:[{id:'activation',description:'Core outcome'}]}},{id:'web',kind:'app',image:'registry/web@sha256:'+'c'.repeat(64),port:8080,replicas:1,resources:{requestMilliCpu:100,limitMilliCpu:500,requestMemoryMiB:128,limitMemoryMiB:512},telemetry:{resourceId:'collector',endpoint:'',clientTLS:{resourceName:'client-tls',caKey:'ca',certKey:'cert',keyKey:'key'},instrumentation:{runtime:'nodejs',dependencies:Object.fromEntries(nodeTelemetryPackages.map(name=>[name,'2.0.1']))}}}]});
const request=():KubernetesReview['request']=>({identity:{projectId:'p-one',actorId:'alice',targetId:'target-one',targetFingerprint:'d'.repeat(64),operationId:'workload-'+'e'.repeat(32),planDigest:'f'.repeat(64)},blueprint:blueprint(),approvalReference:'workload-'+'e'.repeat(32),expected:[]});
const sample=():KubernetesTelemetry=>({state:'recorded',collectorId:'collector',serviceUid:'svc-one',deploymentUid:'deployment-one',collectorPodUid:'collector-pod',applicationPodUids:['app-pod'],planDigest:'f'.repeat(64),sourceCommit:'b'.repeat(40),observedAt:'2026-10-31T00:00:00Z',continuity:'baseline',samples:[{serviceId:'web',instanceUid:'app-pod',eventId:'activation',outcome:'success',reportedCount:7}],omitted:false,evidenceDigest:'a'.repeat(64)});
it('retains exact Collector and app mTLS schema while rejecting unknown configuration fields',()=>{
  const value=blueprint();expect(validStoredBlueprint(value,'p-one')).toBe(true);
  const changed=JSON.parse(JSON.stringify(value));changed.services[0].collector.inlineToken='PRIVATE';expect(validStoredBlueprint(changed,'p-one')).toBe(false);
});
it('accepts only original resource-linked finite event samples and preserves explicit no-data',()=>{
  const review={request:request()},resources=[{kind:'Service',name:'collector-012345abcdef',observedUid:'svc-one'},{kind:'Deployment',name:'collector-012345abcdef',observedUid:'deployment-one'}];
  expect(validKubernetesTelemetry(sample(),review,resources)).toBe(true);
  expect(validKubernetesTelemetry(sample(),review,resources.map(row=>({...row,name:'web-012345abcdef'})))).toBe(false);
  expect(validKubernetesTelemetry({...sample(),sourceCommit:'0'.repeat(40)},review,resources)).toBe(false);
  expect(validKubernetesTelemetry({...sample(),samples:[{...sample().samples[0],eventId:'unknown'}]},review,resources)).toBe(false);
  expect(validKubernetesTelemetry({...sample(),samples:[{...sample().samples[0],reportedCount:Number.NaN}]},review,resources)).toBe(false);
  expect(validKubernetesTelemetry({...sample(),state:'no-data',samples:[]},review,resources)).toBe(true);
  for(const value of [{...sample(),state:['recorded']},{...sample(),continuity:['baseline']},{...sample(),samples:[{...sample().samples[0],outcome:['success']}]}])expect(validKubernetesTelemetry(value,review,resources)).toBe(false);
});
it('verifies supporting integration bytes before accepting or offering generated code',async()=>{
  vi.stubGlobal('crypto',webcrypto);const input=blueprint();input.services[0].dependsOn=['analytics','cluster'];input.services[1].dependsOn=['cluster','collector'];
  const digest='f'.repeat(64),supportNames=['collector.collector-config.json','web.bootstrap.mjs','web.events.mjs','web.dependencies.json','web.instrumentation.md'];
  const value:DeploymentBlueprintPlan={version:1,digest,state:'compiled-prototype',blueprint:input,steps:[],decisions:[],capabilities:[],warnings:[],resourceSummary:{replicas:2,requestMilliCpu:200,limitMilliCpu:1000,requestMemoryMiB:256,limitMemoryMiB:768,gpuCount:0},sourceVerified:false,effectsApplied:false,artifacts:input.services.flatMap(service=>['deployment','service'].map(kind=>({name:`${service.id}-abcdef.${kind}.json`,mediaType:'application/json' as const,sha256:'0'.repeat(64),content:JSON.stringify({apiVersion:kind==='deployment'?'apps/v1':'v1',kind:kind==='deployment'?'Deployment':'Service',metadata:{namespace:'apps',annotations:{'luci.ai/plan-digest':digest}}})+'\n'}))),supportingArtifacts:supportNames.map(name=>({name,mediaType:name.endsWith('.json')?'application/json' as const:name.endsWith('.mjs')?'text/javascript' as const:'text/markdown' as const,sha256:'0'.repeat(64),content:'Review this generated integration file.\n'}))};
  for(const artifact of [...value.artifacts,...value.supportingArtifacts!]){const raw=await webcrypto.subtle.digest('SHA-256',new TextEncoder().encode(artifact.content));artifact.sha256=Array.from(new Uint8Array(raw),byte=>byte.toString(16).padStart(2,'0')).join('');}
  vi.spyOn(productApi,'request').mockResolvedValue(value);expect(await deploymentBlueprintApi.compile(input)).toEqual(value);
  value.supportingArtifacts![1].content+='unreviewed change';await expect(deploymentBlueprintApi.compile(input)).rejects.toThrow('digest check');
});
