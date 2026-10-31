import {cleanup,render,screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach,expect,it,vi} from 'vitest';
import type {SavedDeploymentBlueprint} from '../../api/deploymentBlueprint';
import {RuntimeResourceFields} from './RuntimeResourceFields';

const saved:SavedDeploymentBlueprint={projectId:'p-one',revision:2,digest:'a'.repeat(64),planDigest:'b'.repeat(64),actor:{id:'alice',displayName:'Alice'},at:'2026-10-31T00:00:00Z',blueprint:{version:1,projectId:'p-one',source:{commit:'c'.repeat(40),configurationRevision:0},infrastructure:{id:'cluster',kind:'kubernetes',reference:'cluster-one',namespace:'apps'},services:[{id:'model',kind:'inference',image:'registry.example/model@sha256:'+'d'.repeat(64),port:8080,replicas:1,deploymentStrategy:'recreate',model:{name:'Reviewed model',claimName:'old-claim',mountPath:'/model',relativePath:'weights/model.gguf'},resources:{requestMilliCpu:250,limitMilliCpu:500,requestMemoryMiB:256,limitMemoryMiB:512,gpuCount:0}}]}};
afterEach(cleanup);
it('requires exact public model and builder inputs before retaining a complete new-volume selection',async()=>{
 const change=vi.fn(),user=userEvent.setup();render(<RuntimeResourceFields saved={saved} disabled={false} onChange={change}/>);
 expect(change.mock.calls[0]).toEqual([null,'',false]);await user.click(screen.getByText('Prepare model storage and private runtime connections'));
 await user.click(screen.getByLabelText(/New volume for model/));expect(change.mock.calls[change.mock.calls.length-1]?.[1]).not.toBe('');
 await user.type(screen.getByLabelText('model existing StorageClass'),'managed-csi');await user.type(screen.getByLabelText('model model file HTTPS URL'),'https://models.example/file.gguf');await user.type(screen.getByLabelText('model model SHA-256'),'e'.repeat(64));await user.type(screen.getByLabelText('model exact model bytes'),'5');await user.type(screen.getByLabelText('Existing Go 1.25+ builder image · repository@sha256'),'registry.example/go@sha256:'+'f'.repeat(64));
 const last=change.mock.calls[change.mock.calls.length-1];expect(last?.[1]).toBe('');expect(last?.[0]).toMatchObject({volumes:[{serviceId:'model',storageClass:'managed-csi',sizeGiB:10}],models:[{bytes:5,sha256:'e'.repeat(64)}],acknowledgeNewResources:true});
});
it('keeps resource inputs disabled after access withdrawal and makes no provider request',async()=>{
 const change=vi.fn(),user=userEvent.setup();const view=render(<RuntimeResourceFields saved={saved} disabled={false} onChange={change}/>);await user.click(screen.getByText('Prepare model storage and private runtime connections'));await user.click(screen.getByLabelText(/New volume for model/));view.rerender(<RuntimeResourceFields saved={saved} disabled onChange={change}/>);
 expect(screen.getByLabelText('model existing StorageClass').matches(':disabled')).toBe(true);expect(screen.getByText(/Existing volumes and credentials are never overwritten/)).toBeTruthy();
});
