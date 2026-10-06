// Fresh browser account joins its intended team using a real one-time invite.
// Isolated test identities only; codes/passwords never enter the evidence files.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';
const repo = path.resolve(import.meta.dirname, '..');
const binary = path.resolve(process.argv[2]), web = path.resolve(process.argv[3]);
const fixture = path.resolve(process.argv[4] || path.join(repo, 'artifacts/uat', `team-invitation-${randomUUID()}`));
assert.match(fixture.replaceAll('\\', '/'), /\/artifacts\/uat\/[^/]+$/);
await mkdir(fixture, { mode: 0o700 });
const evidence = path.join(fixture, 'evidence'); await mkdir(evidence, { mode: 0o700 });
const listener = createServer(); await new Promise(done => listener.listen(0, '127.0.0.1', done));
const address = `127.0.0.1:${listener.address().port}`; await new Promise(done => listener.close(done));
const base = `http://${address}`, lifecycle = randomUUID() + randomUUID();
const secrets = [lifecycle], scrub = value => secrets.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const environment = { ...Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('LUCI_'))), LUCI_ADDR: address, LUCI_STATE_DIR: path.join(fixture, 'state'), LUCI_PROJECTS_DIR: path.join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: repo, LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_DOCKER_HOST: 'tcp://127.0.0.1:1', DOCKER_HOST: 'tcp://127.0.0.1:1', OPENAI_API_KEY: '', ANTHROPIC_API_KEY: '', GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: path.join(fixture, 'no-git-config') };
const receipt = { passed: false, binary, binarySha256: createHash('sha256').update(await readFile(binary)).digest('hex'), web, checks: [], startedAt: new Date().toISOString(), externalModels: false, cloudResources: false };
let service, browser, ownerPage, invitedPage;
const errors = [], check = label => receipt.checks.push(label);
async function api(method, route, body, token, expected = 200) {
  const response = await fetch(base + route, { method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) });
  assert.equal(response.status, expected, `${method} ${route}`); return response.status === 204 ? null : response.json();
}
async function start() {
  service = spawn(binary, [], { cwd: repo, env: environment, stdio: 'ignore' });
  for (let i = 0; i < 200; i++) { try { const status = await api('GET', '/v1/control-plane/status'); receipt.version = status.version; return; } catch { await delay(100); } }
  throw new Error('Service not ready');
}
async function stop() {
  if (service?.exitCode === null) {
    await fetch(base + '/v1/control-plane/shutdown', { method: 'POST', headers: { 'X-Luci-Lifecycle-Token': lifecycle } });
    for (let i = 0; i < 150 && service.exitCode === null; i++) await delay(100);
    assert.equal(service.exitCode, 0);
  }
}
async function page() {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const result = await context.newPage(); result.setDefaultTimeout(20000); result.on('pageerror', error => errors.push(scrub(error.message))); return result;
}
try {
  await start();
  const password = randomUUID() + randomUUID(); secrets.push(password);
  const bootstrap = (await readFile(path.join(fixture, 'state/control-plane/bootstrap-token'), 'utf8')).trim(); secrets.push(bootstrap);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'team-owner', displayName: 'Invitation Owner', password }, undefined, 201); secrets.push(owner.token);
  await api('POST', '/v1/product/projects', { name: 'owner-personal' }, owner.token, 201);
  browser = await chromium.launch({ headless: true }); receipt.browserVersion = await browser.version(); ownerPage = await page(); invitedPage = await page();
  await ownerPage.goto(base); await ownerPage.getByLabel('Username', { exact: true }).fill('team-owner'); await ownerPage.getByLabel('Password', { exact: true }).fill(password); await ownerPage.getByRole('button', { name: 'Sign in', exact: true }).click();
  await ownerPage.getByRole('button', { name: 'Manage teams', exact: true }).click(); await ownerPage.getByLabel('Team name', { exact: true }).fill('Invited collaborators'); await ownerPage.getByRole('button', { name: 'Create team', exact: true }).click();
  await ownerPage.getByRole('button', { name: 'Team access', exact: true }).click();
  await ownerPage.getByText('Invite someone to Invited collaborators', { exact: true }).click();
  await ownerPage.getByLabel("Invited person's team role", { exact: true }).selectOption('editor');
  await ownerPage.getByRole('button', { name: 'Create team invitation', exact: true }).click();
  const code = await ownerPage.getByLabel('New team invitation code', { exact: true }).innerText(); secrets.push(code);
  await ownerPage.getByRole('button', { name: 'Hide team invitation code', exact: true }).click(); assert.equal(await ownerPage.getByLabel('New team invitation code').count(), 0);
  const team = (await api('GET', '/v1/teams', undefined, owner.token)).teams[0].team;
  check('Team owner created an editor invitation through actual UI with no additional execution or credential grants, then hid its one-time code');
  await invitedPage.goto(base); await invitedPage.getByRole('button', { name: 'Create account', exact: true }).click();
  await invitedPage.getByLabel('Invitation code', { exact: true }).fill(code); await invitedPage.getByLabel('Display name', { exact: true }).fill('New Collaborator'); await invitedPage.getByLabel('Username', { exact: true }).fill('new-collaborator'); await invitedPage.getByLabel('Password', { exact: true }).fill(password); await invitedPage.getByLabel('Confirm password', { exact: true }).fill(password);
  const accepting = invitedPage.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/v1/auth/invitations/accept'));
  await invitedPage.getByRole('button', { name: 'Create account', exact: true }).click();
  const acceptance = await accepting; assert.equal(acceptance.status(), 201); const joined = await acceptance.json(); secrets.push(joined.token);
  await invitedPage.getByRole('button', { name: 'Team access', exact: true }).waitFor();
  assert.equal(await invitedPage.getByLabel('Workspace', { exact: true }).inputValue(), team.id);
  assert.equal(joined.user.role, 'member'); assert.equal(joined.teamId, team.id);
  assert.equal(await invitedPage.getByRole('treeitem', { name: /owner-personal/ }).count(), 0);
  assert.equal(await invitedPage.evaluate(secret => JSON.stringify(localStorage).includes(secret) || JSON.stringify(sessionStorage).includes(secret), code), false);
  check('A fresh browser created its own standard account with the invite and landed directly in the intended team without personal project leakage or persisted invite code');
  const detail = await api('GET', `/v1/teams/${team.id}`, undefined, joined.token);
  assert.equal(detail.membership.role, 'editor'); assert.deepEqual(detail.membership.grants, { execute: false, secrets: false, deploy: false, admin: false });
  await api('GET', '/v1/admin/users', undefined, joined.token, 403);
  await api('POST', '/v1/auth/invitations/accept', { inviteCode: code, username: 'reused-account', displayName: 'Reuse attempt', password }, undefined, 403);
  await ownerPage.getByRole('button', { name: 'Refresh team invitations', exact: true }).click(); await ownerPage.getByText('Accepted', { exact: false }).waitFor();
  await ownerPage.screenshot({ path: path.join(evidence, 'accepted-invitation.png'), fullPage: true });
  check('Accepted code is visibly consumed, cannot be reused, and grants only the selected team role with no instance administration');
  await ownerPage.getByRole('button', { name: 'Close', exact: true }).click();
  await ownerPage.getByRole('button', { name: 'New project', exact: true }).click();
  const creating = ownerPage.getByRole('dialog', { name: 'Create a project', exact: true }); await creating.getByLabel('Project name', { exact: true }).fill('invitation-handoff'); await creating.getByRole('button', { name: 'Create project', exact: true }).click();
  await ownerPage.getByRole('heading', { name: 'Define your project', exact: true }).waitFor();
  await ownerPage.getByRole('button', { name: 'Team access', exact: true }).click();
  await ownerPage.getByRole('tab', { name: 'Project sharing', exact: true }).click();
  const share = ownerPage.getByRole('group', { name: 'Grant project access', exact: true }); await share.getByLabel('Team member', { exact: true }).selectOption(joined.user.id); await share.getByLabel('Project role', { exact: true }).selectOption('editor'); await share.getByRole('button', { name: 'Share project', exact: true }).click();
  await ownerPage.getByLabel('Project access for New Collaborator', { exact: true }).waitFor();
  await invitedPage.reload(); await invitedPage.getByRole('treeitem', { name: /invitation-handoff$/ }).click(); await invitedPage.getByRole('button', { name: 'Define', exact: true }).click();
  const answers = [['What should this solution do?', 'New collaborators can contribute from a private team invitation.'], ['Who will use it?', 'Invited team contributors.'], ['What does it need to respect?', 'Explicit project access and retained history.'], ['What should we leave out?', 'Agent execution without a grant.'], ['How will we know it works?', 'The contribution remains saved after restart.']];
  for (let index = 0; index < answers.length; index++) { await invitedPage.getByRole('textbox', { name: answers[index][0], exact: true }).fill(answers[index][1]); await invitedPage.getByRole('button', { name: index === answers.length - 1 ? 'Review specification' : 'Next question', exact: true }).click(); }
  const saving = invitedPage.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/specification')); await invitedPage.getByRole('button', { name: 'Save definition', exact: true }).click(); assert.equal((await saving).status(), 200);
  await invitedPage.screenshot({ path: path.join(evidence, 'invited-contribution.png'), fullPage: true });
  check('Owner explicitly shared a project through UI and the newly invited account saved a definition using its edit-only grant');
  await stop(); await start(); await invitedPage.reload(); await invitedPage.getByRole('button', { name: 'Define', exact: true }).click(); await invitedPage.getByRole('tab', { name: 'Specification', exact: true }).click();
  assert.equal(await invitedPage.getByRole('textbox', { name: 'The idea', exact: true }).inputValue(), answers[0][1]); assert.equal(await invitedPage.getByLabel('Workspace', { exact: true }).inputValue(), team.id);
  check('Gateway and worker restart preserved invitation membership, selected team and the new contributor’s saved context');
  assert.deepEqual(errors, []); receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error);
  for (const [name, current] of [['owner', ownerPage], ['invited', invitedPage]]) if (current) {
    await current.locator('output').evaluateAll(nodes => nodes.forEach(node => { node.textContent = '[redacted]'; })).catch(() => {});
    await current.screenshot({ path: path.join(evidence, `${name}-failure.png`), fullPage: true }).catch(() => {});
    await writeFile(path.join(evidence, `${name}-failure-dom.txt`), scrub(await current.locator('body').innerText())).catch(() => {});
  }
  process.exitCode = 1;
} finally {
  await browser?.close(); try { await stop(); } catch (error) { receipt.cleanupError = scrub(error); service?.kill(); }
  await writeFile(path.join(evidence, 'result.json'), JSON.stringify(receipt, null, 2)); console.log(JSON.stringify(receipt));
}
