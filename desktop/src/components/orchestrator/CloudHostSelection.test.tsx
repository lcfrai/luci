import { useState } from 'react';
import { cleanup,render,screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach,expect,it } from 'vitest';
import { CloudHostSelection,cloudHostRoute } from './CloudHostSelection';
import type { CoordinatorCloudCapacity } from '../../api/coordinatorContinuations';

const capacity:CoordinatorCloudCapacity={provider:'cloud-pool',region:'approved host pool',placementDigest:'a'.repeat(64),hosts:[{id:'host-a',name:'Sydney workspace',provider:'azure',region:'australiaeast',templateId:'task-template',default:true,draining:false,configuredAvailableSlots:2,estimatedTaskCostMicros:1000,maxTaskSeconds:600},{id:'host-b',name:'Existing draining host',provider:'aws',region:'ap-southeast-2',templateId:'old-template',default:false,draining:true,configuredAvailableSlots:0,estimatedTaskCostMicros:1000,maxTaskSeconds:600}],configuredGlobalSlots:2,configuredProjectSlots:1,configuredAvailableSlots:2,estimatedTaskCostMicros:1000,estimatedMonthMicros:0,monthlyBudgetMicros:10000,maxTaskSeconds:600,retainedProjectVMs:0,retainedAttempts:[],moreRetainedAttempts:0,observedAt:'2026-10-31T00:00:00Z',limitation:'Configured limits only.'};
afterEach(cleanup);
it('keeps omission default-only and captures only explicitly selected current hosts',async()=>{
 function HostChoice(){const [selected,change]=useState<string[]>([]);return <><CloudHostSelection capacity={capacity} selected={selected} disabled={false} onChange={change}/><output aria-label="Reviewed placement">{JSON.stringify(cloudHostRoute(capacity,selected))}</output></>}
 const user=userEvent.setup();render(<HostChoice/>);
 expect(screen.getByLabelText('Reviewed placement').textContent).toBe('{}');
 await user.click(screen.getByLabelText(/Sydney workspace/));
 expect(screen.getByLabelText('Reviewed placement').textContent).toBe(JSON.stringify({cloudHostIds:['host-a'],cloudPoolDigest:'a'.repeat(64)}));
 expect(screen.getByLabelText(/Existing draining host/).matches(':disabled')).toBe(true);
 await user.click(screen.getByRole('button',{name:'Use default host only'}));expect(screen.getByLabelText('Reviewed placement').textContent).toBe('{}');
});
it('holds removed, draining or duplicate choices instead of silently substituting another host',()=>{
 for(const ids of [['missing'],['host-b'],['host-a','host-a']])expect(()=>cloudHostRoute(capacity,ids)).toThrow(/changed/);
 render(<CloudHostSelection capacity={capacity} selected={['missing']} disabled onChange={()=>{throw new Error('readonly change')}}/>);
 expect(screen.getByRole('alert').textContent).toContain('existing attempts have not moved');
 expect(screen.getByLabelText(/Sydney workspace/).matches(':disabled')).toBe(true);
});
