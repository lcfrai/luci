import { afterEach, expect, it, vi } from 'vitest';
import { bindDesignRaster, readDesignRaster } from './designSceneAssets';
import { productApi } from './product';
import { selectTeamScope } from './teamScope';
import type { DesignRasterPin } from './designScene';
const pin: DesignRasterPin = {sourceCommit:'a'.repeat(40),gitBlob:'b'.repeat(40),digest:'c'.repeat(64),mimeType:'image/png',bytes:3,width:1,height:1};
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); selectTeamScope(''); });
it('accepts only an exact project/path/commit binding from the normal scoped client', async () => {
  const request = vi.spyOn(productApi,'request').mockResolvedValue({projectId:'p-one',path:'hero.png',content:pin});
  const signal=new AbortController().signal;
  expect(await bindDesignRaster('p-one','hero.png',pin.sourceCommit,signal)).toEqual(pin);
  expect(request.mock.calls[0][0]).toContain('asset-binding?path=hero.png&commit=');
  request.mockResolvedValue({projectId:'p-other',path:'hero.png',content:pin});
  await expect(bindDesignRaster('p-one','hero.png',pin.sourceCommit,signal)).rejects.toThrow('did not match');
  await expect(bindDesignRaster('p-one','hero.png','HEAD',signal)).rejects.toThrow('full 40');
});
it('rejects changed headers, incomplete bytes and digest mismatch before returning any raster', async () => {
  const signal = new AbortController().signal;
  for (const response of [new Response('abc',{headers:{'Content-Type':'text/html','Content-Length':'3'}}),new Response('ab',{headers:{'Content-Type':'image/png','Content-Length':'3'}}),new Response('abc',{headers:{'Content-Type':'image/png','Content-Length':'3'}})]) {
    vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response));
    await expect(readDesignRaster('p-one',1,'hero',pin,signal)).rejects.toThrow();
  }
});
it('returns verified bytes and cancels a response from a changed workspace', async () => {
  vi.stubGlobal('crypto',{subtle:{digest:vi.fn().mockResolvedValue(new Uint8Array(32).fill(0xcc).buffer)}});
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response('abc',{headers:{'Content-Type':'image/png','Content-Length':'3'}})));
  const result=await readDesignRaster('p-one',1,'hero',pin,new AbortController().signal);
  expect(result.size).toBe(3); expect(result.type).toBe('image/png');
  vi.stubGlobal('fetch',vi.fn().mockImplementation(async () => { selectTeamScope(''); return new Response('abc',{headers:{'Content-Type':'image/png','Content-Length':'3'}}); }));
  await expect(readDesignRaster('p-one',1,'hero',pin,new AbortController().signal)).rejects.toThrow('access changed');
});
