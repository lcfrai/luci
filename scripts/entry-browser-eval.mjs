// Exercise the real entry UI and Go service using fresh, disposable identities.
// node scripts/entry-browser-eval.mjs <binary> <web> <runtime> [fixture]
// No native bridge is simulated, no real profile is read, and no model/cloud runs.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import path from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { chromium } from "../desktop/node_modules/playwright/index.mjs";

const repo = path.resolve(import.meta.dirname, "..");
assert(process.argv[2] && process.argv[3] && process.argv[4]);
const [binary, web, runtime] = process.argv.slice(2, 5).map(value => path.resolve(value));
const fixture = path.resolve(process.argv[5] || path.join(repo, "artifacts/uat", "entry-" + randomUUID()));
assert.match(fixture.replaceAll("\\", "/"), /\/artifacts\/uat\/[^/]+$/);
await mkdir(path.dirname(fixture), { recursive: true });
await mkdir(fixture, { mode: 0o700 }); // Never reuse existing data.
const evidence = path.join(fixture, "evidence");
await mkdir(evidence, { mode: 0o700 });
const secrets = new Set();
const secret = () => { const value = randomUUID() + randomUUID(); secrets.add(value); return value; };
const redact = value => { let text = String(value); for (const value of secrets) text = text.replaceAll(value, "[redacted]"); return text; };
const result = { passed: false, binary, web, startedAt: new Date().toISOString(), checks: [], errors: [], limits: [
  "Actual compiled service and browser; no native IPC imitation, cloud, model or real user-profile access.",
  "Invitation expiry uses only a stopped disposable service's persisted invitation expiry; system time is unchanged.",
] };
const services = [];
const browser = await chromium.launch({ headless: true });
const pass = (name, detail) => result.checks.push({ name, detail, status: "passed" });
const tokenKey = "luci.control-plane.session";
async function freePort() {
  const server = createServer();
  await new Promise((done, fail) => server.once("error", fail).listen(0, "127.0.0.1", done));
  const port = server.address().port;
  await new Promise(done => server.close(done));
  return port;
}
async function api(service, method, route, body, token, expected = 200) {
  const response = await fetch(service.base + route, {
    method, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }), signal: AbortSignal.timeout(15000),
  });
  assert.equal(response.status, expected, `${method} ${route} status`);
  return response.json();
}
async function makeService(name, portableOwner = false) {
  const root = path.join(fixture, name), state = path.join(root, "state"), projects = path.join(root, "projects"), home = path.join(root, "home");
  for (const dir of [root, state, projects, home]) await mkdir(dir, { mode: 0o700 });
  const port = await freePort(), lifecycle = secret();
  const inherited = Object.fromEntries(Object.entries(process.env).filter(([key]) => /^(PATH|Path|SystemRoot|SYSTEMROOT|WINDIR|COMSPEC|PATHEXT|TEMP|TMP|LANG)$/i.test(key)));
  const service = { name, root, state, projects, base: `http://127.0.0.1:${port}`, lifecycle, process: null, log: "",
    environment: { ...inherited, HOME: home, USERPROFILE: home, LUCI_ADDR: `127.0.0.1:${port}`, LUCI_STATE_DIR: state,
      LUCI_PROJECTS_DIR: projects, LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: runtime, LUCI_BACKEND: "docker",
      LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: path.join(home, "gitconfig"),
      GH_CONFIG_DIR: path.join(home, "gh"), AZURE_CONFIG_DIR: path.join(home, "azure"),
      AWS_CONFIG_FILE: path.join(home, "aws-config"), AWS_SHARED_CREDENTIALS_FILE: path.join(home, "aws-credentials"),
      ...(portableOwner ? { LUCI_PORTABLE_OWNER_ACCESS: "1" } : {}) } };
  services.push(service);
  await start(service);
  return service;
}
async function start(service) {
  let startError;
  const child = spawn(binary, [], { cwd: repo, env: service.environment, windowsHide: true });
  service.process = child;
  child.once("error", error => { startError = error; });
  for (const stream of [child.stdout, child.stderr]) stream.on("data", data => { if (service.log.length < 128 * 1024) service.log += redact(data.toString()); });
  for (let count = 0; count < 300; count++) {
    if (startError) throw startError;
    assert.equal(child.exitCode, null, "Fixture service exited before readiness");
    try {
      const response = await fetch(service.base + "/v1/control-plane/status", { signal: AbortSignal.timeout(500) });
      if (response.ok && (await response.json()).product === "Luci") return;
    } catch {}
    await delay(100);
  }
  throw new Error("Fixture service did not become ready");
}
async function stop(service) {
  if (service.process?.exitCode !== null) return;
  const response = await fetch(service.base + "/v1/control-plane/shutdown", { method: "POST", headers: { "X-Luci-Lifecycle-Token": service.lifecycle }, signal: AbortSignal.timeout(5000) });
  assert.equal(response.status, 200);
  for (let count = 0; count < 150 && service.process.exitCode === null; count++) await delay(100);
  assert.equal(service.process.exitCode, 0, "Fixture service did not shut down cleanly");
}
async function newPage(service, suffix = "") {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  page.on("pageerror", error => result.errors.push(redact(error.message)));
  await page.goto(service.base + suffix);
  return page;
}
async function profile(service, page) {
  const token = await page.evaluate(key => sessionStorage.getItem(key), tokenKey);
  assert(token); secrets.add(token);
  return { token, ...(await api(service, "GET", "/v1/auth/me", undefined, token)) };
}
async function closeSettings(page) {
  const close = page.getByRole("button", { name: "Close settings", exact: true });
  if (await close.isVisible()) await close.click();
}
async function signOut(page) {
  await closeSettings(page);
  await page.locator(".rail-account button").first().click();
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await page.getByRole("button", { name: "Sign in", exact: true }).waitFor();
}
async function signIn(page, username, password) {
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.locator(".shell").waitFor();
}
async function register(page, { username, displayName, password }, button = "Create account") {
  await page.getByLabel("Display name", { exact: true }).fill(displayName);
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByLabel("Confirm password", { exact: true }).fill(password);
  const show = page.getByRole("checkbox", { name: /^Show password/ });
  await show.check();
  assert.equal(await page.getByLabel("Password", { exact: true }).getAttribute("type"), "text");
  await show.uncheck();
  await page.getByRole("button", { name: button, exact: true }).click();
}
async function invite(service, token) {
  const answer = await api(service, "POST", "/v1/admin/invitations", { role: "member" }, token, 201);
  assert(answer.inviteCode && answer.invitation?.id); secrets.add(answer.inviteCode);
  return answer;
}
async function acceptInvite(service, code, username, expected) {
  const page = await newPage(service);
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await page.getByLabel("Invitation code", { exact: true }).fill(code);
  const response = page.waitForResponse(response => response.url() === service.base + "/v1/auth/invitations/accept" && response.request().method() === "POST");
  await register(page, { username, displayName: "Invited Test Member", password: secret() }, "Create account");
  assert.equal((await response).status(), expected, "Invitation result must come from the real service");
  if (expected === 201) {
    await page.locator(".shell").waitFor();
    assert.equal((await profile(service, page)).user.role, "member");
  } else {
    await page.getByRole("alert").waitFor();
    assert.equal(await page.locator(".shell").count(), 0);
    assert.equal(await page.evaluate(key => sessionStorage.getItem(key), tokenKey), null);
  }
  await page.context().close();
}

