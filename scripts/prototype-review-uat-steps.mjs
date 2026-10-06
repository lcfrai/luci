// Runs inside the existing team-amendment fixture. Every write is a visible UI
// action against its real application preview; no project seeding or lifecycle.
// The human observation creates a NEW design draft. It is not approval, a
// verification receipt, or evidence that a historical accepted run changed.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';

export const prototypeReviewHelperSHA256 = createHash('sha256').update(readFileSync(new URL(import.meta.url))).digest('hex');

export async function verifyPrototypeReview({ page, projectId, projectRoute, release, design, specification, lens, screenshot, check, mutation }) {
  assert.equal(release.projectId, projectId);
  assert.equal(release.status, 'ready');
  assert.match(release.commit, /^[a-f0-9]{40,64}$/);
  assert.equal(design.projectId, projectId);
  assert.equal(design.status, 'approved', 'The review starts from an approved handoff; its new notes must not silently keep that approval.');
  assert.equal(specification.projectId, projectId);
  const planned = design.screens.find(item => item.name === 'Garden notes');
  assert(planned, 'The existing journey must supply its reviewed Garden notes screen.');
  assert.equal(design.screens.filter(item => item.id === planned.id).length, 1);
  assert.equal(planned.route, '/');
  assert(planned.widths.includes(390), 'The actual 390 px viewport must be a designed target.');
  const outcomes = specification.requirements.filter(item => planned.requirementIds.includes(item.id));
  assert(outcomes.length, 'The screen must carry real reviewed acceptance criteria.');
  const writes = [], forbiddenActions = [];
  const observeRequest = request => {
    if (request.method() !== 'POST') return;
    const path = new URL(request.url()).pathname;
    if (path === projectRoute + '/design-plan' || path === projectRoute + '/design-plan/approve') writes.push(path);
    if (path.startsWith(projectRoute + '/runs') || path.startsWith(projectRoute + '/assist') || path.startsWith(projectRoute + '/checks') || path === projectRoute + '/releases' || path.endsWith('/v1/sessions')) forbiddenActions.push(path);
  };
  page.on('request', observeRequest);
  const originalViewport = page.viewportSize();
  try {
    await page.setViewportSize({ width: 1680, height: 1080 });
    await lens('Design');
    const root = page.locator('.design-workspace');
    await root.getByRole('tab', { name: 'Prototype', exact: true }).click();
    await root.getByRole('combobox', { name: 'Prototype release', exact: true }).selectOption(release.id);
    await root.getByRole('combobox', { name: 'Screen to review', exact: true }).selectOption(planned.id);
    await root.getByRole('combobox', { name: 'Preview width', exact: true }).selectOption('390');
    const review = root.getByRole('region', { name: 'Screen review', exact: true });
    for (const state of planned.states.filter(value => value.trim())) await review.getByText(state, { exact: true }).waitFor();
    for (const outcome of outcomes) {
      await review.getByText(outcome.title, { exact: true }).waitFor();
      await review.getByText(outcome.acceptance, { exact: true }).waitFor();
    }
    assert.equal(await root.locator('iframe[title="Design prototype"]').count(), 0);
    assert.equal(await root.getByRole('button', { name: 'Record an observation', exact: true }).count(), 0);
    async function openPlannedScreen() {
      const invitation = await mutation(() => root.getByRole('button', { name: 'Open planned screen', exact: true }).click(), '/projects/' + projectId + '/browser-access');
      // Invitations are private and single-use. Assert in memory, never include
      // their ticket values in the receipt or an assertion's actual/expected.
      assert(typeof invitation.url === 'string', 'A team prototype requires a browser invitation.');
      const address = new URL(invitation.url);
      assert(address.pathname === '/.well-known/luci-team-access' && address.searchParams.has('ticket'), 'Expected a one-use team browser invitation.');
      const app = page.frameLocator('iframe[title="Design prototype"]');
      await app.getByRole('heading', { name: 'Garden notes', exact: true }).waitFor();
      assert.equal(await app.locator('html').evaluate(() => window.innerWidth), 390, 'The application viewport must match the designed width, excluding Luci frame borders.');
      const src = await root.locator('iframe[title="Design prototype"]').getAttribute('src');
      assert(src === invitation.url, 'The iframe must use the fresh invitation from this explicit open.');
      return { app, invitation: src };
    }
    const first = await openPlannedScreen();
    await first.app.getByLabel('Garden area', { exact: true }).fill('Greenhouse');
    await first.app.getByLabel('Garden note', { exact: true }).fill('Prototype review preserves the garden area');
    await first.app.getByRole('button', { name: 'Save note', exact: true }).click();
    await first.app.getByText('Greenhouse · Prototype review preserves the garden area', { exact: true }).waitFor();
    const frameBounds = await root.locator('iframe[title="Design prototype"]').boundingBox(), reviewBounds = await review.boundingBox();
    assert(frameBounds && reviewBounds && reviewBounds.x >= frameBounds.x + frameBounds.width, 'The wide prototype view should place expected behavior alongside the interactive application.');
    await screenshot('08-prototype-review-actual-screen-and-outcomes');
    await page.setViewportSize({ width: 390, height: 844 });
    const viewport = root.locator('.design-prototype-viewport');
    await viewport.scrollIntoViewIfNeeded();
    const geometry = await viewport.evaluate(element => ({ width: element.clientWidth, contentWidth: element.scrollWidth, pageWidth: document.documentElement.scrollWidth, viewportWidth: window.innerWidth }));
    assert(geometry.width > 0 && geometry.contentWidth > geometry.width, 'The phone layout should contain the designed viewport in its own scroll area.');
    assert(geometry.pageWidth <= geometry.viewportWidth + 1, 'Prototype overflow must stay inside its viewport rather than widening the whole app.');
    assert.equal(await first.app.locator('html').evaluate(() => window.innerWidth), 390);
    await screenshot('09-prototype-review-phone-contained');
    await page.setViewportSize({ width: 1680, height: 1080 });
    check('The replacement application is reviewed beside its planned states and linked acceptance criteria, performs the real garden-area save at exactly 390 px, and remains contained on a phone viewport', { releaseID: release.id, commit: release.commit, screenID: planned.id, viewportWidth: 390, requirementIDs: outcomes.map(item => item.id), helperSHA256: prototypeReviewHelperSHA256, scope: 'Actual deterministic fixture application behavior; not model-quality or design approval.' });

    const note = 'Observed at 390 px: saving a Greenhouse note displayed both the area and note in the list. Requested change: explain an empty submission without clearing the draft.';
    await root.getByRole('button', { name: 'Record an observation', exact: true }).click();
    await root.getByRole('textbox', { name: 'Observation or requested change', exact: true }).fill(note);
    await root.getByText(/Saving creates a draft that needs design review again/).waitFor();
    const tabDiagnostics = await observePrototypeTabTransition(page);
    let notes, tabNavigation;
    try {
      await root.getByRole('button', { name: 'Done writing', exact: true }).click();
      const handoff = root.getByRole('tab', { name: 'Implementation handoff', exact: true });
      tabNavigation = await preparePrototypeTabClick(handoff);
      await handoff.click();
      notes = await root.getByRole('textbox', { name: 'Implementation notes', exact: true }).inputValue();
      assert.equal(await root.getByRole('tab', { name: 'Implementation handoff', exact: true }).getAttribute('aria-selected'), 'true', 'The handoff tab must actually be selected.');
    } catch (cause) {
      // Preserve the original failure and timeout. The driver retains this
      // bounded control-only trace; no field values, invitations or raw errors.
      cause.prototypeTabDiagnostics = { ...await tabDiagnostics.finish(), navigation: tabNavigation };
      throw cause;
    } finally {
      await tabDiagnostics.finish();
    }
    assert(notes.startsWith(design.implementationNotes.trimEnd()), 'The observation must preserve prior handoff notes.');
    for (const expected of ['human note; not verification', 'Release: ' + release.id, 'Source commit: ' + release.commit, 'Screen: ' + planned.name + ' (' + planned.id + ')', 'Viewport: 390 px', note]) assert(notes.includes(expected), 'The handoff observation lost its actual source, selected screen, intended width or human note.');
    assert(!notes.includes('ticket='), 'Private one-use invitations must never enter project context.');
    assert.deepEqual(writes, [], 'Selecting a release and drafting an observation must not save or approve implicitly.');

    await root.getByRole('tab', { name: 'Prototype', exact: true }).click();
    assert.equal(await root.locator('iframe[title="Design prototype"]').count(), 0, 'Returning to Prototype must not replay a consumed browser invitation.');
    const second = await openPlannedScreen();
    assert(first.invitation !== second.invitation, 'Returning must obtain a fresh one-use invitation.');
    await second.app.getByText('Greenhouse · Prototype review preserves the garden area', { exact: true }).waitFor();
    await root.getByRole('button', { name: 'Amend this screen', exact: true }).click();
    const record = root.getByRole('group', { name: 'Screen ' + (design.screens.findIndex(item => item.id === planned.id) + 1), exact: true });
    const screenName = record.getByRole('textbox', { name: 'Screen name', exact: true });
    assert.equal(await screenName.inputValue(), planned.name);
    assert(await screenName.evaluate(element => element === document.activeElement), 'Amend this screen must focus the exact reviewed screen.');
    const requestedState = 'An empty note submission explains the required text and preserves the draft.';
    await record.getByRole('textbox', { name: 'States and recovery', exact: true }).fill([...planned.states, requestedState].join('\n'));
    assert.deepEqual(writes, [], 'The screen amendment must remain a draft until explicit Save.');
    const saved = await mutation(() => root.getByRole('button', { name: 'Save design plan', exact: true }).click(), projectRoute + '/design-plan');
    assert.equal(saved.status, 'draft');
    assert(saved.revision > design.revision);
    assert.equal(saved.prototypeReleaseId, release.id); assert.equal(saved.prototypeCommit, release.commit);
    assert.equal(saved.implementationNotes, notes);
    assert.deepEqual(saved.screens, design.screens.map(item => item.id === planned.id ? { ...item, states: [...item.states, requestedState] } : item));
    assert.deepEqual(writes, [projectRoute + '/design-plan']);
    assert.deepEqual(forbiddenActions, [], 'Prototype review must not launch coding, checks, assistance or releases.');
    await root.getByRole('button', { name: 'Approve design handoff', exact: true }).waitFor();
    await root.getByRole('tab', { name: 'Implementation handoff', exact: true }).click();
    await screenshot('10-prototype-observation-saved-awaiting-review');
    check('A fresh browser invitation preserves the actual running app after navigation; exact-screen amendment and source-bound human observations save only by explicit action and remain an unapproved design draft', { releaseID: release.id, commit: release.commit, screenID: planned.id, savedDesignRevision: saved.revision, savedStatus: saved.status, freshInvitations: 2, note, requestedState, tabNavigation, scope: 'The requested empty-submission behavior is future work, not a newly verified behavior; earlier accepted runs retain their own release and inputs.' });
    return saved;
  } finally {
    page.off('request', observeRequest);
    if (originalViewport) await page.setViewportSize(originalViewport);
  }
}

