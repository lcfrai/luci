import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { emptyGTM, gtmWorkspaceApi, validGTMArchiveChoices, type GTMArchiveChoice, type GTMWorkspace } from './gtmWorkspace';
const source={briefRevision:1,briefDigest:'a'.repeat(64),specificationRevision:2,specificationDigest:'b'.repeat(64),designRevision:1,designDigest:'c'.repeat(64),planRevision:3,planDigest:'d'.repeat(64),checks:'not-selected',release:'not-selected'};
const choice=():GTMArchiveChoice=>({id:'e'.repeat(64),firstRevision:1,lastRevision:2,revisions:2,bytes:4000});
const row=(revision:number):GTMWorkspace=>({projectId:'p-one',revision,source,sourceDigest:'a'.repeat(64),content:{...emptyGTM(),offer:`Offer ${revision}`},digest:'b'.repeat(64),actor:{id:'owner',displayName:'Owner'},at:'2026-10-31T00:00:00Z'});
afterEach(()=>vi.restoreAllMocks());
it('validates contiguous global archive ranges and explicit bounded storage metadata',()=>{
 expect(validGTMArchiveChoices(undefined,undefined)).toBe(true);expect(validGTMArchiveChoices([choice()],2)).toBe(true);
 expect(validGTMArchiveChoices([{...choice(),firstRevision:2}],2)).toBe(false);expect(validGTMArchiveChoices([choice(),choice()],4)).toBe(false);
 expect(validGTMArchiveChoices([{...choice(),bytes:8*1024*1024+1}],2)).toBe(false);expect(validGTMArchiveChoices([choice()],50)).toBe(false);expect(validGTMArchiveChoices([{...choice(),storageBytes:choice().bytes*2}],2)).toBe(true);expect(validGTMArchiveChoices([{...choice(),storageBytes:choice().bytes-1}],2)).toBe(false);
});
it('reads only the selected archived chunk and refuses foreign or reordered historical rows',async()=>{
 const response={projectId:'p-one',archiveId:choice().id,revisions:[row(1),row(2)],historical:true};const send=vi.spyOn(productApi,'request').mockResolvedValue(response);const signal=new AbortController().signal;
 expect((await gtmWorkspaceApi.archive('p-one',choice(),signal)).revisions[0].content.offer).toBe('Offer 1');expect(send).toHaveBeenCalledExactlyOnceWith(`/v1/product/projects/p-one/distribute/gtm?history=${choice().id}`,undefined,'GET',30_000,signal);
 send.mockResolvedValue({...response,revisions:[row(2),row(1)]});await expect(gtmWorkspaceApi.archive('p-one',choice())).rejects.toThrow('changed');
 send.mockResolvedValue({...response,projectId:'p-other'});await expect(gtmWorkspaceApi.archive('p-one',choice())).rejects.toThrow('could not be verified');
});
it('binds exact active or archived revision reads to the requested original digest',async()=>{
 const send=vi.spyOn(productApi,'request').mockResolvedValue({projectId:'p-one',revisions:[row(51)],historical:true});const signal=new AbortController().signal;
 expect((await gtmWorkspaceApi.revision('p-one',51,'b'.repeat(64),signal)).revisions[0].revision).toBe(51);expect(send).toHaveBeenCalledWith('/v1/product/projects/p-one/distribute/gtm?revision=51',undefined,'GET',30_000,signal);
 await expect(gtmWorkspaceApi.revision('p-one',51,'c'.repeat(64))).rejects.toThrow('changed');send.mockResolvedValue({projectId:'p-one',revisions:[row(51)],historical:['true']});await expect(gtmWorkspaceApi.revision('p-one',51,'b'.repeat(64))).rejects.toThrow('could not be verified');
});
