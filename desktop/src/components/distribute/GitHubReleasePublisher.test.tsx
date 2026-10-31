import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { GitHubReleasePublisher } from './GitHubReleasePublisher';
import { githubReleasesApi, type GitHubPublication } from '../../api/githubReleases';
import { distributeArtifactsApi } from '../../api/distributeArtifacts';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';

vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=> 'person/workspace/project'}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>({canRead:true,canEdit:true,loading:false,error:'',permissions:{deploy:true,secrets:true}})}));
vi.mock('../../hooks/useUnsavedProjectDraft',()=>({useUnsavedProjectDraft:vi.fn()}));
vi.mock('../teams/TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>null}));
afterEach(()=>{cleanup();vi.restoreAllMocks();});

it('loads only on request and offers Observe without retrying an unknown original draft',async()=>{
 const row:GitHubPublication={id:'gh-release-original',projectId:'p-release',revision:2,intent:{projectId:'p-release',repository:'owner/repo',repositoryId:7,tag:'v1.0.0',commit:'b'.repeat(40),gitReviewDigest:'a'.repeat(64),contextDigest:'a'.repeat(64),artifactId:'artifact-launch',artifactRevision:1,artifactDigest:'a'.repeat(64),artifactSourceDigest:'a'.repeat(64),title:'Reviewed launch',body:'Full public copy',account:{id:11,login:'publisher'}},intentDigest:'c'.repeat(64),state:'draft-unknown',createdAt:'2026-10-31T00:00:00Z',updatedAt:'2026-10-31T00:00:00Z',expiresAt:'2026-10-31T00:10:00Z',draftAttempted:true,publishAttempted:false,remote:null,message:'Original outcome unknown.',cliAuthScope:'workspace-cli-not-personal-identity'};
 const list=vi.spyOn(githubReleasesApi,'list').mockResolvedValue({projectId:'p-release',publications:[row],earlierAvailable:false});
 vi.spyOn(distributeArtifactsApi,'list').mockResolvedValue({projectId:'p-release',artifacts:[]});
 const action=vi.spyOn(githubReleasesApi,'act').mockResolvedValue({...row,revision:3});
 const user=userEvent.setup();render(<GitHubReleasePublisher projectId="p-release"/>);expect(list).not.toHaveBeenCalled();
 await user.click(screen.getByRole('button',{name:'Read launch assets and publication history'}));
 await user.type(screen.getByLabelText(/^Existing pushed release tag/),'v-next');
 await user.selectOptions(await screen.findByLabelText('Original publication'),'gh-release-original');
 expect(screen.queryByRole('button',{name:'Create reviewed GitHub draft'})).toBeNull();expect(screen.queryByRole('button',{name:'Publish reviewed GitHub Release'})).toBeNull();
 await user.click(screen.getByRole('button',{name:'Observe original release'}));
 expect(action).toHaveBeenCalledWith('p-release',row,'observe',false,expect.any(AbortSignal));expect(action).toHaveBeenCalledTimes(1);
 const draftCalls=vi.mocked(useUnsavedProjectDraft).mock.calls;expect(draftCalls[draftCalls.length-1]?.[1]).toBe(true);
});
