import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { distributeArtifactsApi, type DistributeArtifact } from '../../api/distributeArtifacts';
import { projectRunsApi } from '../../api/projectRuns';
import { resetCockpit } from '../../store/store';
import { ArtifactWorkspace } from './ArtifactWorkspace';

const digest='a'.repeat(64);
const draft:DistributeArtifact={id:'artifact-123',projectId:'p-one',revision:1,kind:'risk',source:{briefRevision:1,briefDigest:digest,specificationRevision:1,specificationDigest:digest,designRevision:1,designDigest:digest,planRevision:1,planDigest:digest,checks:'not-selected',release:'not-selected'},sourceDigest:digest,title:'Risk and evidence review',sections:[{id:'purpose',heading:'Saved product purpose',body:'Saved Orchard tasklist brief',bullets:['Audience: local users'],sourceRefs:[`brief:1:${digest}`]}],gaps:['No exact check receipt was selected.'],assumptions:['Likelihood requires review.'],state:'draft',digest,at:'2026-10-15T00:00:00Z'};
beforeEach(()=>{vi.restoreAllMocks();resetCockpit();vi.spyOn(distributeArtifactsApi,'list').mockResolvedValue({projectId:'p-one',artifacts:[]})});
afterEach(cleanup);

it('keeps preparation private, saves exact source, then records a review without publication',async()=>{
  const prepare=vi.spyOn(distributeArtifactsApi,'prepare').mockResolvedValue({draft,prepareDigest:digest});
  const save=vi.spyOn(distributeArtifactsApi,'save').mockResolvedValue({artifact:draft,freshness:'current'});
  const review=vi.spyOn(distributeArtifactsApi,'review').mockResolvedValue({artifact:{...draft,revision:2,state:'reviewed',review:{actor:{id:'u-one',displayName:'Reviewer'},at:'now',digest}},freshness:'current'});
  const download=vi.spyOn(distributeArtifactsApi,'download').mockResolvedValue();
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.selectOptions(screen.getByLabelText('Artifact kind'),'risk');
  await userEvent.click(screen.getByRole('button',{name:'Prepare source draft'}));
  expect(await screen.findByText('Gap: No exact check receipt was selected.')).toBeTruthy();
  expect(prepare).toHaveBeenCalledExactlyOnceWith('p-one','risk','');
  expect(save).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Save private draft'}));
  await screen.findByText(/Private draft saved/);
  expect(save).toHaveBeenCalledWith('p-one','risk','',digest,expect.stringMatching(/^distribute-artifact-/));
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await screen.findByText(/Review recorded/);
  expect(review).toHaveBeenCalledWith('p-one',draft,expect.stringMatching(/^distribute-artifact-/));
  await userEvent.click(screen.getByRole('button',{name:'Download DOCX'}));
  expect(download).toHaveBeenCalledWith('p-one',expect.objectContaining({revision:2,state:'reviewed'}),'docx',expect.any(Function));
  await userEvent.click(screen.getByRole('button',{name:'Download PPTX'}));
  expect(download).toHaveBeenCalledWith('p-one',expect.objectContaining({revision:2,state:'reviewed'}),'pptx',expect.any(Function));
  expect(screen.queryByRole('button',{name:/Publish/})).toBeNull();
});

it('preserves unsaved edits and blocks review when the saved source is outdated',async()=>{
  vi.spyOn(distributeArtifactsApi,'get').mockResolvedValue({artifact:draft,history:[draft],freshness:'outdated'});
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[{id:draft.id,kind:'risk',title:draft.title,revision:1,state:'draft',freshness:'outdated',digest}]});
  const review=vi.spyOn(distributeArtifactsApi,'review');
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  expect(await screen.findByText(/Source changed; prepare a fresh draft/)).toBeTruthy();
  expect(screen.getByRole('button',{name:'Mark reviewed'}).hasAttribute('disabled')).toBe(true);
  expect(review).not.toHaveBeenCalled();
  await waitFor(()=>expect(screen.getByLabelText('Title')).toHaveProperty('value',draft.title));
});

