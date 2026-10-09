// Actual source components and CSS, controlled HTTP transport only.
import '/workspace/luci/desktop/src/styles/app.css';
import '/workspace/luci/desktop/src/components/journey/journey.css';
import React, {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {ProjectSwarm} from '/workspace/luci/desktop/src/components/journey/ProjectSwarm';
import {FluidProvider} from '/workspace/luci/desktop/src/components/fluid/FluidProvider';
import {productApi} from '/workspace/luci/desktop/src/api/product';
import {ApiError} from '/workspace/luci/desktop/src/api/client';
import type {ProjectRun} from '/workspace/luci/desktop/src/api/projectRuns';
import {resetCockpit, useCockpit} from '/workspace/luci/desktop/src/store/store';
import {swarmHarnessFixture} from '/workspace/luci/desktop/src/test/swarmFixtures';

declare global {interface Window {__cloud048:{mode:string;run:ProjectRun;busy:string;unexpectedSessionOpens:number;switchProject:()=>void}}}
const bootstrap = await fetch('/controlled/bootstrap').then(response=>response.json());
resetCockpit();useCockpit.getState().setHarnesses([swarmHarnessFixture()]);
window.__cloud048={mode:bootstrap.mode,run:bootstrap.run,busy:'',unexpectedSessionOpens:0,switchProject:()=>{throw Error('Fixture not mounted');}};
// Preserve projectRunsApi/projectSwarmApi/swarmQueueApi identity and response checks.
productApi.request=async<T,>(path:string,body?:unknown,method=body===undefined?'GET':'POST',_budget=120000,signal?:AbortSignal):Promise<T>=>{
 const response=await fetch(path,{method,signal,...(body===undefined?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})});
 const value=await response.json();if(!response.ok)throw new ApiError('internal',value.message??'Controlled request refused',response.status);return value as T;
};
function Fixture(){
 const [run,setRun]=useState<ProjectRun>(bootstrap.run),[busy,setBusy]=useState('');
 window.__cloud048.run=run;window.__cloud048.busy=busy;
 window.__cloud048.switchProject=()=>setRun(bootstrap.nextRun);
 return <FluidProvider><main id="fixture-scroll" style={{height:'100vh',overflow:'auto',padding:20}}><div style={{maxWidth:1100,margin:'0 auto'}}>
  <h1>Recovery guidance browser check</h1><p>Actual Luci components; controlled transport. No execution service.</p>
  <ProjectSwarm run={run} canEdit canExecute={bootstrap.mode!=='execute-revoked'} busy={!!busy} onBusy={setBusy} onUpdate={setRun}
   onOpenSession={async()=>{window.__cloud048.unexpectedSessionOpens++;throw Error('No session may open in this check');}} />
 </div></main></FluidProvider>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
