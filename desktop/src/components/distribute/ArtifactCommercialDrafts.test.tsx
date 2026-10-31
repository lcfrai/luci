import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ArtifactWorkspace } from './ArtifactWorkspace';
import { distributeArtifactsApi, type DistributeArtifact } from '../../api/distributeArtifacts';
import { resetCockpit } from '../../store/store';

vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=> 'commercial-owner/workspace/p-one'}));
const hash='a'.repeat(64),gtmDigest='b'.repeat(64);
function fixture(kind:'pricing'|'support-guide'):DistributeArtifact {
 const support=kind==='support-guide';
 return {id:'artifact-commercial',projectId:'p-one',revision:1,kind,source:{briefRevision:1,briefDigest:hash,specificationRevision:2,specificationDigest:hash,designRevision:1,designDigest:hash,planRevision:3,planDigest:hash,gtmRevision:4,gtmDigest,checks:'not-selected',release:'not-selected'},sourceDigest:'c'.repeat(64),title:support?'Support guide and FAQ':'Pricing evidence and assumptions',sections:[{id:support?'support-contact':'commercial-inputs',heading:support?'FAQ: how do I get help?':'Exact saved scenario inputs',body:support?'Draft choice: choose the support contact and availability.':'Price per unit AUD 29.50; variable cost AUD 5.00; monthly scenario only.',bullets:[],sourceRefs:[`gtm:4:${gtmDigest}`]}],gaps:['Review terms before customer use.'],assumptions:['Demand remains a hypothesis.'],state:'draft',digest:hash,at:'2026-10-31T00:00:00Z'};
}
beforeEach(()=>{vi.restoreAllMocks();localStorage.clear();resetCockpit();vi.spyOn(distributeArtifactsApi,'list').mockResolvedValue({projectId:'p-one',artifacts:[]});});
afterEach(cleanup);

it('previews source-linked go-to-market channels before private save',async()=>{
 const row:DistributeArtifact={...fixture('pricing'),kind:'go-to-market',title:'Go-to-market plan',sections:[{id:'commercial-channels',heading:'Saved channels and audience links',body:'Direct pilot — saved channel hypothesis, not a launched campaign.',bullets:['Milestone: record consenting participant feedback.'],sourceRefs:[`gtm:4:${gtmDigest}`]}]};
 const user=userEvent.setup();vi.spyOn(distributeArtifactsApi,'prepare').mockResolvedValue({draft:row,prepareDigest:hash});const save=vi.spyOn(distributeArtifactsApi,'save');
 render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);await user.selectOptions(screen.getByLabelText('Artifact kind'),'go-to-market');await user.click(screen.getByRole('button',{name:'Prepare source draft'}));
 const preview=await screen.findByLabelText('Prepared commercial copy');expect(within(preview).getByText(/Direct pilot/)).not.toBeNull();expect(within(preview).getByText(/Milestone: record/)).not.toBeNull();expect(save).not.toHaveBeenCalled();
});

it('previews saved scenario Pricing and its exact GTM before an explicit private save',async()=>{
 const row=fixture('pricing'),user=userEvent.setup();
 const prepare=vi.spyOn(distributeArtifactsApi,'prepare').mockResolvedValue({draft:row,prepareDigest:hash});
 const save=vi.spyOn(distributeArtifactsApi,'save').mockResolvedValue({artifact:row,freshness:'current'});
 render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
 await user.selectOptions(screen.getByLabelText('Artifact kind'),'pricing');await user.click(screen.getByRole('button',{name:'Prepare source draft'}));
 const preview=await screen.findByLabelText('Prepared commercial copy');
 expect(within(preview).getByText(/Price per unit AUD 29.50/)).not.toBeNull();
 expect(screen.getByText(new RegExp(`GTM 4 · ${gtmDigest}`))).not.toBeNull();expect(prepare).toHaveBeenCalledWith('p-one','pricing','');expect(save).not.toHaveBeenCalled();
 await user.click(screen.getByRole('button',{name:'Save private draft'}));await screen.findByText(/Private draft saved/);
 expect(save).toHaveBeenCalledWith('p-one','pricing','',hash,expect.any(String));
 expect(screen.queryByRole('button',{name:/Publish|Send campaign/})).toBeNull();
});

it('edits support choices through the existing artifact source CAS without publishing',async()=>{
 const row=fixture('support-guide'),user=userEvent.setup();
 vi.spyOn(distributeArtifactsApi,'prepare').mockResolvedValue({draft:row,prepareDigest:hash});
 vi.spyOn(distributeArtifactsApi,'save').mockResolvedValue({artifact:row,freshness:'current'});
 const revise=vi.spyOn(distributeArtifactsApi,'revise').mockResolvedValue({artifact:{...row,revision:2,sections:[{...row.sections[0],body:'Reviewed support contact; availability still to be agreed.'}]},freshness:'current'});
 render(<ArtifactWorkspace projectId="p-one" canRead canEdit/>);
 await user.selectOptions(screen.getByLabelText('Artifact kind'),'support-guide');await user.click(screen.getByRole('button',{name:'Prepare source draft'}));
 const preview=await screen.findByLabelText('Prepared commercial copy');expect(within(preview).getByText(/Draft choice: choose the support contact/)).not.toBeNull();
 await user.click(screen.getByRole('button',{name:'Save private draft'}));await screen.findByText(/Private draft saved/);
 const answer=screen.getByLabelText('Claim or explanation');await user.clear(answer);await user.type(answer,'Reviewed support contact; availability still to be agreed.');await user.click(screen.getByRole('button',{name:'Save artifact edits'}));await screen.findByText(/New private draft revision saved/);
 expect(revise).toHaveBeenCalledWith('p-one',row,row.title,[expect.objectContaining({body:'Reviewed support contact; availability still to be agreed.',sourceRefs:[`gtm:4:${gtmDigest}`]})],row.gaps,row.assumptions,expect.any(String));
 expect(screen.queryByRole('button',{name:/Publish|Send campaign/})).toBeNull();
});
