import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { distributeArtifactsApi, type DistributeArtifact } from '../../api/distributeArtifacts';
import { projectRunsApi } from '../../api/projectRuns';
import { productApi } from '../../api/product';
import { resetCockpit } from '../../store/store';
import { ArtifactWorkspace } from './ArtifactWorkspace';

const workspace=vi.hoisted(()=>({scope:'account-one/workspace-one/p-one' as string|null}));
vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=>workspace.scope}));
const retirementTimers:number[]=[];
const digest='a'.repeat(64);
const draft:DistributeArtifact={id:'artifact-123',projectId:'p-one',revision:1,kind:'risk',source:{briefRevision:1,briefDigest:digest,specificationRevision:1,specificationDigest:digest,designRevision:1,designDigest:digest,planRevision:1,planDigest:digest,checks:'not-selected',release:'not-selected'},sourceDigest:digest,title:'Risk and evidence review',sections:[{id:'purpose',heading:'Saved product purpose',body:'Saved Orchard tasklist brief',bullets:['Audience: local users'],sourceRefs:[`brief:1:${digest}`]}],gaps:['No exact check receipt was selected.'],assumptions:['Likelihood requires review.'],state:'draft',digest,at:'2026-10-15T00:00:00Z'};
beforeEach(()=>{vi.restoreAllMocks();workspace.scope='account-one/workspace-one/p-one';resetCockpit();vi.spyOn(distributeArtifactsApi,'list').mockResolvedValue({projectId:'p-one',artifacts:[]})});
afterEach(()=>{cleanup();for(const timer of retirementTimers.splice(0))window.clearTimeout(timer);vi.unstubAllGlobals()});

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

it('preserves typed key-point lines until saving exactly two normalized bullets',async()=>{
  const bullets=['Audience: local users','Second key point'];
  const revised={...draft,revision:2,digest:'b'.repeat(64),sections:[{...draft.sections[0],bullets}]};
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[{id:draft.id,kind:draft.kind,title:draft.title,revision:1,state:'draft',freshness:'current',digest}]});
  vi.spyOn(distributeArtifactsApi,'get').mockResolvedValue({artifact:draft,freshness:'current'});
  const revise=vi.spyOn(distributeArtifactsApi,'revise').mockResolvedValue({artifact:revised,freshness:'current'});
  const review=vi.spyOn(distributeArtifactsApi,'review');
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  const points=screen.getByLabelText('Key points, one per line');
  await userEvent.click(points);
  await userEvent.keyboard('{End}{Enter}  Second key point  {Enter}{Enter}');
  expect(points).toHaveProperty('value','Audience: local users\n  Second key point  \n\n');
  expect(revise).not.toHaveBeenCalled();
  expect(screen.getByRole('button',{name:'Mark reviewed'})).toHaveProperty('disabled',true);
  expect(screen.getByRole('button',{name:'Download DOCX'})).toHaveProperty('disabled',true);
  await userEvent.click(screen.getByRole('button',{name:'Save artifact edits'}));
  await screen.findByText('New private draft revision saved; review is still required.');
  expect(revise).toHaveBeenCalledExactlyOnceWith('p-one',draft,draft.title,[{...draft.sections[0],bullets}],draft.gaps,draft.assumptions,expect.stringMatching(/^distribute-artifact-/));
  expect(points).toHaveProperty('value',bullets.join('\n'));
  expect(draft.sections[0].bullets).toEqual(['Audience: local users']);
  expect(review).not.toHaveBeenCalled();
});

it('keeps raw key points on a blocked artifact switch and restores the selected saved draft on discard',async()=>{
  const other={...draft,id:'artifact-other',title:'Other private review',sections:[{...draft.sections[0],bullets:['Other saved point']}]};
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[draft,other].map(row=>({id:row.id,kind:row.kind,title:row.title,revision:1,state:row.state,freshness:'current',digest:row.digest}))});
  const get=vi.spyOn(distributeArtifactsApi,'get').mockImplementation(async(_project,id)=>({artifact:id===draft.id?draft:other,freshness:'current'}));
  const revise=vi.spyOn(distributeArtifactsApi,'revise');
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  await userEvent.click(screen.getByLabelText('Key points, one per line'));
  await userEvent.keyboard('{End}{Enter}Unsent point{Enter}');
  await userEvent.click(screen.getByRole('button',{name:/Other private review · revision 1/}));
  expect(await screen.findByRole('alert')).toHaveProperty('textContent','Save or discard artifact edits before opening another artifact.');
  expect(screen.getByLabelText('Key points, one per line')).toHaveProperty('value','Audience: local users\nUnsent point\n');
  expect(get).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole('button',{name:'Discard edits'}));
  expect(screen.getByLabelText('Key points, one per line')).toHaveProperty('value','Audience: local users');
  await userEvent.click(screen.getByRole('button',{name:/Other private review · revision 1/}));
  await waitFor(()=>expect(screen.getByLabelText('Key points, one per line')).toHaveProperty('value','Other saved point'));
  expect(get).toHaveBeenLastCalledWith('p-one',other.id);
  expect(revise).not.toHaveBeenCalled();
});

it.each(['read-only','outdated source'] as const)('does not edit key points with %s',async(mode)=>{
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[{id:draft.id,kind:draft.kind,title:draft.title,revision:1,state:'draft',freshness:'current',digest}]});
  vi.spyOn(distributeArtifactsApi,'get').mockResolvedValue({artifact:draft,freshness:mode==='read-only'?'current':'outdated'});
  const revise=vi.spyOn(distributeArtifactsApi,'revise');
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit={mode!=='read-only'}/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  const points=screen.getByLabelText('Key points, one per line');
  expect(points).toHaveProperty('disabled',true);
  await userEvent.type(points,'{Enter}Unadmitted point');
  expect(points).toHaveProperty('value','Audience: local users');
  expect(screen.getByRole('button',{name:'Save artifact edits'})).toHaveProperty('disabled',true);
  expect(revise).not.toHaveBeenCalled();
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

