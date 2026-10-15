import { StrictMode } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { swarmSupervisionApi } from '../../api/swarmSupervision';
import type { RunOperation } from '../../api/projectRuns';
import { swarmRunFixture } from '../../test/swarmFixtures';
import { swarmMessageFixture, swarmSupervisionFixture } from '../../test/swarmSupervisionFixtures';
import { readMessageRecovery } from './swarmMessageRecovery';
import { SwarmSupervisionPanel } from './SwarmSupervisionView';
beforeEach(() => localStorage.clear()); afterEach(() => vi.restoreAllMocks());
function props() {
  const run = swarmRunFixture();
  run.operations = ['a', 'b'].map(id => ({id: `op-${id}`, taskId: id, kind: 'session', childId: `session-${id}`, worktreeId: `isolated-${id}`, status: 'running', allocation: 'ready', attempt: 1, requestDigest: 'request', createdAt: 'now', updatedAt: 'now'} as RunOperation));
  run.swarm!.tasks = run.swarm!.tasks.map(task => ({...task, operationId: task.id === 'c' ? undefined : `op-${task.id}`}));
  return {run, canRead: true, canEdit: true, paused: false, recoveryScope: 'actor/workspace/team/run', onOpenTask: vi.fn(), onWrite: vi.fn(async (action: () => Promise<void>) => action())};
}
it('mounts read-only without effects, treats claims and ack separately and ignores arbitrary action URLs', async () => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(swarmSupervisionFixture());
  const post = vi.spyOn(swarmSupervisionApi, 'post'), ack = vi.spyOn(swarmSupervisionApi, 'ack'), p = props();
  render(<StrictMode><SwarmSupervisionPanel {...p} canEdit={false} /></StrictMode>);
  await screen.findByText(swarmMessageFixture().summary);
  expect(screen.queryByRole('link')).toBeNull(); expect(screen.queryByText('Record an attributed message')).toBeNull();
  fireEvent.click(screen.getAllByRole('button', {name: 'Inspect task Build b (b)'})[0]); expect(p.onOpenTask).toHaveBeenCalledWith('b');
  expect(post).not.toHaveBeenCalled(); expect(ack).not.toHaveBeenCalled(); expect(p.onWrite).not.toHaveBeenCalled();
});
it('keeps activity/attention/quality/settlement and unknown metrics distinct, including observed zero', async () => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(swarmSupervisionFixture({activityPhase: 'future-state', metrics: {routineDecisions: 0, latency: null, quality: {}, throughput: null, cost: null}}));
  render(<SwarmSupervisionPanel {...props()} />);
  expect(await screen.findByText(/Activity: unknown \(future-state\).*Quality: needs-review.*Settlement: uncertain/)).toBeTruthy();
  expect(screen.getByText(/routineDecisions: 0 reported/)).toBeTruthy(); expect(screen.getByText(/latency: not measured/)).toBeTruthy();
  expect(screen.getByRole('heading', {name: 'Recorded attention (1)'})).toBeTruthy();
});
it('retains uncertain exact post request before sending and never automatically replays on remount', async () => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(swarmSupervisionFixture());
  const post = vi.spyOn(swarmSupervisionApi, 'post').mockRejectedValue(new Error('connection lost')), p = props();
  const page = render(<SwarmSupervisionPanel {...p} />); await screen.findByText(swarmMessageFixture().summary);
  fireEvent.click(screen.getByText('Record an attributed message'));
  fireEvent.change(screen.getByLabelText('Sender attempt'), {target: {value: 'op-a'}});
  fireEvent.change(screen.getByLabelText('Recipient task'), {target: {value: 'b'}});
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: 'Claim only'}});
  fireEvent.click(screen.getByRole('button', {name: 'Record message claim'}));
  await screen.findByText(/connection lost/); const retained = readMessageRecovery(p.recoveryScope);
  expect(retained?.type).toBe('post'); expect(post).toHaveBeenCalledOnce();
  page.unmount(); render(<SwarmSupervisionPanel {...p} />); await screen.findByText(swarmMessageFixture().summary);
  const disposition = vi.spyOn(swarmSupervisionApi, 'disposition').mockResolvedValue({projectId: p.run.projectId, runId: p.run.id, operationId: 'op-a', state: 'unconfirmed', runRevision: 1, message: null});
  expect(post).toHaveBeenCalledOnce(); fireEvent.click(screen.getByRole('button', {name: 'Check exact saved outcome'}));
  await waitFor(() => expect(disposition).toHaveBeenCalledOnce());
  expect(post).toHaveBeenCalledOnce(); expect(readMessageRecovery(p.recoveryScope)).toEqual(retained);
});
it('acknowledges only the exact saved recipient and reconciles uncertain delivery by read, without replay', async () => {
  const get = vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(swarmSupervisionFixture());
  const ack = vi.spyOn(swarmSupervisionApi, 'ack').mockRejectedValue(new Error('connection lost')), p = props();
  render(<SwarmSupervisionPanel {...p} />); await screen.findByText(swarmMessageFixture().summary);
  fireEvent.click(screen.getByRole('button', {name: 'Record delivery receipt for message 1'})); await screen.findByText(/connection lost/);
  expect(ack.mock.calls[0][3]).toEqual({expectedRevision: 1, recipientOperationId: 'op-b'});
  get.mockResolvedValueOnce(swarmSupervisionFixture({runRevision: 2, messages: [swarmMessageFixture({ack: {operationId: 'op-b', childId: 'session-b', actorId: 'reader', at: 'now'}})]}));
  fireEvent.click(screen.getByRole('button', {name: 'Refresh saved messages'}));
  await screen.findByText(/Delivery receipt confirmed in saved history/); expect(readMessageRecovery(p.recoveryScope)).toBeNull(); expect(ack).toHaveBeenCalledOnce();
});
it('reads an uncertain acknowledgement by exact ID outside the display window and never replays it', async () => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(swarmSupervisionFixture({messages: [], page: {after: 0, nextCursor: 0, hasMore: false, omitted: 0}}));
  const ack = vi.spyOn(swarmSupervisionApi, 'ack'), exact = vi.spyOn(swarmSupervisionApi, 'message').mockResolvedValue({projectId: 'p-1', runId: 'run-1', runRevision: 2,
    message: swarmMessageFixture({ack: {operationId: 'op-b', childId: 'session-b', actorId: 'reader', at: 'now'}})});
  const p = props(), pending = {type: 'ack' as const, messageId: 'msg-1', request: {expectedRevision: 1, recipientOperationId: 'op-b'}};
  localStorage.setItem(`luci.swarm-message-pending.v1:${encodeURIComponent(p.recoveryScope)}`, JSON.stringify(pending));
  render(<SwarmSupervisionPanel {...p} />);
  await screen.findByText(/Delivery for saved message msg-1/);
  fireEvent.click(screen.getByRole('button', {name: 'Check exact saved outcome'}));
  await waitFor(() => expect(readMessageRecovery(p.recoveryScope)).toBeNull());
  expect(exact).toHaveBeenCalledWith('p-1', 'run-1', 'msg-1', expect.any(AbortSignal));
  expect(ack).not.toHaveBeenCalled();
});
it('archives a definitively rejected exact claim before allowing new work', async () => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(swarmSupervisionFixture());
  const post = vi.spyOn(swarmSupervisionApi, 'post').mockRejectedValue(new Error('connection lost'));
  const p = props(); render(<SwarmSupervisionPanel {...p} />); await screen.findByText(swarmMessageFixture().summary);
  fireEvent.click(screen.getByText('Record an attributed message'));
  fireEvent.change(screen.getByLabelText('Sender attempt'), {target: {value: 'op-a'}});
  fireEvent.change(screen.getByLabelText('Recipient task'), {target: {value: 'b'}});
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: 'Claim only'}});
  fireEvent.click(screen.getByRole('button', {name: 'Record message claim'}));
  await screen.findByText(/connection lost/);
  vi.spyOn(swarmSupervisionApi, 'disposition').mockResolvedValue({projectId: 'p-1', runId: 'run-1', operationId: 'op-a', state: 'rejected', runRevision: 2, message: null});
  fireEvent.click(screen.getByRole('button', {name: 'Check exact saved outcome'}));
  await waitFor(() => expect(readMessageRecovery(p.recoveryScope)).toBeNull());
  expect(post).toHaveBeenCalledOnce(); expect(screen.getByText(/original rejected request was preserved/)).toBeTruthy();
});
it('cancels reads on revocation, hides retained private claims and drops late results on owner unmount', async () => {
  let resolve!: (value: ReturnType<typeof swarmSupervisionFixture>) => void;
  const get = vi.spyOn(swarmSupervisionApi, 'get').mockImplementation(() => new Promise(done => {resolve = done;})), p = props();
  const page = render(<SwarmSupervisionPanel {...p} />); const signal = get.mock.calls[0][3];
  page.rerender(<SwarmSupervisionPanel {...p} canRead={false} canEdit={false} />);
  expect(signal?.aborted).toBe(true); resolve(swarmSupervisionFixture());
  await waitFor(() => expect(screen.queryByText(swarmMessageFixture().summary)).toBeNull());
  expect(screen.getByText(/current project read access/)).toBeTruthy();
  page.unmount(); expect(p.onWrite).not.toHaveBeenCalled();
});
it('loads later pages only on request and appends in saved sequence order', async () => {
  const get = vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValueOnce(swarmSupervisionFixture({page: {after: 0, nextCursor: 1, hasMore: true, omitted: 2}}));
  render(<SwarmSupervisionPanel {...props()} />); await screen.findByText(swarmMessageFixture().summary); expect(get).toHaveBeenCalledOnce();
  get.mockResolvedValueOnce(swarmSupervisionFixture({messages: [swarmMessageFixture({id: 'msg-2', sequence: 2, summary: 'Next claim'})], page: {after: 1, nextCursor: 2, hasMore: false, omitted: 0}}));
  fireEvent.click(screen.getByRole('button', {name: 'Load next message page'})); await screen.findByText('Next claim');
  expect(get.mock.calls[1][2]).toBe(1); expect(screen.getByText(swarmMessageFixture().summary)).toBeTruthy();
});
