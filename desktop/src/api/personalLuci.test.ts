import { expect, it, vi } from 'vitest';
import { checkedPersonalView, personalLuciApi, validPersonalPreferences, type PersonalView } from './personalLuci';
import { productApi } from './product';
const empty = (): PersonalView => ({ revision: 0, conversationId: 'personal-chat-one', archived: false, archives: [], preferences: { computeProjectId: '', harness: '', model: '', connectionId: '', projectIds: [], goalIds: [] }, context: { version: 0, preferences: { computeProjectId: '', harness: '', model: '', connectionId: '', projectIds: [], goalIds: [] }, digest: '', goals: [], projects: [], unknownProjects: 0, omittedAttention: 0 }, turns: [], hasEarlier: false, beforeSequence: 0 });
it('rejects malformed archive metadata and a context selection above the combined project bound', () => {
  expect(checkedPersonalView(empty())).toEqual(empty());
  expect(() => checkedPersonalView({ ...empty(), archives: [{ id: '../another', lastMessageAt: '2026-10-31T00:00:00Z', turns: 1 }] })).toThrow();
  expect(validPersonalPreferences({ computeProjectId: 'compute', harness: 'codex', model: 'saved', connectionId: 'saved', projectIds: Array.from({ length: 16 }, (_, i) => `project-${i}`), goalIds: [] })).toBe(false);
});
it('reads or exports an exact saved segment and rolls over only through the explicit revision-bound action', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(empty());
  try {
    await personalLuciApi.get(undefined, 12, 'personal-chat-one');
    expect(request.mock.calls[0][0]).toBe('/v1/product/personal-luci?before=12&archiveId=personal-chat-one');
    await personalLuciApi.continue(3, 'personal-chat-one');
    expect(request).toHaveBeenLastCalledWith('/v1/product/personal-luci/continue', { expectedRevision: 3, conversationId: 'personal-chat-one' });
    await personalLuciApi.export('personal-chat-one');
    expect(request.mock.calls[2][0]).toBe('/v1/product/personal-luci/export?archiveId=personal-chat-one');
  } finally { request.mockRestore(); }
});
