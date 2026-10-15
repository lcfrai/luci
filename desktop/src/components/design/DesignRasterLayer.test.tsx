import { act, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DesignRasterLayer } from './DesignRasterLayer';
import { readDesignRaster } from '../../api/designSceneAssets';
import { AUTH_EXPIRED } from '../../api/authSession';
import type { DesignRasterPin } from '../../api/designScene';
vi.mock('../../api/designSceneAssets',()=>({readDesignRaster:vi.fn()}));
const pin: DesignRasterPin={sourceCommit:'a'.repeat(40),gitBlob:'b'.repeat(40),digest:'c'.repeat(64),mimeType:'image/png',bytes:3,width:1,height:1};
const props={projectId:'p-one',revision:1,asset:{id:'hero',path:'hero.png',purpose:'Hero',licence:'Owned',content:pin},alt:'Hero illustration',canRead:true,scope:'scope-one'};
beforeEach(()=>{ vi.mocked(readDesignRaster).mockReset().mockResolvedValue(new Blob(['abc'],{type:'image/png'})); vi.stubGlobal('IntersectionObserver',undefined); const NativeURL = URL; vi.stubGlobal('URL', class extends NativeURL { static createObjectURL = vi.fn(() => 'blob:owned-image'); static revokeObjectURL = vi.fn(); }); });
afterEach(()=>vi.unstubAllGlobals());
it('shares duplicate scoped images and retires their exact object URL after the last view leaves',async()=>{
  const one=render(<DesignRasterLayer {...props}/>),two=render(<DesignRasterLayer {...props}/>);
  await waitFor(()=>expect(screen.getAllByRole('img')).toHaveLength(2)); expect(readDesignRaster).toHaveBeenCalledTimes(1);
  one.unmount(); expect(URL.revokeObjectURL).not.toHaveBeenCalled(); two.unmount(); expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:owned-image');
});
it('retires on access loss and ignores an obsolete in-flight image',async()=>{
  let finish!: (value:Blob)=>void; vi.mocked(readDesignRaster).mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  const page=render(<DesignRasterLayer {...props}/>); await waitFor(()=>expect(readDesignRaster).toHaveBeenCalledOnce());
  page.rerender(<DesignRasterLayer {...props} canRead={false}/>);
  await act(async()=>finish(new Blob(['abc']))); expect(URL.createObjectURL).not.toHaveBeenCalled(); expect(screen.queryByRole('img')).toBeNull();
});
it('auth expiry retires loaded content and unsaved pins never request saved bytes',async()=>{
  const page=render(<DesignRasterLayer {...props}/>); await screen.findByRole('img');
  act(()=>window.dispatchEvent(new Event(AUTH_EXPIRED))); expect(screen.queryByRole('img')).toBeNull(); expect(URL.revokeObjectURL).toHaveBeenCalledOnce(); page.unmount();
  vi.mocked(readDesignRaster).mockClear(); render(<DesignRasterLayer {...props} saved={false}/>); expect(screen.getByText('Save this pinned asset before rendering it')).toBeTruthy(); expect(readDesignRaster).not.toHaveBeenCalled();
});
