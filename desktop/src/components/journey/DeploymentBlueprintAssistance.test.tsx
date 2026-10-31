import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DeploymentBlueprint } from './DeploymentBlueprint';
import { deploymentBlueprintApi } from '../../api/deploymentBlueprint';
import { orchestratorApi } from '../../api/orchestrator';
import { blueprintDraftIntent } from '../../api/orchestratorBlueprint';
import { assistedBlueprintFixture } from '../../test/blueprintAssistanceFixtures';
import { resetCockpit, useCockpit } from '../../store/store';

vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => 'scope/p-1' }));
const permissions = vi.hoisted(() => ({ canRead: true, canEdit: true, canAssist: true, loading: false, error: '', message: '' }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => permissions }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
vi.mock('./KubernetesDeployment', () => ({ KubernetesDeployment: () => null }));
vi.mock('./AzureFoundationHandoff', () => ({ AzureFoundationHandoff: () => null }));
vi.mock('./KubernetesReferencePicker', () => ({ KubernetesReferencePicker: () => null }));
beforeEach(() => {
 vi.restoreAllMocks(); resetCockpit(); permissions.canEdit = true;
 const f = assistedBlueprintFixture();
 vi.spyOn(orchestratorApi, 'get').mockResolvedValue(f.view);
 vi.spyOn(deploymentBlueprintApi, 'saved').mockResolvedValue({ projectId: 'p-1', saved: null, sourceVerified: false, effectsApplied: false });
 vi.spyOn(deploymentBlueprintApi, 'context').mockResolvedValue(f.observed);
 vi.spyOn(deploymentBlueprintApi, 'save').mockImplementation(async (_id, req) => ({ projectId: 'p-1', saved: { projectId: 'p-1', revision: 1, digest: 'e'.repeat(64), blueprint: req.blueprint, planDigest: 'f'.repeat(64), actor: { id: 'alice', displayName: 'Alice' }, at: '2026-10-31T00:00:00Z' }, sourceVerified: false, effectsApplied: false }));
});
afterEach(cleanup);
function incoming() { const f = assistedBlueprintFixture(); useCockpit.setState({ projectBlueprintDraft: blueprintDraftIntent(f.turn, 'scope/p-1') }); return f; }

it('opens a proposal as an unsaved editor draft and saves only after explicit review and click', async () => {
 incoming(); const user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-1" active onPublishing={vi.fn()}/>);
 await screen.findByText(/Luci topology proposal opened as an unsaved draft/);
 expect(deploymentBlueprintApi.save).not.toHaveBeenCalled(); expect(useCockpit.getState().projectBlueprintDraft).toBeNull();
 await user.click(screen.getByRole('button', { name: 'Save service blueprint' }));
 await screen.findByText(/Saved service blueprint revision 1 from your reviewed Luci proposal/);
 expect(deploymentBlueprintApi.save).toHaveBeenCalledWith('p-1', expect.objectContaining({ expectedRevision: 0, expectedDigest: '' }), expect.any(AbortSignal));
});
it('preserves an existing dirty editor and consumes the transfer without starting a background retry', async () => {
 const user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-1" active onPublishing={vi.fn()}/>);
 await screen.findByText(/No saved blueprint yet/); await user.type(screen.getByLabelText('Cluster reference'), 'keep-my-target');
 await act(async () => { incoming(); }); await screen.findByText(/Keep your current edits/);
 expect((screen.getByLabelText('Cluster reference') as HTMLInputElement).value).toBe('keep-my-target'); expect(useCockpit.getState().projectBlueprintDraft).toBeNull();
 expect(orchestratorApi.get).not.toHaveBeenCalled(); expect(deploymentBlueprintApi.save).not.toHaveBeenCalled();
});
it('refuses Save after the captured dirty-source digest changes and retains the topology for review', async () => {
 const f = incoming(), user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-1" active onPublishing={vi.fn()}/>);
 await screen.findByText(/Luci topology proposal opened as an unsaved draft/);
 vi.mocked(deploymentBlueprintApi.context).mockResolvedValue({ ...f.observed, reviewDigest: 'd'.repeat(64) });
 await user.click(screen.getByRole('button', { name: 'Save service blueprint' }));
 await screen.findByText(/blueprint or source changed after this proposal/);
 expect(deploymentBlueprintApi.save).not.toHaveBeenCalled(); expect((screen.getByLabelText('Internal port') as HTMLInputElement).value).toBe('8080');
 await waitFor(() => expect(useCockpit.getState().unsavedProjectDrafts).not.toEqual({}));
});

it('protects an unsupported actual saved baseline even when the proposed subset is editable', async () => {
 const f = assistedBlueprintFixture(), baseline = { ...f.blueprint, externalResources: [{ id: 'database', kind: 'database' as const, reference: 'existing-data' }] };
 f.context.deploymentBlueprint = { ...f.context.deploymentBlueprint!, savedRevision: 1, savedDigest: 'e'.repeat(64), saved: baseline };
 f.proposal.expectedRevision = 1; f.proposal.expectedDigest = 'e'.repeat(64);
 vi.mocked(orchestratorApi.get).mockResolvedValue(f.view);
 vi.mocked(deploymentBlueprintApi.saved).mockResolvedValue({ projectId: 'p-1', saved: { projectId: 'p-1', revision: 1, digest: 'e'.repeat(64), blueprint: baseline, planDigest: 'f'.repeat(64), actor: { id: 'alice', displayName: 'Alice' }, at: '2026-10-31T00:00:00Z' }, sourceVerified: false, effectsApplied: false });
 useCockpit.setState({ projectBlueprintDraft: blueprintDraftIntent(f.turn, 'scope/p-1') });
 render(<DeploymentBlueprint projectId="p-1" active onPublishing={vi.fn()}/>);
 await screen.findByText(/saved topology uses options outside this editor/);
 expect(screen.getByRole('button', { name: 'Save service blueprint' }).matches(':disabled')).toBe(true);
 expect(deploymentBlueprintApi.save).not.toHaveBeenCalled(); expect(screen.queryByText(/opened as an unsaved draft/)).toBeNull();
});
it('withdraws a delayed proposal transfer and settles its local pending state when Edit is revoked', async () => {
 let finish!: (value: Awaited<ReturnType<typeof deploymentBlueprintApi.saved>>) => void;
 vi.mocked(deploymentBlueprintApi.saved).mockReturnValue(new Promise(resolve => { finish = resolve; }));
 incoming(); const mounted = render(<DeploymentBlueprint projectId="p-1" active onPublishing={vi.fn()}/>);
 await waitFor(() => expect(deploymentBlueprintApi.saved).toHaveBeenCalledTimes(1));
 permissions.canEdit = false; mounted.rerender(<DeploymentBlueprint projectId="p-1" active onPublishing={vi.fn()}/>);
 await act(async () => { finish({ projectId: 'p-1', saved: null, sourceVerified: false, effectsApplied: false }); });
 expect(orchestratorApi.get).not.toHaveBeenCalled(); expect(screen.queryByText(/opened as an unsaved draft/)).toBeNull();
 expect(screen.getByRole('button', { name: 'Reload saved blueprint' }).matches(':disabled')).toBe(false);
});
