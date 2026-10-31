import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DeploymentBlueprint } from './DeploymentBlueprint';
import { deploymentBlueprintApi, type BlueprintProjectContext, type DeploymentBlueprint, type DeploymentBlueprintPlan, type SavedDeploymentBlueprint } from '../../api/deploymentBlueprint';
import { resetCockpit } from '../../store/store';

const access = vi.hoisted(() => ({ canRead: true, canEdit: true, loading: false, error: '', message: '' }));
const owner = vi.hoisted(() => ({ actor: 'one' }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: (id: string) => `${owner.actor}:${id}` }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => access }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
const context = (): BlueprintProjectContext => ({ projectId: 'p-one', head: 'a'.repeat(40), clean: true, reviewDigest: 'b'.repeat(64), definitionStatus: 'draft', designStatus: 'approved',
  source: { commit: 'a'.repeat(40), configurationRevision: 2, context: { briefRevision: 1, specificationRevision: 3, designRevision: 4 } } });
beforeEach(() => {
  resetCockpit(); owner.actor = 'one'; access.canRead = true; access.canEdit = true; access.error = ''; localStorage.clear();
  vi.spyOn(deploymentBlueprintApi, 'saved').mockResolvedValue({ projectId: 'p-one', saved: null, sourceVerified: false, effectsApplied: false });
  vi.spyOn(deploymentBlueprintApi, 'save').mockImplementation(async (_id, request) => ({ projectId: 'p-one', saved: savedBlueprint(request.blueprint, request.expectedRevision + 1), sourceVerified: false, effectsApplied: false }));
  vi.spyOn(deploymentBlueprintApi, 'context').mockImplementation(async () => context());
  vi.spyOn(deploymentBlueprintApi, 'compile').mockImplementation(async input => ({ version: 1, digest: 'd'.repeat(64), state: 'decisions-required', blueprint: input,
    sourceVerified: false, effectsApplied: false, steps: [], decisions: [{ path: 'services[0].image', code: 'image-required', message: 'Choose an existing immutable image.' }], artifacts: [], capabilities: [], warnings: [],
    resourceSummary: { replicas: 0, requestMilliCpu: 0, limitMilliCpu: 0, requestMemoryMiB: 0, limitMemoryMiB: 0, gpuCount: 0 } }));
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

function savedBlueprint(blueprint: DeploymentBlueprint, revision = 1): SavedDeploymentBlueprint {
  return { projectId: 'p-one', revision, digest: String(revision).repeat(64), planDigest: 'd'.repeat(64), blueprint,
    actor: { id: 'one', displayName: 'Project owner' }, at: '2026-10-31T00:00:00Z' };
}
const savedTopology = (): DeploymentBlueprint => ({ version: 1, projectId: 'p-one', source: context().source,
  infrastructure: { id: 'cluster', kind: 'kubernetes', reference: 'saved-east', namespace: 'saved-app' },
  services: [{ id: 'app-4', kind: 'app', image: '', port: 0, replicas: 0, resources: { requestMilliCpu: 0, limitMilliCpu: 0, requestMemoryMiB: 0, limitMemoryMiB: 0 }, dependsOn: ['cluster'] }] });

it('keeps unsupported saved topology protected through baseline review and clearing the local draft', async () => {
  const blueprint = savedTopology(); blueprint.infrastructure.kind = 'existing-vm';
  vi.mocked(deploymentBlueprintApi.saved).mockResolvedValue({ projectId: 'p-one', saved: savedBlueprint(blueprint), sourceVerified: false, effectsApplied: false });
  const user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-one" active onPublishing={vi.fn()}/>);
  await screen.findByRole('alert');
  await user.type(screen.getByRole('textbox', { name: 'Namespace' }), 'local-change');
  await user.click(screen.getByRole('button', { name: 'Review saved revision without replacing edits' }));
  await screen.findByRole('alert');
  await user.click(screen.getByRole('checkbox', { name: /Use saved revision 1/ }));
  expect(screen.getByRole('button', { name: 'Save blueprint' }).hasAttribute('disabled')).toBe(true);
  await user.click(screen.getByRole('button', { name: 'Clear planning draft' }));
  expect(screen.getByRole('button', { name: 'Save blueprint' }).hasAttribute('disabled')).toBe(true);
  expect(deploymentBlueprintApi.save).not.toHaveBeenCalled();
});

it('reloads saved topology without trusting old output and saves a new current-context revision', async () => {
  vi.mocked(deploymentBlueprintApi.saved).mockResolvedValue({ projectId: 'p-one', saved: savedBlueprint(savedTopology()), sourceVerified: false, effectsApplied: false });
  const user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-one" active onPublishing={vi.fn()}/>);
  await waitFor(() => expect((screen.getByRole('textbox', { name: 'Namespace' }) as HTMLInputElement).value).toBe('saved-app'));
  expect(deploymentBlueprintApi.compile).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Add application' }));
  expect(screen.getByRole('group', { name: 'Application · app-5' })).not.toBeNull();
  await user.click(screen.getByRole('button', { name: 'Save blueprint' }));
  await screen.findByText(/Saved service blueprint revision 2/);
  const request = vi.mocked(deploymentBlueprintApi.save).mock.calls[0][1];
  expect(request.expectedRevision).toBe(1); expect(request.expectedDigest).toBe('1'.repeat(64)); expect(request.blueprint.source).toEqual(context().source);
  expect(request.blueprint.services).toHaveLength(2); expect(screen.queryByRole('region', { name: 'Generated deployment plan' })).toBeNull();
});

it('preserves edited topology through a saved-revision conflict and requires explicit baseline review', async () => {
  const user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-one" active onPublishing={vi.fn()}/>);
  await waitFor(() => expect(screen.getByRole('button', { name: 'Save blueprint' }).hasAttribute('disabled')).toBe(false));
  await user.type(screen.getByRole('textbox', { name: 'Namespace' }), 'my-unsaved-topology');
  vi.mocked(deploymentBlueprintApi.save).mockRejectedValueOnce(new Error('The saved service blueprint changed. Keep your draft.'));
  await user.click(screen.getByRole('button', { name: 'Save blueprint' }));
  await screen.findByRole('alert');
  vi.mocked(deploymentBlueprintApi.saved).mockResolvedValueOnce({ projectId: 'p-one', saved: savedBlueprint(savedTopology(), 2), sourceVerified: false, effectsApplied: false });
  await user.click(screen.getByRole('button', { name: 'Review saved revision without replacing edits' }));
  await screen.findByRole('checkbox', { name: /Use saved revision 2/ });
  expect((screen.getByRole('textbox', { name: 'Namespace' }) as HTMLInputElement).value).toBe('my-unsaved-topology');
  expect(screen.getByRole('button', { name: 'Save blueprint' }).hasAttribute('disabled')).toBe(true);
  await user.click(screen.getByRole('checkbox', { name: /Use saved revision 2/ }));
  await user.click(screen.getByRole('button', { name: 'Save blueprint' }));
  await screen.findByText(/Saved service blueprint revision 3/);
  const request = vi.mocked(deploymentBlueprintApi.save).mock.calls[1][1];
  expect(request.expectedRevision).toBe(2); expect(request.blueprint.infrastructure.namespace).toBe('my-unsaved-topology');
});

it('submits project-bound app/inference planning inputs and displays unresolved decisions without execution controls', async () => {
  const user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-one" active onPublishing={vi.fn()}/>);
  await user.type(screen.getByRole('textbox', { name: 'Cluster reference' }), 'existing-east');
  await user.type(screen.getByRole('textbox', { name: 'Namespace' }), 'app-one');
  await user.click(screen.getByRole('button', { name: 'Add inference service' }));
  const model = within(screen.getByRole('group', { name: 'Inference · inference-1' }));
  await user.type(model.getByRole('textbox', { name: 'Model name' }), 'approved-model');
  await user.type(model.getByRole('textbox', { name: 'Existing model PVC' }), 'models');
  await user.click(model.getByText('Container entrypoint and arguments'));
  await user.type(model.getByRole('textbox', { name: 'Command' }), '/serve{Enter}--model');
  expect((model.getByRole('textbox', { name: 'Command' }) as HTMLTextAreaElement).value).toBe('/serve\n--model');
  await user.click(screen.getByRole('button', { name: 'Generate deployment plan' }));
  await screen.findByRole('heading', { name: 'Inputs still needed' });
  const sent = vi.mocked(deploymentBlueprintApi.compile).mock.calls[0][0];
  expect(sent.projectId).toBe('p-one'); expect(sent.source).toEqual(context().source);
  expect(sent.services[1].model?.claimName).toBe('models'); expect(sent.services[1].command).toEqual(['/serve', '--model']);
  expect(screen.getByText(/Choose an existing immutable image/)).not.toBeNull();
  expect(screen.queryByRole('button', { name: /^Apply|^Deploy now/ })).toBeNull();
  const keys = Array.from({ length: localStorage.length }, (_, index) => localStorage.key(index)!);
  expect(keys.some(key => key.startsWith('luci.project-draft.'))).toBe(false);
  const stored = keys.map(key => localStorage.getItem(key) ?? '').join('\n');
  for (const input of ['existing-east', 'app-one', 'approved-model', '/serve']) expect(stored).not.toContain(input);
});

it('retains in-memory edits across tabs and repairs a removed dependency explicitly', async () => {
  const user = userEvent.setup(), props = { projectId: 'p-one', onPublishing: vi.fn() };
  const view = render(<DeploymentBlueprint {...props} active/>);
  await user.type(screen.getByRole('textbox', { name: 'Namespace' }), 'keep-me');
  await user.click(screen.getByRole('button', { name: 'Add inference service' }));
  const app = within(screen.getByRole('group', { name: 'Application · app-1' }));
  await user.click(app.getByRole('checkbox', { name: 'inference-1' }));
  await user.click(screen.getByRole('button', { name: 'Remove inference-1' }));
  await user.click(screen.getByRole('button', { name: 'Add inference service' }));
  expect(screen.getByRole('group', { name: 'Inference · inference-2' })).not.toBeNull();
  expect((app.getByRole('checkbox', { name: /removed service/ }) as HTMLInputElement).checked).toBe(true);
  await user.click(app.getByRole('checkbox', { name: /removed service/ }));
  view.rerender(<DeploymentBlueprint {...props} active={false}/>);
  view.rerender(<DeploymentBlueprint {...props} active/>);
  expect((screen.getByRole('textbox', { name: 'Namespace' }) as HTMLInputElement).value).toBe('keep-me');
  expect(deploymentBlueprintApi.context).not.toHaveBeenCalled();
});

it('keeps a requested telemetry binding explicit until the user removes it', async () => {
  const user = userEvent.setup(); render(<DeploymentBlueprint projectId="p-one" active onPublishing={vi.fn()}/>);
  const app = within(screen.getByRole('group', { name: 'Application · app-1' }));
  await user.click(app.getByRole('checkbox', { name: 'Bind this service to the telemetry receiver below' }));
  await user.type(screen.getByRole('textbox', { name: 'Receiver reference' }), 'existing-collector');
  await user.type(screen.getByRole('textbox', { name: /^HTTPS receiver endpoint(?:\s|$)/ }), 'https://collector.example/otlp');
  await user.click(screen.getByRole('button', { name: 'Generate deployment plan' }));
  await screen.findByRole('heading', { name: 'Inputs still needed' });
  const sent = vi.mocked(deploymentBlueprintApi.compile).mock.calls[0][0];
  expect(sent.externalResources).toEqual([{ id: 'analytics', kind: 'analytics', reference: 'existing-collector' }]);
  expect(sent.services[0].telemetry?.endpoint).toBe('https://collector.example/otlp');
  await user.click(screen.getByRole('checkbox', { name: 'Use an existing OTLP/HTTP receiver' }));
  expect((app.getByRole('checkbox', { name: 'Bind this service to the telemetry receiver below' }) as HTMLInputElement).checked).toBe(false);
});

it('withdraws old-scope requests and exposes no private plan after access loss', async () => {
  let finish!: (value: BlueprintProjectContext) => void;
  vi.mocked(deploymentBlueprintApi.context).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const user = userEvent.setup(), props = { projectId: 'p-one', active: true, onPublishing: vi.fn() };
  const view = render(<DeploymentBlueprint {...props}/>);
  await user.click(screen.getByRole('button', { name: 'Generate deployment plan' }));
  owner.actor = 'two'; view.rerender(<DeploymentBlueprint {...props}/>);
  await act(async () => finish(context()));
  expect(deploymentBlueprintApi.compile).not.toHaveBeenCalled();
  access.canRead = false; view.rerender(<DeploymentBlueprint {...props}/>);
  expect(screen.queryByRole('button', { name: 'Generate deployment plan' })).toBeNull();
});

it('refuses a result if source or context changes during compilation', async () => {
  const after = context(); after.source.configurationRevision++;
  vi.mocked(deploymentBlueprintApi.context).mockResolvedValueOnce(context()).mockResolvedValueOnce(after);
  render(<DeploymentBlueprint projectId="p-one" active onPublishing={vi.fn()}/>);
  await userEvent.click(screen.getByRole('button', { name: 'Generate deployment plan' }));
  await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('changed during planning'));
  expect(screen.queryByRole('region', { name: 'Generated deployment plan' })).toBeNull();
});

it('requests an inspected manifest download and opens existing release forms without applying anything', async () => {
  const user = userEvent.setup(), publishing = vi.fn();
  const makeURL = vi.fn((_blob: Blob | MediaSource) => 'blob:reviewed-plan');
  vi.stubGlobal('URL', class extends URL { static createObjectURL = makeURL; static revokeObjectURL = vi.fn(); });
  const download = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  vi.mocked(deploymentBlueprintApi.compile).mockImplementation(async input => ({ version: 1, digest: 'd'.repeat(64), state: 'compiled-prototype', blueprint: input, sourceVerified: false, effectsApplied: false,
    steps: [], decisions: [], artifacts: [{ name: 'app.deployment.json', content: '{"kind":"Deployment"}\n', mediaType: 'application/json', sha256: 'e'.repeat(64) }], capabilities: [], warnings: [],
    resourceSummary: { replicas: 1, requestMilliCpu: 100, limitMilliCpu: 500, requestMemoryMiB: 128, limitMemoryMiB: 512, gpuCount: 0 } } as DeploymentBlueprintPlan));
  render(<DeploymentBlueprint projectId="p-one" active onPublishing={publishing}/>);
  await user.click(screen.getByRole('button', { name: 'Generate deployment plan' }));
  await screen.findByRole('heading', { name: 'Manifests ready for review' });
  await user.click(screen.getByText('app.deployment.json', { selector: 'summary' }));
  expect(screen.getByText('{"kind":"Deployment"}', { selector: 'pre' })).not.toBeNull();
  await user.click(screen.getByRole('button', { name: 'Download app.deployment.json' }));
  expect(makeURL).toHaveBeenCalledOnce(); expect(download).toHaveBeenCalledOnce();
  expect((makeURL.mock.calls[0][0] as Blob).type).toBe('application/json');
  await user.click(screen.getByRole('button', { name: 'Open existing release paths' }));
  expect(publishing).toHaveBeenCalledOnce();
});
