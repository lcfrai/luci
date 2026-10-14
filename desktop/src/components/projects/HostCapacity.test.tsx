import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { HostCapacity } from './HostCapacity';
import { swarmSchedulerApi, type SwarmScheduler } from '../../api/swarmScheduler';
const access = vi.hoisted(() => ({ role: 'admin', team: '' }));
vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { role: access.role } } }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => ({ teamId: access.team }) }));
const snapshot = (): SwarmScheduler => ({ revision: 7, observedAt: '', dispatchPaused: false, policy: { maxActive: 4, maxPerProject: 2, headroomPercent: 25, minHeadroomMiB: 2048, diskReserveMiB: 2048 }, resources: null });
beforeEach(() => { vi.restoreAllMocks(); access.role = 'admin'; access.team = ''; vi.spyOn(swarmSchedulerApi, 'get').mockResolvedValue(snapshot()); vi.spyOn(swarmSchedulerApi, 'configure').mockResolvedValue({ ...snapshot(), revision: 8, dispatchPaused: true }); });
afterEach(() => { cleanup(); vi.useRealTimers(); });

it('keeps legacy settings until a reviewed responsive profile is saved', async () => {
  const user = userEvent.setup(); render(<HostCapacity />); await expandCapacity();
  await user.click(screen.getByRole('button', { name: 'Scheduling settings' }));
  expect((screen.getByRole('combobox', { name: 'Memory reserve profile' }) as HTMLSelectElement).value).toBe('percentage');
  expect(swarmSchedulerApi.configure).not.toHaveBeenCalled();
  await user.selectOptions(screen.getByRole('combobox', { name: 'Memory reserve profile' }), 'responsive');
  expect((screen.getByRole('spinbutton', { name: 'Minimum RAM reserve (MiB)' }) as HTMLInputElement).value).toBe('512');
  expect((screen.getByRole('spinbutton', { name: 'Maximum percentage RAM reserve (MiB)' }) as HTMLInputElement).value).toBe('1024');
  await user.click(screen.getByRole('button', { name: 'Save scheduling ceilings' }));
  expect(swarmSchedulerApi.configure).toHaveBeenCalledWith(expect.objectContaining({ revision: 7 }), { ...snapshot().policy, minHeadroomMiB: 512, maxHeadroomMiB: 1024 }, false);
});

