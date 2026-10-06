// Actual PostgreSQL backup/import/restore/rollback against an isolated compiled
// service. No model or cloud service. Only this fixture's labeled Docker
// containers, volumes and networks are removed at completion.
// node scripts/database-recovery-smoke.mjs <binary> <fresh-evidence-directory>
import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { createWriteStream, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { setTimeout as delay } from "node:timers/promises";

assert.equal(process.platform, "linux");
assert.notEqual(process.getuid(), 0);
const repository = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const binary = resolve(process.argv[2]);
const evidence = resolve(process.argv[3]);
mkdirSync(evidence, { recursive: false, mode: 0o700 });
const fixture = mkdtempSync(join(tmpdir(), "luci-database-recovery-"));
const state = join(fixture, "state"), projects = join(fixture, "projects");
mkdirSync(state, { mode: 0o700 }); mkdirSync(projects, { mode: 0o700 });
const listener = createServer();
await new Promise(done => listener.listen(0, "127.0.0.1", done));
const address = `127.0.0.1:${listener.address().port}`;
await new Promise(done => listener.close(done));
const origin = `http://${address}`;
const lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const environment = {
  ...Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith("LUCI_"))),
  LUCI_ADDR: address, LUCI_BACKEND: "docker", LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: projects,
  LUCI_RUNTIME_DIR: repository, LUCI_WEB_DIR: join(repository, "desktop/dist"),
  LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_SANDBOX_IMAGE: "luci/sandbox:dev",
  GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: join(fixture, "empty-git-config"),
  ANTHROPIC_API_KEY: "", OPENAI_API_KEY: "",
};
const projectLabel = "ai.lcfr.luci.platform.project", instanceLabel = "ai.lcfr.luci.platform.instance", resourceLabel = "ai.lcfr.luci.platform.resource";
const owned = new Map();
const checks = [];
const result = { passed: false, binary, binarySHA256: createHash("sha256").update(readFileSync(binary)).digest("hex"), fixture, startedAt: new Date().toISOString(), checks };
const log = createWriteStream(join(evidence, "control-plane.log"), { mode: 0o600 });
let service, token, spawnError;
const docker = (...args) => execFileSync("docker", args, { encoding: "utf8", timeout: 30_000, maxBuffer: 4 * 1024 * 1024 });

