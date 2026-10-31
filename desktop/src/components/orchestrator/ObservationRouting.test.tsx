import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ObservationRouting } from './ObservationRouting';
import { researchApi, type ObservationView } from '../../api/research';
vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=> 'alice/workspace/project'}));
const authority=vi.hoisted(()=>({secrets:true}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>({canRead:true,canEdit:true,canExecute:true,permissions:{secrets:authority.secrets}})}));
beforeEach(()=>{authority.secrets=true;});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
const saved=():ObservationView=>({routeRevision:3,autoDiscuss:true,route:null,sources:[{kind:'research',digest:'a'.repeat(64),revision:7,summary:'Saved evidence changed.'}],unavailable:[],telemetryOptions:[]});
it('does not read or delegate until explicit actions and sends exact saved digest',async()=>{
 const read=vi.spyOn(researchApi,'observations').mockResolvedValue(saved()),queue=vi.spyOn(researchApi,'queue').mockResolvedValue({retained:true}),user=userEvent.setup();
 render(<ObservationRouting projectId="project"/>);expect(read).not.toHaveBeenCalled();await user.click(screen.getByText('Turn saved findings into reviewable proposals'));await user.click(screen.getByRole('button',{name:'Read saved observation sources'}));await screen.findByText('Saved evidence changed.');
 expect(queue).not.toHaveBeenCalled();await user.click(screen.getByLabelText(/Allow these selected saved observations/));await user.click(screen.getByRole('button',{name:'Ask Luci about this exact saved source'}));expect(queue).toHaveBeenCalledWith('project',3,saved().sources[0],expect.any(AbortSignal));
});
it('saves a bounded source baseline rather than immediately queueing old evidence',async()=>{
 vi.spyOn(researchApi,'observations').mockResolvedValue(saved());const route=vi.spyOn(researchApi,'route').mockResolvedValue({...saved(),routeRevision:4,route:{kinds:['research'],telemetryIds:[],remaining:4,last:{research:'a'.repeat(64)}}}),queue=vi.spyOn(researchApi,'queue'),user=userEvent.setup();
 render(<ObservationRouting projectId="project"/>);await user.click(screen.getByText('Turn saved findings into reviewable proposals'));await user.click(screen.getByRole('button',{name:'Read saved observation sources'}));await screen.findByText('Saved evidence changed.');await user.click(screen.getByLabelText('Saved research evidence'));await user.click(screen.getByLabelText(/Allow these selected saved observations/));await user.click(screen.getByRole('button',{name:'Enable bounded observation proposals'}));
 expect(route).toHaveBeenCalledWith('project',{expectedRouteRevision:3,kinds:['research'],telemetryIds:[],maxTurns:4,acknowledgeModelCharges:true},expect.any(AbortSignal));expect(queue).not.toHaveBeenCalled();
});
it('withdraws cached sensitive sources and late responses when Secrets changes while allowing a fresh public read',async()=>{
 let resolve!:(view:ObservationView)=>void;
 const sensitive:ObservationView={...saved(),sources:[{kind:'operations',digest:'b'.repeat(64),revision:1,summary:'Previously authorized private operational summary.'}]};
 const read=vi.spyOn(researchApi,'observations').mockResolvedValueOnce(sensitive).mockImplementationOnce(()=>new Promise(done=>{resolve=done;})).mockResolvedValueOnce(saved());
 const user=userEvent.setup(),component=render(<ObservationRouting projectId="project"/>);
 await user.click(screen.getByText('Turn saved findings into reviewable proposals'));await user.click(screen.getByRole('button',{name:'Read saved observation sources'}));
 await screen.findByText('Previously authorized private operational summary.');
 await user.click(screen.getByRole('button',{name:'Read saved observation sources'}));await waitFor(()=>expect(read).toHaveBeenCalledTimes(2));
 authority.secrets=false;component.rerender(<ObservationRouting projectId="project"/>);
 expect(screen.queryByText('Previously authorized private operational summary.')).toBeNull();
 await act(async()=>{resolve(sensitive);});expect(screen.queryByText('Previously authorized private operational summary.')).toBeNull();
 await user.click(screen.getByText('Turn saved findings into reviewable proposals'));await user.click(screen.getByRole('button',{name:'Read saved observation sources'}));
 await screen.findByText('Saved evidence changed.');expect(read).toHaveBeenCalledTimes(3);
});
