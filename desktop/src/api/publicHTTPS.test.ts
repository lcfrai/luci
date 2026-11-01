import { afterEach, expect, it, vi } from 'vitest';
import { productApi, ProductApiError } from './product';
import { validHTTPSStatus, publicHTTPSApi, httpsActionSettled, definitiveHTTPSReviewRefusal, type HTTPSReviewInput } from './publicHTTPS';
import { validBlueprintPublicRoute } from './deploymentBlueprint';
import { httpsFixture } from '../test/fixtures/publicHTTPS';

afterEach(()=>vi.restoreAllMocks());
it('does not turn a TLS endpoint or another action receipt into application health or original settlement',()=>{
 const row=httpsFixture();expect(validHTTPSStatus(row,'p-one')).toBe(true);
 row.revision=3;row.actions=[{requestKey:'other_'+'a'.repeat(32),requestDigest:'b'.repeat(64),action:'apply',state:'settled',startedRevision:1,settledRevision:2}];
 expect(httpsActionSettled(row,'original_'+'c'.repeat(32),'apply',1)).toBe(false);
 expect(validHTTPSStatus({...row,url:'https://app.example.com/'},'p-one')).toBe(false);
 expect(validHTTPSStatus({...row,dnsCredential:{clientSecret:'private'}},'p-one')).toBe(false);
 expect(validHTTPSStatus({...row,projectId:'other'},'p-one')).toBe(false);
});
it('retries only the exact original revision/key/body and requires its positive receipt',async()=>{
 const row=httpsFixture(),key='original_'+'d'.repeat(32);row.revision=8;
 row.actions=[{requestKey:key,requestDigest:'e'.repeat(64),action:'pause',state:'settled',startedRevision:2,settledRevision:7}];
 const request=vi.spyOn(productApi,'request').mockResolvedValue(row);
 await expect(publicHTTPSApi.originalAction('p-one',row.id,row.plan.digest,row.reviewKey,2,'pause',key)).resolves.toEqual(row);
 expect(request).toHaveBeenCalledWith(expect.stringContaining(`/${row.id}/action`),{expectedRevision:2,requestKey:key,action:'pause'},'POST',90000,undefined);
 await expect(publicHTTPSApi.originalAction('p-one',row.id,row.plan.digest,row.reviewKey,3,'pause',key)).rejects.toThrow('not yet confirmed');
});
it('looks up a lost review without sending a credential or adopting another original',async()=>{
 const row=httpsFixture(),request=vi.spyOn(productApi,'request').mockResolvedValue({projectId:'p-one',commissions:[row],retainedRecentOnly:false});
 await expect(publicHTTPSApi.list('p-one',undefined,row.reviewKey)).resolves.toEqual([row]);
 expect(request).toHaveBeenCalledWith(expect.stringContaining(`?reviewKey=${row.reviewKey}`),undefined,'GET',30000,undefined);
 request.mockResolvedValue({projectId:'p-one',commissions:[{...row,reviewKey:'foreign_'+'a'.repeat(32)}],retainedRecentOnly:false});
 await expect(publicHTTPSApi.list('p-one',undefined,row.reviewKey)).rejects.toThrow('could not be confirmed');
});
it('preserves the legacy manual route shape and refuses mixed managed/manual ownership',()=>{
 const legacy={profile:'traefik-websecure',serviceId:'app',ingressClassName:'traefik',hostname:'app.example.com',path:'/',pathType:'Prefix',tlsSecretName:'public-tls'};
 expect(validBlueprintPublicRoute(legacy)).toBe(true);
 const managed={...legacy,profile:'luci-managed-https',managedHTTPSId:httpsFixture().id,ingressClassName:'',tlsSecretName:''};
 expect(validBlueprintPublicRoute(managed)).toBe(true);
 expect(validBlueprintPublicRoute({...managed,tlsSecretName:'another-key'})).toBe(false);
 expect(validBlueprintPublicRoute({...legacy,managedHTTPSId:httpsFixture().id})).toBe(false);
 expect(JSON.stringify(legacy)).not.toContain('managedHTTPSId');
});
it('accepts canonical ARM casing for the same zone while keeping review-only refusals narrow',async()=>{
 const row=httpsFixture(),p=row.plan;
 const input:HTTPSReviewInput={reviewKey:row.reviewKey,deploymentId:p.deploymentId,serviceId:p.destination.serviceId,hostname:p.hostname,path:p.path,pathType:p.pathType,azureDnsZoneId:p.azureDnsZoneId,traefikImage:p.traefikImage,storageClass:p.storageClass,acmeEmail:p.acmeEmail,dnsCredential:{tenantId:'tenant',clientId:p.dnsClientId,clientSecret:'window-only'},monthlyBudgetUsd:p.monthlyBudgetUsd,acknowledgeAcmeTerms:true,acknowledgePrivateDnsCredential:true,acknowledgePublicCostsAndRenewal:true};
 row.plan.azureDnsZoneId=row.plan.azureDnsZoneId.replace('/resourcegroups/','/resourceGroups/').replace('/microsoft.network/dnszones/','/Microsoft.Network/dnsZones/');
 vi.spyOn(productApi,'request').mockResolvedValue(row);await expect(publicHTTPSApi.review('p-one',input)).resolves.toEqual(row);
 const refusal=new ProductApiError('public_https_input' as ConstructorParameters<typeof ProductApiError>[0],'Correct the reviewed settings.',422,{});
 expect(definitiveHTTPSReviewRefusal(refusal)).toBe(true);
 expect(definitiveHTTPSReviewRefusal(new ProductApiError(refusal.code,refusal.message,409,{}))).toBe(false);
 expect(definitiveHTTPSReviewRefusal(new Error('Response lost'))).toBe(false);
});
it('reads the entire bounded index including an old paused endpoint without silently truncating it',async()=>{
 const rows=Array.from({length:128},(_,index)=>{const row=httpsFixture(),id='https-'+index.toString(16).padStart(32,'0');return {...row,id,reviewKey:'review_'+index.toString(16).padStart(32,'0'),state:index===0?'paused' as const:'reviewed' as const,plan:{...row.plan,id,hostname:`app${index}.example.com`}};});
 const request=vi.spyOn(productApi,'request').mockResolvedValue({projectId:'p-one',commissions:rows,retainedRecentOnly:true});
 await expect(publicHTTPSApi.list('p-one')).resolves.toEqual(rows);
 request.mockResolvedValue({projectId:'p-one',commissions:[...rows,{...httpsFixture(),id:'https-'+'f'.repeat(32),plan:{...httpsFixture().plan,id:'https-'+'f'.repeat(32)}}],retainedRecentOnly:true});
 await expect(publicHTTPSApi.list('p-one')).rejects.toThrow('could not be confirmed');
});
