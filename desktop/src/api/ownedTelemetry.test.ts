import {describe,it,expect} from 'vitest';
import {validTelemetryStorage,validCollector} from './deploymentBlueprint';
import {validRuntimeResourcePlan,resourceSelectionMatches,type RuntimeResourcePlan} from './runtimeResources';
import {newTelemetryStore} from '../components/journey/TelemetryStorageEditor';
const tls={resourceName:'client',caKey:'ca.crt',certKey:'tls.crt',keyKey:'tls.key'};
describe('Owned telemetry contracts',()=>{
 it('keeps external HTTPS configuration separate from explicit private destinations',()=>{
  const external={profile:'otelcol-contrib-0.163.0',destination:{resourceId:'analytics',endpoint:'https://owned.example/otlp'},receiverTLS:tls,events:[{id:'activation',description:'Core outcome'}]};expect(validCollector(external)).toBe(true);
  expect(validCollector({...external,destination:{resourceId:'',endpoint:''},ownedDestination:{metricsServiceId:'metrics',tracesServiceId:'traces',clientTLS:tls}})).toBe(true);
  expect(validCollector({...external,ownedDestination:{metricsServiceId:'metrics',tracesServiceId:'traces',clientTLS:tls}})).toBe(false);
  expect(validCollector({...external,ownedDestination:{metricsServiceId:['metrics'],tracesServiceId:'traces',clientTLS:tls}})).toBe(false);
 });
 it('requires the exact durable storage profile and refuses array-coerced profiles',()=>{
  const metrics=newTelemetryStore('metrics-store','metrics');expect(validTelemetryStorage(metrics.telemetryStorage)).toBe(true);
  expect(validTelemetryStorage({...metrics.telemetryStorage,profile:['prometheus-3.15.1']})).toBe(false);
  expect(validTelemetryStorage({...metrics.telemetryStorage,profile:'tempo-2.10.10-azure'})).toBe(false);
 });
 it('binds private Blob imports without pretending they generated TLS',()=>{
  const blob={jobId:'cj-original',planDigest:'a'.repeat(64),accountResourceId:'/subscriptions/a/storage',accountCreatedAt:'2026-10-31T00:00:00Z',containerResourceId:'/subscriptions/a/storage/blobServices/default/containers/traces',serviceId:'traces',secretName:'blob-key',secretKey:'account.key',generationId:'secret-original'};
  const plan:RuntimeResourcePlan={targetId:'cluster-original',targetFingerprint:'b'.repeat(64),namespace:'apps',namespaceUid:'ns-original',volumes:[],models:[],secrets:[{name:'blob-key',kind:'telemetry-storage-key',keys:['account.key'],generationId:'secret-original',serviceId:'traces'}],telemetryBlob:blob};
  expect(validRuntimeResourcePlan(plan)).toBe(true);expect(resourceSelectionMatches(plan,{telemetryBlobJobId:'cj-original',generateTelemetryTls:false,generateImagePullSecret:false,acknowledgeNewResources:true})).toBe(true);
  expect(resourceSelectionMatches(plan,{telemetryBlobJobId:'cj-other',generateTelemetryTls:false,generateImagePullSecret:false,acknowledgeNewResources:true})).toBe(false);
  expect(validRuntimeResourcePlan({...plan,telemetryBlob:undefined})).toBe(false);expect(validRuntimeResourcePlan({...plan,telemetryBlob:{...blob,accountKeyValue:'must-not-appear'}})).toBe(false);
 });
});
