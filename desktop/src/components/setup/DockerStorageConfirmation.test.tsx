import { useLayoutEffect } from 'react';
import { act, cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DockerStorageConfirmation } from './DockerStorageConfirmation';
import { ApiError, type ProblemCode } from '../../api/client';
import type { ExecutionStorage, ExecutionStorageClient } from '../../api/executionStorage';
const authority = vi.hoisted(() => ({ role: 'super-admin', id: 'owner', team: '' }));
vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { role: authority.role, id: authority.id } } }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => ({ teamId: authority.team }) }));
const snapshot = (patch: Partial<ExecutionStorage> = {}): ExecutionStorage => ({ platform: 'windows', scope: 'local-docker-desktop', required: true, state: 'confirmation-required', revision: 3, reasonCode: 'confirmation_required', message: 'Confirm the active disk location before new work starts.', basis: 'Operator-confirmed local layout; measured mounted-volume minimum.', desktopVersion: '4.44.0', ...patch });
function client(value = snapshot()): ExecutionStorageClient {
  return { get: vi.fn().mockResolvedValue(value), confirm: vi.fn().mockResolvedValue(snapshot({ state: 'pending-observation', revision: 4, message: 'Waiting for measurement.' })) };
}
const fullPath = String.raw`D:\Docker\wsl\disk\docker_data.vhdx`;
async function review() {
  await userEvent.type(await screen.findByLabelText('Active Docker VHD file'), fullPath);
  await userEvent.click(screen.getByRole('checkbox', { name: /I checked that this is Docker Desktop/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Review storage location' }));
}
beforeEach(() => { vi.restoreAllMocks(); authority.role = 'super-admin'; authority.id = 'owner'; authority.team = ''; });
afterEach(() => { cleanup(); vi.useRealTimers(); });

it.each(['admin', 'member'])('does not read or reveal host storage paths to role %s', role => {
  authority.role = role; const api = client(); render(<DockerStorageConfirmation client={api} enabled />);
  expect(api.get).not.toHaveBeenCalled(); expect(screen.queryByRole('textbox')).toBeNull();
  expect(screen.getByText(/A Super Admin on this computer/)).toBeTruthy();
});
it('does not read privileged host paths in a team or disabled/remote context', () => {
  authority.team = 'team-selected'; const api = client(); const view = render(<DockerStorageConfirmation client={api} enabled />);
  expect(api.get).not.toHaveBeenCalled();
  authority.team = ''; view.rerender(<DockerStorageConfirmation client={api} enabled={false} />);
  expect(api.get).not.toHaveBeenCalled(); expect(screen.queryByRole('heading')).toBeNull();
});
it('separates guest space and physical minimum, preserves measured zero, and requires exact-path review before confirming', async () => {
  const api = client(snapshot({ hostAvailableBytes: 0, guestAvailableBytes: 8 * 1024 ** 3, mountedVolumeCount: 2 }));
  render(<DockerStorageConfirmation client={api} enabled />);
  await screen.findByText('0.0 GiB'); expect(screen.getByText('8.0 GiB')).toBeTruthy();
  expect(screen.getByText(/does not identify free space on only the VHD/)).toBeTruthy();
  expect(api.confirm).not.toHaveBeenCalled(); await review(); expect(api.confirm).not.toHaveBeenCalled();
  expect(screen.getByText(fullPath, { exact: true })).toBeTruthy();
  expect(screen.getByText(/Previously authorized queued work may start/)).toBeTruthy();
  await userEvent.click(screen.getByRole('button', { name: 'Confirm Docker storage location' }));
  await screen.findByText('Waiting for a fresh background measurement. Confirmation alone does not establish enough free space.');
  expect(api.confirm).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ revision: 3 }), fullPath);
  expect(screen.queryByRole('button', { name: 'Confirm Docker storage location' })).toBeNull();
});
it('shows unavailable observations without inventing zero or making a confirmation', async () => {
  const api = client(snapshot({ state: 'unavailable', reasonCode: 'engine_unavailable', message: 'Start Docker Desktop and refresh.' }));
  render(<DockerStorageConfirmation client={api} enabled />);
  await screen.findByText(/Start Docker Desktop and refresh/);
  expect(screen.getAllByText('Unavailable')).toHaveLength(2);
  expect(api.confirm).not.toHaveBeenCalled();
});
it('preserves an uncertain draft, requires fresh status, and never automatically repeats confirmation', async () => {
  const api = client(); vi.mocked(api.confirm).mockRejectedValue(new Error('Response interrupted.'));
  render(<DockerStorageConfirmation client={api} enabled />); await review();
  await userEvent.click(screen.getByRole('button', { name: 'Confirm Docker storage location' }));
  await screen.findByText(/service may already have accepted it/);
  expect((screen.getByLabelText('Active Docker VHD file') as HTMLInputElement).value).toBe(fullPath);
  expect((screen.getByRole('button', { name: 'Confirm Docker storage location' }) as HTMLButtonElement).disabled).toBe(true);
  expect(api.confirm).toHaveBeenCalledOnce();
  vi.mocked(api.get).mockResolvedValue(snapshot({ revision: 8, state: 'invalidated', reasonCode: 'desktop_changed', message: 'Docker Desktop changed; confirm its current location.' }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh Docker storage status' }));
  await screen.findByText(/Docker Desktop changed; confirm/);
  expect(screen.queryByRole('button', { name: 'Confirm Docker storage location' })).toBeNull();
  expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(false); expect(api.confirm).toHaveBeenCalledOnce();
});
it('fences a pending confirmation across access revocation and regrant, clearing private path and old consent', async () => {
  let resolve!: (value: ExecutionStorage) => void;
  const api = client(); vi.mocked(api.confirm).mockImplementation(() => new Promise(done => { resolve = done; }));
  const changed = vi.fn(), view = render(<DockerStorageConfirmation client={api} enabled onChanged={changed} />);
  await review(); await userEvent.click(screen.getByRole('button', { name: 'Confirm Docker storage location' }));
  authority.role = 'member'; view.rerender(<DockerStorageConfirmation client={api} enabled onChanged={changed} />);
  expect(screen.queryByRole('textbox')).toBeNull(); expect(screen.queryByText(fullPath)).toBeNull();
  authority.role = 'super-admin'; view.rerender(<DockerStorageConfirmation client={api} enabled onChanged={changed} />);
  await screen.findByLabelText('Active Docker VHD file');
  await act(async () => resolve(snapshot({ state: 'confirmed', vhdPath: fullPath, message: 'Stale accepted reply.' })));
  expect(screen.queryByText(/Stale accepted reply/)).toBeNull();
  expect((screen.getByLabelText('Active Docker VHD file') as HTMLInputElement).value).toBe('');
  expect(screen.queryByRole('button', { name: 'Confirm Docker storage location' })).toBeNull();
  expect(changed).not.toHaveBeenCalled(); expect(api.confirm).toHaveBeenCalledOnce();
});
it('aborts abandoned status reads and ignores their private result after the client changes', async () => {
  let resolve!: (value: ExecutionStorage) => void;
  const old = client(); vi.mocked(old.get).mockImplementation(() => new Promise(done => { resolve = done; }));
  const next = client(snapshot({ message: 'Current host.' }));
  const view = render(<DockerStorageConfirmation client={old} enabled />);
  const signal = vi.mocked(old.get).mock.calls[0][0];
  view.rerender(<DockerStorageConfirmation client={next} enabled />);
  await screen.findByText(/Current host/); expect(signal?.aborted).toBe(true);
  await act(async () => resolve(snapshot({ vhdPath: fullPath, message: 'Old host private result.' })));
  expect(screen.queryByText(/Old host private result/)).toBeNull(); expect(screen.queryByText(fullPath)).toBeNull();
});
it('polls a pending observation without replaying confirmation and announces the fresh result', async () => {
  vi.useFakeTimers();
  const api = client(snapshot({ state: 'pending-observation', revision: 4, message: 'Pending sample.' })), changed = vi.fn();
  render(<DockerStorageConfirmation client={api} enabled onChanged={changed} />);
  await act(async () => {});
  vi.mocked(api.get).mockResolvedValue(snapshot({ state: 'confirmed', revision: 4, message: 'Current storage measured.', hostAvailableBytes: 5 * 1024 ** 3 }));
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  expect(screen.getByText(/Current storage measured/)).toBeTruthy();
  expect(api.confirm).not.toHaveBeenCalled(); expect(changed).toHaveBeenCalledOnce();
});

it('clears private paths after a fresh HTTP403 even if the cached account still says Super Admin', async () => {
  const api = client(snapshot({ vhdPath: fullPath }));
  render(<DockerStorageConfirmation client={api} enabled />); await review();
  vi.mocked(api.confirm).mockRejectedValue(new ApiError('permission_denied' as ProblemCode, 'Host authority was revoked.', 403));
  await userEvent.click(screen.getByRole('button', { name: 'Confirm Docker storage location' }));
  await screen.findByText('Host authority was revoked.');
  expect(screen.queryByText(fullPath)).toBeNull(); expect(screen.queryByRole('textbox')).toBeNull();
  expect(api.confirm).toHaveBeenCalledOnce();
  vi.mocked(api.get).mockResolvedValue(snapshot());
  await userEvent.click(screen.getByRole('button', { name: 'Refresh Docker storage status' }));
  await screen.findByLabelText('Active Docker VHD file');
  expect((screen.getByLabelText('Active Docker VHD file') as HTMLInputElement).value).toBe('');
  expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(false);
});

it('never paints a previous Super Admin’s private location when identity changes before passive effects', async () => {
  const api = client(snapshot({ vhdPath: fullPath })), observed: string[] = [];
  function Observe({ identity }: { identity: string }) {
    useLayoutEffect(() => { observed.push(document.body.textContent || ''); }, [identity]);
    return <DockerStorageConfirmation client={api} enabled />;
  }
  const view = render(<Observe identity="owner" />);
  await screen.findByText(fullPath, { exact: true });
  vi.mocked(api.get).mockImplementation(() => new Promise(() => {}));
  authority.id = 'other-host-owner'; view.rerender(<Observe identity="other-host-owner" />);
  expect(observed[observed.length - 1]).not.toContain(fullPath);
  expect(screen.queryByText(fullPath, { exact: true })).toBeNull();
});

it('refreshes an unchanged cached observation without invalidating unrelated readiness', async () => {
  const api=client(snapshot({state:'confirmed'})),changed=vi.fn();
  render(<DockerStorageConfirmation client={api} enabled onChanged={changed}/>);
  await screen.findByRole('button',{name:'Review or change storage location'});
  await userEvent.click(screen.getByRole('button',{name:'Refresh Docker storage status'}));
  expect(api.get).toHaveBeenCalledTimes(2);expect(changed).not.toHaveBeenCalled();
  vi.mocked(api.get).mockResolvedValue(snapshot({revision:4,state:'invalidated',message:'Changed confirmed layout.'}));
  await userEvent.click(screen.getByRole('button',{name:'Refresh Docker storage status'}));
  await screen.findByText(/Changed confirmed layout/);
  expect(changed).toHaveBeenCalledOnce();expect(api.confirm).not.toHaveBeenCalled();
});
