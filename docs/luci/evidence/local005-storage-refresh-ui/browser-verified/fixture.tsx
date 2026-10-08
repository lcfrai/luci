import '../../../desktop/src/styles/app.css';
import '../../../desktop/src/components/journey/journey.css';
import React from 'react';
import {createRoot} from 'react-dom/client';
import {SetupPanel} from '../../../desktop/src/components/setup/SetupPanel';
import {createProductClient,type SetupStatus} from '../../../desktop/src/api/product';
import type {ExecutionStorage} from '../../../desktop/src/api/executionStorage';
const scenario=new URLSearchParams(location.search).get('scenario')||'settings';
const status:SetupStatus={product:'Luci',version:'controlled-fixture',platform:'windows',configured:true,config:{projectsDir:'C:\\Projects',mode:'local',cloudProvider:'',region:'',domain:'',monthlyBudgetUsd:0},checks:[
 {id:'git',label:'Git',status:'ready',message:'Recorded Git ready.',action:''},{id:'docker',label:'Docker',status:'ready',message:'Recorded Docker ready.',action:''},
 {id:'sandbox-image',label:'Runtime',status:'ready',message:'Recorded runtime ready.',action:''},{id:'docker-backing-storage',label:'Docker storage readiness',status:'ready',message:'Recorded readiness from the controlled fixture.',action:''}
],cloudProviders:[]};
let observation:ExecutionStorage={platform:'windows',scope:'local-docker-desktop',required:true,state:scenario==='confirmation'?'confirmation-required':'confirmed',revision:1,reasonCode:'fixture',message:'Controlled cached storage observation.',basis:'Controlled fixture; no physical storage measurement.',hostAvailableBytes:12*1024**3,guestAvailableBytes:24*1024**3,mountedVolumeCount:2};
const calls:object[]=[];const statuses:SetupStatus[]=[];let pending=false,complete:(()=>void)|undefined;
declare global{interface Window{__settingsFixture:{calls:object[];statuses:SetupStatus[];closed:number;deferStorage():void;completeStorage():void;markConfirmed():void}}}
window.__settingsFixture={calls,statuses,closed:0,deferStorage(){pending=true;},completeStorage(){complete?.();},markConfirmed(){observation={...observation,state:'confirmed',message:'Controlled location measured; readiness remains separate.'};}};
const fetcher:typeof fetch=async(url,init)=>{
 const path=new URL(String(url)).pathname,method=init?.method||'GET';calls.push({path,method});
 if(path==='/v1/admin/execution-storage'){
  if(method==='POST')observation={...observation,state:'pending-observation',revision:observation.revision+1,message:'Controlled confirmation saved; waiting for measurement.'};
  else if(pending){pending=false;await new Promise<void>(resolve=>{complete=resolve;});}
  return new Response(JSON.stringify(observation));
 }
 if(path==='/v1/setup/check')return new Response(JSON.stringify(status));
 if(path==='/v1/setup/runtime/install')return new Response(JSON.stringify({status:'ready',message:'Recorded runtime fixture.'}));
 if(path==='/v1/setup/git/identity')return new Response(JSON.stringify({name:'Fixture owner',email:'owner@example.invalid'}));
 throw new Error('Unexpected controlled-fixture request '+path);
};
const client=createProductClient(location.origin,fetcher);
createRoot(document.getElementById('root')!).render(<SetupPanel initialStatus={status} presentation={scenario==='portal'?'portal':'dialog'} client={client} onStatus={value=>statuses.push(value)} onClose={()=>{window.__settingsFixture.closed++;}} onProjectsChanged={()=>{}}/>);