async function request(method, path, body, expected = 200, headers = {}) {
  const binaryBody = Buffer.isBuffer(body);
  const response = await fetch(origin + path, {
    method, redirect: "error", signal: AbortSignal.timeout(200_000),
    headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(body === undefined ? {} : { "Content-Type": binaryBody ? "application/octet-stream" : "application/json" }), ...headers },
    body: body === undefined ? undefined : binaryBody ? body : JSON.stringify(body),
  });
  const bytes = Buffer.from(await response.arrayBuffer());
  assert.ok([expected].flat().includes(response.status), `${method} ${path}: ${response.status} ${bytes.toString().slice(0, 1200)}`);
  return response.headers.get("Content-Type")?.startsWith("application/octet-stream") ? bytes : bytes.length ? JSON.parse(bytes.toString()) : undefined;
}
async function until(label, read, budget = 45_000) {
  const end = Date.now() + budget;
  while (Date.now() < end) {
    if (spawnError) throw spawnError;
    assert.equal(service.exitCode, null, "Fixture service exited unexpectedly");
    const value = await read(); if (value) return value;
    await delay(150);
  }
  throw new Error(`${label} timed out`);
}
async function start() {
  service = spawn(binary, [], { cwd: repository, env: environment, stdio: ["ignore", "pipe", "pipe"] });
  service.on("error", error => { spawnError = error; });
  service.stdout.pipe(log, { end: false }); service.stderr.pipe(log, { end: false });
  const control = await until("Service readiness", async () => {
    try { return await request("GET", "/v1/control-plane/status"); } catch { return false; }
  });
  assert.equal(control.product, "Luci"); result.version = control.version;
}
async function stop() {
  if (!service || service.exitCode !== null) return;
  await request("POST", "/v1/control-plane/shutdown", undefined, 200, { "X-Luci-Lifecycle-Token": lifecycle });
  const end = Date.now() + 15_000;
  while (service.exitCode === null && Date.now() < end) await delay(100);
  assert.equal(service.exitCode, 0);
}
function databaseContainer(projectId) {
  const ids = docker("ps", "--all", "--filter", `label=${projectLabel}=${projectId}`, "--filter", `label=${resourceLabel}=postgres`, "--format", "{{.ID}}").trim().split(/\s+/).filter(Boolean);
  assert.equal(ids.length, 1);
  const [container] = JSON.parse(docker("inspect", ids[0]));
  assert.equal(container.Config.Labels[projectLabel], projectId);
  assert.equal(container.Config.Labels[resourceLabel], "postgres");
  const instance = container.Config.Labels[instanceLabel]; assert.ok(instance);
  if (owned.has(projectId)) assert.equal(instance, owned.get(projectId)); else owned.set(projectId, instance);
  assert.equal(container.HostConfig.PidsLimit, 256);
  return container.Id;
}
function sql(projectId, database, command) {
  assert.match(database, /^[a-z][a-z0-9_]{0,62}$/);
  return docker("exec", databaseContainer(projectId), "psql", "-U", "luci", "-d", database, "-v", "ON_ERROR_STOP=1", "-Atqc", command).trim();
}
async function settle(projectId, operation) {
  const state = await until("Database recovery", async () => {
    const value = await request("GET", `/v1/platform/projects/${projectId}`);
    return !value.busy && value.project.databaseOperation?.id === operation.id && value.project.databaseOperation.status !== "running" && value.project;
  }, 180_000);
  assert.equal(state.databaseOperation.status, "succeeded", state.databaseOperation.message);
  assert.equal(state.databaseOperation.stage, "complete");
  return state;
}
function cleanup() {
  for (const [projectId, instance] of owned) {
    const filters = ["--filter", `label=${projectLabel}=${projectId}`, "--filter", `label=${instanceLabel}=${instance}`];
    for (const id of docker("ps", "--all", ...filters, "--format", "{{.ID}}").trim().split(/\s+/).filter(Boolean)) {
      const [entry] = JSON.parse(docker("inspect", id));
      assert.equal(entry.Config.Labels[projectLabel], projectId); assert.equal(entry.Config.Labels[instanceLabel], instance);
      docker("rm", "--force", id);
    }
    for (const kind of ["volume", "network"]) {
      const names = docker(kind, "ls", ...filters, "--format", "{{.Name}}").trim().split(/\s+/).filter(Boolean);
      for (const name of names) {
        const [entry] = JSON.parse(docker(kind, "inspect", name));
        assert.equal(entry.Labels[projectLabel], projectId); assert.equal(entry.Labels[instanceLabel], instance);
        docker(kind, "rm", name);
      }
    }
  }
}

