import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PersonalLuci } from './PersonalLuci';
import { personalLuciApi, type PersonalView, type PersonalTurn } from '../../api/personalLuci';

const environment = vi.hoisted(() => ({ scope: 'alice', project: vi.fn(), open: vi.fn() }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => environment.scope }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
vi.mock('./ProjectHomeProvider', () => ({ useProjectHome: () => ({ open: environment.open }) }));
vi.mock('../../api/orchestrator', () => ({ orchestratorApi: { get: environment.project } }));
vi.mock('../../hooks/useAgentConnections', () => ({ useAgentConnections: () => ({ catalogue: null, error: '' }), agentConnectionIssue: () => '', selectedAgentConnection: () => undefined, configuredAgentModel: () => 'saved-model' }));
const projects = [{ id: 'p-one', name: 'Appointments', path: '/appointments' }];
const preferences = () => ({ computeProjectId: 'p-one', harness: 'codex', model: 'saved-model', connectionId: 'connection-one', projectIds: ['p-one'], goalIds: [] });
const context = () => ({ version: 1, preferences: preferences(), digest: 'a'.repeat(64), goals: [], projects: [{ project: { id: 'p-one', name: 'Appointments', status: 'ready' }, attention: [] }], unknownProjects: 0, omittedAttention: 0 });
const view = (): PersonalView => ({ revision: 1, conversationId: 'personal-chat-one', archived: false, archives: [], preferences: preferences(), context: context(), turns: [], hasEarlier: false, beforeSequence: 0 });
const turn = (): PersonalTurn => ({ id: 'personal-turn-one', sequence: 1, status: 'succeeded', message: 'PRIVATE MESSAGE', context: context(), createdAt: '2026-10-31T00:00:00Z', answer: { summary: 'Review the next project decision.', question: '', options: [], discussion: { projectId: 'p-one', message: 'Suggested project question' } } });
beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear(); environment.scope = 'alice'; environment.project.mockReset(); environment.open.mockReset();
  vi.spyOn(personalLuciApi, 'get').mockResolvedValue(view());
  vi.spyOn(personalLuciApi, 'send').mockRejectedValue(new Error('Acknowledgement lost.'));
  vi.spyOn(personalLuciApi, 'resolve').mockResolvedValue({ status: 'accepted', turnId: 'personal-turn-one' });
  vi.spyOn(personalLuciApi, 'handoff').mockResolvedValue({ handoff: { id: 'handoff-one', projectId: 'p-one', message: 'Reviewed exact text', status: 'forwarded', projectTurnId: 'turn-discussion-one', projectJobId: 'assist-project' } });
  vi.spyOn(personalLuciApi, 'continue').mockResolvedValue({ ...view(), revision: 4, conversationId: 'personal-chat-two', archives: [{ id: 'personal-chat-one', lastMessageAt: turn().createdAt, turns: 1 }] });
});
afterEach(cleanup);

it('retains an uncertain original request across remount and checks it without another send', async () => {
  const user = userEvent.setup(); const first = render(<PersonalLuci projects={projects} />);
  const input = await screen.findByLabelText('Message Luci'); await user.type(input, 'Help prioritize these projects'); await user.click(screen.getByRole('button', { name: 'Send', exact: true }));
  await screen.findByRole('alert'); expect(personalLuciApi.send).toHaveBeenCalledTimes(1); const original = vi.mocked(personalLuciApi.send).mock.calls[0][0];
  first.unmount(); render(<PersonalLuci projects={projects} />);
  await screen.findByRole('button', { name: 'Check original request' });
  expect(personalLuciApi.send).toHaveBeenCalledTimes(1); expect(personalLuciApi.resolve).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Check original request' }));
  await waitFor(() => expect(personalLuciApi.resolve).toHaveBeenCalledWith(original));
  expect(personalLuciApi.send).toHaveBeenCalledTimes(1);
});

it('sends only the explicitly edited project discussion after fresh context review', async () => {
  vi.mocked(personalLuciApi.get).mockResolvedValue({ ...view(), turns: [turn()], revision: 3 });
  environment.project.mockResolvedValue({ revision: 7, context: { digest: 'b'.repeat(64) } });
  const user = userEvent.setup(); render(<PersonalLuci projects={projects} />);
  await user.click(await screen.findByRole('button', { name: 'Review message for Appointments' }));
  expect(personalLuciApi.handoff).not.toHaveBeenCalled();
  const message = screen.getByLabelText('Reviewed message'); await user.clear(message); await user.type(message, 'Reviewed exact text');
  await user.click(screen.getByRole('button', { name: 'Review current project context' }));
  await user.click(await screen.findByRole('button', { name: 'Send reviewed message' }));
  await waitFor(() => expect(personalLuciApi.handoff).toHaveBeenCalledTimes(1));
  const [id, request] = vi.mocked(personalLuciApi.handoff).mock.calls[0];
  expect(id).toBe(turn().id); expect(request.message).toBe('Reviewed exact text'); expect(request.projectRevision).toBe(7); expect(request.projectContextDigest).toBe('b'.repeat(64));
  expect(JSON.stringify(request)).not.toContain('PRIVATE MESSAGE'); expect(personalLuciApi.send).not.toHaveBeenCalled();
});

it('continues in the same workspace while keeping the draft and selectable saved history', async () => {
  vi.mocked(personalLuciApi.get).mockResolvedValue({ ...view(), turns: [turn()], revision: 3 });
  const user = userEvent.setup(); render(<PersonalLuci projects={projects} />);
  await user.type(await screen.findByLabelText('Message Luci'), 'Keep my next question');
  await user.click(screen.getByRole('button', { name: 'Continue conversation' }));
  await waitFor(() => expect(personalLuciApi.continue).toHaveBeenCalledWith(3, 'personal-chat-one'));
  expect((screen.getByLabelText('Message Luci') as HTMLTextAreaElement).value).toBe('Keep my next question');
  vi.mocked(personalLuciApi.get).mockResolvedValue({ ...view(), archived: true, turns: [turn()], archives: [{ id: 'personal-chat-one', lastMessageAt: turn().createdAt, turns: 1 }] });
  await user.selectOptions(screen.getByLabelText('Conversation history'), 'personal-chat-one');
  await screen.findByText(/Saved history. Return to Current conversation/);
  expect(screen.getByRole('button', { name: 'Send', exact: true }).matches(':disabled')).toBe(true);
  expect(personalLuciApi.send).not.toHaveBeenCalled();
});

it('discards late private history when the authenticated draft scope changes', async () => {
  let resolve!: (value: PersonalView) => void;
  vi.mocked(personalLuciApi.get).mockImplementationOnce(() => new Promise<PersonalView>(done => { resolve = done; })).mockResolvedValue(view());
  const mounted = render(<PersonalLuci projects={projects} />); environment.scope = 'bob'; mounted.rerender(<PersonalLuci projects={projects} />);
  await screen.findByLabelText('Message Luci');
  await act(async () => resolve({ ...view(), turns: [turn()] }));
  expect(screen.queryByText('PRIVATE MESSAGE')).toBeNull(); expect(personalLuciApi.send).not.toHaveBeenCalled();
});