it('reports enforced worker and independent build limits with effective reserves', async () => {
  const now = new Date().toISOString();
  vi.mocked(swarmSchedulerApi.get).mockResolvedValue({ ...snapshot(), resources: { limits: { worker: { memoryMiB: 512, cpuMillis: 500 }, runtimeBuild: { memoryMiB: 1024, cpuMillis: 1000 } }, memoryReserve: { hostBytes: 1024 ** 3, executionBytes: 1024 ** 3 }, host: { observedAt: now, status: 'ready', platform: 'windows', scope: 'host', memory: { effectiveTotalBytes: 32 * 1024 ** 3, effectiveAvailableBytes: 3 * 1024 ** 3, limitSource: 'physical' }, cpu: { logicalProcessors: 8, effectiveMillis: 8000, utilizationPercent: 10 }, disks: [], unavailable: [] }, execution: { observedAt: now, status: 'ready', scope: 'docker', totalMemoryBytes: 8 * 1024 ** 3, cpuMillis: 8000, unavailable: [] } } });
  render(<HostCapacity />); await expandCapacity();
  expect(screen.getByText(/Actual new agent allocation: 512 MiB · 0.5 CPU cores/)).toBeTruthy();
  expect(screen.getByText(/One-time runtime build: 1024 MiB · 1 CPU cores/)).toBeTruthy();
  expect(screen.getByText(/Effective RAM reserve: 1.0 GiB on this computer/)).toBeTruthy();
  expect(screen.getByText(/Waiting work stays queued/)).toBeTruthy();
});
async function expandCapacity() {
  await userEvent.click(await screen.findByRole('button', { name: 'Expand capacity and settings' }));
}
it('does not read global resources for a member or team-scoped administrator', () => {
  access.role = 'member'; const view = render(<HostCapacity />); expect(swarmSchedulerApi.get).not.toHaveBeenCalled();
  access.role = 'admin'; access.team = 'team-private'; view.rerender(<HostCapacity />); expect(swarmSchedulerApi.get).not.toHaveBeenCalled();
});
it('shows scheduling ceilings and unknown execution capacity without promising four running agents', async () => {
  render(<HostCapacity />); await expandCapacity(); await screen.findByText('Workspace ceiling: 4 agents · up to 2 per project');
  expect(screen.getByText(/Runtime capacity unknown/)).toBeTruthy();
  expect(screen.getByText(/Actual concurrency also depends on account and provider limits/)).toBeTruthy();
});
it('reviews workspace-wide impact before pausing future starts with the policy revision', async () => {
  const user = userEvent.setup(); render(<HostCapacity />);
  await expandCapacity();
  await user.click(await screen.findByRole('button', { name: 'Pause new starts' }));
  expect(screen.getByRole('dialog', { name: 'Pause new queued starts?' })).toBeTruthy();
  expect(screen.getByText(/Agents that have already started keep running/)).toBeTruthy();
  expect(swarmSchedulerApi.configure).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Pause future starts' }));
  await screen.findByRole('button', { name: 'Resume new starts' });
  expect(swarmSchedulerApi.configure).toHaveBeenCalledWith(expect.objectContaining({ revision: 7 }), snapshot().policy, true);
});
it('retains failed setting edits, blocks replay, and explicitly refreshes the compare-and-swap basis', async () => {
  vi.mocked(swarmSchedulerApi.configure).mockRejectedValueOnce(new Error('Connection interrupted'));
  const user = userEvent.setup(); render(<HostCapacity />);
  await expandCapacity();
  await user.click(await screen.findByRole('button', { name: 'Scheduling settings' }));
  const ceiling = screen.getByRole('spinbutton', { name: 'Workspace agent ceiling' });
  await user.clear(ceiling); await user.type(ceiling, '6');
  await user.click(screen.getByRole('button', { name: 'Save scheduling ceilings' }));
  await screen.findByText(/This change could not be confirmed/);
  expect((ceiling as HTMLInputElement).value).toBe('6');
  expect(screen.getByRole('button', { name: 'Save scheduling ceilings' })).toHaveProperty('disabled', true);
  vi.mocked(swarmSchedulerApi.get).mockResolvedValue({ ...snapshot(), revision: 11 });
  await user.click(screen.getByRole('button', { name: 'Refresh and review current settings' }));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Save scheduling ceilings' })).toHaveProperty('disabled', false));
  expect((ceiling as HTMLInputElement).value).toBe('6');
  await user.click(screen.getByRole('button', { name: 'Save scheduling ceilings' }));
  expect(swarmSchedulerApi.configure).toHaveBeenLastCalledWith(expect.objectContaining({ revision: 11 }), { ...snapshot().policy, maxActive: 6 }, false);
});
it('removes private capacity and closes pending settings when permission is revoked', async () => {
  let resolve!: (value: SwarmScheduler) => void;
  vi.mocked(swarmSchedulerApi.configure).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const user = userEvent.setup(), view = render(<HostCapacity />);
  await expandCapacity();
  await user.click(await screen.findByRole('button', { name: 'Pause new starts' })); await user.click(screen.getByRole('button', { name: 'Pause future starts' }));
  access.role = 'member'; view.rerender(<HostCapacity />);
  expect(screen.queryByRole('region', { name: 'Workspace capacity' })).toBeNull();
  await act(async () => resolve({ ...snapshot(), dispatchPaused: true }));
  expect(screen.queryByRole('dialog')).toBeNull();
});
it('does not revive a late write after administrator access is revoked and granted again', async () => {
  let resolve!: (value: SwarmScheduler) => void;
  vi.mocked(swarmSchedulerApi.configure).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const user = userEvent.setup(), view = render(<HostCapacity />);
  await expandCapacity();
  await user.click(await screen.findByRole('button', { name: 'Pause new starts' })); await user.click(screen.getByRole('button', { name: 'Pause future starts' }));
  access.role = 'member'; view.rerender(<HostCapacity />);
  access.role = 'admin'; view.rerender(<HostCapacity />);
  await expandCapacity();
  await screen.findByRole('button', { name: 'Pause new starts' });
  await act(async () => resolve({ ...snapshot(), dispatchPaused: true, revision: 9 }));
  expect(screen.queryByRole('button', { name: 'Resume new starts' })).toBeNull();
  expect(screen.getByRole('button', { name: 'Pause new starts' })).toHaveProperty('disabled', true);
  await user.click(screen.getByRole('button', { name: 'Refresh workspace capacity' }));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Pause new starts' })).toHaveProperty('disabled', false));
  expect(swarmSchedulerApi.configure).toHaveBeenCalledTimes(1);
});

it('keeps scheduling controls out of the compact summary and expands without changing policy', async () => {
  const user = userEvent.setup(); render(<HostCapacity />);
  const disclosure = await screen.findByRole('button', { name: 'Expand capacity and settings' });
  expect(disclosure.getAttribute('aria-expanded')).toBe('false');
  expect(screen.queryByRole('button', { name: 'Scheduling settings' })).toBeNull();
  expect(screen.queryByText(/Workspace ceiling:/)).toBeNull();
  await user.click(disclosure);
  expect(screen.getByRole('button', { name: 'Hide capacity and settings' }).getAttribute('aria-expanded')).toBe('true');
  expect(screen.getByRole('button', { name: 'Scheduling settings' })).toBeTruthy();
  expect(swarmSchedulerApi.configure).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Hide capacity and settings' }));
  expect(screen.queryByRole('button', { name: 'Scheduling settings' })).toBeNull();
});

