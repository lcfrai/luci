// Controlled source-component fixture: no HTTP backend, Docker or model execution.
import '../../../desktop/src/styles/app.css';
import '../../../desktop/src/components/journey/journey.css';
import React,{useState} from 'react';
import {createRoot} from 'react-dom/client';
import {ProjectOverviewBoard} from '../../../desktop/src/components/projects/ProjectOverviewBoard';
import {ProjectSwarm} from '../../../desktop/src/components/journey/ProjectSwarm';
import {ReviewedContextNote} from '../../../desktop/src/components/develop/ReviewedContextNote';
import {aSession} from '../../../desktop/src/test/fixtures';
import {projectOverviewApi} from '../../../desktop/src/api/projectOverview';
import {swarmHandoffApi} from '../../../desktop/src/api/swarmHandoff';
import {swarmCheckpointApi} from '../../../desktop/src/api/swarmCheckpoint';
import {projectRunsApi} from '../../../desktop/src/api/projectRuns';
import {api} from '../../../desktop/src/api';
import {useCockpit,resetCockpit} from '../../../desktop/src/store/store';
import {handoffFixture,handoffAnswerResult} from '../../../desktop/src/test/handoffFixtures';
import {swarmHarnessFixture} from '../../../desktop/src/test/swarmFixtures';
const projects=['alpha','beta','gamma','delta'].map(id=>({id,name:id.charAt(0).toUpperCase()+id.slice(1),path:'/fixture/'+id}));
const work=[{id:'earlier-task-review',state:'awaiting-checks',kind:'task-review',contextStatus:'current'}, {id:'earlier-verification',state:'awaiting-checks',kind:'verification',contextStatus:'changed'},{id:'cancelled-parent-with-retained-agent-output',state:'cancelled',kind:'task-review',contextStatus:'unavailable'}];
const summary=id=>({projectId:id,observedAt:'2026-10-08T12:00:00Z',status:'ready',goal:{text:'Build a useful, private notes app with reviewed shared context.',revision:1,updatedAt:'',truncated:false},phase:{id:'develop',label:'Develop'},context:{briefRevision:1,specificationRevision:1,definitionStatus:'approved',designRevision:1,designStatus:'approved',planRevision:2,tasks:{total:3,recordedDone:0}},questions:{status:'recorded',count:0,items:[],omitted:0},execution:{scope:'swarm-queue',status:'recorded',observedAt:'',running:0,queued:0,paused:0,reserved:0,blocked:0,unknown:0,dispatchPaused:false},latestRun:{status:'recorded',id:'newest-prepared',state:'ready',contextStatus:'current'},attention:{status:'recorded',count:4,items:work,omitted:1},nextAction:{id:'inspect-run',label:'Review earlier task changes',lens:'develop',runId:'earlier-task-review'},limitations:[]});
const f=handoffFixture();f.operation.handoff={status:'captured',digest:f.handoff.digest,summary:'Worker claim',questions:1,unanswered:1,capturedAt:f.run.createdAt};f.run.operations.push({...f.operation,id:'op-b',taskId:'b',childId:'child-b',worktreeId:'task-b',status:'failed'});f.run.swarm.tasks[1].operationId='op-b';
f.operation.message="The agent process ended. Its exit code and task correctness are not established by this observation. Review retained changes and verification evidence before integration.";f.run.operations.find(op=>op.id==='op-b').message='The agent stopped without a confirmed successful result. Retain and inspect its checkout before retrying.';
const mode=new URLSearchParams(location.search).get('mode');
if(mode==='earlier') {
  f.operation.attempt=2;
  f.run.operations.unshift({...f.operation,id:'earlier-question-op',childId:'earlier-question-child',worktreeId:'earlier-question-tree',attempt:1,status:'cancelled'});
  work[0]={id:f.run.id,state:'cancelled',kind:'task-question',contextStatus:'current'};
}
const capturedSession=aSession({id:'context-session',projectId:f.run.projectId,worktreeId:'task-a',title:'Dependency-bound task',implementationContext:{...f.run.input.implementation,worktreeId:'task-a',dependencyContextDigest:'d'.repeat(64)},reviewedContextDelivery:{status:'captured',version:2,digest:'e'.repeat(64)}});
let currentRun=f.run,currentHandoff=f.handoff;
window.__handoffFixture={writes:[],reads:[],starts:0,selection:()=>useCockpit.getState().projectRunInspection};
resetCockpit();useCockpit.setState({projects:[...projects,{id:f.run.projectId,name:'North star',path:'/fixture/north-star'}],harnesses:[swarmHarnessFixture()]});
projectOverviewApi.get=async id=>summary(id);
swarmHandoffApi.get=async(_project,_run,operation)=>{
  window.__handoffFixture.reads.push({projectId:_project,runId:_run,operationId:operation.id});
  return structuredClone(operation.id==='earlier-question-op'
    ? {...currentHandoff,operationId:operation.id,receipt:{...currentHandoff.receipt,operationId:operation.id,childId:operation.childId,worktreeId:operation.worktreeId,reported:{...currentHandoff.receipt.reported,summary:'An earlier attempt still needs this decision.'}}}
    : currentHandoff);
};
projectRunsApi.get=async()=>structuredClone(currentRun);
swarmHandoffApi.answer=async(run,op,digest,id,answer)=>{window.__handoffFixture.writes.push({kind:'answer',projectId:run.projectId,operationId:op.id,revision:run.revision,id,digest,answer});const result=handoffAnswerResult(run,currentHandoff,answer);currentRun=result.run;currentHandoff=result.handoff;return result;};
swarmHandoffApi.collect=async()=>{throw Error('Capture is outside this controlled witness.');};
swarmCheckpointApi.review=async(run,op)=>({projectId:run.projectId,runId:run.id,operationId:op.id,childId:op.childId,taskId:op.taskId,expectedRevision:run.revision,sourceCommit:'b'.repeat(40),targetCommit:'a'.repeat(40),reviewDigest:'d'.repeat(64),canContinue:true,reason:'Controlled complete changes can seed a new attempt.',totalBytes:8,files:[{path:'src/a.ts',status:'modified',before:'old\n',after:'new\n',beforeMode:'100644',afterMode:'100644'}]});
swarmCheckpointApi.reserve=async(run,op,review,key)=>{window.__handoffFixture.writes.push({kind:'checkpoint',projectId:run.projectId,operationId:op.id,revision:run.revision,digest:review.reviewDigest,key});const operation={...op,handoff:undefined,id:'new-a',childId:'new-child-a',worktreeId:'new-task-a',attempt:op.attempt+1,status:'reserved',checkpoint:{sourceOperationId:op.id,sourceChildId:op.childId,sourceCommit:review.sourceCommit,sourceBaseCommit:'a'.repeat(40),targetCommit:review.targetCommit,seedCommit:'c'.repeat(40),reviewDigest:review.reviewDigest,contextDigest:'e'.repeat(64),createdAt:run.createdAt}};const next={...run,revision:run.revision+1,operations:[...run.operations,operation],swarm:{...run.swarm,tasks:run.swarm.tasks.map(t=>t.id===op.taskId?{...t,operationId:operation.id}:t)}};currentRun=next;if(mode==='checkpoint-lost')throw Error('Controlled response lost after reservation.');return {run:next,operation};};
api.getSessionReviewedContext=async(id,document)=>({sessionId:id,projectId:capturedSession.projectId,worktreeId:capturedSession.worktreeId,version:2,digest:capturedSession.reviewedContextDelivery.digest,implementationContext:capturedSession.implementationContext,document:{kind:document,sha256:'f'.repeat(64),bytes:44,value:{purpose:'Captured V2 '+document,revision:1}}});
api.createSession=async()=>{window.__handoffFixture.starts++;throw Error('Execution is forbidden in this controlled witness.');};
function Swarm(){const [run,setRun]=useState(currentRun),[busy,setBusy]=useState('');return <ProjectSwarm run={run} canEdit canExecute busy={!!busy} onBusy={setBusy} onUpdate={setRun} onOpenSession={async()=>{throw Error('Session inspection is outside this controlled witness.');}}/>;}
function Fixture(){const inspection=useCockpit(state=>state.projectRunInspection);return <main style={{padding:20,maxWidth:1200,margin:'auto'}}><h1 style={{fontSize:20}}>Controlled task review fixture</h1><p style={{fontSize:13}}>Actual source components; controlled responses. No backend, Docker or agent execution.</p>{mode==='home'?<ProjectOverviewBoard projects={projects}/>:mode==='earlier'&&inspection?.runId!==f.run.id?<ProjectOverviewBoard projects={[{id:f.run.projectId,name:'North star',path:'/fixture/north-star'}]}/>:mode==='context'?<ReviewedContextNote session={capturedSession} canInspect/>:<Swarm/>}</main>;}
createRoot(document.getElementById('root')!).render(<Fixture/>);
