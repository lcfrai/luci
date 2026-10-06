// Real compiled-service acceptance for one-team commissioning, retained service
// access, WebSocket revocation and explicitly owned shell cleanup. No inference,
// cloud provisioning, image build, or payment provider.
// node scripts/managed-team-service-smoke.mjs <binary> <fresh-evidence-directory>
import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { createWriteStream, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { setTimeout as delay } from "node:timers/promises";

assert.equal(process.platform, "linux"); assert.notEqual(process.getuid(), 0);
assert.equal(process.argv.length, 4, "Provide a compiled binary and fresh evidence directory");
const repository = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const binary = resolve(process.argv[2]), evidence = resolve(process.argv[3]);
mkdirSync(evidence, { recursive: false, mode: 0o700 });
const fixture = mkdtempSync(join(tmpdir(), "luci-managed-service-"));
const state = join(fixture, "state"), projects = join(fixture, "projects");
mkdirSync(state, { mode: 0o700 }); mkdirSync(projects, { mode: 0o700 });
const listener = createServer();
await new Promise(done => listener.listen(0, "127.0.0.1", done));
const address = `127.0.0.1:${listener.address().port}`;
await new Promise(done => listener.close(done));
const origin = `http://${address}`, password = randomUUID() + randomUUID(), lifecycle = randomUUID() + randomUUID();
const environment = {
  ...Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith("LUCI_"))),
  LUCI_ADDR: address, LUCI_BACKEND: "docker", LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: projects,
  LUCI_RUNTIME_DIR: repository, LUCI_WEB_DIR: join(repository, "desktop/dist"),
  LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_SANDBOX_IMAGE: "luci/sandbox:dev",
  ANTHROPIC_API_KEY: "", OPENAI_API_KEY: "", GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: join(fixture, "empty-git-config"),
};
const checks = [], sessions = new Set();
const result = { passed: false, binary, binarySHA256: createHash("sha256").update(readFileSync(binary)).digest("hex"), fixture, startedAt: new Date().toISOString(), paidInference: false, cloudResourcesCreated: false, checks };
const log = createWriteStream(join(evidence, "control-plane.log"), { mode: 0o600 });
let service, token, socket, spawnError, teamId;
const docker = (...args) => execFileSync("docker", args, { encoding: "utf8", timeout: 30_000, maxBuffer: 4 * 1024 * 1024 });

async function request(method, path, body, expected = 200, headers = {}) {
  const response = await fetch(origin + path, { method, redirect: "error", signal: AbortSignal.timeout(100_000), headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(body === undefined ? {} : { "Content-Type": "application/json" }), ...headers }, body: body === undefined ? undefined : JSON.stringify(body) });
  const bytes = Buffer.from(await response.arrayBuffer());
  assert.ok([expected].flat().includes(response.status), `${method} ${path}: ${response.status} ${bytes.toString().slice(0, 800)}`);
  return response.headers.get("Content-Type")?.startsWith("application/json") ? JSON.parse(bytes.toString()) : bytes;
}
async function until(label, read, budget = 35_000) {
  const end = Date.now() + budget;
  while (Date.now() < end) {
    if (spawnError) throw spawnError;
    assert.equal(service.exitCode, null, "Fixture service exited unexpectedly");
    const value = await read(); if (value) return value;
    await delay(100);
  }
  throw new Error(`${label} timed out`);
}
async function start(binding) {
  spawnError = undefined;
  service = spawn(binary, [], { cwd: repository, env: { ...environment, ...(binding ? { LUCI_MANAGED_TEAM_ID: binding } : {}) }, stdio: ["ignore", "pipe", "pipe"] });
  service.on("error", error => { spawnError = error; });
  service.stdout.pipe(log, { end: false }); service.stderr.pipe(log, { end: false });
  const status = await until("Service readiness", async () => { try { return await request("GET", "/v1/control-plane/status"); } catch { return false; } });
  assert.equal(status.product, "Luci"); result.version = status.version;
}
async function stop() {
  if (!service || service.exitCode !== null) return;
  await request("POST", "/v1/control-plane/shutdown", undefined, 200, { "X-Luci-Lifecycle-Token": lifecycle });
  const end = Date.now() + 15_000;
  while (service.exitCode === null && Date.now() < end) await delay(100);
  assert.equal(service.exitCode, 0, "Owned service did not stop cleanly");
}
function ownedContainers(sessionId) {
  const ids = docker("ps", "--all", "--filter", `label=luci.session-id=${sessionId}`, "--format", "{{.ID}}").trim().split(/\s+/).filter(Boolean);
  return ids.map(id => {
    const [container] = JSON.parse(docker("inspect", id));
    assert.equal(container.Config.Labels["luci.session-id"], sessionId);
    assert.ok(container.Mounts.some(mount => mount.Source.startsWith(join(state, "control-plane/teams", teamId) + "/")), "Container does not mount this fixture's team state");
    return container;
  });
}
function removeSessionContainers(sessionId) {
  for (const container of ownedContainers(sessionId)) docker("rm", "--force", container.Id);
  assert.equal(ownedContainers(sessionId).length, 0);
}
async function signIn(username = "service-owner") { token = (await request("POST", "/v1/auth/login", { username, password })).token; }
const access = (revision, status) => ({ expectedRevision: revision, mode: "managed", status, reason: "Local operations acceptance; no payment provider" });

