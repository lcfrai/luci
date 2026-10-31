import {render,screen,fireEvent,waitFor,act} from '@testing-library/react';
import {describe,it,expect,vi,afterEach} from 'vitest';
import {productApi,type CloudJob} from '../../api/product';
import {RuntimeBlobJobPicker} from './RuntimeBlobJobPicker';
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>({canEdit:true,permissions:{deploy:true,secrets:true}})}));
const original:CloudJob={id:'cj-original',kind:'telemetry-storage',planId:'cp-original',provider:'azure',name:'trace-store',region:'australiaeast',domain:'',status:'resources_ready',message:'Observed',publicIp:'',resourceId:'/subscriptions/a/resourceGroups/original',createdAt:'2026-10-31T00:00:00Z',updatedAt:'2026-10-31T00:00:00Z',telemetryStorage:{accountResourceId:'/subscriptions/a/providers/Microsoft.Storage/storageAccounts/ownedtraces',accountCreatedAt:'2026-10-31T00:00:00Z',containerResourceId:'/subscriptions/a/storage/containers/traces',containerVersion:'1',state:'resources_ready',observedAt:'2026-10-31T00:00:00Z'}};
afterEach(()=>vi.restoreAllMocks());
describe('Private Blob job selection',()=>{
 it('loads only on request and requires explicit selection of a matching original',async()=>{
  const read=vi.spyOn(productApi,'cloudJobs').mockResolvedValue({jobs:[original,{...original,id:'cj-unknown',status:'unknown'}]}),change=vi.fn();
  render(<RuntimeBlobJobPicker projectId="p" accountName="ownedtraces" value="" disabled={false} onChange={change}/>);expect(read).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button',{name:'Read matching owned Blob jobs'}));await screen.findByRole('option',{name:/trace-store/});expect(change).not.toHaveBeenCalled();
  fireEvent.change(screen.getByRole('combobox',{name:'Original dedicated storage job'}),{target:{value:'cj-original'}});expect(change).toHaveBeenCalledWith('cj-original');expect(screen.queryByRole('option',{name:/unknown/})).toBeNull();
 });
 it('withdraws an in-flight result when current authority is removed',async()=>{
  let complete!:(value:{jobs:CloudJob[]})=>void;const read=vi.spyOn(productApi,'cloudJobs').mockReturnValue(new Promise(resolve=>{complete=resolve})),change=vi.fn();
  const view=render(<RuntimeBlobJobPicker projectId="p" accountName="ownedtraces" value="" disabled={false} onChange={change}/>);fireEvent.click(screen.getByRole('button',{name:'Read matching owned Blob jobs'}));await waitFor(()=>expect(read).toHaveBeenCalledTimes(1));
  view.rerender(<RuntimeBlobJobPicker projectId="p" accountName="ownedtraces" value="" disabled onChange={change}/>);await act(async()=>complete({jobs:[original]}));expect(screen.queryByRole('option',{name:/trace-store/})).toBeNull();expect(change).not.toHaveBeenCalled();
 });
});
