// Passive observation of alpha integration's ordinary context-refresh reads.
// This is a completion boundary for one cohort, never a source lock or retry.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

export const sourceReadSuffixes = ['/brief','/design','/specification','/design-plan','/git','/orchestrator'];
const problem = code => Object.assign(Error(code), {code});

export function assertSourceReadUI(value) {
  for (const key of ['factsVisible','factsReady','factsCurrentCommit','understandingVisible','understandingReady']) {
    assert.equal(value?.[key], true, 'Source read UI is not ready: '+key);
  }
  assert.equal(value.factsAlert, false);
  assert.equal(value.understandingAlert, false);
}

// Runs in the page through waitForFunction. Hidden or still-pending controls
// cannot pass, and a failed read is not made successful by loading becoming false.
export function sourceReadUI({commit}) {
  const visible = element => !!element && element.getClientRects().length > 0 && getComputedStyle(element).visibility !== 'hidden';
  const facts = document.querySelector('details.project-work');
  const understanding = document.querySelector('aside.orchestrator');
  const button = (root, text) => [...(root?.querySelectorAll('button') ?? [])].find(value => value.textContent.trim() === text);
  const factsButton = button(facts, 'Refresh project facts');
  const understandingButton = button(understanding, 'Refresh project understanding');
  const result = {
    factsVisible:visible(factsButton),
    factsReady:!!factsButton && !factsButton.disabled && ![...(facts?.querySelectorAll('[role=status]') ?? [])].some(value => value.textContent.startsWith('Refreshing saved project facts')),
    factsCurrentCommit:[...(facts?.querySelectorAll('.project-work-facts code') ?? [])].some(value => value.textContent.trim() === commit.slice(0, 8)),
    factsAlert:!!facts?.querySelector('.project-work-body > [role=alert]'),
    understandingVisible:visible(understandingButton),
    understandingReady:!!understandingButton && !understandingButton.disabled && !!understanding?.querySelector('.understanding-next button:not(:disabled)') && ![...(understanding?.querySelectorAll('[role=status]') ?? [])].some(value => /^(Updating saved|Loading) project understanding|^Showing last recorded context/.test(value.textContent)),
    understandingAlert:!!understanding?.querySelector('.orchestrator-error[role=alert]'),
  };
  if (result.factsAlert || result.understandingAlert) throw Error('source-read-cohort-ui-read-failed');
  return Object.entries(result).every(([key,value]) => key.endsWith('Alert') ? !value : value) ? result : false;
}