function deferredArtifactExport(){
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[{id:draft.id,kind:draft.kind,title:draft.title,revision:draft.revision,state:draft.state,freshness:'current',digest:draft.digest}]});
  vi.spyOn(distributeArtifactsApi,'get').mockResolvedValue({artifact:draft,freshness:'current'});
  let finish!:(value:Blob)=>void;
  const request=vi.spyOn(productApi,'requestBlob').mockImplementation(()=>new Promise<Blob>(resolve=>{finish=resolve}));
  const create=vi.fn(()=> 'blob:private-artifact');
  const revoke=vi.fn();
  vi.stubGlobal('URL',class extends URL {static createObjectURL=create;static revokeObjectURL=revoke});
  const click=vi.spyOn(HTMLAnchorElement.prototype,'click').mockImplementation(()=>{});
  const originalTimeout=window.setTimeout.bind(window);
  vi.spyOn(window,'setTimeout').mockImplementation((handler,timeout,...args)=>{
    const timer=originalTimeout(handler,timeout,...args);
    // Keep the real timing; clear pending one-second URL callbacks before
    // restoring the URL test double. No timeout fires early to pass a test.
    if(timeout===1000)retirementTimers.push(timer);
    return timer;
  });
  return {request,create,click,revoke,finish:()=>finish(new Blob(['private saved artifact']))};
}

it.each(['unmount','project change and back','account change and back','workspace change and back','read grant change and back'])(
  'does not publish a delayed private artifact export after %s',async(change)=>{
    const pending=deferredArtifactExport();
    const view=render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
    await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
    await userEvent.click(screen.getByRole('button',{name:'Download DOCX'}));
    expect(pending.request).toHaveBeenCalledExactlyOnceWith(`/v1/product/projects/p-one/distribute/artifacts/${draft.id}/exports/docx?digest=${digest}`);
    if(change==='unmount')view.unmount();
    else if(change==='project change and back'){
      view.rerender(<ArtifactWorkspace projectId="p-two" canRead canEdit/>);
      view.rerender(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
    }else if(change==='read grant change and back'){
      view.rerender(<ArtifactWorkspace projectId="p-one" canRead={false} canEdit={false}/>);
      view.rerender(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
    }else{
      workspace.scope=change==='account change and back'?'account-two/workspace-one/p-one':'account-one/workspace-two/p-one';
      view.rerender(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
      workspace.scope='account-one/workspace-one/p-one';
      view.rerender(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
    }
    await act(async()=>{pending.finish()});
    expect(pending.create).not.toHaveBeenCalled();
    expect(pending.click).not.toHaveBeenCalled();
    expect(pending.request).toHaveBeenCalledTimes(1);
  },
);

it.each(['DOCX','PPTX','XLSX'])('downloads the exact saved %s artifact while current read-only access remains valid',async(format)=>{
  const pending=deferredArtifactExport();
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit={false}/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:`Download ${format}`}));
  await act(async()=>{pending.finish()});
  expect(pending.request).toHaveBeenCalledExactlyOnceWith(`/v1/product/projects/p-one/distribute/artifacts/${draft.id}/exports/${format.toLowerCase()}?digest=${digest}`);
  expect(pending.create).toHaveBeenCalledTimes(1);
  expect(pending.click).toHaveBeenCalledTimes(1);
});

it('requires a known account and workspace before offering private artifact downloads',async()=>{
  const pending=deferredArtifactExport();
  workspace.scope=null;
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  expect(screen.getByRole('button',{name:'Download DOCX'}).hasAttribute('disabled')).toBe(true);
  expect(pending.request).not.toHaveBeenCalled();
});

it('keeps PDF unavailable when the server has not advertised its verified font bundle',async()=>{
  const pending=deferredArtifactExport();
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  expect(screen.queryByRole('button',{name:'Download PDF'})).toBeNull();
  expect(pending.request).not.toHaveBeenCalled();
});

it.each(['current','read grant revoked and restored'] as const)('downloads a server-advertised PDF only while its read identity remains %s',async(state)=>{
  const pending=deferredArtifactExport();
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({projectId:'p-one',artifacts:[{id:draft.id,kind:draft.kind,title:draft.title,revision:draft.revision,state:draft.state,freshness:'current',digest}],exportFormats:['docx','pptx','xlsx','pdf']});
  const view=render(<ArtifactWorkspace projectId="p-one" canRead canEdit={false}/>);
  await userEvent.click(await screen.findByRole('button',{name:/Risk and evidence review · revision 1/}));
  await userEvent.click(screen.getByRole('button',{name:'Download PDF'}));
  expect(pending.request).toHaveBeenCalledExactlyOnceWith(`/v1/product/projects/p-one/distribute/artifacts/${draft.id}/exports/pdf?digest=${digest}`);
  if(state!=='current'){
    view.rerender(<ArtifactWorkspace projectId="p-one" canRead={false} canEdit={false}/>);
    view.rerender(<ArtifactWorkspace projectId="p-one" canRead canEdit={false}/>);
  }
  await act(async()=>{pending.finish()});
  expect(pending.create).toHaveBeenCalledTimes(state==='current'?1:0);
  expect(pending.click).toHaveBeenCalledTimes(state==='current'?1:0);
});
