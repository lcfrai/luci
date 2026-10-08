import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import type { RunOperation } from '../../api/projectRuns';
import { swarmWorkspaceApi, type SwarmWorkspace, type SwarmWorkspaceFile } from '../../api/swarmWorkspace';
import { SwarmWorkspaceReview } from './SwarmWorkspaceReview';

const access = vi.hoisted(() => ({scope: 'account-A/workspace-A', canRead: true, loading: false, error: ''}));
vi.mock('../../hooks/useProjectDraft', () => ({useProjectDraftScope: () => access.scope}));
vi.mock('../../hooks/useProjectAccess', () => ({useProjectAccess: () => ({...access, message: access.loading ? 'Checking project permissions…' : 'Reading permission unavailable.'})}));
const operation: RunOperation = {id: 'op-a', taskId: 'task-a', worktreeId: 'isolated-a', childId: 'missing-child', kind: 'session', status: 'cancelled', allocation: 'abandoned', attempt: 2, requestDigest: '', createdAt: '', updatedAt: ''};
const workspace = (patch: Partial<SwarmWorkspace> = {}): SwarmWorkspace => ({operationId: operation.id, taskId: operation.taskId!, worktreeId: operation.worktreeId!, exists: true, files: [{path: 'src/added.ts', status: '??'}, {path: 'src/other.ts', status: ' M'}], filesOmitted: 0, filesTruncated: false, patch: '+const changed = true;', patchTruncated: false, reason: '', ...patch});
const file = (path: string, patch: Partial<SwarmWorkspaceFile> = {}): SwarmWorkspaceFile => ({operationId: operation.id, taskId: operation.taskId!, worktreeId: operation.worktreeId!, path, bytes: 20, content: `Retained ${path}`, binary: false, omitted: false, reason: '', ...patch});
const props = {projectId: 'p-1', runId: 'run-1', operation, canRead: true, onClose: vi.fn()};
function pending<T>() { let resolve!: (value: T) => void; return {promise: new Promise<T>(done => {resolve = done;}), resolve}; }
beforeEach(() => {
  vi.restoreAllMocks(); Object.assign(access, {scope: 'account-A/workspace-A', canRead: true, loading: false, error: ''});
  vi.spyOn(swarmWorkspaceApi, 'inspect').mockResolvedValue(workspace());
  vi.spyOn(swarmWorkspaceApi, 'file').mockImplementation(async (_p, _r, _op, path) => file(path));
});

it('inspects an abandoned attempt without a session and reads only deliberately selected files as inert text', async () => {
  const hostile = '<img src="https://invalid.example/secret" onerror="alert(1)">';
  vi.mocked(swarmWorkspaceApi.file).mockResolvedValue(file('src/added.ts', {content: hostile}));
  render(<SwarmWorkspaceReview {...props} />);
  const selector = await screen.findByLabelText('File to inspect');
  expect(swarmWorkspaceApi.file).not.toHaveBeenCalled();
  expect(screen.getByText(/Attempt 2/)).not.toBeNull();
  await userEvent.selectOptions(selector, 'src/added.ts');
  expect(await screen.findByText(hostile)).not.toBeNull();
  expect(screen.getByRole('dialog').querySelector('img, iframe, [contenteditable], textarea')).toBeNull();
  expect(swarmWorkspaceApi.file).toHaveBeenCalledExactlyOnceWith('p-1', 'run-1', operation, 'src/added.ts', expect.any(AbortSignal));
  expect(screen.queryByRole('button', {name: /Start|Integrate|Save|Retry task/})).toBeNull();
});

