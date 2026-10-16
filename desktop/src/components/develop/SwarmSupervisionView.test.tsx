import { StrictMode } from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { swarmSupervisionApi } from '../../api/swarmSupervision';
import type { RunOperation } from '../../api/projectRuns';
import { swarmRunFixture } from '../../test/swarmFixtures';
import { swarmMessageFixture, swarmSupervisionFixture } from '../../test/swarmSupervisionFixtures';
import { readMessageRecovery, readRejectedMessageArchive } from './swarmMessageRecovery';
import { SwarmSupervisionPanel } from './SwarmSupervisionView';
beforeEach(() => localStorage.clear()); afterEach(() => vi.restoreAllMocks());
function props() {
  const run = swarmRunFixture();
  run.operations = ['a', 'b'].map(id => ({id: `op-${id}`, taskId: id, kind: 'session', childId: `session-${id}`, worktreeId: `isolated-${id}`, status: 'running', allocation: 'ready', attempt: 1, requestDigest: 'request', createdAt: 'now', updatedAt: 'now'} as RunOperation));
  run.swarm!.tasks = run.swarm!.tasks.map(task => ({...task, operationId: task.id === 'c' ? undefined : `op-${task.id}`}));
  return {run, canRead: true, canEdit: true, paused: false, recoveryScope: 'actor/workspace/team/run', onOpenTask: vi.fn(), onWrite: vi.fn(async (action: () => Promise<void>) => action())};
}
const resolutionMessage = () => swarmMessageFixture({id: 'msg-resolution', sequence: 301, kind: 'decision', sender: {taskId: 'b', operationId: 'op-b', childId: 'session-b', worktreeId: 'isolated-b', contextDigest: 'b'.repeat(64)}, recipientTaskId: 'a', causalMessageId: 'msg-1', resolvesQuestionId: 'msg-1', summary: 'Keep archived items outside the empty state.'});
const openQuestions = () => swarmSupervisionFixture({questionResolutions: [], tasks: ['a', 'b'].map(id => ({taskId: id, operationId: `op-${id}`, role: 'coding' as const, state: 'working', integrationState: 'none'}))});
it('LOCAL151 drafts an exact reciprocal decision and posts only on explicit Record resolution', async () => {
  const get = vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(openQuestions());
  const post = vi.spyOn(swarmSupervisionApi, 'post').mockResolvedValue({message: resolutionMessage(), runRevision: 2});
  const p = props(); render(<SwarmSupervisionPanel {...p} />);
  fireEvent.click(await screen.findByRole('button', {name: 'Resolve question 1'}));
  expect(post).not.toHaveBeenCalled(); expect(p.onWrite).not.toHaveBeenCalled();
  expect((screen.getByLabelText('Sender attempt') as HTMLSelectElement).value).toBe('op-b');
  expect((screen.getByLabelText('Recipient task') as HTMLSelectElement).value).toBe('a');
  expect((screen.getByLabelText('Claim kind') as HTMLSelectElement).value).toBe('decision');
  expect(document.activeElement).toBe(screen.getByLabelText('Attributed claim'));
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: resolutionMessage().summary}});
  get.mockResolvedValue(swarmSupervisionFixture({runRevision: 2, questionResolutions: [{questionId: 'msg-1', resolutionMessageId: 'msg-resolution'}], attention: []}));
  fireEvent.click(screen.getByRole('button', {name: 'Record resolution'}));
  await screen.findByText(/Question resolution recorded/);
  expect(post).toHaveBeenCalledOnce(); expect(p.onWrite).toHaveBeenCalledOnce();
  expect(post.mock.calls[0].slice(0, 3)).toEqual(['p-1', 'run-1', 'op-b']);
  expect(post.mock.calls[0][3]).toMatchObject({expectedRevision: 1, kind: 'decision', recipientTaskId: 'a', causalMessageId: 'msg-1', resolvesQuestionId: 'msg-1', summary: resolutionMessage().summary});
  expect(readMessageRecovery(p.recoveryScope)).toBeNull();
  await screen.findByText(/Resolution recorded by decision msg-resolution/);
  expect(screen.queryByRole('button', {name: 'Resolve question 1'})).toBeNull();
});
it.each([
  {label: 'Sender attempt', changed: 'op-a', original: 'op-b'},
  {label: 'Recipient task', changed: 'b', original: 'a'},
  {label: 'Claim kind', changed: 'question', original: 'decision'},
])('LOCAL151 refuses stale resolution intent after manual $label drift, including switching back', async ({label, changed, original}) => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(openQuestions());
  const post = vi.spyOn(swarmSupervisionApi, 'post'); render(<SwarmSupervisionPanel {...props()} />);
  fireEvent.click(await screen.findByRole('button', {name: 'Resolve question 1'}));
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: 'Reviewed answer'}});
  fireEvent.change(screen.getByLabelText(label), {target: {value: changed}});
  fireEvent.change(screen.getByLabelText(label), {target: {value: original}});
  expect(screen.getByText(/Resolution draft is stale/)).toBeTruthy();
  expect((screen.getByRole('button', {name: 'Record resolution'}) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole('button', {name: 'Record resolution'})); expect(post).not.toHaveBeenCalled();
});
it.each(['revision', 'operation', 'access', 'scope', 'context'] as const)('LOCAL151 blocks a selected resolution across %s drift', async change => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(openQuestions());
  const post = vi.spyOn(swarmSupervisionApi, 'post'), p = props();
  const page = render(<SwarmSupervisionPanel {...p} />);
  fireEvent.click(await screen.findByRole('button', {name: 'Resolve question 1'}));
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: 'Reviewed answer'}});
  if (change === 'revision') page.rerender(<SwarmSupervisionPanel {...p} run={{...p.run, revision: p.run.revision + 1}} />);
  if (change === 'operation') page.rerender(<SwarmSupervisionPanel {...p} run={{...p.run, operations: p.run.operations.map(op => op.id === 'op-b' ? {...op, childId: 'replacement-child'} : op)}} />);
  if (change === 'scope') page.rerender(<SwarmSupervisionPanel {...p} recoveryScope="another-owner" />);
  if (change === 'context') {
    vi.mocked(swarmSupervisionApi.get).mockResolvedValue({...openQuestions(), context: {...openQuestions().context, freshness: 'changed'}});
    fireEvent.click(screen.getByRole('button', {name: 'Refresh saved messages'}));
  }
  if (change === 'access') {
    page.rerender(<SwarmSupervisionPanel {...p} canEdit={false} />);
    page.rerender(<SwarmSupervisionPanel {...p} />);
  }
  await screen.findByText(/Resolution draft is stale/);
  expect((screen.getByRole('button', {name: 'Record resolution'}) as HTMLButtonElement).disabled).toBe(true);
  expect(post).not.toHaveBeenCalled(); expect(readMessageRecovery(p.recoveryScope)).toBeNull();
});
it('LOCAL151 rechecks selected resolution after a deferred parent write admission', async () => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(openQuestions());
  const post = vi.spyOn(swarmSupervisionApi, 'post'), p = props(); let admitted!: () => Promise<void>;
  p.onWrite = vi.fn(async (action: () => Promise<void>) => { admitted = action; });
  render(<SwarmSupervisionPanel {...p} />);
  fireEvent.click(await screen.findByRole('button', {name: 'Resolve question 1'}));
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: 'Reviewed answer'}});
  fireEvent.click(screen.getByRole('button', {name: 'Record resolution'}));
  fireEvent.change(screen.getByLabelText('Claim kind'), {target: {value: 'question'}});
  await act(async () => { await admitted(); });
  await screen.findByText(/No write was sent/);
  expect(post).not.toHaveBeenCalled(); expect(readMessageRecovery(p.recoveryScope)).toBeNull();
});
it.each(['legacy', 'readonly', 'resolved-offpage'] as const)('LOCAL151 keeps history visible without resolution authority for %s', async state => {
  const value = state === 'legacy' ? swarmSupervisionFixture() : state === 'resolved-offpage'
    ? swarmSupervisionFixture({questionResolutions: [{questionId: 'msg-1', resolutionMessageId: 'msg-resolution'}], attention: []}) : openQuestions();
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(value);
  const post = vi.spyOn(swarmSupervisionApi, 'post'); render(<SwarmSupervisionPanel {...props()} canEdit={state !== 'readonly'} />);
  await screen.findByText(swarmMessageFixture().summary);
  if (state === 'legacy') expect((screen.getByRole('button', {name: 'Resolve question 1'}) as HTMLButtonElement).disabled).toBe(true);
  else expect(screen.queryByRole('button', {name: 'Resolve question 1'})).toBeNull();
  expect(post).not.toHaveBeenCalled();
  if (state === 'legacy') expect(screen.getByText(/Question resolution status unavailable/)).toBeTruthy();
  if (state === 'resolved-offpage') expect(screen.getByText(/Resolution recorded by decision msg-resolution/)).toBeTruthy();
});
it('LOCAL151 replaces the complete resolution index on later pages instead of inferring from the window', async () => {
  const get = vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValueOnce({...openQuestions(), page: {after: 0, nextCursor: 1, hasMore: true, omitted: 301}});
  render(<SwarmSupervisionPanel {...props()} />); await screen.findByRole('button', {name: 'Resolve question 1'});
  get.mockResolvedValueOnce(swarmSupervisionFixture({runRevision: 2, messages: [swarmMessageFixture({id: 'msg-2', sequence: 2, kind: 'discovery', summary: 'Unrelated discovery'})], questionResolutions: [{questionId: 'msg-1', resolutionMessageId: 'msg-resolution'}], attention: [], page: {after: 1, nextCursor: 2, hasMore: true, omitted: 300}}));
  fireEvent.click(screen.getByRole('button', {name: 'Load next message page'})); await screen.findByText('Unrelated discovery');
  expect(screen.getByText(/Resolution recorded by decision msg-resolution/)).toBeTruthy();
  expect(screen.queryByRole('button', {name: 'Resolve question 1'})).toBeNull();
  get.mockResolvedValueOnce(openQuestions()); fireEvent.click(screen.getByRole('button', {name: 'Refresh saved messages'}));
  await screen.findByText(/predates the retained run snapshot/);
  expect(screen.queryByRole('button', {name: 'Resolve question 1'})).toBeNull();
});
it('LOCAL151 keeps an uncertain exact resolution across remount and reconciles without a second post', async () => {
  vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(openQuestions());
  const post = vi.spyOn(swarmSupervisionApi, 'post').mockRejectedValue(new Error('response unavailable')), p = props();
  const page = render(<SwarmSupervisionPanel {...p} />);
  fireEvent.click(await screen.findByRole('button', {name: 'Resolve question 1'}));
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: resolutionMessage().summary}});
  fireEvent.click(screen.getByRole('button', {name: 'Record resolution'})); await screen.findByText(/response unavailable/);
  const retained = readMessageRecovery(p.recoveryScope); expect(retained?.type === 'post' && retained.request.resolvesQuestionId).toBe('msg-1');
  page.unmount(); render(<SwarmSupervisionPanel {...p} />); await screen.findByText(swarmMessageFixture().summary);
  const disposition = vi.spyOn(swarmSupervisionApi, 'disposition').mockResolvedValueOnce({projectId: 'p-1', runId: 'run-1', operationId: 'op-b', state: 'recorded', runRevision: 2, message: resolutionMessage()});
  expect(post).toHaveBeenCalledOnce(); fireEvent.click(screen.getByRole('button', {name: 'Check exact saved outcome'}));
  await waitFor(() => expect(readMessageRecovery(p.recoveryScope)).toBeNull());
  expect(disposition.mock.calls[0][3]).toEqual(retained?.request); expect(post).toHaveBeenCalledOnce();
  await screen.findByText(/supervision response predates a confirmed resolution/);
  expect((screen.getByRole('button', {name: 'Resolve question 1'}) as HTMLButtonElement).disabled).toBe(true);
});
it('LOCAL151 preserves a competing resolution through refused retirement and settles only by exact disposition', async () => {
  const get = vi.spyOn(swarmSupervisionApi, 'get').mockResolvedValue(openQuestions());
  const post = vi.spyOn(swarmSupervisionApi, 'post').mockRejectedValue(new Error('response unavailable')), p = props();
  render(<SwarmSupervisionPanel {...p} />); fireEvent.click(await screen.findByRole('button', {name: 'Resolve question 1'}));
  fireEvent.change(screen.getByLabelText('Attributed claim'), {target: {value: resolutionMessage().summary}});
  fireEvent.click(screen.getByRole('button', {name: 'Record resolution'})); await screen.findByText(/response unavailable/);
  const retained = readMessageRecovery(p.recoveryScope);
  vi.spyOn(swarmSupervisionApi, 'disposition').mockResolvedValue({projectId: 'p-1', runId: 'run-1', operationId: 'op-b', state: 'unconfirmed', runRevision: 1, message: null});
  fireEvent.click(screen.getByRole('button', {name: 'Check exact saved outcome'})); await screen.findByText(/can still be in flight/);
  get.mockResolvedValue(swarmSupervisionFixture({runRevision: 2, questionResolutions: [{questionId: 'msg-1', resolutionMessageId: 'competing-resolution'}], attention: []}));
  fireEvent.click(screen.getByRole('button', {name: 'Refresh saved messages'})); await screen.findByText(/Resolution recorded by decision competing-resolution/);
  const retire = vi.spyOn(swarmSupervisionApi, 'retirePost').mockRejectedValue(new Error('The original revision has advanced.'));
  fireEvent.click(screen.getByRole('button', {name: 'Close original request revision'}));
  await screen.findByText(/original revision has advanced/);
  expect(readMessageRecovery(p.recoveryScope)).toEqual(retained);
  vi.mocked(swarmSupervisionApi.disposition).mockResolvedValue({projectId: 'p-1', runId: 'run-1', operationId: 'op-b', state: 'rejected', runRevision: 2, message: null});
  fireEvent.click(screen.getByRole('button', {name: 'Check exact saved outcome'}));
  await waitFor(() => expect(readMessageRecovery(p.recoveryScope)).toBeNull());
  expect(retire.mock.calls[0][3]).toEqual(retained?.request); expect(post).toHaveBeenCalledOnce();
  expect(readRejectedMessageArchive(p.recoveryScope)).toEqual([{request: retained, state: 'rejected', runRevision: 2}]);
});
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
