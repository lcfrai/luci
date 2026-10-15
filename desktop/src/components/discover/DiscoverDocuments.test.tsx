import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ProjectDiscoverDocuments } from './DiscoverDocuments';
import { discoveryDocumentsApi, type DiscoveryDocument } from '../../api/discoveryDocuments';
import { retainDiscoveryRecovery } from '../../api/discoveryDocumentRecovery';
import { resetCockpit } from '../../store/store';
vi.mock('../../api/discoveryDocuments',async importOriginal=>{const original=await importOriginal<typeof import('../../api/discoveryDocuments')>();return{...original,discoveryDocumentsApi:{list:vi.fn(),get:vi.fn(),upload:vi.fn(),original:vi.fn(),start:vi.fn(),cancel:vi.fn(),correct:vi.fn(),discussion:vi.fn(),disposition:vi.fn()}};});
const id=`doc-${'a'.repeat(64)}`;
const document:DiscoveryDocument={id,projectId:'p-one',revision:3,original:{name:'brief.txt',bytes:12,digest:'b'.repeat(64),mimeType:'text/plain'},createdAt:'2026-10-16T00:00:00Z',createdBy:{id:'person-one',displayName:'One'},attempts:[],extractions:[{revision:1,attemptId:'extract-123',originalDigest:'b'.repeat(64),extractor:'utf8',extractorVersion:'luci-text-v1',status:'ready',digest:'c'.repeat(64),at:'2026-10-16T00:00:00Z',actor:{id:'person-one',displayName:'One'},issues:[],pages:[{page:1,width:0,height:0,segments:[{id:'p1-line1',kind:'text',text:'People need to resume saved work.',textDigest:'d'.repeat(64),location:{x:0,y:0,width:0,height:0,firstLine:1,lastLine:1},confidence:null,uncertainty:''}]}]}]};
beforeEach(()=>{localStorage.clear();resetCockpit();vi.clearAllMocks();vi.mocked(discoveryDocumentsApi.list).mockResolvedValue({documents:[document],nextCursor:'',omitted:false});vi.mocked(discoveryDocumentsApi.get).mockResolvedValue(document);});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
it('mounts read-only evidence without uploading, extracting, correcting or sending a model request',async()=>{
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit={false}/>);
  await screen.findByRole('button',{name:/brief.txt/});expect(screen.getByLabelText('Attach a private document')).toHaveProperty('disabled',true);
  expect(discoveryDocumentsApi.upload).not.toHaveBeenCalled();expect(discoveryDocumentsApi.start).not.toHaveBeenCalled();expect(discoveryDocumentsApi.discussion).not.toHaveBeenCalled();
});
it('inspects exact page evidence and hands only an unsent normal Luci message to the existing conversation',async()=>{
  const opened=vi.fn();window.addEventListener('luci:orchestrator-open',opened);vi.mocked(discoveryDocumentsApi.discussion).mockResolvedValue({projectId:'p-one',message:'UNTRUSTED DOCUMENT EVIDENCE: exact selected source',links:[]});
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));
  fireEvent.click(await screen.findByRole('checkbox',{name:/Page 1.*line 1/}));fireEvent.click(screen.getByRole('button',{name:'Discuss selected evidence with Luci'}));
  await waitFor(()=>expect(opened).toHaveBeenCalledOnce());expect(vi.mocked(discoveryDocumentsApi.discussion)).toHaveBeenCalledTimes(1);
  expect(vi.mocked(discoveryDocumentsApi.discussion).mock.calls[0][1][0]).toMatchObject({documentId:id,originalDigest:'b'.repeat(64),extractionRevision:1,page:1,segmentId:'p1-line1',textDigest:'d'.repeat(64)});
  expect((opened.mock.calls[0][0] as CustomEvent).detail).toMatchObject({projectId:'p-one',scope:'scope-one',message:'UNTRUSTED DOCUMENT EVIDENCE: exact selected source'});window.removeEventListener('luci:orchestrator-open',opened);
});
it('keeps a retained uncertain request on mount and inspects disposition without repeating extraction',async()=>{
  retainDiscoveryRecovery('scope-one',{action:'extract',idempotencyKey:'document-extract-1234567',documentId:id,expectedRevision:1});vi.mocked(discoveryDocumentsApi.disposition).mockResolvedValue({state:'recorded',document,attemptId:'extract-123'});
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  fireEvent.click(await screen.findByRole('button',{name:'Inspect exact saved request'}));await waitFor(()=>expect(screen.queryByRole('button',{name:'Inspect exact saved request'})).toBeNull());expect(discoveryDocumentsApi.start).not.toHaveBeenCalled();
});
it('keeps uncontained PDF/OCR disabled and clears source text when current read authority is lost',async()=>{
  const pdf={...document,original:{...document.original,name:'scan.pdf',mimeType:'application/pdf'}};vi.mocked(discoveryDocumentsApi.get).mockResolvedValue(pdf);
  const page=render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));await screen.findByRole('heading',{name:'scan.pdf'});
  expect(screen.getByRole('button',{name:'Extract text / start fresh attempt'})).toHaveProperty('disabled',true);
  act(()=>page.rerender(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead={false} canEdit={false}/>));expect(screen.queryByText('People need to resume saved work.')).toBeNull();expect(discoveryDocumentsApi.start).not.toHaveBeenCalled();
});
it('retries an uncertain extraction only after explicit user action with its exact retained key and CAS',async()=>{
  const pending={action:'extract' as const,idempotencyKey:'document-extract-1234567',documentId:id,expectedRevision:1};
  retainDiscoveryRecovery('scope-one',pending);vi.mocked(discoveryDocumentsApi.start).mockResolvedValue({document,attempt:{id:'extract-123',status:'ready',startedAt:'2026-10-16T00:00:00Z',expiresAt:'2026-10-16T00:02:00Z',actor:document.createdBy}});
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  const retry=await screen.findByRole('button',{name:'Retry exact retained request'});expect(discoveryDocumentsApi.start).not.toHaveBeenCalled();fireEvent.click(retry);
  await waitFor(()=>expect(discoveryDocumentsApi.start).toHaveBeenCalledWith('p-one',id,1,pending.idempotencyKey,expect.any(AbortSignal)));
  await waitFor(()=>expect(screen.queryByRole('button',{name:'Retry exact retained request'})).toBeNull());
});
