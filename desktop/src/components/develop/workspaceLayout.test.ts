import { beforeEach, describe, expect, it } from 'vitest';
import { readWorkspaceLayout, workspaceLayoutKey } from './workspaceLayout';

beforeEach(() => localStorage.clear());
describe('scoped Develop layout preferences', () => {
  it('creates no anonymous bucket and distinguishes accounts, workspaces and branches', () => {
    expect(workspaceLayoutKey(null, 'b')).toBeNull();
    expect(workspaceLayoutKey('actor/workspace/project', null)).toBeNull();
    expect(workspaceLayoutKey('a/w/p', 'b')).not.toBe(workspaceLayoutKey('other/w/p', 'b'));
    expect(workspaceLayoutKey('a/w/p', 'b')).not.toBe(workspaceLayoutKey('a/remote/p', 'b'));
    expect(workspaceLayoutKey('a/w/p', 'b')).not.toBe(workspaceLayoutKey('a/w/p', 'other'));
  });
  it('refuses corrupt or excessive persisted input rather than applying unsafe geometry', () => {
    for (const raw of ['{', 'null', '{"mode":"focus","columnWidth":999999}', '{"mode":"unknown","columnWidth":520}', 'x'.repeat(257)]) {
      localStorage.setItem('layout', raw);
      expect(() => readWorkspaceLayout('layout')).toThrow();
    }
    expect(readWorkspaceLayout(null)).toEqual({ mode: 'tiles', columnWidth: 520 });
  });
});
