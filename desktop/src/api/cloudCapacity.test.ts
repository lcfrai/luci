import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { capacityMoney, cloudCapacityApi, type CloudCapacityInput, type CloudCapacityPolicy } from './cloudCapacity';

afterEach(()=>vi.restoreAllMocks());
const policy:CloudCapacityPolicy={id:'capacity-owner',actor:{userId:'alice',projectId:'p-1'},harness:'codex',revision:2,profileDigest:'a'.repeat(64),catalogueDigest:'b'.repeat(64),maxHosts:4,hostSeconds:3600,idleSeconds:300,budgetMicros:20_000_000,expiresAt:'2030-01-01T00:00:00Z',enabled:true};
const input:CloudCapacityInput={harness:'codex',expectedRevision:1,maxHosts:4,hostSeconds:3600,idleSeconds:300,budgetMicros:20_000_000,expiresAt:policy.expiresAt,acknowledgeCost:false};

it('retains exact fractional dollar units and refuses rounded or exponent allowances',()=>{
  expect(capacityMoney('0.000001')).toBe(1);
  expect(capacityMoney('12.340005')).toBe(12_340_005);
  expect(capacityMoney('9007199254.740992')).toBeNull();
  expect(capacityMoney('1e4')).toBeNull();
  expect(capacityMoney('0.0000001')).toBeNull();
});
it('refuses a capacity review that changes the requested host allowance',async()=>{
  vi.spyOn(productApi,'request').mockResolvedValue({reviewId:'review-1',digest:'c'.repeat(64),policy:{...policy,maxHosts:8}});
  await expect(cloudCapacityApi.review('p-1','alice',input,new AbortController().signal)).rejects.toThrow('did not match');
});
it('does not enable a different review under an existing acknowledgment',async()=>{
  const request=vi.spyOn(productApi,'request');
  const review={reviewId:'review-1',digest:'c'.repeat(64),policy};
  await expect(cloudCapacityApi.enable('p-1','alice',{...input,acknowledgeCost:true,reviewDigest:review.digest,reviewId:'review-2'},review,new AbortController().signal)).rejects.toThrow('changed');
  expect(request).not.toHaveBeenCalled();
});
it('refuses private capacity belonging to another project',async()=>{
  vi.spyOn(productApi,'request').mockResolvedValue({configured:true,operator:{parentHostId:'azure',profileDigest:'a'.repeat(64),maxHosts:8,minHostSeconds:1800,bootstrapSeconds:900,maxHostSeconds:7200,hostCostMicrosPerHour:1_000_000,budgetMicros:50_000_000},policy:{...policy,actor:{userId:'alice',projectId:'p-other'}},hosts:[]});
  await expect(cloudCapacityApi.read('p-1','alice',new AbortController().signal)).rejects.toThrow('could not be read');
});

it('does not send a retained enable from a different team',async()=>{
  const request=vi.spyOn(productApi,'request');
  const review={reviewId:'review-1',digest:'c'.repeat(64),policy:{...policy,actor:{...policy.actor,teamId:'team-old'}}};
  await expect(cloudCapacityApi.enable('p-1','alice',{...input,acknowledgeCost:true,reviewDigest:review.digest,reviewId:review.reviewId},review,new AbortController().signal,'team-new')).rejects.toThrow('changed');
  expect(request).not.toHaveBeenCalled();
});
it('refuses a same-user policy returned from another team',async()=>{
  vi.spyOn(productApi,'request').mockResolvedValue({configured:false,policy:{...policy,actor:{...policy.actor,teamId:'team-old'}},hosts:[]});
  await expect(cloudCapacityApi.read('p-1','alice',new AbortController().signal,'team-new')).rejects.toThrow('could not be read');
});
