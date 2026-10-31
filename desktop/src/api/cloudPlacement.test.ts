import { expect,it } from 'vitest';
import { validCloudHostRoute } from './projectRuns';
import { validCloudPoolFields } from './coordinatorContinuations';
it('requires a real closed host allow-list and catalogue digest while preserving omitted legacy routes',()=>{
 expect(validCloudHostRoute({})).toBe(true);
 expect(validCloudHostRoute({executionLocation:'cloud-microvm',cloudHostIds:['host-a'],cloudPoolDigest:'a'.repeat(64)})).toBe(true);
 for(const row of [{cloudHostIds:['host-a']},{executionLocation:'local-docker',cloudHostIds:['host-a'],cloudPoolDigest:'a'.repeat(64)},{executionLocation:'cloud-microvm',cloudHostIds:['host-a','host-a'],cloudPoolDigest:'a'.repeat(64)},{executionLocation:'cloud-microvm',cloudHostIds:['https://external/'],cloudPoolDigest:'a'.repeat(64)},{executionLocation:'cloud-microvm',cloudHostIds:['host-a'],cloudPoolDigest:['a'.repeat(64)]}])expect(validCloudHostRoute(row)).toBe(false);
});
it('rejects malformed or credential-bearing pool descriptors without changing legacy capacity',()=>{
 expect(validCloudPoolFields({provider:'azure-e2b-embed'})).toBe(true);
 const host={id:'host-a',name:'Approved host',provider:'azure',region:'australiaeast',templateId:'template',default:true,draining:false,configuredAvailableSlots:1,estimatedTaskCostMicros:100,maxTaskSeconds:60};
 const pool={provider:'cloud-pool',placementDigest:'a'.repeat(64),hosts:[host]};expect(validCloudPoolFields(pool)).toBe(true);
 for(const value of [{...pool,hosts:[host,host]},{...pool,placementDigest:['a'.repeat(64)]},{...pool,hosts:[{...host,endpoint:'https://private'}]},{...pool,hosts:[{...host,default:false}]},{...pool,hosts:[{...host,draining:'false'}]}])expect(validCloudPoolFields(value)).toBe(false);
});
