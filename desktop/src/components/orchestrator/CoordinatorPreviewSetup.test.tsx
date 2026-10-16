import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { productApi } from '../../api/product';
import { CoordinatorPreviewSetup } from './CoordinatorPreviewSetup';
import { resetCockpit } from '../../store/store';
const props = () => ({ projectId: 'p-1', scope: 'alice/p-1', disabled: false, enabled: true, onChange: vi.fn() });
async function open() { await userEvent.click(screen.getByText(/Preview after independent checks/)); await screen.findByLabelText('Preview Dockerfile'); }
async function delegate() { await userEvent.click(screen.getByRole('button', { name: 'Read preview configuration' })); await screen.findByText(/configuration revision 3/); await userEvent.click(screen.getByRole('checkbox', { name: /I delegate this exact Dockerfile/ })); }
beforeEach(() => { vi.restoreAllMocks(); resetCockpit(); localStorage.clear(); vi.spyOn(productApi, 'request').mockResolvedValue({ revision: 3, variables: [], updatedAt: '' }); });
it('reads only metadata on explicit request, binds revision and requires exact preview consent', async () => {
  const input = props(); render(<CoordinatorPreviewSetup {...input} />); expect(productApi.request).not.toHaveBeenCalled(); expect(screen.queryByLabelText('Preview Dockerfile')).toBeNull(); await open();
  expect(input.onChange.mock.lastCall?.[0].plan).toBeNull(); await delegate();
  expect(input.onChange.mock.lastCall?.[0].plan).toEqual({ dockerfile: 'Dockerfile', port: 3000, expectedConfigurationRevision: 3, acknowledgePreviewBuild: true });
  expect(productApi.request).toHaveBeenCalledTimes(1); expect(vi.mocked(productApi.request).mock.calls[0].slice(0, 4)).toEqual(['/v1/product/projects/p-1/configuration', undefined, 'GET', 15000]);
  fireEvent.change(screen.getByLabelText(/^Preview app port/), { target: { value: '4000' } }); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});
it('recovers only scoped build options, never consent or the old configuration revision', async () => {
  const input = props(), ui = render(<CoordinatorPreviewSetup {...input} />); await open(); await delegate(); ui.unmount();
  render(<CoordinatorPreviewSetup {...input} />); await open(); await userEvent.click(screen.getByRole('button', { name: 'Restore draft' }));
  expect(screen.getByRole('checkbox', { name: /I delegate this exact Dockerfile/ })).toHaveProperty('checked', false); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull(); expect(screen.queryByText(/configuration revision 3/)).toBeNull();
});
it('does not publish a held configuration response after authority is lost and regained', async () => {
  let resolve!: (value: unknown) => void; vi.mocked(productApi.request).mockImplementation(() => new Promise(value => { resolve = value; }));
  const input = props(), ui = render(<CoordinatorPreviewSetup {...input} />); await open(); await userEvent.click(screen.getByRole('button', { name: 'Read preview configuration' }));
  const signal = vi.mocked(productApi.request).mock.calls[0][4] as AbortSignal; ui.rerender(<CoordinatorPreviewSetup {...input} disabled />); ui.rerender(<CoordinatorPreviewSetup {...input} />);
  expect(signal.aborted).toBe(true); await act(async () => resolve({ revision: 3, variables: [], updatedAt: '' })); expect(screen.queryByText(/configuration revision 3/)).toBeNull(); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});
it.each(['missing revision', 'invalid port', 'escaping Dockerfile'])('does not delegate %s', async fault => {
  const input = props(); if (fault === 'missing revision') vi.mocked(productApi.request).mockResolvedValue({ variables: [] });
  render(<CoordinatorPreviewSetup {...input} />); await open(); await userEvent.click(screen.getByRole('button', { name: 'Read preview configuration' }));
  if (fault === 'missing revision') await screen.findByRole('alert'); else { await screen.findByText(/configuration revision 3/); fireEvent.change(screen.getByLabelText(fault === 'invalid port' ? /^Preview app port/ : 'Preview Dockerfile'), { target: { value: fault === 'invalid port' ? '65536' : '../Dockerfile' } }); }
  await waitFor(() => expect(screen.getByRole('checkbox', { name: /I delegate this exact Dockerfile/ })).toHaveProperty('disabled', true)); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});
it('allows abandoning a pristine failed optional preview read after preview permission is paused', async () => {
  vi.mocked(productApi.request).mockRejectedValue(new Error('Configuration unavailable'));
  const input = props(), ui = render(<CoordinatorPreviewSetup {...input} />); await open(); await userEvent.click(screen.getByRole('button', { name: 'Read preview configuration' })); await screen.findByRole('alert');
  ui.rerender(<CoordinatorPreviewSetup {...input} enabled={false} />);
  await userEvent.click(screen.getByRole('button', { name: 'Discard continued-work preview setup' }));
  await waitFor(() => expect(input.onChange.mock.lastCall?.[0]).toMatchObject({ plan: null, dirty: false, problem: '' }));
  expect(productApi.request).toHaveBeenCalledTimes(1); expect(screen.queryByRole('alert')).toBeNull();
});
it('delegates reviewed-source only with an explicit kind, empty Dockerfile and empty runtime configuration', async () => {
  const input = props(); render(<CoordinatorPreviewSetup {...input} mode="reviewed-source" />);
  await userEvent.click(screen.getByText(/Preview after independent checks/));
  expect(screen.queryByLabelText('Preview Dockerfile')).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Read preview configuration' }));
  await screen.findByText(/configuration revision 3/);
  await userEvent.click(screen.getByRole('checkbox', { name: /I delegate checked static-source serving/ }));
  expect(input.onChange.mock.lastCall?.[0].plan).toEqual({ kind: 'reviewed-source', dockerfile: '', port: 3000, expectedConfigurationRevision: 3, acknowledgePreviewBuild: true });
});
it('refuses reviewed-source delegation while any runtime configuration value exists', async () => {
  vi.mocked(productApi.request).mockResolvedValue({ revision: 3, variables: [{ name: 'PRIVATE_VALUE' }], updatedAt: '' });
  const input = props(); render(<CoordinatorPreviewSetup {...input} mode="reviewed-source" />);
  await userEvent.click(screen.getByText(/Preview after independent checks/));
  await userEvent.click(screen.getByRole('button', { name: 'Read preview configuration' }));
  await screen.findByRole('alert');
  expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});
