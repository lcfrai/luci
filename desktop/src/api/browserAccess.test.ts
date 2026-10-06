import { afterEach, describe, expect, it, vi } from 'vitest';
import { prepareBrowserAccess } from './browserAccess';
import { productApi } from './product';
import { selectTeamScope } from './teamScope';

const team = 'team-'+'a'.repeat(22);
const origin = `http://${'b'.repeat(32)}.${'c'.repeat(32)}.localhost:8731`;
const handoff = origin+'/.well-known/luci-team-access?ticket='+'d'.repeat(43);
afterEach(()=>{selectTeamScope('');vi.restoreAllMocks();});
describe('team browser access',()=>{
  it('uses authenticated handoff only for isolated team applications',async()=>{
    const request=vi.spyOn(productApi,'request').mockResolvedValue({url:handoff});
    await expect(prepareBrowserAccess('p-one',origin)).resolves.toBe(origin);
    expect(request).not.toHaveBeenCalled();
    selectTeamScope(team);
    await expect(prepareBrowserAccess('p-one',origin+'/app')).resolves.toBe(handoff);
    expect(request).toHaveBeenCalledWith(`/v1/teams/${team}/projects/p-one/browser-access`,{url:origin+'/app'});
    await expect(prepareBrowserAccess('p-one','https://public.example/app')).resolves.toBe('https://public.example/app');
    expect(request).toHaveBeenCalledTimes(1);
  });
  it('rejects unisolated addresses and redirected or malformed handoffs',async()=>{
    selectTeamScope(team);
    const request=vi.spyOn(productApi,'request');
    await expect(prepareBrowserAccess('p-one','http://127.0.0.1:3000')).rejects.toThrow('isolated browser gateway');
    expect(request).not.toHaveBeenCalled();
    for(const url of ['https://attacker.example/'+handoff,origin+'/app?ticket='+ 'd'.repeat(43),handoff+'&ticket=second']){
      request.mockResolvedValue({url});
      await expect(prepareBrowserAccess('p-one',origin)).rejects.toThrow('browser invitation');
    }
  });
  it('does not open a browser after workspace changes during admission',async()=>{
    selectTeamScope(team);
    vi.spyOn(productApi,'request').mockImplementation(async()=>{selectTeamScope('');return {url:handoff};});
    await expect(prepareBrowserAccess('p-one',origin)).rejects.toThrow('workspace changed');
  });
});

it('authorizes HTTPS team gateway addresses before navigation and keeps invitation origin exact',async()=>{
  selectTeamScope(team);
  const address=`https://p-${'b'.repeat(51)}a.previews.example.test`;
  const invitation=address+'/.well-known/luci-team-access?ticket='+'d'.repeat(43);
  const request=vi.spyOn(productApi,'request').mockResolvedValue({url:invitation});
  await expect(prepareBrowserAccess('p-one',address+'/app')).resolves.toBe(invitation);
  expect(request).toHaveBeenCalledWith(`/v1/teams/${team}/projects/p-one/browser-access`,{url:address+'/app'});
  request.mockResolvedValue({url:invitation.replace('previews.example.test','attacker.example.test')});
  await expect(prepareBrowserAccess('p-one',address)).rejects.toThrow('browser invitation');
  await expect(prepareBrowserAccess('p-one',address.replace('https:','http:'))).rejects.toThrow('isolated browser gateway');
});