try {
  const service = await makeService("manual"), owner = { username: "entry-owner", displayName: "Browser Entry Owner", password: secret() };
  const page = await newPage(service);
  await page.getByRole("button", { name: "Create account", exact: true }).waitFor();
  assert.equal(await page.getByLabel(/^Bootstrap token/).inputValue(), "");
  assert.equal(await page.locator(".shell").count(), 0);
  pass("fresh_browser_without_token", "Fresh ordinary browser explains first-account setup without silently acquiring a host token or showing private content.");

  await page.evaluate(key => sessionStorage.setItem(key, "invalid-fixture-session"), tokenKey);
  await page.reload();
  await page.getByRole("button", { name: "Create account", exact: true }).waitFor();
  assert.equal(await page.evaluate(key => sessionStorage.getItem(key), tokenKey), null);
  pass("fresh_stale_session", "A stale browser bearer is discarded without hiding fresh first-account setup.");

  await page.route("**/v1/control-plane/status", route => route.abort("failed"));
  await page.reload();
  await page.getByRole("button", { name: "Retry connection", exact: true }).waitFor();
  await page.unroute("**/v1/control-plane/status");
  await page.getByRole("button", { name: "Retry connection", exact: true }).click();
  await page.getByRole("button", { name: "Create account", exact: true }).waitFor();
  pass("network_retry", "A deliberately interrupted status request recovers through the visible retry control and the real service.");

  const bootstrap = (await readFile(path.join(service.state, "control-plane/bootstrap-token"), "utf8")).trim(); secrets.add(bootstrap);
  await page.getByLabel(/^Bootstrap token/).fill(bootstrap);
  await register(page, owner);
  await page.locator(".shell").waitFor();
  const ownerProfile = await profile(service, page);
  assert.equal(ownerProfile.user.role, "super-admin");
  await signOut(page); await signIn(page, owner.username, owner.password);
  assert.equal((await profile(service, page)).user.id, ownerProfile.user.id);
  pass("manual_account_create_signout_signin", "The manually supplied one-time token creates a real owner; confirmation and password visibility work; sign-out and sign-in preserve identity.");
  await signOut(page);
  // Invitation administration has its own real login. The browser's previous
  // session was intentionally revoked by sign-out above.
  const administrator = await api(service, "POST", "/v1/auth/login", { username: owner.username, password: owner.password });
  secrets.add(administrator.token);
  await api(service, "GET", "/v1/auth/me", undefined, administrator.token);
  await page.evaluate(key => sessionStorage.setItem(key, "invalid-fixture-session"), tokenKey);
  await page.reload();
  await page.getByRole("button", { name: "Create account", exact: true }).waitFor();
  await page.getByRole("button", { name: "Forgot sign-in?", exact: true }).click();
  assert.equal(await page.getByRole("button", { name: "Recover owner account", exact: true }).count(), 0);
  await page.getByText(/administrator|installed app/i).first().waitFor();
  assert.equal(await page.evaluate(key => sessionStorage.getItem(key), tokenKey), null);
  for (const [method, route, body] of [["GET", "/v1/auth/local-owner"], ["POST", "/v1/auth/local-account", owner], ["POST", "/v1/auth/local-owner/recover", { newPassword: secret() }]]) {
    await api(service, method, route, body, undefined, 403);
    await api(service, method, route, body, administrator.token, 403);
  }
  pass("configured_stale_session_and_browser_help", "Configured browser retains entry choices after a stale session and offers help; neither anonymous nor ordinary bearer requests gain native account/recovery authority.");
  await page.context().close();

  await acceptInvite(service, "invalid-fixture-invitation", "invalid-entry-member", 403);
  const first = await invite(service, administrator.token);
  await acceptInvite(service, first.inviteCode, "accepted-entry-member", 201);
  await acceptInvite(service, first.inviteCode, "reuse-entry-member", 403);
  const revoked = await invite(service, administrator.token);
  await api(service, "DELETE", "/v1/admin/invitations/" + revoked.invitation.id, undefined, administrator.token);
  await acceptInvite(service, revoked.inviteCode, "revoked-entry-member", 403);
  pass("real_invitation_admission", "Real admin-issued invitation creates one member; invalid, consumed and revoked codes produce recoverable errors without authentication.");

  const expired = await invite(service, administrator.token);
  await stop(service);
  const identityPath = path.join(service.state, "control-plane/identity.json");
  const identity = JSON.parse(await readFile(identityPath, "utf8"));
  assert(identity.invitations[expired.invitation.id]);
  identity.invitations[expired.invitation.id].expiresAt = "2000-01-01T00:00:00Z";
  await writeFile(identityPath, JSON.stringify(identity), { mode: 0o600 });
  await start(service);
  await acceptInvite(service, expired.inviteCode, "expired-entry-member", 403);
  pass("expired_invitation", "A persisted expired invitation is rejected after a real fixture process restart; no production clock or endpoint is mocked.");
  await stop(service);

  const launchService = await makeService("launcher-fragment");
  const code = (await readFile(path.join(launchService.state, "control-plane/bootstrap-token"), "utf8")).trim(); secrets.add(code);
  const launched = await newPage(launchService, "/#bootstrap=" + encodeURIComponent(code));
  await launched.getByRole("button", { name: "Create account", exact: true }).waitFor();
  assert.equal(new URL(launched.url()).hash, "");
  assert.equal(await launched.getByLabel(/^Bootstrap token/).inputValue(), code);
  await register(launched, { username: "fragment-owner", displayName: "Launcher Entry Owner", password: secret() });
  await launched.locator(".shell").waitFor();
  assert.equal((await profile(launchService, launched)).user.role, "super-admin");
  pass("launcher_fragment_first_account", "Real portable launcher fragment preloads the one-time code, immediately removes it from the URL and creates an owner through the visible account form.");
  await launched.context().close(); await stop(launchService);

  const portable = await makeService("portable-owner", true);
  const ownerProof = (await readFile(path.join(portable.state, "control-plane/owner-access-token"), "utf8")).trim(); secrets.add(ownerProof);
  assert.match(ownerProof, /^[A-Za-z0-9_-]{43}$/);
  const portableBootstrap = (await readFile(path.join(portable.state, "control-plane/bootstrap-token"), "utf8")).trim(); secrets.add(portableBootstrap);
  const ownerPage = await newPage(portable, "/#owner=" + encodeURIComponent(ownerProof) + "&bootstrap=" + encodeURIComponent(portableBootstrap));
  await ownerPage.getByRole("button", { name: "Create account", exact: true }).waitFor();
  assert.equal(new URL(ownerPage.url()).hash, "", "Owner proof and bootstrap must leave the address bar immediately");
  const portableIdentity = { username: "portable-owner", displayName: "Portable Entry Owner", password: secret() };
  await register(ownerPage, portableIdentity);
  await ownerPage.locator(".shell").waitFor();
  const original = await profile(portable, ownerPage);
  const project = await api(portable, "POST", "/v1/product/projects", { name: "entry-preserved-project" }, original.token, 201);
  const purpose = "The original portable owner keeps this saved project after account recovery.";
  await api(portable, "POST", `/v1/product/projects/${project.id}/brief`, { revision: 0, purpose, audience: "The existing owner", constraints: "No cloud or model calls", criteria: ["Retain saved work"] }, original.token);
  const briefPath = path.join(project.path, "LUCI.md"), before = await readFile(briefPath, "utf8");
  assert(before.includes(purpose));
  await signOut(ownerPage);
  await ownerPage.getByLabel("Username", { exact: true }).fill(portableIdentity.username);
  await ownerPage.getByLabel("Password", { exact: true }).fill("incorrect-portable-owner-password");
  const refused = ownerPage.waitForResponse(response => response.url() === portable.base + "/v1/auth/login");
  await ownerPage.getByRole("button", { name: "Sign in", exact: true }).click();
  assert.equal((await refused).status(), 401);
  await ownerPage.getByRole("button", { name: "Create account", exact: true }).click();
  assert.equal(await ownerPage.getByLabel("Invitation code", { exact: true }).count(), 0, "An OS-owned launcher must use its actual local capability");
  await register(ownerPage, { username: "portable-second-owner", displayName: "Separate Portable Account", password: secret() }, "Create account");
  await ownerPage.locator(".shell").waitFor();
  const second = await profile(portable, ownerPage);
  assert.notEqual(second.user.id, original.user.id);
  assert.equal(second.user.role, "super-admin");
  assert(!(await api(portable, "GET", "/v1/projects", undefined, second.token)).projects.some(row => row.id === project.id));
  await signOut(ownerPage);
  // A second, still-live owner session proves recovery revokes more than the
  // browser's already-signed-out bearer. It belongs only to this fixture.
  const prior = await api(portable, "POST", "/v1/auth/login", { username: portableIdentity.username, password: portableIdentity.password }); secrets.add(prior.token);
  await api(portable, "GET", "/v1/auth/me", undefined, prior.token);
  await ownerPage.getByRole("button", { name: "Forgot sign-in?", exact: true }).click();
  const replacement = secret();
  await ownerPage.getByLabel("New password", { exact: true }).fill(replacement);
  await ownerPage.getByLabel("Confirm password", { exact: true }).fill(replacement);
  await ownerPage.getByRole("button", { name: "Recover owner account", exact: true }).click();
  await ownerPage.locator(".shell").waitFor();
  const recovered = await profile(portable, ownerPage);
  assert.equal(recovered.user.id, original.user.id);
  assert.equal(await readFile(briefPath, "utf8"), before);
  assert.equal((await api(portable, "GET", `/v1/product/projects/${project.id}/brief`, undefined, recovered.token)).purpose, purpose);
  await api(portable, "GET", "/v1/auth/me", undefined, prior.token, 401);
  await api(portable, "POST", "/v1/auth/login", { username: portableIdentity.username, password: portableIdentity.password }, undefined, 401);
  await signOut(ownerPage); await signIn(ownerPage, portableIdentity.username, replacement);
  assert.equal((await profile(portable, ownerPage)).user.id, original.user.id);
  pass("portable_owner_create_and_recover", "An actual flag-enabled launcher proof is cleared from the URL, creates a distinct local account and recovers the original owner without changing saved work; old password and a previously valid session are rejected.");
  await ownerPage.context().close(); await stop(portable);
  assert.deepEqual(result.errors, []);
  result.passed = true;
} catch (error) {
  result.error = redact(error.stack || error);
  process.exitCode = 1;
} finally {
  await browser.close();
  for (const service of services) {
    try { await stop(service); } catch (error) { result.cleanupError = redact(error); if (service.process?.exitCode === null) service.process.kill(); }
    await writeFile(path.join(evidence, service.name + ".log"), redact(service.log), { mode: 0o600 });
  }
  result.completedAt = new Date().toISOString();
  await writeFile(path.join(evidence, "entry-results.json"), JSON.stringify(result, null, 2) + "\n");
  console.log(JSON.stringify(result));
}
