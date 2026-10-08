// LOCAL-015: real check UI and hooks, controlled account/team and API responses.
// No HTTP check execution, backend, worker, provider or allocation is exercised.
import '../../../desktop/src/styles/app.css';
import React, {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {ProjectChecks} from '../../../desktop/src/components/journey/ProjectChecks';
import {productApi, type CheckRun} from '../../../desktop/src/api/product';
import {ApiError, type ProblemCode} from '../../../desktop/src/api/client';
import {resetCockpit} from '../../../desktop/src/store/store';

const commands = [{label:'Controlled acceptance command', executable:'node', args:['check.mjs']}];
const events: Record<string, unknown>[] = [];
const subscribers = new Set<() => void>();
let permissions = {read:true, edit:true, execute:true, secrets:true};
let pending: {resolve(run: CheckRun): void; reject(error: Error): void; projectId: string} | null = null;
let selected = 'alpha';
let selectProject: (projectId: string) => void;
const event = (kind: string, details = {}) => events.push({number:events.length + 1, kind, ...details});
const coverage = (projectId: string) => ({projectId, specification:{revision:1,status:'approved',digest:'spec'},design:{revision:1,status:'approved',digest:'design'},source:{available:true,commit:'a'.repeat(40),sourceDigest:'source',dirty:false},rows:[]});
productApi.checks = async projectId => { event('history', {projectId}); return {runs:[],suggested:commands,coverage:coverage(projectId)}; };
productApi.runChecks = async (projectId, values, trusted, execution) => {
  event('start', {projectId, commands:values, trusted, execution});
  if (pending) throw Error('The controlled witness received a second start before settlement.');
  return new Promise<CheckRun>((resolve, reject) => { pending = {resolve,reject,projectId}; });
};
const control = {
  subscribe(listener: () => void) { subscribers.add(listener); return () => { subscribers.delete(listener); }; },
  permissionSnapshot: () => permissions,
  setEdit(edit: boolean) { permissions = {...permissions,edit}; event('edit', {edit}); subscribers.forEach(listener => listener()); },
  settle(outcome: 'rejection' | 'success') {
    if (!pending) throw Error('No exact pending start to settle.');
    const original = pending; pending = null; event('settled', {outcome,projectId:original.projectId});
    if (outcome === 'rejection') original.reject(new ApiError('request_timeout' as ProblemCode,'Private old-authority failure detail',0));
    else original.resolve({id:'private-alpha-check',projectId:original.projectId,status:'passed',execution:'docker',dirty:false,createdAt:'2026-10-08',updatedAt:'2026-10-08',message:'Private old-authority successful receipt',commands:[],evidenceStatus:'unverified'});
  },
  switchProject(projectId: string) { event('selection', {projectId}); selected = projectId; selectProject(projectId); },
  snapshot: () => ({events, selected, permissions, pending:!!pending, starts:events.filter(row => row.kind === 'start').length, histories:events.filter(row => row.kind === 'history').length}),
};
Object.assign(window, {__local015:control});
resetCockpit();
function Fixture() {
 const [projectId,setProjectId] = useState('alpha'); selectProject = setProjectId;
 return <main style={{maxWidth:960,padding:16,margin:'0 auto'}}><h1>Controlled LOCAL-015 check start witness</h1><p>Selected project: {projectId}</p><ProjectChecks projectId={projectId} /></main>;
}
createRoot(document.getElementById('root')!).render(<Fixture />);
