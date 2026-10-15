import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { distributeArtifactsApi, type DistributeArtifact } from '../../api/distributeArtifacts';
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
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
  await userEvent.selectOptions(screen.getByLabelText('Artifact kind'),'risk');
  await userEvent.click(screen.getByRole('button',{name:'Prepare source draft'}));
  expect(await screen.findByText('Gap: No exact check receipt was selected.')).toBeTruthy();
  expect(prepare).toHaveBeenCalledExactlyOnceWith('p-one','risk');
  expect(save).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Save private draft'}));
  await screen.findByText(/Private draft saved/);
  expect(save).toHaveBeenCalledWith('p-one','risk',digest,expect.stringMatching(/^distribute-artifact-/));
  await userEvent.click(screen.getByRole('button',{name:'Mark reviewed'}));
  await screen.findByText(/Review recorded/);
  expect(review).toHaveBeenCalledWith('p-one',draft,expect.stringMatching(/^distribute-artifact-/));
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
