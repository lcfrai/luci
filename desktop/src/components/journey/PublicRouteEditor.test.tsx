import {useState} from 'react';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {afterEach,expect,it} from 'vitest';
import type {BlueprintPublicRoute,BlueprintService} from '../../api/deploymentBlueprint';
import {PublicRouteEditor} from './PublicRouteEditor';
afterEach(cleanup);
const services:BlueprintService[]=[
 {id:'web',kind:'app',image:'',port:8080,replicas:1,resources:{requestMilliCpu:100,limitMilliCpu:500,requestMemoryMiB:128,limitMemoryMiB:512}},
 {id:'model',kind:'inference',image:'',port:8000,replicas:1,resources:{requestMilliCpu:100,limitMilliCpu:500,requestMemoryMiB:128,limitMemoryMiB:512}},
];
function Editor({disabled=false}:{disabled?:boolean}){
 const [value,setValue]=useState<BlueprintPublicRoute>();
 return <><PublicRouteEditor value={value} services={services} disabled={disabled} onChange={setValue}/><output data-testid="draft">{JSON.stringify(value??null)}</output></>;
}
it('makes existing controller DNS and TLS prerequisites explicit and keeps the app backend port',()=>{
 render(<Editor/>);
 fireEvent.click(screen.getByLabelText('Expose one application through reviewed HTTPS routing'));
 expect(screen.getByText('web · internal Service port 8080')).toBeTruthy();
 expect(screen.queryByText('model · internal Service port 8000')).toBeNull();
 fireEvent.change(screen.getByLabelText('Dedicated DNS hostname'),{target:{value:'APP.EXAMPLE.COM'}});
 fireEvent.change(screen.getByLabelText(/^Existing namespace TLS Secret name/),{target:{value:'site-tls'}});
 const draft=JSON.parse(screen.getByTestId('draft').textContent??'null') as BlueprintPublicRoute;
 expect(draft).toMatchObject({serviceId:'web',hostname:'app.example.com',tlsSecretName:'site-tls',profile:'traefik-websecure'});
 expect(screen.getByText(/Luci does not install a controller/)).toBeTruthy();
 expect(screen.queryByRole('link')).toBeNull();
 expect(document.querySelector('input[type="password"]')).toBeNull();
});
it('withdraws only the explicit unsaved route and respects inherited read-only controls',()=>{
 const {rerender}=render(<Editor/>);const enabled=screen.getByLabelText('Expose one application through reviewed HTTPS routing');
 fireEvent.click(enabled);fireEvent.click(enabled);expect(screen.getByTestId('draft').textContent).toBe('null');
 rerender(<Editor disabled/>);expect(screen.getByLabelText('Expose one application through reviewed HTTPS routing').matches(':disabled')).toBe(true);
});
