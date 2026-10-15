import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { DesignAssetBinding } from './DesignAssetBinding';
import { bindDesignRaster } from '../../api/designSceneAssets';
vi.mock('../../api/designSceneAssets',()=>({bindDesignRaster:vi.fn()}));
afterEach(()=>vi.clearAllMocks());
const asset={id:'hero',path:'hero.png',purpose:'Hero',licence:'Owned'};
const pin={sourceCommit:'a'.repeat(40),gitBlob:'b'.repeat(40),digest:'c'.repeat(64),mimeType:'image/png' as const,bytes:3,width:1,height:1};
it('only explicit verification adds a binding to the same unsaved asset and never saves or approves',async()=>{
  const change=vi.fn(); vi.mocked(bindDesignRaster).mockResolvedValue(pin);
  render(<DesignAssetBinding projectId="p-one" asset={asset} disabled={false} onChange={change}/>);
  expect(bindDesignRaster).not.toHaveBeenCalled(); fireEvent.change(screen.getByLabelText('Full project Git commit'),{target:{value:pin.sourceCommit}});
  fireEvent.click(screen.getByRole('button',{name:'Verify immutable image'}));
  await waitFor(()=>expect(change).toHaveBeenCalledWith({...asset,content:pin})); expect(bindDesignRaster).toHaveBeenCalledTimes(1);
});
it('an obsolete path response cannot attach old bytes to a renamed asset',async()=>{
  let complete!: (value:typeof pin)=>void; vi.mocked(bindDesignRaster).mockReturnValue(new Promise(resolve=>{complete=resolve;})); const change=vi.fn();
  const page=render(<DesignAssetBinding projectId="p-one" asset={asset} disabled={false} onChange={change}/>);
  fireEvent.change(screen.getByLabelText('Full project Git commit'),{target:{value:pin.sourceCommit}}); fireEvent.click(screen.getByRole('button',{name:'Verify immutable image'}));
  page.rerender(<DesignAssetBinding projectId="p-one" asset={{...asset,path:'other.png'}} disabled={false} onChange={change}/>);
  complete(pin); await waitFor(()=>expect(screen.getByRole('button',{name:'Verify immutable image'})).not.toHaveProperty('disabled',true)); expect(change).not.toHaveBeenCalled();
});
