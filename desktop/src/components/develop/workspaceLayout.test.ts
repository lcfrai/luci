import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { readWorkspaceLayout, workspaceLayoutKey } from './workspaceLayout';
import { bindNativePreferenceIdentity } from '../../preferences/serviceIdentity';

beforeEach(() => { localStorage.clear(); bindNativePreferenceIdentity('', null); });
afterEach(() => bindNativePreferenceIdentity('', null));

const nativeOrigin = 'http://127.0.0.1:51001';
const stateDir = 'E:/Luci/native-state';
const projectScope = (origin = nativeOrigin) => [
  `${origin}/v1/teams/team-a/workspace`, 'alice', 'local', '', '', 'alice-workspace', 'E:/Projects', 'counter',
];
const focusLayout = { mode: 'focus', columnWidth: 680 };
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
    expect(readWorkspaceLayout(null)).toEqual({ mode: 'focus', columnWidth: 520 });
  });

  it('restores Focus and pane width after the same native workspace restarts on another port', () => {
    bindNativePreferenceIdentity(nativeOrigin, stateDir);
    const firstScope = JSON.stringify(projectScope());
    const firstKey = workspaceLayoutKey(firstScope, 'main')!;
    localStorage.setItem(firstKey, JSON.stringify(focusLayout));

    const restartedOrigin = 'http://127.0.0.1:51002';
    bindNativePreferenceIdentity(restartedOrigin, stateDir);
    const restartedScope = JSON.stringify(projectScope(restartedOrigin));
    expect(restartedScope).not.toBe(firstScope);
    expect(workspaceLayoutKey(restartedScope, 'main')).toBe(firstKey);
    expect(readWorkspaceLayout(workspaceLayoutKey(restartedScope, 'main'))).toEqual(focusLayout);
  });

  it.each([
    'account', 'workspace mode', 'connection', 'host', 'workspace account', 'project root', 'project',
    'branch', 'team', 'native installation', 'remote service', 'unbound loopback service', 'external override',
  ] as const)('does not restore native layout preferences when the %s changes', change => {
    bindNativePreferenceIdentity(nativeOrigin, stateDir);
    const originalKey = workspaceLayoutKey(JSON.stringify(projectScope()), 'main')!;
    localStorage.setItem(originalKey, JSON.stringify(focusLayout));
    const next = projectScope();
    let branch = 'main';
    if (change === 'account') next[1] = 'bob';
    else if (change === 'workspace mode') next[2] = 'cloud';
    else if (change === 'connection') next[3] = 'remote-connection';
    else if (change === 'host') next[4] = 'other-host';
    else if (change === 'workspace account') next[5] = 'other-workspace-account';
    else if (change === 'project root') next[6] = 'E:/OtherProjects';
    else if (change === 'project') next[7] = 'another-project';
    else if (change === 'branch') branch = 'another-branch';
    else if (change === 'team') next[0] = `${nativeOrigin}/v1/teams/team-b/workspace`;
    else if (change === 'native installation') bindNativePreferenceIdentity(nativeOrigin, 'E:/Luci/other-state');
    else if (change === 'remote service') next[0] = 'https://other.example/v1/teams/team-a/workspace';
    else if (change === 'unbound loopback service') next[0] = 'http://127.0.0.1:51002/v1/teams/team-a/workspace';
    else bindNativePreferenceIdentity(nativeOrigin, null);
    const nextKey = workspaceLayoutKey(JSON.stringify(next), branch);
    expect(nextKey).not.toBe(originalKey);
    expect(readWorkspaceLayout(nextKey)).toEqual({ mode: 'focus', columnWidth: 520 });
  });

  it('preserves existing remote keys and does not infer an identity for an unfamiliar scope shape', () => {
    bindNativePreferenceIdentity(nativeOrigin, stateDir);
    for (const scope of [JSON.stringify(projectScope('https://existing.example')), JSON.stringify([nativeOrigin, 'alice'])]) {
      const existingKey = 'luci.develop-layout.v1:' + JSON.stringify([scope, 'main']);
      localStorage.setItem(existingKey, JSON.stringify(focusLayout));
      expect(workspaceLayoutKey(scope, 'main')).toBe(existingKey);
      expect(readWorkspaceLayout(workspaceLayoutKey(scope, 'main'))).toEqual(focusLayout);
    }
  });

  it('does not migrate a layout from an unproven old native transport address', () => {
    const oldScope = JSON.stringify(projectScope());
    localStorage.setItem('luci.develop-layout.v1:' + JSON.stringify([oldScope, 'main']), JSON.stringify(focusLayout));
    bindNativePreferenceIdentity('http://127.0.0.1:51002', stateDir);
    expect(readWorkspaceLayout(workspaceLayoutKey(JSON.stringify(projectScope('http://127.0.0.1:51002')), 'main')))
      .toEqual({ mode: 'focus', columnWidth: 520 });
  });

  it('defaults only an absent preference to Focus and preserves an explicitly saved Tiles arrangement', () => {
    expect(readWorkspaceLayout('new-layout')).toEqual({ mode: 'focus', columnWidth: 520 });
    localStorage.setItem('saved-layout', JSON.stringify({ mode: 'tiles', columnWidth: 640 }));
    expect(readWorkspaceLayout('saved-layout')).toEqual({ mode: 'tiles', columnWidth: 640 });
  });
});
