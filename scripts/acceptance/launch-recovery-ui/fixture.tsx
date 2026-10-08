import '../../../desktop/src/styles/app.css';
import '../../../desktop/src/components/journey/journey.css';
import '../../../desktop/src/components/setup/setup.css';
import React, {useEffect, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {ProjectSwarm} from '../../../desktop/src/components/journey/ProjectSwarm';
import {projectSwarmApi, type SwarmRecoveryAvailability} from '../../../desktop/src/api/projectSwarm';
import {projectRunsApi, type ProjectRun} from '../../../desktop/src/api/projectRuns';
import {useCockpit} from '../../../desktop/src/store/store';
import {swarmHarnessFixture, swarmRunFixture} from '../../../desktop/src/test/swarmFixtures';

// Explicit callback fixtures, not real recovery, worker or Docker observations.
const mode = new URLSearchParams(location.search).get('scenario') || 'supported';
let saved = swarmRunFixture();
saved = {...saved, operations: [{id:'op-a', childId:'child-a', taskId:'a', worktreeId:'isolated-a', attempt:2, kind:'session', status:'unknown', allocation:'ready', requestDigest:'fixture-only', createdAt:saved.createdAt, updatedAt:saved.updatedAt}], swarm:{...saved.swarm!, tasks:[{...saved.swarm!.tasks[0], operationId:'op-a', title:'Preserve customer amendments during a coordinated build'}]}};
const calls: object[] = [];
let finishRead: (() => void) | undefined;
declare global {interface Window {__recoveryFixture: {calls: object[]; switchProject(): void; completeRead(): void}}}
window.__recoveryFixture = {calls, switchProject(){}, completeRead(){finishRead?.();}};
projectSwarmApi.recovery = async (projectId, run, operation) => {
  calls.push({method:'GET availability', projectId, runId:run.id, operationId:operation.id, revision:run.revision});
  const value: SwarmRecoveryAvailability = {expectedRevision:run.revision, operationId:operation.id, childId:operation.childId, availability:{supported:mode !== 'legacy', reasonCode:mode === 'legacy' ? 'legacy_launch' : 'supported', message:mode === 'legacy' ? 'This older launch has no recovery protocol. Its reservation stays held for host inspection.' : 'This exact saved attempt supports reviewed launch recovery.'}};
  if (mode === 'delayed') await new Promise<void>(resolve => {finishRead=resolve;});
  return value;
};
projectSwarmApi.abandon = async (projectId, runId, taskId, expectedRevision, consent) => {
  calls.push({method:'POST stop',projectId,runId,taskId,expectedRevision,...consent});
  const uncertain = mode === 'unknown-response';
  saved = {...saved, revision:saved.revision+1, operations:[{...saved.operations[0], status:uncertain?'unknown':'cancelled', recovery:{state:uncertain?'pending':'finalized', requestedAt:saved.updatedAt,...(uncertain?{}:{provedAt:saved.updatedAt,finalizedAt:saved.updatedAt})}}]};
  if (uncertain) throw new Error('Controlled lost response: recovery may have advanced.');
  return saved;
};
projectRunsApi.get = async (projectId, runId) => {calls.push({method:'GET readback',projectId,runId});return saved;};
useCockpit.getState().setHarnesses([swarmHarnessFixture()]);
useCockpit.getState().setProjects([{id:'p-1',name:'Customer planning app',path:'/fixture/customer-app'},{id:'p-2',name:'Operations dashboard',path:'/fixture/operations'}]);
function Fixture(){
  const [run,setRun]=useState<ProjectRun>(saved),[busy,setBusy]=useState('');
  useEffect(()=>{window.__recoveryFixture.switchProject=()=>{saved={...saved,projectId:'p-2',input:{...saved.input,implementation:{...saved.input.implementation,projectId:'p-2'}}};setRun(saved);};},[]);
  return <main style={{maxWidth:1080,margin:'0 auto',height:'100dvh',overflowY:'auto',padding:20}}><h1>Controlled recovery UI witness</h1><p>Actual source controls with fixture responses. No Docker, worker, backend or model execution.</p><ProjectSwarm run={run} canEdit canExecute busy={!!busy} onBusy={setBusy} onUpdate={setRun} onOpenSession={async()=>{throw new Error('Session execution is unavailable in this fixture.');}}/></main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