async function preparePrototypeTabClick(tab) {
  // Chromium 151 routed an immediate auto-scroll click into the cross-process
  // iframe's OLD bounds. The full packaged-app comparison is retained in
  // artifacts/onboarding/prototype-click-synchronization-comparison.json.
  // Scroll visibly first, observe both geometries settle, then click ONCE.
  // This synchronizes browser hit testing; it never activates or retries a tab.
  await tab.scrollIntoViewIfNeeded();
  return tab.evaluate(element => new Promise((resolve, reject) => {
    let last = '', stable = 0, frames = 0, finished = false;
    const deadline = setTimeout(() => finish(new Error('Prototype tab and iframe geometry did not settle')), 5_000);
    function finish(error, value) {
      if (finished) return;
      finished = true; clearTimeout(deadline);
      if (error) reject(error); else resolve(value);
    }
    const measure = node => { const rect = node.getBoundingClientRect(); return { x: rect.x, y: rect.y, width: rect.width, height: rect.height }; };
    function frame() {
      if (finished) return;
      const iframe = document.querySelector('iframe[title="Design prototype"]');
      if (!element.isConnected || !iframe) return finish(new Error('The open prototype or handoff tab disappeared before navigation'));
      const geometry = { tab: measure(element), iframe: measure(iframe) };
      const { x, y, width, height } = geometry.tab;
      const hit = document.elementFromPoint(x + width / 2, y + height / 2);
      const hitIsTab = width > 0 && height > 0 && (hit === element || element.contains(hit));
      const current = JSON.stringify(geometry);
      stable = current === last && hitIsTab ? stable + 1 : 0;
      last = current; frames++;
      if (stable >= 3) return finish(null, { frames, stableFrames: stable, geometry, hitIsTab });
      if (frames >= 90) return finish(new Error('Prototype tab and iframe geometry did not settle'));
      requestAnimationFrame(frame);
    }
    requestAnimationFrame(frame);
  }));
}


