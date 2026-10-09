import { beforeEach, afterEach, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TeamWorkspaceControl } from './TeamWorkspaceControl';
import { teamsApi, emptyGrants, personalPermissions, type TeamDetail, type TeamSummary } from '../../api/teams';
import { resetCockpit, useCockpit } from '../../store/store';
import { ApiError, type ProblemCode } from '../../api/client';
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


// CLOUD-060: actual TeamWorkspaceControl and WorkflowDialog/Radix with the
// existing controlled workspace. These scenarios do not use a real server,
// browser, provider, Docker, or any successful mutation.
function deferredTeamRead<T>() {
  let resolve!: (value: T) => void, reject!: (cause: Error) => void;
  const promise = new Promise<T>((accept, fail) => {resolve = accept; reject = fail;});
  return {promise, resolve, reject};
}
function expectPrivateTeamModalHidden() {
  expect(screen.queryByRole('dialog', {name: 'Team access'})).toBeNull();
  expect(screen.queryByRole('button', {name: 'Save access'})).toBeNull();
  expect(screen.queryByRole('button', {name: 'Share project'})).toBeNull();
  expect(screen.queryByRole('button', {name: 'Remove access'})).toBeNull();
  expect(document.body.style.pointerEvents).not.toBe('none');
  // Activity may retain DOM and editor state. Its private panel must actually
  // be visually hidden as well as absent from the accessible modal tree.
  for (const panel of document.querySelectorAll<HTMLElement>('.team-panel')) {
    let hidden = false;
    for (let node: HTMLElement | null = panel; node; node = node.parentElement) {
      const style = window.getComputedStyle(node);
      if (node.hidden || style.display === 'none' || style.visibility === 'hidden' || style.visibility === 'collapse') {hidden = true; break;}
    }
    expect(hidden).toBe(true);
  }
}
function expectNoTeamMutation() {
  expect(teamsApi.addMember).not.toHaveBeenCalled();
  expect(teamsApi.saveMember).not.toHaveBeenCalled();
  expect(teamsApi.removeMember).not.toHaveBeenCalled();
  expect(teamsApi.saveProjectGrant).not.toHaveBeenCalled();
}

it('CLOUD060 hides stale team administrator controls through a current revision read, denial and fresh viewer restoration', async () => {
  workspace.revision = 0; workspace.teamId = detail.team.id;
  const user = userEvent.setup(), view = render(<TeamWorkspaceControl/>);
  await user.click(screen.getByRole('button', {name: 'Team access'}));
  await screen.findByRole('heading', {name: /^Members/});
  await user.click(screen.getByLabelText('Team access for Bob'));
  const member = screen.getByRole('group', {name: 'Bob'});
  await user.selectOptions(within(member).getByLabelText('Role'), 'viewer');
  expect(within(member).getByRole('button', {name: 'Save access'})).toHaveProperty('disabled', false);
  expectNoTeamMutation();

  const denied = deferredTeamRead<TeamDetail>();
  vi.mocked(teamsApi.detail).mockReturnValueOnce(denied.promise);
  workspace.revision++;
  view.rerender(<TeamWorkspaceControl/>);
  expectPrivateTeamModalHidden();
  await waitFor(() => expect(teamsApi.detail).toHaveBeenCalledTimes(2));
  expect(vi.mocked(teamsApi.detail).mock.calls.every(([id]) => id === detail.team.id)).toBe(true);
  await act(async () => denied.reject(new ApiError('permission_denied' as ProblemCode, 'Membership was revoked.', 403)));
  expectPrivateTeamModalHidden(); expectNoTeamMutation();

  const fresh = deferredTeamRead<TeamDetail>();
  vi.mocked(teamsApi.detail).mockReturnValueOnce(fresh.promise);
  const viewer: TeamDetail = {...structuredClone(detail), membership: {...editor, role: 'viewer'}, permissions: {...emptyGrants, read: true, edit: false}};
  workspace.selected = viewer; workspace.revision++;
  view.rerender(<TeamWorkspaceControl/>);
  await waitFor(() => expect(teamsApi.detail).toHaveBeenCalledTimes(3));
  expectPrivateTeamModalHidden();
  await act(async () => fresh.resolve(viewer));
  const restored = await screen.findByRole('dialog', {name: 'Team access'});
  expect(within(restored).getByText('Bob · editor')).toBeTruthy();
  expect(within(restored).queryByRole('button', {name: 'Save access'})).toBeNull();
  expect(within(restored).queryByRole('group', {name: 'Add an existing Luci account'})).toBeNull();
  expect(within(restored).queryByRole('button', {name: 'Remove access'})).toBeNull();
  expect(teamsApi.projectAccess).not.toHaveBeenCalled(); expectNoTeamMutation();
});

