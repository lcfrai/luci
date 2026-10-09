import '/workspace/luci/desktop/src/styles/app.css';
import React from 'react';
import {createRoot} from 'react-dom/client';
import {AppShell} from '/workspace/luci/desktop/src/components/AppShell';
import {resetCockpit,useCockpit} from '/workspace/luci/desktop/src/store/store';
import {aProject,aMachine} from '/workspace/luci/desktop/src/test/fixtures';
import {context,view} from '/workspace/luci/desktop/src/components/orchestrator/fixtures';
import {projectOverviewApi} from '/workspace/luci/desktop/src/api/projectOverview';
import {swarmSchedulerApi} from '/workspace/luci/desktop/src/api/swarmScheduler';
import {definitionApi} from '/workspace/luci/desktop/src/api/definition';
import {orchestratorApi} from '/workspace/luci/desktop/src/api/orchestrator';
import {productApi} from '/workspace/luci/desktop/src/api/product';
import {requestProjectContextEdit} from '/workspace/luci/desktop/src/api/projectContextEvents';
const ids=['alpha','beta','gamma','delta'];
const text=(id:string,kind:string,n:number)=>`${id} ${kind} ${n}: ${'Retain exact project boundaries, user intent, decisions and evidence. '.repeat(3)}`;
function specification(id:string){return {...context(id).specification,status:'draft',revision:2,briefRevision:3,purpose:`Large context ${id}`,audience:`${id} users`,constraints:`${id} boundaries`,outOfScope:[],
 requirements:Array.from({length:60},(_,i)=>({id:`${id}-AC-${i}`,title:`${id} outcome ${i}`,priority:'must',acceptance:text(id,'acceptance',i)})),
 journeys:Array.from({length:20},(_,i)=>({id:`${id}-journey-${i}`,name:`${id} journey ${i}`,actor:`${id} customer`,steps:text(id,'steps',i),requirementIds:[`${id}-AC-${i%60}`]})),
 roles:Array.from({length:25},(_,i)=>({id:`${id}-role-${i}`,name:`${id} role ${i}`,canDo:text(id,'can',i),cannotDo:text(id,'cannot',i)})),
 entities:Array.from({length:50},(_,i)=>({id:`${id}-entity-${i}`,name:`${id} entity ${i}`,fields:text(id,'fields',i),privacy:text(id,'privacy',i)})),
 integrations:Array.from({length:20},(_,i)=>({id:`${id}-integration-${i}`,name:`${id} integration ${i}`,purpose:text(id,'purpose',i),constraint:text(id,'constraints',i)})),
 glossary:Array.from({length:100},(_,i)=>({id:`${id}-term-${i}`,term:`${id} term ${i}`,meaning:text(id,'meaning',i)})),
 decisions:Array.from({length:50},(_,i)=>({id:`${id}-decision-${i}`,title:`${id} decision ${i}`,decision:text(id,'decision',i),reason:text(id,'reason',i),alternatives:text(id,'alternatives',i)})),
 questions:Array.from({length:30},(_,i)=>({id:`${id}-question-${i}`,question:`${id} question ${i}`,answer:i===29?'':text(id,'answer',i)}))};}
const documents=Object.fromEntries(ids.map(id=>[id,specification(id)]));
const reads:string[]=[];const saves:any[]=[];
function summary(id:string){return {projectId:id,observedAt:new Date().toISOString(),status:'ready',goal:{text:`Large context ${id}`,revision:2,updatedAt:'',truncated:false},phase:{id:'define',label:'Define'},context:{briefRevision:3,specificationRevision:2,definitionStatus:'draft',designRevision:0,designStatus:'draft',planRevision:0,tasks:{total:0,recordedDone:0}},questions:{status:'recorded',count:1,items:[{id:`${id}-question-29`,source:'specification',text:`${id} question 29`,truncated:false}],omitted:0},execution:{scope:'swarm-queue',status:'recorded',observedAt:'',running:0,queued:0,paused:0,reserved:0,blocked:0,unknown:0,failed:0,dispatchPaused:false},attention:{status:'recorded',count:0,omitted:0,items:[]},nextAction:{id:'define',label:'Review definition',lens:'discover'},limitations:[]};}
projectOverviewApi.get=async id=>summary(id);
swarmSchedulerApi.get=async()=>({revision:1,observedAt:new Date().toISOString(),dispatchPaused:false,policy:{maxActive:4,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:2048},resources:{host:{observedAt:new Date().toISOString(),status:'unavailable',platform:'linux',scope:'controlled fixture',memory:{},cpu:{},disks:[],unavailable:['No runtime observation in this fixture.']},execution:{status:'unavailable',scope:'controlled fixture',unavailable:['No runtime.']},reservations:{status:'ready',count:0,memoryMiB:0,cpuMillis:0}}});
definitionApi.get=async id=>{reads.push(id);return structuredClone(documents[id]);};
definitionApi.save=async(id,value,revision,briefRevision)=>{if(revision!==documents[id].revision||briefRevision!==documents[id].briefRevision)throw Error('Stale fixture revision');saves.push({id,value:structuredClone(value),revision,briefRevision});documents[id]={...structuredClone(value),revision:revision+1};return structuredClone(documents[id]);};
definitionApi.history=async id=>({revisions:[structuredClone(documents[id])]});
definitionApi.listAssistance=async()=>({jobs:[]});
definitionApi.assist=async()=>{throw Error('Provider calls are forbidden in this fixture.');};
orchestratorApi.get=async id=>view({projectId:id,context:{...context(id),specification:documents[id]}});
productApi.agentConnections=async()=>({revision:1,activeConnectionId:'',connections:[]});
window.__scaleFixture={reads,saves,documents,permissionVersion:'read-edit',selected:()=>useCockpit.getState().selection.projectId,permissions(value:string){this.permissionVersion=value;window.dispatchEvent(new Event('fixture:permissions'));},question(id:string,index=29){const scope=JSON.stringify([productApi.baseUrl,'fixture','local','','','fixture','/fixture',id]);requestProjectContextEdit(scope,id,'definition',`${id}-question-${index}`);},missing(id:string){documents[id].journeys[0].requirementIds.push('removed-outcome');},home:()=>useCockpit.getState().selectProject(null)};
resetCockpit();useCockpit.setState({projects:ids.map(id=>aProject({id,name:id,path:'/fixture/'+id})),projectsLoaded:true,machines:[aMachine()]});
document.documentElement.dataset.theme='light';
createRoot(document.getElementById('root')!).render(<AppShell/>);
