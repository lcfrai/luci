import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { validStoredBlueprint } from './deploymentBlueprint';
import { kubernetesDeploymentApi, validKubernetesReview, type KubernetesReferenceView } from './kubernetesDeployment';
import { clusterReviewFixture, clusterTargetFixture } from '../test/kubernetesDeploymentFixtures';

afterEach(()=>vi.restoreAllMocks());
const metadata=():KubernetesReferenceView=>{const target=clusterTargetFixture();return {projectId:'p-one',targetId:target.id,targetFingerprint:target.fingerprint,namespaceUid:target.namespaceUid,namespace:target.namespace,items:[{kind:'secret',name:'model-access',namespace:target.namespace,uid:'secret-one',resourceVersion:'5',keys:['token'],keysRead:true,partial:false}],partial:false};};

it('sends only explicit resource identity and refuses leaked values or wrong namespace metadata',async()=>{
 const target=clusterTargetFixture(),response=metadata(),send=vi.spyOn(productApi,'request').mockResolvedValue(response),signal=new AbortController().signal;
 expect(await kubernetesDeploymentApi.references('p-one',target.id,'secret','model-access',signal)).toEqual(response);
 expect(send).toHaveBeenLastCalledWith(`/v1/product/projects/p-one/kubernetes/targets/${target.id}/references/read`,{kind:'secret',name:'model-access'},'POST',20_000,signal);
 send.mockResolvedValue({...response,items:[{...response.items[0],data:{token:'PRIVATE-VALUE'}}]});
 await expect(kubernetesDeploymentApi.references('p-one',target.id,'secret','model-access')).rejects.toThrow(/could not be confirmed/);
 send.mockResolvedValue({...response,items:[{...response.items[0],namespace:'foreign'}]});
 await expect(kubernetesDeploymentApi.references('p-one',target.id,'secret','model-access')).rejects.toThrow(/could not be confirmed/);
});

it('accepts typed probe and key references while refusing executable or value-bearing probe shapes',()=>{
 const b=clusterReviewFixture().request.blueprint;
 b.services[0].environment=[{name:'TOKEN',kind:'secret',resourceName:'model-access',key:'token'}];
 b.services[0].health={readiness:{kind:'http',path:'/ready',initialDelaySeconds:0,periodSeconds:5,timeoutSeconds:1,failureThreshold:3}};
 b.services[0].deploymentStrategy='recreate';expect(validStoredBlueprint(b,'p-one')).toBe(true);
 const malformed=JSON.parse(JSON.stringify(b));malformed.services[0].health.readiness.headers={Authorization:'private'};expect(validStoredBlueprint(malformed,'p-one')).toBe(false);
 delete malformed.services[0].health.readiness.headers;malformed.services[0].health.readiness.kind=['http'];expect(validStoredBlueprint(malformed,'p-one')).toBe(false);
 const values=JSON.parse(JSON.stringify(b));values.services[0].environment[0].value='private';expect(validStoredBlueprint(values,'p-one')).toBe(false);
});

it('requires reviewed reference scope to match the exact namespace and target revision fingerprint',()=>{
 const review=clusterReviewFixture(),target=review.target;
 review.request.references=[{kind:'secret',name:'model-access',namespace:target.namespace,namespaceUid:target.namespaceUid,targetFingerprint:target.fingerprint,uid:'secret-one',resourceVersion:'5',keys:['token'],metadataDigest:'a'.repeat(64)}];
 expect(validKubernetesReview(review,'p-one')).toBe(true);
 review.request.references[0].namespaceUid='replacement-namespace';expect(validKubernetesReview(review,'p-one')).toBe(false);
 review.request.references[0].namespaceUid=target.namespaceUid;review.request.references[0].targetFingerprint='b'.repeat(64);expect(validKubernetesReview(review,'p-one')).toBe(false);
});