it('CLOUD060 suspends old project sharing while a different exact project grant read is pending or denied', async () => {
  workspace.revision = 0; workspace.teamId = detail.team.id;
  useCockpit.setState({selection: {projectId: 'p1', branchId: null, worktreeId: null, sessionId: null}, projects: [{id: 'p1', name: 'Atlas', path: '/team/atlas'}, {id: 'p2', name: 'Beacon', path: '/team/beacon'}]});
  const atlasAccess = {team: detail.team, projectId: 'p1', permissions: personalPermissions, grants: [{userId: 'bob', role: 'editor' as const, grants: emptyGrants, revision: 1}]};
  vi.mocked(teamsApi.projectAccess).mockResolvedValueOnce(atlasAccess);
  const user = userEvent.setup(), view = render(<TeamWorkspaceControl/>);
  await user.click(screen.getByRole('button', {name: 'Team access'}));
  await screen.findByRole('heading', {name: /^Members/});
  await user.click(screen.getByRole('tab', {name: 'Project sharing'}));
  await screen.findByRole('heading', {name: 'Share Atlas'});
  await user.click(screen.getByLabelText('Project access for Bob'));
  expect(screen.getByRole('group', {name: 'Bob'})).toBeTruthy();
  expect(teamsApi.projectAccess).toHaveBeenCalledExactlyOnceWith(detail.team.id, 'p1');
  expectNoTeamMutation();

  const pending = deferredTeamRead<typeof atlasAccess>();
  vi.mocked(teamsApi.projectAccess).mockImplementationOnce((teamId, projectId) => {
    expect(teamId).toBe(detail.team.id); expect(projectId).toBe('p2'); return pending.promise;
  });
  act(() => useCockpit.setState({selection: {projectId: 'p2', branchId: null, worktreeId: null, sessionId: null}}));
  expectPrivateTeamModalHidden();
  await waitFor(() => expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2));
  expect(screen.queryByRole('heading', {name: 'Share Atlas'})).toBeNull();
  expect(screen.queryByRole('heading', {name: 'Share Beacon'})).toBeNull();
  await act(async () => pending.reject(new ApiError('permission_denied' as ProblemCode, 'This project is not shared.', 403)));
  expectPrivateTeamModalHidden(); expectNoTeamMutation();

  const beaconAccess = {...atlasAccess, projectId: 'p2', grants: []};
  const restoredRead = deferredTeamRead<typeof beaconAccess>();
  vi.mocked(teamsApi.projectAccess).mockImplementationOnce((teamId, projectId) => {
    expect(teamId).toBe(detail.team.id); expect(projectId).toBe('p2'); return restoredRead.promise;
  });
  workspace.revision++; view.rerender(<TeamWorkspaceControl/>);
  await waitFor(() => expect(teamsApi.projectAccess).toHaveBeenCalledTimes(3));
  expectPrivateTeamModalHidden();
  await act(async () => restoredRead.resolve(beaconAccess));
  const restored = await screen.findByRole('dialog', {name: 'Team access'});
  expect(within(restored).getByRole('heading', {name: 'Share Beacon'})).toBeTruthy();
  expect(within(restored).queryByRole('heading', {name: 'Share Atlas'})).toBeNull();
  expect(within(restored).queryByRole('group', {name: 'Bob'})).toBeNull();
  expect(within(restored).getByRole('group', {name: 'Grant project access'})).toBeTruthy();
  expect(vi.mocked(teamsApi.projectAccess).mock.calls.map(([teamId, projectId]) => [teamId, projectId])).toEqual([[detail.team.id, 'p1'], [detail.team.id, 'p2'], [detail.team.id, 'p2']]);
  expectNoTeamMutation();
});
