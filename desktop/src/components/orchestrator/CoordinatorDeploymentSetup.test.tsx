import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { ApiError } from '../../api/client';
import { coordinatorDeploymentApi, type CoordinatorDeploymentInput, type CoordinatorDeploymentPlan } from '../../api/coordinatorDeployment';
import { CoordinatorDeploymentSetup } from './CoordinatorDeploymentSetup';
import { resetCockpit } from '../../store/store';
const target = { id: 'target-1', name: 'Existing server', baseUrl: 'https://deploy.example.com', serverUuid: 'server-1' };
const props = () => ({ projectId: 'p-1', scope: 'alice/p-1', disabled: false, enabled: true, onChange: vi.fn(), onNavigateDeploy: vi.fn() });
function prepared(input: CoordinatorDeploymentInput): CoordinatorDeploymentPlan { return { ...input, targetFingerprint: 'a'.repeat(64), serverUuid: target.serverUuid, expectedConfigurationRevision: 3, expectedDataDomainRevision: 4, expectedDataBindingFingerprint: 'b'.repeat(64), acknowledgeRepositoryPush: false, acknowledgePublic: false, acknowledgeDeploymentCosts: false }; }
async function open() { await userEvent.click(screen.getByText(/Deploy after checks and local preview/)); await screen.findByLabelText('Repository branch'); }
async function fill() {
  await userEvent.click(screen.getByRole('button', { name: 'Read connected deployment servers' })); await screen.findByRole('option', { name: target.name });
  await userEvent.selectOptions(screen.getByLabelText('Deployment server'), target.id);
  await userEvent.selectOptions(screen.getByLabelText('Repository access'), 'public');
  fireEvent.change(screen.getByLabelText('Repository URL · HTTPS'), { target: { value: 'https://github.com/team/app' } });
  fireEvent.change(screen.getByLabelText('Application domain'), { target: { value: 'https://app.example.com' } });
}
async function prepare() { await userEvent.click(screen.getByRole('button', { name: 'Read current deployment plan' })); await screen.findByRole('region', { name: 'Review current deployment plan' }); }
async function consent() { for (const name of [/I authorize pushing/, /I authorize public deployment/, /I accept ongoing deployment/]) await userEvent.click(screen.getByRole('checkbox', { name })); }
beforeEach(() => { vi.restoreAllMocks(); resetCockpit(); localStorage.clear(); vi.spyOn(coordinatorDeploymentApi, 'targets').mockResolvedValue([target]); vi.spyOn(coordinatorDeploymentApi, 'prepare').mockImplementation(async (_project, input) => prepared(input)); });
it('performs only explicit metadata reads and requires three independent approvals after exact review', async () => {
  const input = props(); render(<CoordinatorDeploymentSetup {...input} />);
  expect(coordinatorDeploymentApi.targets).not.toHaveBeenCalled(); expect(coordinatorDeploymentApi.prepare).not.toHaveBeenCalled(); expect(screen.queryByLabelText('Repository branch')).toBeNull();
  await open(); await fill(); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
  for (const box of screen.getAllByRole('checkbox')) expect(box).toHaveProperty('disabled', true);
  await prepare(); expect(screen.getByText(/server server-1/)).not.toBeNull(); expect(screen.getByText(/configuration revision 3/)).not.toBeNull(); expect(screen.getByText(/Data domain revision 4/)).not.toBeNull();
  await userEvent.click(screen.getByRole('checkbox', { name: /I authorize pushing/ })); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
  await userEvent.click(screen.getByRole('checkbox', { name: /I authorize public deployment/ })); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
  await userEvent.click(screen.getByRole('checkbox', { name: /I accept ongoing deployment/ }));
  expect(input.onChange.mock.lastCall?.[0].plan).toMatchObject({ targetId: target.id, serverUuid: target.serverUuid, expectedConfigurationRevision: 3, expectedDataDomainRevision: 4, acknowledgeRepositoryPush: true, acknowledgePublic: true, acknowledgeDeploymentCosts: true });
  expect(coordinatorDeploymentApi.targets).toHaveBeenCalledTimes(1); expect(coordinatorDeploymentApi.prepare).toHaveBeenCalledTimes(1);
});
it.each(['branch', 'read', 'disabled', 'enabled'] as const)('drops approvals and current bindings after a %s change', async change => {
  const input = props(), ui = render(<CoordinatorDeploymentSetup {...input} />); await open(); await fill(); await prepare(); await consent();
  if (change === 'branch') fireEvent.change(screen.getByLabelText('Repository branch'), { target: { value: 'release' } });
  if (change === 'read') await userEvent.click(screen.getByRole('button', { name: 'Read connected deployment servers' }));
  if (change === 'disabled') { ui.rerender(<CoordinatorDeploymentSetup {...input} disabled />); ui.rerender(<CoordinatorDeploymentSetup {...input} />); }
  if (change === 'enabled') { ui.rerender(<CoordinatorDeploymentSetup {...input} enabled={false} />); ui.rerender(<CoordinatorDeploymentSetup {...input} />); }
  expect(input.onChange.mock.lastCall?.[0].plan).toBeNull(); expect(screen.queryByRole('region', { name: 'Review current deployment plan' })).toBeNull();
  for (const box of screen.getAllByRole('checkbox')) expect(box).toHaveProperty('checked', false);
});
it.each(['edit', 'authority', 'scope', 'project'] as const)('does not accept late preparation after %s changes', async change => {
  let finish!: (value: CoordinatorDeploymentPlan) => void;
  vi.mocked(coordinatorDeploymentApi.prepare).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const input = props(), ui = render(<CoordinatorDeploymentSetup {...input} />); await open(); await fill(); await userEvent.click(screen.getByRole('button', { name: 'Read current deployment plan' }));
  const call = vi.mocked(coordinatorDeploymentApi.prepare).mock.calls[0], signal = call[2]!;
  if (change === 'edit') fireEvent.change(screen.getByLabelText('Repository branch'), { target: { value: 'release' } });
  if (change === 'authority') { ui.rerender(<CoordinatorDeploymentSetup {...input} disabled />); ui.rerender(<CoordinatorDeploymentSetup {...input} />); }
  if (change === 'scope') ui.rerender(<CoordinatorDeploymentSetup {...input} scope="bob/p-1" />);
  if (change === 'project') ui.rerender(<CoordinatorDeploymentSetup {...input} projectId="p-2" scope="alice/p-2" />);
  expect(signal.aborted).toBe(true); await act(async () => finish(prepared(call[1])));
  expect(input.onChange.mock.lastCall?.[0].plan).toBeNull(); expect(screen.queryByRole('region', { name: 'Review current deployment plan' })).toBeNull();
});
it('recovers ordinary settings explicitly while discarding all previous bindings and consents', async () => {
  const input = props(), ui = render(<CoordinatorDeploymentSetup {...input} />); await open(); await fill(); await prepare(); await consent();
  const raw = Object.keys(localStorage).filter(key => key.startsWith('luci.project-draft.')).map(key => localStorage.getItem(key)).join('');
  expect(raw).toContain('https://github.com/team/app');
  for (const privateField of ['targetFingerprint', 'serverUuid', 'expectedConfigurationRevision', 'expectedDataBindingFingerprint', 'acknowledgeRepositoryPush', 'acknowledgePublic', 'acknowledgeDeploymentCosts']) expect(raw).not.toContain(privateField);
  ui.unmount(); render(<CoordinatorDeploymentSetup {...input} />); await open(); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Restore deployment draft' }));
  expect(screen.getByLabelText('Repository URL · HTTPS')).toHaveProperty('value', 'https://github.com/team/app');
  expect(screen.queryByRole('region', { name: 'Review current deployment plan' })).toBeNull(); for (const box of screen.getAllByRole('checkbox')) expect(box).toHaveProperty('checked', false);
});
it('keeps a failed browser draft discard as a blocking problem independently of its dirty flag', async () => {
  const input = props(), ui = render(<CoordinatorDeploymentSetup {...input} />); await open(); await fill(); ui.unmount();
  render(<CoordinatorDeploymentSetup {...input} />); await open(); vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => { throw new DOMException('Blocked', 'SecurityError'); });
  await userEvent.click(screen.getByRole('button', { name: 'Discard browser deployment draft' }));
  expect(input.onChange.mock.lastCall?.[0].plan).toBeNull(); expect(input.onChange.mock.lastCall?.[0].problem).not.toBe('');
  expect(coordinatorDeploymentApi.prepare).not.toHaveBeenCalled();
});
it('rejects a changed connected server and clears metadata when current read access is refused', async () => {
  const input = props(); render(<CoordinatorDeploymentSetup {...input} />); await open(); await fill();
  vi.mocked(coordinatorDeploymentApi.prepare).mockImplementation(async (_project, value) => ({ ...prepared(value), serverUuid: 'replacement' }));
  await userEvent.click(screen.getByRole('button', { name: 'Read current deployment plan' })); await screen.findByRole('alert'); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
  vi.mocked(coordinatorDeploymentApi.targets).mockRejectedValue(new ApiError('conflict', 'Denied', 403));
  await userEvent.click(screen.getByRole('button', { name: 'Read connected deployment servers' }));
  expect((await screen.findByRole('alert')).textContent).toBe('Current access to deployment settings was refused.');
  expect(screen.queryByRole('option', { name: target.name })).toBeNull(); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});
it('keeps missing-target setup in Deploy and navigation never prepares or delegates', async () => {
  vi.mocked(coordinatorDeploymentApi.targets).mockResolvedValue([]); const input = props(); render(<CoordinatorDeploymentSetup {...input} />); await open();
  await userEvent.click(screen.getByRole('button', { name: 'Read connected deployment servers' })); await screen.findByText(/No deployment server connected/);
  await userEvent.click(screen.getByRole('button', { name: 'Open Deploy' })); expect(input.onNavigateDeploy).toHaveBeenCalledOnce(); expect(coordinatorDeploymentApi.prepare).not.toHaveBeenCalled();
});