it('keeps lower-bound omitted counts, partial text, binary files and a truncated patch explicit', async () => {
  vi.mocked(swarmWorkspaceApi.inspect).mockResolvedValue(workspace({filesOmitted: 4, filesTruncated: true, patchTruncated: true}));
  vi.mocked(swarmWorkspaceApi.file).mockResolvedValueOnce(file('src/added.ts', {bytes: 300_000, omitted: true, content: 'Prefix only'})).mockResolvedValueOnce(file('src/other.ts', {binary: true, omitted: true, content: ''}));
  render(<SwarmWorkspaceReview {...props} />);
  await screen.findByText(/At least 4 additional files/);
  await userEvent.selectOptions(screen.getByLabelText('File to inspect'), 'src/added.ts');
  await screen.findByText(/Partial file preview/);
  expect(screen.getByLabelText('Partial retained file text').textContent).toBe('Prefix only');
  await userEvent.selectOptions(screen.getByLabelText('File to inspect'), 'src/other.ts');
  await screen.findByText('Binary file. A text preview is unavailable.');
  expect(screen.queryByText('Prefix only')).toBeNull();
  await userEvent.click(screen.getByText('Tracked changes preview'));
  expect(screen.getByText(/changes preview is truncated/)).not.toBeNull();
  expect(screen.getByLabelText('Retained tracked changes').textContent).toBe('+const changed = true;');
});

it('does not let a slow previous file replace the newer selection, and clearing then selecting reads it again', async () => {
  const delayed = pending<SwarmWorkspaceFile>(); vi.mocked(swarmWorkspaceApi.file).mockReturnValueOnce(delayed.promise);
  render(<SwarmWorkspaceReview {...props} />);
  const selector = await screen.findByLabelText('File to inspect');
  await userEvent.selectOptions(selector, 'src/added.ts');
  await userEvent.selectOptions(selector, 'src/other.ts');
  await screen.findByText('Retained src/other.ts');
  await act(async () => delayed.resolve(file('src/added.ts', {content: 'Old private file'})));
  expect(screen.queryByText('Old private file')).toBeNull();
  await userEvent.selectOptions(selector, '');
  await userEvent.selectOptions(selector, 'src/other.ts');
  await screen.findByText('Retained src/other.ts');
  expect(swarmWorkspaceApi.file).toHaveBeenCalledTimes(3);
});

it.each(['scope', 'permission', 'parent permission', 'unmount'] as const)('aborts and fences a pending private file after %s changes', async change => {
  const delayed = pending<SwarmWorkspaceFile>(); vi.mocked(swarmWorkspaceApi.file).mockReturnValueOnce(delayed.promise);
  const view = render(<SwarmWorkspaceReview {...props} />);
  await userEvent.selectOptions(await screen.findByLabelText('File to inspect'), 'src/added.ts');
  const signal = vi.mocked(swarmWorkspaceApi.file).mock.calls[0][4]!;
  if (change === 'scope') access.scope = 'other-account';
  if (change === 'permission') access.canRead = false;
  if (change === 'unmount') view.unmount(); else view.rerender(<SwarmWorkspaceReview {...props} canRead={change !== 'parent permission'} />);
  expect(signal.aborted).toBe(true);
  await act(async () => delayed.resolve(file('src/added.ts', {content: 'Private previous workspace'})));
  expect(screen.queryByText('Private previous workspace')).toBeNull();
  expect(screen.queryByRole('region', {name: 'Retained files'})).toBeNull();
});

it('does not revive an old inventory response after reading permission is revoked and restored', async () => {
  const delayed = pending<SwarmWorkspace>(); vi.mocked(swarmWorkspaceApi.inspect).mockReturnValueOnce(delayed.promise);
  const view = render(<SwarmWorkspaceReview {...props} />);
  await waitFor(() => expect(swarmWorkspaceApi.inspect).toHaveBeenCalledOnce());
  access.canRead = false; view.rerender(<SwarmWorkspaceReview {...props} />);
  access.canRead = true; view.rerender(<SwarmWorkspaceReview {...props} />);
  await screen.findByLabelText('File to inspect');
  await act(async () => delayed.resolve(workspace({reason: 'Old denied content'})));
  expect(screen.queryByText('Old denied content')).toBeNull();
  expect(swarmWorkspaceApi.inspect).toHaveBeenCalledTimes(2);
});