export function observeSourceReadCohort({page, origin, projectPath, mutationPath, assertActive, timeoutMs, persist}) {
  assert(Number.isFinite(timeoutMs) && timeoutMs > 0);
  assert.match(projectPath, /^\/v1\/product\/projects\/p-[a-f0-9]+$/);
  assert(mutationPath.startsWith(projectPath+'/runs/') && mutationPath.endsWith('/swarm/tasks/task-a/integrate'));
  const expectedPaths = sourceReadSuffixes.map(suffix => projectPath+suffix);
  const projectID = projectPath.split('/').at(-1);
  const receipt = {phase:'armed', armedAt:new Date().toISOString(), expectedPaths, mutation:null, requests:[], stableUIBoundary:null};
  const requests = new Map(), bodies = new Map();
  let active = true, failure, timer, finishStarted = false, resolveReads, rejectFailure;
  const readsDone = new Promise(resolve => { resolveReads = resolve; });
  const failed = new Promise((_, reject) => { rejectFailure = reject; });
  void failed.catch(() => {});
  const save = () => persist(structuredClone(receipt));
  const detach = () => { clearTimeout(timer); page.off('request', onRequest); page.off('requestfinished', onFinished); page.off('requestfailed', onFailed); page.off('close', onClose); };
  function cancel(error = problem('source-read-cohort-cancelled')) {
    if (!active) return;
    failure = error; active = false; detach();
    receipt.phase = 'failed'; receipt.failureCode = typeof error.code === 'string' ? error.code.slice(0,128) : 'source-read-cohort-failed';
    receipt.finishedAt = new Date().toISOString();
    try { save(); } catch (writeError) { failure = writeError; }
    rejectFailure(failure);
  }
  function usable() { if (!active) throw failure ?? problem('source-read-cohort-closed'); assertActive(); }
  function observed(request) {
    const url = new URL(request.url());
    return url.origin === origin && url.search === '' ? {method:request.method(), path:url.pathname} : null;
  }
  function onRequest(request) {
    if (!active) return;
    try {
      usable(); const value = observed(request); if (!value) return;
      const mutation = value.method === 'POST' && value.path === mutationPath;
      if (!mutation && !(receipt.mutation && value.method === 'GET' && expectedPaths.includes(value.path))) return;
      if (mutation ? receipt.mutation : receipt.requests.some(row => row.path === value.path)) throw problem('source-read-cohort-duplicate-request');
      const row = {...value, startedAt:new Date().toISOString(), finishedAt:null, status:null, state:'pending'};
      requests.set(request, row);
      if (mutation) { receipt.mutation = row; receipt.phase = 'mutation-requested'; }
      else receipt.requests.push(row);
      save();
    } catch (error) { cancel(error); }
  }
  async function finishRequest(request) {
    const row = requests.get(request); if (!row || !active) return;
    usable();
    const response = await request.response(); usable();
    if (!response) throw problem('source-read-cohort-response-missing');
    row.status = response.status(); save();
    const ended = await response.finished(); usable();
    if (ended) throw problem('source-read-cohort-response-unfinished');
    const bytes = await response.body(); usable();
    row.bytes = bytes.length; row.finishedAt = new Date().toISOString();
    if (bytes.length > 2*1024**2) throw problem('source-read-cohort-response-bound');
    row.bodySHA256 = createHash('sha256').update(bytes).digest('hex');
    row.state = 'finished'; save();
    if (row.status !== 200) throw problem('source-read-cohort-status-'+row.status);
    const body = JSON.parse(bytes.toString('utf8'));
    if (row !== receipt.mutation) {
      if (row.path !== projectPath+'/git' && body.projectId !== projectID) throw problem('source-read-cohort-project-mismatch');
      bodies.set(row.path, body);
    }
    if (receipt.mutation?.state === 'finished' && expectedPaths.every(path => bodies.has(path))) resolveReads();
  }
  function onFinished(request) { void finishRequest(request).catch(error => { if (active) cancel(error); }); }
  function onFailed(request) {
    const row = requests.get(request); if (!row || !active) return;
    row.state = 'failed'; row.finishedAt = new Date().toISOString(); cancel(problem('source-read-cohort-request-failed'));
  }
  function onClose() { cancel(problem('source-read-cohort-page-closed')); }
  page.on('request', onRequest); page.on('requestfinished', onFinished); page.on('requestfailed', onFailed); page.on('close', onClose);
  timer = setTimeout(() => cancel(problem('source-read-cohort-deadline-exceeded')), timeoutMs);
  try { usable(); save(); } catch (error) { cancel(error); throw error; }
  return {
    cancel,
    async finish({commit, readUI}) {
      assert.equal(finishStarted, false); finishStarted = true;
      try {
        usable(); assert.match(commit, /^[a-f0-9]{40,64}$/);
        await Promise.race([readsDone, failed]); usable();
        const git = bodies.get(projectPath+'/git');
        assert.equal(git.clean, true); assert.equal(git.log?.[0]?.hash, commit);
        const context = bodies.get(projectPath+'/orchestrator')?.context;
        assert.equal(context?.projectId, projectID); assert.equal(context?.sourceCommit, commit);
        assert.equal(context?.sourceKnown, true); assert.equal(context?.sourceDirty, false);
        receipt.phase = 'waiting-for-ready-ui'; save();
        const pendingUI = Promise.resolve().then(() => { usable(); return readUI(usable); });
        void pendingUI.catch(() => {});
        const ui = await Promise.race([pendingUI, failed]); usable();
        assertSourceReadUI(ui);
        receipt.stableUIBoundary = {...ui, commit, at:new Date().toISOString()};
        receipt.phase = 'ready'; receipt.finishedAt = new Date().toISOString(); save();
        active = false; detach(); bodies.clear(); return structuredClone(receipt);
      } catch (error) { cancel(error); throw error; }
    },
  };
}
