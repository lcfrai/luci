import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { kubernetesDeploymentApi, validKubernetesReview, validKubernetesStatus, validKubernetesTarget } from './kubernetesDeployment';

import { clusterReviewFixture, clusterTargetFixture } from '../test/kubernetesDeploymentFixtures';

afterEach(()=>vi.restoreAllMocks());

it('rejects foreign targets, leaked credentials and malformed resource or health claims',()=>{
 const target=clusterTargetFixture(),review=clusterReviewFixture();
 expect(validKubernetesTarget(target,'p-one')).toBe(true);
 expect(validKubernetesTarget({...target,credentials:{token:'private'}},'p-one')).toBe(false);
 expect(validKubernetesTarget({...target,projectId:'foreign'},'p-one')).toBe(false);
 expect(validKubernetesReview(review,'p-one')).toBe(true);
 expect(validKubernetesReview({...review,request:{...review.request,expected:[review.request.expected[0],review.request.expected[0]]}},'p-one')).toBe(false);
 const status={review:{...review,applied:true},operation:null,state:'unknown',applicationHealthVerified:false,imageProvenance:'user-supplied immutable digest; checked-source-to-image provenance not verified'};
 expect(validKubernetesStatus(status,'p-one')).toBe(true);
 expect(validKubernetesStatus({...status,applicationHealthVerified:true},'p-one')).toBe(false);
});

it('submits only the selected review identity and explicit acknowledgements, never a client plan or credentials',async()=>{
 const review=clusterReviewFixture(),value={review:{...review,applied:true},operation:null,state:'unknown',applicationHealthVerified:false,imageProvenance:'user-supplied immutable digest; checked-source-to-image provenance not verified'};
 const request=vi.spyOn(productApi,'request').mockResolvedValue(value);
 const controller=new AbortController();await kubernetesDeploymentApi.action('p-one',review,'apply',true,controller.signal);
 expect(request).toHaveBeenLastCalledWith(`/v1/product/projects/p-one/kubernetes/deployments/${review.id}/apply`,{acknowledgeCost:true,acknowledgeImageProvenance:true},'POST',150_000,controller.signal);
 request.mockResolvedValue({...value,review:{...value.review,id:`workload-${'c'.repeat(32)}`}});
 await expect(kubernetesDeploymentApi.action('p-one',review,'observe',false)).rejects.toThrow(/could not be confirmed/);
});
