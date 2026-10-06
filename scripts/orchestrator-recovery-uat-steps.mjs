// Browser-local composer recovery inside the owning team-amendment fixture.
// The network fault is deliberately BEFORE backend admission. It does NOT
// establish dropped-after-acceptance recovery; backend race tests cover that.
// No provider call, retry, or new Send is permitted after the recovery fence.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';

export const orchestratorRecoveryHelperSHA256 = createHash('sha256').update(readFileSync(new URL(import.meta.url))).digest('hex');

export async function exerciseOrchestratorRecovery({ page, projectId, projectRoute, lens, mutation, observe, check, screenshot, modelCalls, connectionLabel = 'Local amendment fixture', expectedModel = 'amendment-fixture' }) {
  assert.equal(typeof modelCalls, 'function', 'The owner must supply its actual provider request counter.');
  assert(projectRoute.endsWith('/v1/product/projects/' + projectId), 'Recovery must remain in the exact selected team project.');
  const origin = new URL(page.url()).origin;
  const sendPath = projectRoute + '/assist/orchestrator', resolvePath = projectRoute + '/orchestrator/resolve';
  const originalMessage = 'Keep my question about explaining an empty garden-note submission while preserving the draft. I will review it before any implementation.';
  const callsBefore = modelCalls(), writes = [];
  let interceptedRequest, interceptionError, interceptedCount = 0, resolutionRequest;
  const requestFields = new Set(['expectedRevision', 'expectedContextDigest', 'idempotencyKey', 'message', 'replyToTurnId', 'harness', 'model', 'agentConnectionId']);
  const track = request => {
    const url = new URL(request.url());
    if (url.origin !== origin || !url.pathname.startsWith(projectRoute + '/') || ['GET', 'HEAD', 'OPTIONS'].includes(request.method())) return;
    writes.push({ path: url.pathname, method: request.method() });
    if (url.pathname === resolvePath) resolutionRequest = request.postDataJSON();
  };
  const matchSend = url => url.origin === origin && url.pathname === sendPath;
  const blockBeforeAdmission = async route => {
    if (route.request().method() !== 'POST') return route.continue();
    interceptedCount++;
    try {
      const input = route.request().postDataJSON();
      assert(input && typeof input === 'object' && Object.keys(input).every(key => requestFields.has(key)), 'The recorded request may contain only public composer fields, never credentials.');
      assert.equal(input.message, originalMessage);
      assert.match(input.idempotencyKey, /^[!-~]{16,128}$/);
      assert.match(input.expectedContextDigest, /^[a-f0-9]{64}$/);
      assert(Number.isSafeInteger(input.expectedRevision) && input.expectedRevision >= 0);
      assert(['codex', 'claude-code', 'opencode'].includes(input.harness));
      assert.equal(interceptedCount, 1, 'The application attempted an unexpected duplicate Send.');
      interceptedRequest = structuredClone(input);
    } catch (error) { interceptionError = error; }
    // Abort every matching attempt, including an unexpected duplicate. This
    // keeps the fixture from invoking a provider if an assertion later fails.
    await route.abort('failed');
  };
  const panel = () => page.getByRole('complementary', { name: 'Project understanding', exact: true });
  async function openDiscussion() {
    await panel().waitFor();
    const show = panel().getByRole('button', { name: 'Show project overview', exact: true });
    if (await show.isVisible()) await show.click();
    const summary = panel().getByText('Discuss or request a change', { exact: true });
    await summary.waitFor();
    if (!await summary.evaluate(element => element.parentElement.open)) await summary.click();
    await panel().getByRole('textbox', { name: 'Message Luci', exact: true }).waitFor();
  }
  async function reloadAndOpen() {
    await page.reload({ waitUntil: 'domcontentloaded' });
    await openDiscussion();
  }
  async function openAgentSettings() {
    const summary = panel().getByText('Agent and model', { exact: true });
    if (!await summary.evaluate(element => element.parentElement.open)) await summary.click();
  }
  async function assertRetainedRoute(connectionID) {
    await panel().getByText(`Using ${connectionLabel} · ${expectedModel}. Saved context and this conversation are sent to the selected provider. Model usage may be charged.`, { exact: true }).waitFor();
    await openAgentSettings();
    assert.equal(await panel().getByRole('combobox', { name: 'Luci agent', exact: true }).inputValue(), 'codex');
    assert.equal(await panel().getByRole('combobox', { name: 'Luci connection', exact: true }).inputValue(), connectionID);
    assert.equal(await panel().getByRole('textbox', { name: 'Luci model', exact: true }).inputValue(), expectedModel);
  }
  page.on('request', track);
  await page.route(matchSend, blockBeforeAdmission);
  try {
    await lens('Define');
    await openDiscussion();
    // This fixture's provider is saved, but intentionally not the implicit
    // active default. Select its actual public route through the same UI.
    await openAgentSettings();
    await panel().getByRole('combobox', { name: 'Luci agent', exact: true }).selectOption('codex');
    const connection = panel().getByRole('combobox', { name: 'Luci connection', exact: true });
    await connection.selectOption({ label: connectionLabel });
    const connectionID = await connection.inputValue();
    assert(connectionID && connectionID !== '@active', 'The recovery case requires one explicitly selected saved connection.');
    await assertRetainedRoute(connectionID);
    const message = panel().getByRole('textbox', { name: 'Message Luci', exact: true });
    assert.equal(await message.inputValue(), '', 'This fresh fixture must not overwrite an existing unsent message.');
    assert.equal(await panel().getByRole('button', { name: 'Restore unsent message', exact: true }).count(), 0);
    await message.fill(originalMessage);
    await reloadAndOpen();
    await assertRetainedRoute(connectionID);
    assert.equal(await message.inputValue(), '', 'Reload must offer explicit restore without exposing the saved message in the composer automatically.');
    await panel().getByRole('button', { name: 'Restore unsent message', exact: true }).click();
    assert.equal(await message.inputValue(), originalMessage);
    assert.deepEqual(writes, [], 'Restoring a draft must not send, save context or start work.');
    assert.equal(modelCalls(), callsBefore);
    await screenshot('11-orchestrator-unsent-message-restored');
    check('An unsent project message survives a real browser reload and returns only after explicit Restore, with no inference or shared-context mutation', {
      projectID: projectId, helperSHA256: orchestratorRecoveryHelperSHA256, modelCallDelta: 0,
      scope: 'Browser-local draft recovery in the existing authenticated team/project; no provider request.',
    });

    // Synchronize the exact saved conversation before recording the intended
    // Send. This observes shared context without constructing it via an API.
    const before = await mutation(() => panel().getByRole('button', { name: 'Refresh conversation', exact: true }).click(), projectRoute + '/orchestrator', 'GET');
    assert.equal(before.projectId, projectId);
    assert(!before.turns.some(turn => ['reserved', 'queued', 'running', 'unknown'].includes(turn.status)), 'A prior active request must be resolved before this isolated fault case.');
    await panel().getByRole('button', { name: 'Send to Luci', exact: true }).click();
    await panel().getByText(/The outcome is unconfirmed\. Check the original request/).waitFor();
    if (interceptionError) throw interceptionError;
    assert.equal(interceptedCount, 1); assert(interceptedRequest);
    assert.equal(interceptedRequest.expectedRevision, before.revision);
    assert.equal(interceptedRequest.expectedContextDigest, before.context.digest);
    assert.equal(interceptedRequest.harness, 'codex');
    assert.equal(interceptedRequest.agentConnectionId, connectionID);
    assert.equal(interceptedRequest.model, expectedModel);
    assert.equal(modelCalls(), callsBefore, 'The deliberately aborted request must never reach inference.');

    await reloadAndOpen();
    await assertRetainedRoute(connectionID);
    await panel().getByRole('button', { name: 'Check original request', exact: true }).waitFor();
    assert.equal(await message.inputValue(), '', 'Pending content also requires an explicit review after reload.');
    assert.equal(await panel().getByRole('button', { name: 'Send to Luci', exact: true }).isEnabled(), false);
    assert.equal(interceptedCount, 1, 'Reload must not retry the original request.');
    await panel().getByRole('button', { name: 'Review original request', exact: true }).click();
    assert.equal(await message.inputValue(), originalMessage);
    await screenshot('12-orchestrator-before-admission-fault-retained');
    const resolution = await mutation(() => panel().getByRole('button', { name: 'Check original request', exact: true }).click(), resolvePath);
    assert.equal(resolution.projectId, projectId); assert.equal(resolution.status, 'not-sent');
    assert(!resolution.job && !resolution.jobId && !resolution.turnId, 'An absent request must not fabricate an assistant job or turn.');
    assert.deepEqual(resolutionRequest, interceptedRequest, 'Resolution must use the entire original input and idempotency key after reload.');
    await panel().getByText(/The original request was not sent and can no longer start/).waitFor();
    await panel().getByRole('button', { name: 'Restore unsent message', exact: true }).click();
    assert.equal(await message.inputValue(), originalMessage);
    assert.equal(await panel().getByRole('button', { name: 'Check original request', exact: true }).count(), 0);
    assert.equal(interceptedCount, 1, 'Recovery must not create a retry or new paid request.');
    assert.equal(modelCalls(), callsBefore);
    const after = await observe(projectRoute + '/orchestrator');
    assert.deepEqual(after.turns, before.turns, 'Fencing a never-admitted request must not create conversation turns.');
    assert.equal(after.revision, before.revision + 1, 'Exactly one durable no-dispatch fence should advance recovery storage.');
    assert.deepEqual(writes, [{ path: sendPath, method: 'POST' }, { path: resolvePath, method: 'POST' }]);
    await screenshot('13-orchestrator-fenced-original-restored-without-send');
    check('A clearly injected browser-network failure before server admission retains the original request across reload; explicit Check records a durable not-sent fence and restores the exact unsent text without any retry or model call', {
      projectID: projectId, helperSHA256: orchestratorRecoveryHelperSHA256,
      fault: 'Playwright aborts the initial orchestrator POST before backend admission; no response or accepted job is simulated.',
      originalRequest: interceptedRequest, resolution, modelCallDelta: modelCalls() - callsBefore,
      beforeRevision: before.revision, afterRevision: after.revision, conversationTurnCount: after.turns.length,
      scope: 'Before-admission failure only. This is not dropped-after-acceptance proof; server concurrency and receipt tests cover that separately.',
    });
    return { interceptedRequest, resolution, modelCallDelta: modelCalls() - callsBefore, helperSHA256: orchestratorRecoveryHelperSHA256 };
  } finally {
    page.off('request', track);
    await page.unroute(matchSend, blockBeforeAdmission);
  }
}