it.each(['project', 'run', 'operation'] as const)('does not display a prior attempt file when the %s changes', async change => {
  const delayed = pending<SwarmWorkspaceFile>(); vi.mocked(swarmWorkspaceApi.file).mockReturnValueOnce(delayed.promise);
  const view = render(<SwarmWorkspaceReview {...props} />);
  await userEvent.selectOptions(await screen.findByLabelText('File to inspect'), 'src/added.ts');
  const signal = vi.mocked(swarmWorkspaceApi.file).mock.calls[0][4]!;
  const next = {...props, projectId: change === 'project' ? 'p-2' : props.projectId, runId: change === 'run' ? 'run-2' : props.runId,
    operation: change === 'operation' ? {...operation, id: 'op-b', taskId: 'task-b', worktreeId: 'isolated-b'} : operation};
  vi.mocked(swarmWorkspaceApi.inspect).mockResolvedValue(workspace({operationId: next.operation.id, taskId: next.operation.taskId!, worktreeId: next.operation.worktreeId!}));
  view.rerender(<SwarmWorkspaceReview {...next} />);
  expect(signal.aborted).toBe(true);
  await screen.findByLabelText('File to inspect');
  await act(async () => delayed.resolve(file('src/added.ts', {content: 'Private previous attempt'})));
  expect(screen.queryByText('Private previous attempt')).toBeNull();
  expect(screen.getByLabelText('File to inspect')).toHaveProperty('value', '');
  expect(swarmWorkspaceApi.inspect).toHaveBeenLastCalledWith(next.projectId, next.runId, next.operation, expect.any(AbortSignal));
});

it('requires a known scope and read authority before the first inspection', async () => {
  access.canRead = false;
  const view = render(<SwarmWorkspaceReview {...props} />);
  expect(swarmWorkspaceApi.inspect).not.toHaveBeenCalled();
  expect(screen.getByRole('status').textContent).toContain('Reading access is unavailable');
  view.unmount(); access.canRead = true; access.scope = '';
  render(<SwarmWorkspaceReview {...props} />);
  expect(swarmWorkspaceApi.inspect).not.toHaveBeenCalled();
});

it('preserves a completed preview during a permission recheck without starting new reads', async () => {
  const view = render(<SwarmWorkspaceReview {...props} />);
  await userEvent.selectOptions(await screen.findByLabelText('File to inspect'), 'src/added.ts');
  const content = await screen.findByText('Retained src/added.ts');
  access.loading = true; view.rerender(<SwarmWorkspaceReview {...props} />);
  expect(screen.getByText('Retained src/added.ts')).toBe(content);
  expect(screen.getByLabelText('File to inspect')).toHaveProperty('disabled', true);
  access.loading = false; view.rerender(<SwarmWorkspaceReview {...props} />);
  await waitFor(() => expect(screen.getByLabelText('File to inspect')).toHaveProperty('disabled', false));
  expect(swarmWorkspaceApi.inspect).toHaveBeenCalledOnce(); expect(swarmWorkspaceApi.file).toHaveBeenCalledOnce();
});

it('keeps missing output honest, retries read failures deliberately, and refresh clears a prior file selection', async () => {
  vi.mocked(swarmWorkspaceApi.inspect).mockRejectedValueOnce(new Error('The retained workspace could not be inspected.')).mockResolvedValueOnce(workspace({exists: false, files: [], reason: 'This attempt has no retained workspace.'})).mockResolvedValue(workspace());
  render(<SwarmWorkspaceReview {...props} />);
  await screen.findByRole('alert');
  await userEvent.click(screen.getByRole('button', {name: 'Refresh retained files'}));
  await screen.findByText('This attempt has no retained workspace.');
  expect(screen.queryByLabelText('File to inspect')).toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Refresh retained files'}));
  vi.mocked(swarmWorkspaceApi.file).mockRejectedValueOnce(new Error('The file changed while reading.'));
  await userEvent.selectOptions(await screen.findByLabelText('File to inspect'), 'src/added.ts');
  await screen.findByText('The file changed while reading.');
  await userEvent.click(screen.getByRole('button', {name: 'Retry file read'}));
  await screen.findByText('Retained src/added.ts');
  await userEvent.click(screen.getByRole('button', {name: 'Refresh retained files'}));
  await screen.findByLabelText('File to inspect');
  expect(screen.queryByText('Retained src/added.ts')).toBeNull();
  expect(swarmWorkspaceApi.file).toHaveBeenCalledTimes(2);
});
