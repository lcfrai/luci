import type { CoordinatorCloudCapacity } from '../../api/coordinatorContinuations';
import { Button } from '../fluid/components/button';

export function cloudHostRoute(capacity:CoordinatorCloudCapacity|null,selected:string[]):{cloudHostIds?:string[];cloudPoolDigest?:string}{
 if(!selected.length)return {};
 if(capacity?.provider!=='cloud-pool'||!capacity.placementDigest||selected.length>8||new Set(selected).size!==selected.length||selected.some(id=>!capacity.hosts?.some(host=>host.id===id&&!host.draining)))throw new Error('The selected cloud hosts changed. Refresh capacity and review the allowed hosts again.');
 return {cloudHostIds:[...selected].sort(),cloudPoolDigest:capacity.placementDigest};
}
export function CloudHostSelection({capacity,selected,disabled,onChange}:{capacity:CoordinatorCloudCapacity|null;selected:string[];disabled:boolean;onChange(ids:string[]):void}){
 if(capacity?.provider!=='cloud-pool')return null;
 const missing=selected.filter(id=>!capacity.hosts?.some(host=>host.id===id&&!host.draining));
 return <fieldset disabled={disabled}><legend>Allowed cloud hosts for this run</legend>
  <p>Choose where new tasks may run. Luci selects among these approved hosts; each attempt keeps its original host through recovery.</p>
  {capacity.hosts?.map(host=><label className="setup-check" key={host.id}><input type="checkbox" checked={selected.includes(host.id)} disabled={host.draining} onChange={event=>onChange(event.target.checked?[...selected,host.id]:selected.filter(id=>id!==host.id))}/>{host.name} · {host.region} · {host.configuredAvailableSlots} configured slots{host.default?' · default':''}{host.draining?' · existing work only':''} · up to ${(host.estimatedTaskCostMicros/1_000_000).toFixed(2)} USD estimated per task</label>)}
  {!selected.length&&<p>With none selected, only the existing default host is allowed. This does not authorize acquiring hosts or switching local work to cloud.</p>}
  {missing.length>0&&<p role="alert">A selected host is no longer available for new work: {missing.join(', ')}. Refresh and choose again; existing attempts have not moved.</p>}
  {selected.length>0&&<Button type="button" variant="tertiary" onClick={()=>onChange([])}>Use default host only</Button>}
 </fieldset>;
}
