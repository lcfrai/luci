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

const refreshedDocument:DiscoveryDocument={...document,revision:4,extractions:[...document.extractions,{...document.extractions[0],revision:2,digest:'e'.repeat(64),pages:[{...document.extractions[0].pages[0],segments:[{...document.extractions[0].pages[0].segments[0],kind:'correction',text:'A reviewed correction is now saved.',textDigest:'f'.repeat(64)}]}]}]};

it('refreshes the open document from its exact saved readback even outside the displayed source page without repeating extraction',async()=>{
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));
  fireEvent.click(await screen.findByRole('checkbox',{name:/Page 1.*line 1/}));
  vi.mocked(discoveryDocumentsApi.list).mockResolvedValue({documents:[],nextCursor:'',omitted:false});
  vi.mocked(discoveryDocumentsApi.get).mockResolvedValue(refreshedDocument);
  fireEvent.click(screen.getByRole('button',{name:'Refresh documents'}));
  await screen.findByText('A reviewed correction is now saved.');
  expect(discoveryDocumentsApi.get).toHaveBeenLastCalledWith('p-one',id,expect.any(AbortSignal));
  expect(screen.getByText('0 exact source segments selected')).toBeTruthy();
  expect(discoveryDocumentsApi.start).not.toHaveBeenCalled();
  expect(discoveryDocumentsApi.correct).not.toHaveBeenCalled();
  expect(discoveryDocumentsApi.upload).not.toHaveBeenCalled();
});

it('preserves exact selected evidence when refresh confirms the same saved document revision',async()=>{
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));
  const segment=await screen.findByRole('checkbox',{name:/Page 1.*line 1/});fireEvent.click(segment);
  fireEvent.click(screen.getByRole('button',{name:'Refresh documents'}));
  await screen.findByText(/Saved documents and the open source are refreshed/);
  expect(screen.getByRole('checkbox',{name:/Page 1.*line 1/})).toHaveProperty('checked',true);
  expect(screen.getByText('1 exact source segments selected')).toBeTruthy();
});

it('keeps an unsaved correction intact until discarded before refreshing its saved source',async()=>{
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));
  fireEvent.click(await screen.findByRole('button',{name:'Correct this extraction'}));
  fireEvent.change(screen.getByLabelText('Corrected text'),{target:{value:'Unsaved human correction'}});
  await waitFor(()=>expect(screen.getByRole('button',{name:'Refresh documents'})).toHaveProperty('disabled',true));
  fireEvent.click(screen.getByRole('button',{name:'Refresh documents'}));
  expect(discoveryDocumentsApi.get).toHaveBeenCalledTimes(1);
  expect(screen.getByLabelText('Corrected text')).toHaveProperty('value','Unsaved human correction');
  fireEvent.click(screen.getByRole('button',{name:'Discard correction draft'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Refresh documents'})).toHaveProperty('disabled',false));
});

it('preserves the open evidence and page when selected document refresh fails',async()=>{
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));await screen.findByText('People need to resume saved work.');
  vi.mocked(discoveryDocumentsApi.list).mockResolvedValue({documents:[],nextCursor:'',omitted:false});
  vi.mocked(discoveryDocumentsApi.get).mockRejectedValue(new Error('Saved source is temporarily unavailable.'));
  fireEvent.click(screen.getByRole('button',{name:'Refresh documents'}));
  await screen.findByRole('alert');
  expect(screen.getByText('People need to resume saved work.')).toBeTruthy();
  expect(screen.getByRole('button',{name:/brief.txt/})).toBeTruthy();
});

it('rejects a different document returned by selected source refresh',async()=>{
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));await screen.findByText('People need to resume saved work.');
  vi.mocked(discoveryDocumentsApi.get).mockResolvedValue({...refreshedDocument,id:`doc-${'c'.repeat(64)}`});
  fireEvent.click(screen.getByRole('button',{name:'Refresh documents'}));
  await screen.findByText(/selected document readback could not be verified/);
  expect(screen.queryByText('A reviewed correction is now saved.')).toBeNull();
  expect(screen.getByText('People need to resume saved work.')).toBeTruthy();
});

it('does not restore a pending selected readback after read permission is revoked and restored',async()=>{
  const page=render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  fireEvent.click(await screen.findByRole('button',{name:/brief.txt/}));await screen.findByText('People need to resume saved work.');
  let finish!:(value:DiscoveryDocument)=>void;
  vi.mocked(discoveryDocumentsApi.get).mockImplementation(()=>new Promise(resolve=>{finish=resolve}));
  fireEvent.click(screen.getByRole('button',{name:'Refresh documents'}));
  await waitFor(()=>expect(discoveryDocumentsApi.get).toHaveBeenCalledTimes(2));
  page.rerender(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead={false} canEdit={false}/>);
  page.rerender(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  await act(async()=>finish(refreshedDocument));
  expect(screen.queryByText('A reviewed correction is now saved.')).toBeNull();
  expect(screen.queryByRole('heading',{name:'brief.txt'})).toBeNull();
  expect(discoveryDocumentsApi.start).not.toHaveBeenCalled();
});

it('labels an incomplete retained source list and clears that warning only after complete readback',async()=>{
  vi.mocked(discoveryDocumentsApi.list).mockResolvedValue({documents:[document],nextCursor:'',omitted:true});
  render(<ProjectDiscoverDocuments projectId="p-one" scope="scope-one" canRead canEdit/>);
  await screen.findByText(/Some retained source records could not be read/);
  vi.mocked(discoveryDocumentsApi.list).mockResolvedValue({documents:[document],nextCursor:'',omitted:false});
  fireEvent.click(screen.getByRole('button',{name:'Refresh documents'}));
  await waitFor(()=>expect(screen.queryByText(/Some retained source records could not be read/)).toBeNull());
  expect(discoveryDocumentsApi.get).not.toHaveBeenCalled();
});
