import { afterEach, expect, it, vi } from 'vitest';
import { azureClusterApi, validAzureClusterCommission, type AzureClusterCommission } from './azureCluster';
import { productApi } from './product';

export const aksCommissionFixture=():AzureClusterCommission=>({id:'aks-'+'a'.repeat(32),projectId:'p-one',revision:1,actor:{userId:'alice',projectId:'p-one'},jobId:'cj-original',planDigest:'b'.repeat(64),cluster:{resourceId:'/subscriptions/'+'a'.repeat(8)+'-aaaa-aaaa-aaaa-'+'a'.repeat(12)+'/resourceGroups/owned/providers/Microsoft.ContainerService/managedClusters/owned-aks',resourceUid:'original-cluster-generation',principalId:'dddddddd-dddd-dddd-dddd-dddddddddddd',nodeResourceGroupId:'/subscriptions/exact/resourceGroups/owned-nodes',fqdn:'owned.example',kubernetesVersion:'1.34.1',state:'created',observedAt:'2026-10-31T00:00:00Z',targetReady:false,capacityVerified:false},namespace:'project-apps',mode:'create',expectedNamespaceUid:'',expectedNamespaceVersion:'',principal:{subscriptionId:'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',tenantId:'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',objectId:'cccccccc-cccc-cccc-cccc-cccccccccccc',kind:'User',accountName:'operator@example.com'},roleDefinitionId:'/subscriptions/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/providers/Microsoft.Authorization/roleDefinitions/b1ff04bb-8a4e-4dc4-8eb5-8693973ce19b',roleAssignmentId:'/subscriptions/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/resourceGroups/owned/providers/Microsoft.ContainerService/managedClusters/owned-aks/providers/Microsoft.Authorization/roleAssignments/original',roleAttempted:false,roleObserved:false,roleExistingAtReview:false,namespaceAttempted:false,namespaceUid:'',targetId:'cluster-'+'c'.repeat(32),monthlyBudgetUsd:100,state:'reviewed',createdAt:'2026-10-31T00:00:00Z',expiresAt:'2026-10-31T00:10:00Z',observedAt:'2026-10-31T00:00:00Z'});
afterEach(()=>vi.restoreAllMocks());
it('keeps cluster creation distinct from connected namespace readiness and rejects foreign actor scope',()=>{
 const row=aksCommissionFixture();expect(validAzureClusterCommission(row,'p-one')).toBe(true);
 expect(validAzureClusterCommission({...row,namespace:'123apps--blue'},'p-one')).toBe(true);
 expect(validAzureClusterCommission({...row,projectId:'foreign'},'p-one')).toBe(false);
 expect(validAzureClusterCommission({...row,principal:{...row.principal,kind:['User']}},'p-one')).toBe(false);
 expect(validAzureClusterCommission({...row,state:'connected'},'p-one')).toBe(false);
 expect(validAzureClusterCommission({...row,cluster:{...row.cluster,targetReady:true}},'p-one')).toBe(false);
});
it('uses the original reviewed action and refuses a response for a replacement cluster',async()=>{
 const row=aksCommissionFixture(),call=vi.spyOn(productApi,'request').mockResolvedValue({...row,revision:2,roleAttempted:true,state:'access-unknown'});
 await azureClusterApi.action('p-one',row,'access',true,true);
 expect(call).toHaveBeenCalledWith(`/v1/product/projects/p-one/azure-clusters/commissions/${row.id}/access`,{acknowledgeCost:true,acknowledgeAccessGrant:true},'POST',120000,undefined);
 call.mockResolvedValue({...row,cluster:{...row.cluster,resourceUid:'replacement-generation'}});
 await expect(azureClusterApi.action('p-one',row,'observe',false,false)).rejects.toThrow();
});
it('refuses unbounded or foreign retained records without starting any creation request',async()=>{
 const row=aksCommissionFixture(),call=vi.spyOn(productApi,'request').mockResolvedValue({jobs:[],commissions:[{...row,actor:{userId:'bob',projectId:'foreign'}}]});
 await expect(azureClusterApi.list('p-one')).rejects.toThrow();
 expect(call).toHaveBeenCalledTimes(1);expect(call.mock.calls[0][2]).toBe('GET');
});
