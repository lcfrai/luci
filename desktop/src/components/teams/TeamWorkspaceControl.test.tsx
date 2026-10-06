import { beforeEach, afterEach, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TeamWorkspaceControl } from './TeamWorkspaceControl';
import { teamsApi, emptyGrants, personalPermissions, type TeamDetail, type TeamSummary } from '../../api/teams';
import { resetCockpit, useCockpit } from '../../store/store';
const workspace=vi.hoisted(()=>({selected:null as TeamSummary|null,teams:[] as TeamSummary[],refresh:vi.fn(),select:vi.fn(),revision:0,teamId:''}));
vi.mock('../auth/TeamServiceStatus',()=>({TeamServiceStatus:()=>null}));
vi.mock('./TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>workspace}));
const owner={userId:'alice',role:'owner' as const,grants:{...emptyGrants,admin:true},revision:1,user:{id:'alice',username:'alice',displayName:'Alice',role:'member' as const,disabled:false}};
const editor={userId:'bob',role:'editor' as const,grants:{...emptyGrants},revision:1,user:{id:'bob',username:'bob',displayName:'Bob',role:'member' as const,disabled:false}};
const detail:TeamDetail={team:{id:'team-abcdefghijklmnopqrstuv',name:'Acme',revision:7,createdAt:'',updatedAt:''},membership:owner,permissions:personalPermissions,members:[owner,editor],executionBoundary:'Separate workspace'};
beforeEach(()=>{
  resetCockpit();workspace.selected=detail;workspace.teams=[detail];workspace.refresh.mockReset().mockResolvedValue(undefined);workspace.select.mockReset();
  vi.spyOn(teamsApi,'detail').mockResolvedValue(structuredClone(detail));
  vi.spyOn(teamsApi,'addMember').mockResolvedValue(detail);
  vi.spyOn(teamsApi,'saveMember').mockResolvedValue(detail);
  vi.spyOn(teamsApi,'removeMember').mockResolvedValue({});
  vi.spyOn(teamsApi,'projectAccess').mockResolvedValue({team:detail.team,projectId:'p1',permissions:personalPermissions,grants:[]});
  vi.spyOn(teamsApi,'saveProjectGrant').mockResolvedValue({team:detail.team,projectId:'p1',permissions:personalPermissions,grants:[]});
});
afterEach(()=>vi.restoreAllMocks());
async function open(){render(<TeamWorkspaceControl/>);await userEvent.click(screen.getByRole('button',{name:'Team access'}));await screen.findByRole('heading',{name:/^Members/});}
it('adds a collaborator with explicit independent grants and the current team revision',async()=>{
  await open();await userEvent.click(screen.getByText('Add an existing member'));const form=screen.getByRole('group',{name:'Add an existing Luci account'});
  await userEvent.type(within(form).getByLabelText('Username'),'charlie');
  await userEvent.selectOptions(within(form).getByLabelText('Role'),'editor');
  await userEvent.click(within(form).getByLabelText('Run agents and checks'));
  await userEvent.click(within(form).getByRole('button',{name:'Add member'}));
  await waitFor(()=>expect(teamsApi.addMember).toHaveBeenCalledWith(detail.team.id,7,'charlie','editor',{...emptyGrants,execute:true}));
  expect((within(form).getByLabelText('Username') as HTMLInputElement).value).toBe('');
});
it('shows a stale-save error and refreshes the actual role without silently retrying the edit',async()=>{
  vi.mocked(teamsApi.saveMember).mockRejectedValue(new Error('Team changed. Reload current access before saving.'));
  await open();await userEvent.click(screen.getByLabelText('Team access for Bob'));const bob=screen.getByRole('group',{name:'Bob'});
  await userEvent.selectOptions(within(bob).getByLabelText('Role'),'viewer');
  await userEvent.click(within(bob).getByRole('button',{name:'Save access'}));
  expect((await screen.findByRole('alert')).textContent).toContain('Team changed. Reload current access before saving.');
  expect(teamsApi.saveMember).toHaveBeenCalledTimes(1);expect(workspace.refresh).toHaveBeenCalled();
});
it('requires explicit member removal and explains that shared work remains',async()=>{
  await open();await userEvent.click(screen.getByLabelText('Team access for Bob'));const bob=screen.getByRole('group',{name:'Bob'});
  await userEvent.click(within(bob).getByRole('button',{name:'Remove access'}));
  expect(teamsApi.removeMember).not.toHaveBeenCalled();
  expect(within(bob).getByText(/existing shared work and running resources will remain/)).toBeTruthy();
  await userEvent.click(within(bob).getByRole('button',{name:'Confirm removal'}));
  await waitFor(()=>expect(teamsApi.removeMember).toHaveBeenCalledWith(detail.team.id,7,'bob'));
});
it('keeps nonadministrators out of membership and project mutation controls',async()=>{
  const viewer={...detail,membership:{...editor,role:'viewer' as const},permissions:{...emptyGrants,read:true,edit:false}};
  workspace.selected=viewer;vi.mocked(teamsApi.detail).mockResolvedValue(viewer);
  await open();expect(screen.queryByRole('button',{name:'Save access'})).toBeNull();expect(screen.queryByLabelText('Username')).toBeNull();expect(screen.getByText('Bob · editor')).toBeTruthy();
});
it('shares only the selected project and never invents team execution or secret grants',async()=>{
  useCockpit.setState({selection:{projectId:'p1',branchId:null,worktreeId:null,sessionId:null},projects:[{id:'p1',name:'Atlas',path:'/team/atlas'}]});
  await open();await userEvent.click(screen.getByRole('tab',{name:'Project sharing'}));const share=await screen.findByRole('group',{name:'Grant project access'});
  await userEvent.selectOptions(within(share).getByLabelText('Team member'),'bob');await userEvent.selectOptions(within(share).getByLabelText('Project role'),'editor');
  await userEvent.click(within(share).getByRole('button',{name:'Share project'}));
  await waitFor(()=>expect(teamsApi.saveProjectGrant).toHaveBeenCalledWith(detail.team.id,'p1',7,{userId:'bob',role:'editor',grants:emptyGrants}));
});
