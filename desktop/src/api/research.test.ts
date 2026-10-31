import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { researchApi, validResearchEvidence } from './research';

afterEach(()=>vi.restoreAllMocks());
it('keeps query and explicit external-read consent separate from a model request',async()=>{
  const request=vi.spyOn(productApi,'request').mockResolvedValue({search:{provider:'brave',query:'repair queue',fetchedAt:'2026-10-31T00:00:00Z',results:[]},captures:[],unavailable:[],omitted:0});
  await researchApi.investigate('project','repair queue');
  expect(request).toHaveBeenCalledWith('/v1/product/projects/project/research/investigate',{query:'repair queue',acknowledgeSearchCost:true,acknowledgePublicReads:true},'POST',50_000,undefined);
  expect(request).toHaveBeenCalledTimes(1);
});
it('rejects forged provenance shapes and keeps explicit partial capture metadata',()=>{
  const evidence={captureId:'research-original',requestedUrl:'https://public.example/page',url:'https://public.example/page',title:'Source',fetchedAt:'2026-10-31T00:00:00Z',bodySha256:'a'.repeat(64),textSha256:'b'.repeat(64),quotes:['Exact quote'],omittedTextBytes:12000};
  expect(validResearchEvidence(evidence)).toBe(true);
  expect(validResearchEvidence({...evidence,omittedTextBytes:-1})).toBe(false);
  expect(validResearchEvidence({...evidence,quotes:['Exact quote','Exact quote']})).toBe(false);
  expect(validResearchEvidence({...evidence,textSha256:['b'.repeat(64)]})).toBe(false);
});
it('requires actual closed string source kinds in a saved observation response',async()=>{
  vi.spyOn(productApi,'request').mockResolvedValue({routeRevision:1,autoDiscuss:true,route:null,sources:[{kind:['research'],digest:'a'.repeat(64),revision:1,summary:'Captured research'}],unavailable:[],telemetryOptions:[]});
  await expect(researchApi.observations('project')).rejects.toThrow('could not be verified');
});
