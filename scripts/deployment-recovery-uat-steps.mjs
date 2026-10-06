// Real day-two preview recovery, invoked by an owning browser journey after its
// working preview and accepted run exist. This module creates no independent
// process, fixture, provider, or Docker lifecycle. Every mutation uses the UI.
import assert from 'node:assert/strict';

export async function exerciseDeploymentRecovery({ page, projectId, projectRoute, release, lens, mutation, observe, check, screenshot, onRelease }) {
  assert.equal(release.projectId, projectId);
  assert.equal(release.target, 'local');
  assert.equal(release.status, 'ready');
  assert.match(release.id, /^rel-[a-f0-9]{24}$/);
  assert.match(release.commit, /^[a-f0-9]{40}([a-f0-9]{24})?$/);
  assert.equal(typeof onRelease, 'function', 'The owning journey must retain every created release for cleanup');
  const mutations = [], pendingReceipts = new Set();
  const request = req => {
    const path = new URL(req.url()).pathname;
    if (path.startsWith(projectRoute + '/') && !['GET', 'HEAD', 'OPTIONS'].includes(req.method())) mutations.push({ path, method: req.method(), body: req.postDataJSON() });
  };
  const response = res => {
    if (res.request().method() !== 'POST' || new URL(res.url()).pathname !== projectRoute + '/releases' || !res.ok()) return;
    // Record ownership promptly even if a later UI assertion fails. The driver
    // also recovers private owned receipts during its final cleanup.
    const pending = res.json().then(value => {
      if (value.projectId === projectId && /^rel-[a-f0-9]{24}$/.test(value.id)) onRelease(value.id);
    }).catch(() => {}).finally(() => pendingReceipts.delete(pending));
    pendingReceipts.add(pending);
  };
  page.on('request', request); page.on('response', response);
  try {
    await lens('Deploy');
    const overview = page.getByRole('region', { name: 'Application status and recovery', exact: true });
    await overview.getByRole('heading', { name: 'Your applications', exact: true }).waitFor();
    // This fixture owns exactly one initial preview. Refuse ambiguous discovery
    // rather than silently selecting another retained application.
    const entry = overview.getByRole('listitem').filter({ hasText: 'Source ' + release.commit.slice(0, 12) });
    await entry.getByRole('button', { name: 'Inspect workspace preview', exact: true }).waitFor();
    assert.equal(await entry.count(), 1, 'Retained release discovery must identify one exact fixture candidate');
    assert((await entry.innerText()).includes('Last verified healthy'));
    await entry.getByRole('button', { name: 'Inspect workspace preview', exact: true }).click();
    const original = page.getByRole('listitem', { name: 'Release ' + release.id, exact: true });
    await original.waitFor();
    await page.waitForFunction(id => document.activeElement?.getAttribute('aria-label') === 'Release ' + id, release.id, { timeout: 10_000 });
    assert((await original.innerText()).includes(release.commit.slice(0, 8)));
    assert.equal(await page.getByRole('tab', { name: 'Previews and publishing', exact: true }).getAttribute('aria-selected'), 'true');
    assert.equal(mutations.length, 0, 'Returning to retained history must not create or mutate resources');
    await screenshot('06-returning-user-discovers-exact-preview');

    const before = await observe(projectRoute + '/releases');
    assert.equal(before.releases.find(row => row.id === release.id)?.status, 'ready');
    const stopped = await mutation(() => original.getByRole('button', { name: 'Stop preview', exact: true }).click(), projectRoute + '/releases/' + release.id + '/stop');
    assert.equal(stopped.id, release.id); assert.equal(stopped.status, 'stopped');
    await original.getByText('stopped', { exact: true }).waitFor();
    assert.equal(await original.getByRole('button', { name: 'Open preview', exact: true }).count(), 0);
    const observed = await observe(projectRoute + '/releases');
    const retained = observed.releases.find(row => row.id === release.id);
    assert.equal(retained?.status, 'stopped'); assert.equal(retained?.canStop, false); assert.equal(retained?.commit, release.commit);
    assert.deepEqual(mutations.map(({ path, method }) => ({ path, method })), [{ path: projectRoute + '/releases/' + release.id + '/stop', method: 'POST' }]);

    await original.getByRole('button', { name: 'Prepare replacement preview', exact: true }).click();
    const review = page.getByRole('dialog', { name: 'Review replacement preview', exact: true });
    await review.waitFor();
    assert((await review.innerText()).includes('retains the earlier release record'));
    assert((await review.innerText()).includes('Current runtime configuration applies'));
    assert.equal(await review.getByText(release.commit, { exact: true }).count(), 2, 'Both prior and replacement source must remain visible');
    const build = review.getByRole('button', { name: 'Build replacement preview', exact: true });
    assert.equal(await build.isEnabled(), false, 'Replacement requires a separate acknowledgement');
    assert.equal(mutations.length, 1, 'Opening recovery review must not build or retry');
    await screenshot('07-explicit-replacement-review');
    await review.getByRole('checkbox', { name: 'Build this selected commit as a new preview; keep the earlier release in history.', exact: true }).check();
    const replacement = await mutation(() => build.click(), projectRoute + '/releases', 'POST', 240_000);
    onRelease(replacement.id);
    assert.match(replacement.id, /^rel-[a-f0-9]{24}$/); assert.notEqual(replacement.id, release.id);
    assert.equal(replacement.projectId, projectId); assert.equal(replacement.target, 'local'); assert.equal(replacement.status, 'ready'); assert.equal(replacement.commit, release.commit);
    const builds = mutations.filter(row => row.path === projectRoute + '/releases');
    assert.equal(builds.length, 1, 'Exactly one acknowledged replacement may be created');
    assert.equal(builds[0].body.expectedCommit, release.commit, 'Backend build admission must receive the reviewed source fence');
    const next = page.getByRole('listitem', { name: 'Release ' + replacement.id, exact: true });
    await next.getByText('ready', { exact: true }).waitFor();
    await original.getByText('stopped', { exact: true }).waitFor();
    const after = await observe(projectRoute + '/releases');
    assert.equal(after.releases.find(row => row.id === release.id)?.status, 'stopped');
    assert.equal(after.releases.find(row => row.id === replacement.id)?.status, 'ready');
    await screenshot('08-replacement-ready-original-retained');
    check('A returning teammate finds the exact retained preview, explicitly stops its actual container, reviews the source and creates one replacement while retaining the stopped release', {
      originalReleaseID: release.id, replacementReleaseID: replacement.id, commit: release.commit,
      observedOriginalStatus: retained.status, observedReplacementStatus: replacement.status,
      mutationPaths: mutations.map(({ path, method }) => ({ path, method })),
      scope: 'Actual owned fixture preview stop/build/health observations; no injected responses, cloud operations, or new run acceptance. This replacement has not been attached to the earlier accepted run.',
    });
    return replacement;
  } finally {
    page.off('request', request); page.off('response', response);
    await Promise.allSettled([...pendingReceipts]);
  }
}
