import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { GTMMetricImports } from './GTMMetricImports';
import { parseMetricCSV, type GTMMetricImport } from '../../api/gtmMetricImports';
vi.mock('../../api/gtmMetricImports', async original => ({ ...await original<typeof import('../../api/gtmMetricImports')>(), parseMetricCSV: vi.fn() }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });
const fixture = (): GTMMetricImport => ({ version:1, sourceName:'results.csv', sourceSha256:'a'.repeat(64), sourceBytes:100, rows:[{row:2,metric:'signups',start:'2026-01-01',end:'2026-01-31',value:20,currency:'',decimals:0}] });
function file(){const f=new File(['x'],'results.csv',{type:'text/csv'});Object.defineProperty(f,'arrayBuffer',{value:async()=>new ArrayBuffer(1)});return f;}
it('previews first and changes only the unsaved draft after explicit add',async()=>{
 vi.mocked(parseMetricCSV).mockResolvedValue(fixture());const change=vi.fn();const user=userEvent.setup();render(<GTMMetricImports projectId="p-one" contextKey="r1" value={[]} editable canDiscuss={false} onChange={change}/>);
 fireEvent.click(screen.getByText(/Analytics and revenue results/));fireEvent.change(screen.getByLabelText('Preview aggregate CSV'),{target:{files:[file()]}});await screen.findByRole('heading',{name:'Review results.csv'});expect(change).not.toHaveBeenCalled();await user.click(screen.getByRole('button',{name:'Add these results to draft'}));expect(change).toHaveBeenCalledWith([fixture()]);expect(screen.queryByRole('heading',{name:'Review results.csv'})).toBeNull();
});
it('withdraws a pending import when project access or baseline changes',async()=>{
 let done!:(b:GTMMetricImport)=>void;vi.mocked(parseMetricCSV).mockImplementation(()=>new Promise(resolve=>{done=resolve}));const change=vi.fn();const view=render(<GTMMetricImports projectId="p-one" contextKey="r1" value={[]} editable canDiscuss={false} onChange={change}/>);fireEvent.click(screen.getByText(/Analytics and revenue results/));fireEvent.change(screen.getByLabelText('Preview aggregate CSV'),{target:{files:[file()]}});await act(async()=>{});view.rerender(<GTMMetricImports projectId="p-two" contextKey="r2" value={[]} editable={false} canDiscuss={false} onChange={change}/>);await act(async()=>done(fixture()));expect(screen.queryByRole('heading',{name:'Review results.csv'})).toBeNull();expect(change).not.toHaveBeenCalled();
});
it('opens an unsent finding only for saved results and leaves removal in the draft',async()=>{
 const change=vi.fn(), event=vi.fn();window.addEventListener('luci:orchestrator-open',event);try{const view=render(<GTMMetricImports projectId="p-one" contextKey="r1" value={[fixture()]} editable canDiscuss={false} onChange={change}/>);fireEvent.click(screen.getByText(/Analytics and revenue results/));fireEvent.click(screen.getByText(/results.csv ·/));expect((screen.getByRole('button',{name:'Discuss these results with Luci'}) as HTMLButtonElement).disabled).toBe(true);view.rerender(<GTMMetricImports projectId="p-one" contextKey="r1" value={[fixture()]} editable canDiscuss onChange={change}/>);fireEvent.click(screen.getByRole('button',{name:'Discuss these results with Luci'}));expect(event).toHaveBeenCalledTimes(1);expect((event.mock.calls[0][0] as CustomEvent).detail).toEqual(expect.objectContaining({projectId:'p-one',message:expect.stringContaining('user-reported')}));fireEvent.click(screen.getByRole('button',{name:'Remove results.csv from draft'}));expect(change).toHaveBeenCalledWith([]);}finally{window.removeEventListener('luci:orchestrator-open',event)}
});