const reservationTime = Date.parse('2026-10-08T22:00:00Z');
const reservations = (age = 0) => ({ status: 'ready' as const, observedAt: new Date(reservationTime - age).toISOString(), count: 2, memoryMiB: 3072, cpuMillis: 1500 });
it('shows fresh machine allocations as workers and checks, including a verified zero', async () => {
  vi.spyOn(Date, 'now').mockReturnValue(reservationTime);
  vi.mocked(swarmSchedulerApi.get).mockResolvedValue({ ...snapshot(), resources: { reservations: reservations(29_999) } });
  const view = render(<HostCapacity />); await expandCapacity();
  expect(screen.getByText(/Reserved execution demand: 2 allocations · 3.0 GiB · 1.5 CPU cores/)).toBeTruthy();
  expect(screen.getByText(/includes workers and checks across this machine/)).toBeTruthy();
  view.unmount();
  vi.mocked(swarmSchedulerApi.get).mockResolvedValue({ ...snapshot(), resources: { reservations: { ...reservations(), count: 0, memoryMiB: 0, cpuMillis: 0 } } });
  render(<HostCapacity />); await expandCapacity();
  expect(screen.getByText(/Reserved execution demand: 0 allocations · 0.0 GiB · 0.0 CPU cores/)).toBeTruthy();
});

it.each([
  ['absent', undefined], ['null', null], ['unavailable', { status: 'unavailable' }],
  ['legacy zero', { observedAt: '2026-10-08T22:00:00Z', count: 0, memoryMiB: 0, cpuMillis: 0 }],
  ['stale', reservations(30_000)], ['future', reservations(-1)],
  ['invalid time', { ...reservations(), observedAt: 'invalid' }],
])('shows reservations as unavailable for %s observations', async (_label, reservation) => {
  vi.spyOn(Date, 'now').mockReturnValue(reservationTime);
  vi.mocked(swarmSchedulerApi.get).mockResolvedValue({ ...snapshot(), resources: { reservations: reservation } } as SwarmScheduler);
  render(<HostCapacity />); await expandCapacity();
  expect(screen.getByText(/Reserved execution demand: Unavailable/)).toBeTruthy();
  expect(screen.queryByText(/\d+ allocations ·/)).toBeNull();
});

it('expires a visible reservation while its next read remains pending', async () => {
  vi.useFakeTimers(); vi.setSystemTime(reservationTime);
  vi.mocked(swarmSchedulerApi.get).mockResolvedValueOnce({ ...snapshot(), resources: { reservations: reservations() } }).mockImplementation(() => new Promise(() => {}));
  render(<HostCapacity />);
  await act(async () => {});
  fireEvent.click(screen.getByRole('button', { name: 'Expand capacity and settings' }));
  expect(screen.getByText(/Reserved execution demand: 2 allocations/)).toBeTruthy();
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(screen.getByText(/Reserved execution demand: Unavailable/)).toBeTruthy();
  expect(screen.queryByText(/\d+ allocations ·/)).toBeNull();
});

it('saves higher configured ceilings with unchanged resource policy and exact revision', async () => {
  const user = userEvent.setup(); render(<HostCapacity />); await expandCapacity();
  await user.click(screen.getByRole('button', {name: 'Scheduling settings'}));
  const global = screen.getByRole('spinbutton', {name: 'Workspace agent ceiling'});
  const project = screen.getByRole('spinbutton', {name: 'Agents per project'});
  expect(global.getAttribute('max')).toBe('500'); expect(project.getAttribute('max')).toBe('200');
  await user.clear(global); await user.type(global, '24');
  await user.clear(project); await user.type(project, '6');
  await user.click(screen.getByRole('button', {name: 'Save scheduling ceilings'}));
  expect(swarmSchedulerApi.configure).toHaveBeenCalledWith(expect.objectContaining({revision: 7}), {...snapshot().policy, maxActive: 24, maxPerProject: 6}, false);
});

it('rejects ceilings outside queue/task bounds and project ceilings above workspace capacity', async () => {
  const user = userEvent.setup(); render(<HostCapacity />); await expandCapacity();
  await user.click(screen.getByRole('button', {name: 'Scheduling settings'}));
  const global = screen.getByRole('spinbutton', {name: 'Workspace agent ceiling'});
  const project = screen.getByRole('spinbutton', {name: 'Agents per project'});
  for (const [g, p] of [['501', '6'], ['500', '201'], ['6', '7']]) {
    await user.clear(global); await user.type(global, g);
    await user.clear(project); await user.type(project, p);
    expect(screen.getByRole('button', {name: 'Save scheduling ceilings'})).toHaveProperty('disabled', true);
  }
  expect(swarmSchedulerApi.configure).not.toHaveBeenCalled();
});