/** Diagnostic only: does not activate controls, retry clicks or alter timing. */
async function observePrototypeTabTransition(page) {
  const browserErrors = [];
  const observeError = error => {
    if (browserErrors.length >= 8) return;
    const allowed = ['TypeError', 'ReferenceError', 'RangeError', 'SyntaxError'];
    browserErrors.push({ category: allowed.includes(error.name) ? error.name : 'JavaScriptError' });
  };
  page.on('pageerror', observeError);
  try {
    await page.evaluate(() => {
      const labels = ['Screens and states', 'Sketch', 'Prototype', 'Implementation handoff', 'Done writing', 'Record an observation'];
      const describe = target => {
        if (!(target instanceof Element)) return { role: 'none', label: '' };
        const button = target.closest('button');
        const role = button?.getAttribute('role') === 'tab' ? 'tab' : button ? 'button' : target.matches('textarea') ? 'textbox' : 'other';
        const text = button?.getAttribute('aria-label') || button?.textContent || '';
        return { role, label: labels.find(label => text === label || text === label + label) || (button ? 'other button' : '') };
      };
      const tabs = () => [...document.querySelectorAll('.design-workspace [role="tab"]')].slice(0, 4).map(tab => ({ ...describe(tab), selected: tab.getAttribute('aria-selected') === 'true', disabled: tab.getAttribute('aria-disabled') === 'true' || tab.hasAttribute('disabled') }));
      const before = { tabs: tabs(), focus: describe(document.activeElement) };
      const events = [], start = performance.now();
      const record = event => {
        if (events.length >= 64) return;
        const item = { sequence: events.length + 1, event: event.type, elapsedMs: Math.round(performance.now() - start), target: describe(event.target), focus: describe(document.activeElement), defaultPrevented: event.defaultPrevented };
        if (event instanceof MouseEvent) Object.assign(item, { button: event.button, buttons: event.buttons, alt: event.altKey, control: event.ctrlKey, meta: event.metaKey, shift: event.shiftKey });
        events.push(item);
        // Capture the final flag even if a later capture/bubble handler consumes
        // the event. No second event handler is dispatched by this observer.
        queueMicrotask(() => { item.defaultPrevented = event.defaultPrevented; });
      };
      const kinds = ['pointerdown', 'mousedown', 'pointerup', 'mouseup', 'click', 'focusin', 'focusout'];
      for (const kind of kinds) document.addEventListener(kind, record, true);
      window.__luciPrototypeTabDiagnostic = () => {
        for (const kind of kinds) document.removeEventListener(kind, record, true);
        delete window.__luciPrototypeTabDiagnostic;
        return { before, events, after: { tabs: tabs(), focus: describe(document.activeElement) } };
      };
    });
  } catch (cause) {
    page.off('pageerror', observeError);
    throw cause;
  }
  let result;
  return { async finish() {
    if (result) return result;
    page.off('pageerror', observeError);
    const trace = await page.evaluate(() => window.__luciPrototypeTabDiagnostic?.()).catch(() => ({ unavailable: true }));
    result = { ...trace, browserErrors };
    return result;
  } };
}
