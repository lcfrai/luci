import {render,screen,fireEvent} from '@testing-library/react';
import {describe,it,expect,vi} from 'vitest';
import {TelemetryStorageEditor,newTelemetryStore} from './TelemetryStorageEditor';
import {TelemetryCollectorEditor,newCollectorService} from './TelemetryEditor';
vi.mock('./KubernetesReferencePicker',()=>({KubernetesReferencePicker:()=>null}));
describe('Owned storage editing',()=>{
 it('retains exact TLS and Blob references while editing durable storage capacity',()=>{
  const store=newTelemetryStore('trace-store','traces'),change=vi.fn();render(<TelemetryStorageEditor projectId="p" targetId="cluster" namespace="apps" active value={store.telemetryStorage!} onChange={change}/>);
  fireEvent.change(screen.getByRole('spinbutton',{name:'Reserved storage (GiB)'}),{target:{value:'20'}});
  expect(change).toHaveBeenLastCalledWith({...store.telemetryStorage,capacityGiB:20});expect(screen.getByText(/One replica uses Recreate/)).toBeTruthy();
 });
 it('requires explicit store selection and clears the old external Authorization reference',()=>{
  const collector=newCollectorService('collector').collector!,change=vi.fn();collector.destination.authorization={resourceName:'old-auth',key:'token'};
  render(<TelemetryCollectorEditor projectId="p" targetId="cluster" namespace="apps" active value={collector} onChange={change}/>);
  fireEvent.click(screen.getByRole('checkbox',{name:'Store metrics and traces in this planned private backend'}));
  expect(change).toHaveBeenLastCalledWith(expect.objectContaining({destination:{resourceId:'',endpoint:''},ownedDestination:{metricsServiceId:'',tracesServiceId:'',clientTLS:{resourceName:'',caKey:'',certKey:'',keyKey:''}}}));
 });
});
