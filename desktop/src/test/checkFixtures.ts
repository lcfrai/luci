import type {CheckRun} from '../api/product';
import type {CheckRecoveryReview} from '../api/projectCheckContracts';
export function checkRunFixture(patch: Partial<CheckRun> = {}): CheckRun {
  return {id:'check-a',projectId:'p-1',revision:4,resourcesHeld:true,stage:'cleanup',reasonCode:'cleanup_unconfirmed',commit:'a'.repeat(40),sourceDigest:'b'.repeat(64),dirty:false,execution:'docker',status:'unknown',evidenceStatus:'unverified',createdAt:'2026-10-08T00:00:00Z',updatedAt:'2026-10-08T00:00:00Z',message:'Cleanup could not be confirmed.',commands:[{label:'Unit tests',executable:'npm',args:['test'],allocationId:'allocation-a',status:'passed',exitCode:0,output:'Retained test output',truncated:false,cleanupConfirmed:false}],...patch};
}
export function checkRecoveryFixture(patch: Partial<CheckRecoveryReview> = {}): CheckRecoveryReview {
  return {projectId:'p-1',checkId:'check-a',revision:4,state:'available',method:'remove-exact',commandIndex:0,recoveryDigest:'d'.repeat(64),reasonCode:'exact_cleanup',message:'The exact original allocation can be stopped and removed.',...patch};
}
