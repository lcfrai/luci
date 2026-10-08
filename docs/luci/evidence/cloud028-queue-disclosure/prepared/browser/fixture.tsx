// Controlled source-component fixture. No Luci service, Docker or model execution.
import '../../../luci/desktop/src/styles/app.css';
import '../../../luci/desktop/src/components/journey/journey.css';
import React, {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {SwarmRunQueue} from '../../../luci/desktop/src/components/journey/SwarmRunQueue';
import {productApi} from '../../../luci/desktop/src/api/product';
import {ApiError, type ProblemCode} from '../../../luci/desktop/src/api/client';
import {type ProjectSwarmQueue} from '../../../luci/desktop/src/api/swarmQueue';
import {resetCockpit, useCockpit} from '../../../luci/desktop/src/store/store';
import {swarmHarnessFixture, swarmRunFixture} from '../../../luci/desktop/src/test/swarmFixtures';

const selected = new URLSearchParams(location.search).get('project') ?? 'p-a';
if (selected !== 'p-a' && selected !== 'p-b') throw Error('Unknown controlled queue project');
const projectId = selected;
const runId = projectId === 'p-a' ? 'run-a' : 'run-b';
const tasks = projectId === 'p-a'
  ? [{id:'build-a', title:'Build a'}, {id:'cancel-a', title:'Cancel this extra queued task'}]
  : [{id:'build-b', title:'Build b'}];
const run = swarmRunFixture();
run.projectId = projectId;
run.id = runId;
run.input.implementation.projectId = projectId;
run.input.implementation.taskIds = tasks.map(task => task.id);
run.input.implementation.taskTitles = tasks.map(task => task.title);
run.swarm!.tasks = tasks.map(task => ({...task, dependsOn:[], writePaths:[task.id+'-result.json'], acceptance:'Controlled '+task.title+' result'}));

declare global {
  interface Window {
    __queueFixture: {
      projectId: string;
      runId: string;
      queue: ProjectSwarmQueue | null;
      queueChanges: number;
      busy: string;
    };
  }
}
window.__queueFixture = {projectId, runId, queue:null, queueChanges:0, busy:''};
resetCockpit();
useCockpit.getState().setHarnesses([swarmHarnessFixture()]);

// Preserve the actual swarmQueueApi ownership/revision/receipt validation.
// Only its HTTP transport is controlled by the browser runner's middleware.
productApi.request = async <T,>(path: string, body?: unknown,
  method = body === undefined ? 'GET' : 'POST', _budgetMs = 120_000,
  signal?: AbortSignal): Promise<T> => {
  const response = await fetch(path, {
    method, signal,
    ...(body === undefined ? {} : {headers:{'Content-Type':'application/json'}, body:JSON.stringify(body)}),
  });
  const raw = await response.text();
  let value: unknown;
  try { value = raw === '' ? undefined : JSON.parse(raw); }
  catch { throw new ApiError('internal', 'Controlled queue response was unreadable', response.status); }
  if (!response.ok) {
    const problem = value as {error?: string; message?: string} | undefined;
    throw new ApiError((problem?.error ?? 'internal') as ProblemCode,
      problem?.message ?? 'Controlled queue request failed', response.status);
  }
  return value as T;
};

function Fixture() {
  const [busy, setBusy] = useState('');
  return <main style={{padding:20, maxWidth:1100, margin:'auto'}}>
    <h1>Controlled CLOUD028 component fixture</h1>
    <h2>Current project {projectId}</h2>
    <p>Actual SwarmRunQueue component with controlled HTTP responses. No agent or execution backend.</p>
    <SwarmRunQueue run={run} canEdit canExecute busy={!!busy}
      onBusy={label => {window.__queueFixture.busy=label; setBusy(label);}}
      onQueueChange={queue => {window.__queueFixture.queue=structuredClone(queue); window.__queueFixture.queueChanges++;}} />
  </main>;
}

createRoot(document.getElementById('root')!).render(<Fixture />);