it('selects only a read-back accepted run and keeps its exact receipt in preparation',async()=>{
  vi.spyOn(projectRunsApi,'list').mockResolvedValue([{id:'run-accepted',state:'complete',acceptance:{checkId:'check-one',releaseId:'release-one',acceptedAt:'2026-10-15T00:00:00Z'}},{id:'run-waiting',state:'awaiting-release'}] as Awaited<ReturnType<typeof projectRunsApi.list>>);
  const prepare=vi.spyOn(distributeArtifactsApi,'prepare').mockResolvedValue({draft:{...draft,source:{...draft.source,selectedEvidence:{runId:'run-accepted',runRevision:4,checkId:'check-one',releaseId:'release-one',commit:'b'.repeat(40),sourceDigest:digest,checkDigest:digest,releaseDigest:digest,acceptedAt:'2026-10-15T00:00:00Z'}}},prepareDigest:digest});
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(screen.getByRole('button',{name:'Load accepted run evidence'}));
  expect(await screen.findByRole('option',{name:/run-accepted/})).toBeTruthy();
  expect(screen.queryByRole('option',{name:/run-waiting/})).toBeNull();
  await userEvent.selectOptions(screen.getByLabelText('Exact check and release'),'run-accepted');
  await userEvent.click(screen.getByRole('button',{name:'Prepare source draft'}));
  await screen.findByText(/accepted run run-accepted/);
  expect(prepare).toHaveBeenCalledWith('p-one','proposal','run-accepted');
});

it('uses distinct review keys for two saved artifacts',async()=>{
  const other={...draft,id:'artifact-other',title:'Other private review',digest:'b'.repeat(64)};
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[draft,other].map(row=>({id:row.id,kind:row.kind,title:row.title,revision:row.revision,state:row.state,freshness:'current',digest:row.digest}))});
  vi.spyOn(distributeArtifactsApi,'get').mockImplementation(async(_project,id)=>({artifact:id===draft.id?draft:other,freshness:'current'}));
  const review=vi.spyOn(distributeArtifactsApi,'review').mockImplementation(async(_project,row)=>({artifact:{...row,revision:row.revision+1,state:'reviewed'},freshness:'current'}));
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await waitFor(()=>expect(review).toHaveBeenCalledTimes(1));
  await userEvent.click(await screen.findByRole('button',{name:/Other private review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await waitFor(()=>expect(review).toHaveBeenCalledTimes(2));
  expect(review.mock.calls[0][2]).not.toBe(review.mock.calls[1][2]);
});

it('uses a new review key after the same artifact is revised',async()=>{
  const reviewed={...draft,revision:2,state:'reviewed' as const,digest:'b'.repeat(64)};
  const revised={...draft,revision:3,digest:'c'.repeat(64),title:'Revised risk review'};
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[{id:draft.id,kind:draft.kind,title:draft.title,revision:1,state:'draft',freshness:'current',digest}]});
  vi.spyOn(distributeArtifactsApi,'get').mockResolvedValue({artifact:draft,freshness:'current'});
  const review=vi.spyOn(distributeArtifactsApi,'review').mockResolvedValueOnce({artifact:reviewed,freshness:'current'}).mockResolvedValueOnce({artifact:{...revised,revision:4,state:'reviewed'},freshness:'current'});
  vi.spyOn(distributeArtifactsApi,'revise').mockResolvedValue({artifact:revised,freshness:'current'});
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await waitFor(()=>expect(screen.getByText(/Review recorded/)).toBeTruthy());
  await userEvent.clear(screen.getByLabelText('Title'));
  await userEvent.type(screen.getByLabelText('Title'),'Revised risk review');
  await userEvent.click(screen.getByRole('button',{name:'Save artifact edits'}));
  await waitFor(()=>expect(screen.getByText(/New private draft revision saved/)).toBeTruthy());
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await waitFor(()=>expect(review).toHaveBeenCalledTimes(2));
  expect(review.mock.calls[0][2]).not.toBe(review.mock.calls[1][2]);
  expect(review.mock.calls[1][1]).toMatchObject({id:draft.id,revision:3,digest:revised.digest});
});

it('retries an uncertain exact review with the same key after switching artifacts',async()=>{
  const other={...draft,id:'artifact-other',title:'Other private review',digest:'b'.repeat(64)};
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[draft,other].map(row=>({id:row.id,kind:row.kind,title:row.title,revision:row.revision,state:row.state,freshness:'current',digest:row.digest}))});
  vi.spyOn(distributeArtifactsApi,'get').mockImplementation(async(_project,id)=>({artifact:id===draft.id?draft:other,freshness:'current'}));
  const review=vi.spyOn(distributeArtifactsApi,'review').mockRejectedValueOnce(new Error('Review outcome unconfirmed')).mockResolvedValueOnce({artifact:{...draft,revision:2,state:'reviewed'},freshness:'readback'});
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await screen.findByRole('alert');
  await userEvent.click(screen.getByRole('button',{name:/Other private review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:/Risk and evidence review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await waitFor(()=>expect(review).toHaveBeenCalledTimes(2));
  expect(review.mock.calls[0][2]).toBe(review.mock.calls[1][2]);
  expect(review.mock.calls[0][1]).toMatchObject({id:draft.id,revision:1,digest:draft.digest});
  expect(review.mock.calls[1][1]).toMatchObject({id:draft.id,revision:1,digest:draft.digest});
});