try {
  await start();
  const bootstrapToken = readFileSync(join(state, "control-plane/bootstrap-token"), "utf8").trim();
  token = (await request("POST", "/v1/auth/bootstrap", { bootstrapToken, username: "service-owner", displayName: "Service owner", password }, 201)).token;
  await request("POST", "/v1/admin/users", { username: "service-outsider", displayName: "Outside account", password, role: "member" }, 201);
  const created = await request("POST", "/v1/teams", { name: "Bound service team" }, 201);
  teamId = created.team.id; result.teamId = teamId;
  const workspace = `/v1/teams/${teamId}/workspace`, servicePath = `/v1/admin/team-services/${teamId}`;
  const project = await request("POST", workspace + "/v1/product/projects", { name: "retained-team-project" }, 201);
  const projectPath = workspace + `/v1/product/projects/${project.id}`;
  let brief = await request("GET", projectPath + "/brief");
  brief = await request("POST", projectPath + "/brief", { revision: brief.revision, purpose: "Retain context across service suspension", audience: "Team members", constraints: "No inference", criteria: ["Personal workspaces cannot bypass the bound team"] });
  await stop(); token = undefined; await start(teamId); await signIn();
  assert.equal((await request("GET", "/v1/teams")).managedTeamId, teamId);
  const marker = readFileSync(join(state, "control-plane/managed-team.json"));
  assert.equal(JSON.parse(marker.toString()).teamId, teamId);
  for (const [method, path, body] of [["GET", "/v1/projects"], ["POST", "/v1/teams", { name: "Bypass" }], ["POST", "/v1/sessions", { harness: "shell" }]]) await request(method, path, body, 403);
  await signIn("service-outsider");
  assert.deepEqual(await request("GET", "/v1/teams"), { teams: [], managedTeamId: teamId });
  await request("GET", workspace + "/v1/projects", undefined, 403);
  await request("GET", "/v1/projects", undefined, 403);
  await signIn();
  checks.push({ name: "immutable commissioning blocks personal and additional-team bypass without granting outsiders membership", teamId });

  const { machines } = await request("GET", workspace + "/v1/machines");
  const { worktrees } = await request("GET", workspace + `/v1/projects/${project.id}/worktrees`);
  const body = { machineId: machines[0].id, worktreeId: worktrees[0].id, harness: "shell", title: "Service boundary proof" };
  const session = await request("POST", workspace + "/v1/sessions", body, 201); sessions.add(session.id);
  const [container] = ownedContainers(session.id); assert.ok(container); assert.equal(container.HostConfig.PidsLimit, 256);
  const ticket = await request("POST", "/v1/auth/socket-ticket", { path: session.attachPath });
  socket = new WebSocket(origin.replace("http:", "ws:") + session.attachPath + "?ticket=" + encodeURIComponent(ticket.ticket));
  await Promise.race([new Promise((done, reject) => { socket.addEventListener("open", done, { once: true }); socket.addEventListener("error", () => reject(new Error("Terminal could not open")), { once: true }); }), delay(15_000).then(() => { throw new Error("Terminal open timed out"); })]);
  const closed = new Promise(done => socket.addEventListener("close", done, { once: true }));
  await request("PUT", servicePath, access(0, "suspended"));
  await Promise.race([closed, delay(5_000).then(() => { throw new Error("Suspended terminal remained open"); })]);
  assert.equal((await request("POST", workspace + "/v1/sessions", body, 403)).error, "team_service_suspended");
  await request("POST", "/v1/auth/socket-ticket", { path: session.attachPath }, 403);
  await request("DELETE", workspace + `/v1/sessions/${session.id}`);
  for (const item of ownedContainers(session.id)) assert.equal(item.State.Running, false);
  removeSessionContainers(session.id);
  brief = await request("POST", projectPath + "/brief", { revision: brief.revision, purpose: "Context remains editable while execution is suspended", audience: "Team members", constraints: "No inference", criteria: ["Explicit stop and export remain available"] });
  const backup = await request("POST", projectPath + "/backups", {});
  assert.ok(Buffer.isBuffer(backup) && backup.length > 0, "Suspension blocked the project export");
  writeFileSync(join(evidence, "retained-project.zip"), backup, { mode: 0o600 });
  checks.push({ name: "suspension closes a real shell WebSocket and denies new work while explicit owned stop, context edit and export remain available", sessionId: session.id, backupBytes: backup.length });

  await stop(); token = undefined; await start(); await signIn();
  assert.deepEqual(readFileSync(join(state, "control-plane/managed-team.json")), marker);
  assert.equal((await request("GET", "/v1/teams")).managedTeamId, teamId);
  assert.equal((await request("GET", `/v1/teams/${teamId}/service`)).status, "suspended");
  assert.equal((await request("GET", projectPath + "/brief")).purpose, brief.purpose);
  await request("GET", "/v1/projects", undefined, 403);
  await request("PUT", servicePath, access(0, "active"), 409);
  await request("PUT", servicePath, access(1, "active"));
  const resumed = await request("POST", workspace + "/v1/sessions", body, 201); sessions.add(resumed.id);
  assert.equal(ownedContainers(resumed.id).length, 1);
  await request("DELETE", workspace + `/v1/sessions/${resumed.id}`);
  removeSessionContainers(resumed.id);
  checks.push({ name: "restart without binding environment retains team restriction, suspended state and context; explicit current-revision resume permits a new real shell", resumedSessionId: resumed.id });
  result.passed = true;
} catch (error) {
  result.error = String(error); process.exitCode = 1; console.error(error.stack);
} finally {
  socket?.close();
  try { await stop(); } catch (error) { result.stopError = String(error); service?.kill("SIGTERM"); process.exitCode = 1; }
  try { for (const id of sessions) removeSessionContainers(id); result.fixtureContainersRemoved = true; } catch (error) { result.cleanupError = String(error); process.exitCode = 1; }
  log.end(); result.finishedAt = new Date().toISOString();
  if (process.exitCode) result.passed = false;
  writeFileSync(join(evidence, "result.json"), JSON.stringify(result, null, 2) + "\n", { mode: 0o600 });
  console.log(JSON.stringify(result));
}