try {
  await start();
  const bootstrapToken = readFileSync(join(state, "control-plane/bootstrap-token"), "utf8").trim();
  token = (await request("POST", "/v1/auth/bootstrap", { bootstrapToken, username: "database-owner", displayName: "Database fixture", password }, [200, 201])).token;
  await request("GET", "/v1/setup/status");
  const source = await request("POST", "/v1/product/projects", { name: "source-database" }, [200, 201]);
  const recovered = await request("POST", "/v1/product/projects", { name: "recovered-database" }, [200, 201]);
  const from = `/v1/platform/projects/${source.id}`, to = `/v1/platform/projects/${recovered.id}`;
  await request("POST", from + "/database", { action: "start" });
  sql(source.id, "app", "CREATE TABLE recovery_probe (marker text NOT NULL); INSERT INTO recovery_probe VALUES ('saved-source');");
  const backup = await request("POST", from + "/database", { action: "backup" }, 201);
  const bytes = await request("GET", `${from}/backups/${backup.id}`);
  assert.ok(Buffer.isBuffer(bytes)); assert.equal(bytes.subarray(0, 5).toString(), "PGDMP");
  assert.equal(createHash("sha256").update(bytes).digest("hex"), backup.sha256);
  writeFileSync(join(evidence, "fixture.dump"), bytes, { mode: 0o600 });
  await request("GET", `${to}/backups/${backup.id}`, undefined, 404);
  const headers = { "X-Luci-Trust-Backup": "true", "X-Luci-Backup-SHA256": backup.sha256 };
  await request("POST", `${to}/backups/import`, bytes, 400, { "X-Luci-Backup-SHA256": backup.sha256 });
  const imported = await request("POST", `${to}/backups/import`, bytes, 201, headers);
  assert.equal(imported.imported, true); assert.notEqual(imported.id, backup.id); assert.equal(imported.sha256, backup.sha256);
  assert.deepEqual(await request("GET", `${to}/backups/${imported.id}`), bytes);
  assert.deepEqual(await request("POST", `${to}/backups/import`, bytes, 200, headers), imported);
  const untouched = (await request("GET", to)).project;
  assert.equal(untouched.database.status, "not-created"); assert.equal(untouched.databaseOperation, undefined);
  assert.equal(docker("ps", "--all", "--filter", `label=${projectLabel}=${recovered.id}`, "--format", "{{.ID}}").trim(), "");
  checks.push({ name: "real custom-format dump download and explicitly trusted import into a different project", bytes: imported.bytes, sha256: imported.sha256, noDatabaseStarted: true, retryReusedReceipt: true });

  await request("POST", to + "/database", { action: "start" });
  sql(recovered.id, "app", "CREATE TABLE recovery_probe (marker text NOT NULL); INSERT INTO recovery_probe VALUES ('previous-target');");
  await request("POST", to + "/database", { action: "restore", backupId: imported.id }, 400);
  await request("POST", to + "/database", { action: "restore", backupId: imported.id, confirmRestore: true, acknowledgeResources: true }, 409);
  await request("POST", to + "/database", { action: "stop" });
  const accepted = await request("POST", to + "/database", { action: "restore", backupId: imported.id, confirmRestore: true, acknowledgeResources: true }, 202);
  const restored = await settle(recovered.id, accepted);
  assert.notEqual(restored.database.database, "app");
  assert.equal(sql(recovered.id, restored.database.database, "SELECT marker FROM recovery_probe"), "saved-source");
  assert.equal(sql(recovered.id, "app", "SELECT marker FROM recovery_probe"), "previous-target");
  checks.push({ name: "real pg_restore into a verified candidate preserves the original database", restoreId: accepted.id, previousDatabase: "app", restoredDatabase: restored.database.database });

  await request("POST", to + "/database", { action: "stop" });
  const rollback = await request("POST", to + "/database", { action: "rollback-restore", restoreId: accepted.id, confirmRestore: true, acknowledgeResources: true }, 202);
  const rolledBack = await settle(recovered.id, rollback);
  assert.equal(rolledBack.database.database, "app");
  assert.equal(sql(recovered.id, "app", "SELECT marker FROM recovery_probe"), "previous-target");
  assert.equal(sql(recovered.id, restored.database.database, "SELECT marker FROM recovery_probe"), "saved-source");
  await stop(); token = undefined; await start();
  token = (await request("POST", "/v1/auth/login", { username: "database-owner", password })).token;
  const retained = (await request("GET", to)).project;
  assert.equal(retained.database.database, "app");
  assert.ok(retained.backups.some(row => row.id === imported.id && row.sha256 === imported.sha256));
  assert.deepEqual(retained.databaseRestoreHistory.map(row => row.id), [rollback.id, accepted.id]);
  assert.equal(sql(recovered.id, "app", "SELECT marker FROM recovery_probe"), "previous-target");
  checks.push({ name: "explicit rollback and real service restart preserve both data generations and private backup/history receipts", rollbackId: rollback.id });
  await request("POST", from + "/database", { action: "stop" });
  await request("POST", to + "/database", { action: "stop" });
  result.passed = true;
} catch (error) {
  result.error = String(error); process.exitCode = 1; console.error(error.stack);
} finally {
  try { await stop(); } catch (error) { result.stopError = String(error); service?.kill("SIGTERM"); process.exitCode = 1; }
  try { cleanup(); result.fixtureResourcesRemoved = true; } catch (error) { result.cleanupError = String(error); process.exitCode = 1; }
  log.end(); result.finishedAt = new Date().toISOString();
  if (process.exitCode) result.passed = false;
  writeFileSync(join(evidence, "result.json"), JSON.stringify(result, null, 2) + "\n", { mode: 0o600 });
  console.log(JSON.stringify(result));
}
