import '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/styles/app.css';
import React from 'react';
import {createRoot} from 'react-dom/client';
import {AppShell} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/components/AppShell';
import {resetCockpit,useCockpit} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/store/store';
import {aProject,aMachine} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/test/fixtures';
import {context,view} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/components/orchestrator/fixtures';
import {projectOverviewApi} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/api/projectOverview';
import {swarmSchedulerApi} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/api/swarmScheduler';
import {definitionApi} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/api/definition';
import {orchestratorApi} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/api/orchestrator';
import {productApi} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/api/product';
import {ApiError} from '/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/controlled/source/desktop/src/api/client';
const ids=['alpha','beta','gamma','delta'];
const values=['Identical synthetic saved goal.','Identical synthetic saved goal.','Synthetic gamma goal.','Synthetic delta goal.'];
let definitionDelay=0,definitionDenied=false;
const accessListeners=new Set<()=>void>();
let access={canRead:true,canEdit:true,canExecute:true,loading:false,error:'',message:''};
window.__readinessAccess={subscribe:(fn:()=>void)=>{accessListeners.add(fn);return ()=>accessListeners.delete(fn);},get:()=>access};
const pending=new Map<string,Array<{resolve:(v:any)=>void;reject:(e:Error)=>void}>>();
const reads:Array<{kind:string;id?:string}>=[];
function defer(key:string){return new Promise<any>((resolve,reject)=>pending.set(key,[...(pending.get(key)||[]),{resolve,reject}]));}
function summary(id:string){return {projectId:id,observedAt:new Date().toISOString(),status:'ready',goal:{text:'Build '+id+' with durable project context and reviewed task results.',revision:1,updatedAt:'',truncated:false},phase:{id:'develop',label:'Develop'},context:{briefRevision:1,specificationRevision:1,definitionStatus:'approved',designRevision:1,designStatus:'approved',planRevision:2,tasks:{total:4,recordedDone:1}},questions:{status:'recorded',count:1,items:[{id:'question-'+id,source:'definition',text:'Which deployment target should '+id+' use?',truncated:false}],omitted:0},execution:{scope:'swarm-queue',status:'recorded',observedAt:'',running:id==='alpha'?1:0,queued:2,paused:0,reserved:0,blocked:id==='gamma'?1:0,unknown:0,failed:0,dispatchPaused:false},attention:{status:'recorded',count:2,omitted:0,items:[{id:'run-'+id,state:'awaiting-checks',kind:'task-review',contextStatus:'current'},{id:'older-run-'+id,state:'awaiting-checks',kind:'task-review',contextStatus:'current'}]},nextAction:{id:'inspect-run',runId:'run-'+id,label:'Review retained task changes',lens:'develop'},limitations:['Counts are recorded observations, not a live process check.']};}
function capacity(){const now=new Date().toISOString();return {revision:1,observedAt:now,dispatchPaused:false,policy:{maxActive:4,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:2048},resources:{host:{observedAt:now,status:'ready',platform:'linux',scope:'controlled fixture',memory:{effectiveTotalBytes:32*1024**3,effectiveAvailableBytes:8*1024**3,limitSource:'fixture'},cpu:{logicalProcessors:8,effectiveMillis:8000,utilizationPercent:20},disks:[],unavailable:[]},execution:{observedAt:now,status:'ready',scope:'controlled fixture',totalMemoryBytes:16*1024**3,cpuMillis:4000,unavailable:[]},reservations:{status:'ready',observedAt:now,count:1,memoryMiB:1024,cpuMillis:1000}}};}
projectOverviewApi.get=(id,signal)=>{reads.push({kind:'overview',id});return defer(id);};
swarmSchedulerApi.get=()=>{reads.push({kind:'capacity'});return defer('capacity');};
swarmSchedulerApi.configure=async()=>{throw Error('No scheduling mutation permitted.');};
definitionApi.get=async id=>{const wait=definitionDelay;await new Promise(resolve=>setTimeout(resolve,wait));if(definitionDenied)throw new ApiError('permission_denied','Controlled denied read',403);return {...context(id).specification,purpose:values[ids.indexOf(id)]};};
orchestratorApi.get=async id=>view({projectId:id,context:context(id)});
productApi.agentConnections=async()=>({revision:1,activeConnectionId:'',connections:[]});
window.__homeFixture={reads,values,ids,config:()=>({projects:ids.map((id,i)=>({id,name:id,value:values[i]})),rounds:100,authorityGeneration:1}),owners:()=>ids.map((id,i)=>({owner:{id,name:id},goal:values[i]})),delay(ms:number){definitionDelay=ms;},deny(value:boolean){definitionDenied=value;},access(edit:boolean,read=true){access={...access,canEdit:edit,canRead:read,message:read?'':'Controlled access unavailable'};accessListeners.forEach(fn=>fn());},select(id:string|null){useCockpit.getState().selectProject(id);},rename(id:string,name:string){useCockpit.getState().setProjects(useCockpit.getState().projects.map(p=>p.id===id?{...p,name}:p));},selected:()=>useCockpit.getState().selection.projectId,release(key:string,mode='ready'){
 const entries=pending.get(key)||[];pending.delete(key);
 for(const entry of entries){if(mode==='denied')entry.reject(new ApiError('permission_denied','Access revoked',403));else if(mode==='busy')entry.reject(new ApiError('branch_busy','Source is being prepared',409));else{const value=key==='capacity'?capacity():summary(key);if(mode==='warning'){value.execution={...value.execution,status:'unavailable',unknown:1,dispatchPaused:true};value.questions={status:'unavailable',count:null,items:[],omitted:0};value.latestRun={status:'unavailable'};}entry.resolve(value);}}
 return entries.length;
},home:()=>useCockpit.getState().selectProject(null)};
resetCockpit();useCockpit.setState({projects:ids.map(id=>aProject({id,name:id,path:'/fixture/'+id})),projectsLoaded:true,machines:[aMachine()]});
document.documentElement.dataset.theme='light';
const root=createRoot(document.getElementById('root')!);root.render(<AppShell/>);window.__homeFixture.unmount=()=>root.unmount();
