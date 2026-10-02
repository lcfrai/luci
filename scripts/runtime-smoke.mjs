// Real Linux/Docker acceptance smoke. No model account or cloud resource is used.
// node scripts/runtime-smoke.mjs <compiled-luci-binary> <evidence-directory>
import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { createWriteStream, mkdtempSync, mkdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { setTimeout as delay } from "node:timers/promises";

assert.equal(process.platform, "linux", "This smoke needs a Linux Docker host");
assert.notEqual(process.getuid(), 0, "Run as the ordinary Docker-enabled host user");
const repository = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const binary = resolve(process.argv[2] ?? "artifacts/runtime-smoke/luci");
const evidence = resolve(process.argv[3] ?? "artifacts/runtime-smoke/evidence");
mkdirSync(evidence, { recursive: true, mode: 0o700 });
execFileSync("docker", ["info", "--format", "{{.ServerVersion}}"], { stdio: "inherit" });
const fixture = mkdtempSync(join(tmpdir(), "luci-runtime-smoke-"));
const projects = join(fixture, "projects");
const state = join(fixture, "state");
mkdirSync(projects, { mode: 0o700 });
mkdirSync(state, { mode: 0o700 });
const listener = createServer();
await new Promise((resolveReady, reject) => listener.once("error", reject).listen(0, "127.0.0.1", resolveReady));
const port = listener.address().port;
await new Promise((done) => listener.close(done));
const origin = `http://127.0.0.1:${port}`;
const image = `luci/sandbox:smoke-${randomUUID().replaceAll("-", "")}`;
const providerProbeID = randomUUID();
const output = createWriteStream(join(evidence, "control-plane.log"), { mode: 0o600 });
const child = spawn(binary, [], {
  cwd: repository,
  env: {
    ...process.env,
    LUCI_ADDR: `127.0.0.1:${port}`, LUCI_BACKEND: "docker",
    LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: projects, LUCI_RUNTIME_DIR: repository,
    LUCI_SANDBOX_IMAGE: image, LUCI_GIT_NAME: "Luci Runtime Test",
    LUCI_GIT_EMAIL: "runtime-test@example.invalid", LUCI_REAP_INTERVAL: "5s",
    LUCI_SESSION_CPU_MILLIS: "500", LUCI_SESSION_MEMORY_MIB: "512",
    GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: join(fixture, "empty-git-config"),
    ANTHROPIC_API_KEY: "", OPENAI_API_KEY: "",
  },
  stdio: ["ignore", "pipe", "pipe"],
});
child.stdout.pipe(output, { end: false });
child.stderr.pipe(output, { end: false });
const exited = new Promise((done) => child.once("close", done));
let spawnError;
child.once("error", (error) => { spawnError = error; });
const result = { passed: false, startedAt: new Date().toISOString(), fixture, checks: [] };
let session;
let release;
let project;
let authToken;
let userProjects;

async function api(method, path, body, timeout = 30_000) {
  const response = await fetch(origin + path, {
    method, redirect: "error", signal: AbortSignal.timeout(timeout),
    headers: { ...(body === undefined ? {} : { "Content-Type": "application/json" }), ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${text.slice(0, 4000)}`);
  return text ? JSON.parse(text) : null;
}

async function until(label, operation, timeout = 30_000) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    if (spawnError) throw spawnError;
    if (child.exitCode !== null) throw new Error(`Luci exited (${child.exitCode}); see control-plane.log`);
    try { const value = await operation(); if (value) return value; } catch (error) {
      if (error.fatal) throw error;
      last = error;
    }
    await delay(500);
  }
  throw new Error(`${label} timed out${last ? `: ${last.message}` : ""}`);
}

function containerFor(label, value) {
  const ids = execFileSync("docker", ["ps", "--all", "--filter", `label=${label}=${value}`, "--format", "{{.ID}}"], { encoding: "utf8" }).trim().split(/\s+/).filter(Boolean);
  assert.equal(ids.length, 1, `Expected one owned container for ${label}`);
  return inspectContainer(ids[0], label, value);
}

function inspectContainer(id, label, value) {
  assert.match(id, /^[a-f0-9]{12,64}$/);
  const info = JSON.parse(execFileSync("docker", ["inspect", id], { encoding: "utf8" }))[0];
  assert.equal(info.Config.Labels[label], value);
  return info;
}

function assertLoopbackPorts(info) {
  const bindings = Object.values(info.NetworkSettings.Ports ?? {}).flatMap((rows) => rows ?? []);
  assert.ok(bindings.length > 0, "No published loopback port");
  assert.ok(bindings.every((binding) => binding.HostIp === "127.0.0.1"), "A runtime port escaped loopback");
}

async function proveShell(sessionID) {
  const path = `/v1/sessions/${sessionID}/attach`;
  const { ticket } = await api("POST", "/v1/auth/socket-ticket", { path });
  const socket = new WebSocket(origin.replace("http:", "ws:") + path + `?ticket=${encodeURIComponent(ticket)}`);
  socket.binaryType = "arraybuffer";
  const suffix = randomUUID().replaceAll("-", "");
  const marker = `LUCISMOKE_${suffix}`;
  let transcript = "";
  const probeCode = `const fs=require("fs"); const report=JSON.stringify({uid:process.getuid(),gid:process.getgid(),status:fs.readFileSync("/proc/self/status","utf8"),brief:fs.readFileSync("LUCI.md","utf8")}); fs.writeFileSync("runtime-proof.json",report,{mode:384});`;
  const command = `node -e '${probeCode}' && git add runtime-proof.json && git commit -m 'Runtime smoke shell write' && printf '\\nLUCISMOKE_%s\\n' '${suffix}'\r`;
  try {
    await new Promise((done, reject) => {
      const timeout = setTimeout(() => reject(new Error("Shell WebSocket proof timed out")), 30_000);
      const fail = (error) => { clearTimeout(timeout); reject(error); };
      socket.addEventListener("error", () => fail(new Error("Shell WebSocket failed")));
      socket.addEventListener("close", () => { if (!transcript.includes(marker)) fail(new Error("Shell closed before executing the probe")); });
      socket.addEventListener("open", () => {
        socket.send(Uint8Array.from([2, 0, 100, 0, 30]));
        socket.send(Buffer.concat([Buffer.from([1]), Buffer.from(command)]));
      });
      socket.addEventListener("message", (event) => {
        const bytes = Buffer.from(event.data);
        if (bytes[0] === 0) {
          transcript = (transcript + bytes.subarray(1).toString("utf8")).slice(-128 * 1024);
          if (transcript.includes(marker)) { clearTimeout(timeout); done(); }
        } else if (bytes[0] === 3 && !transcript.includes(marker)) {
          fail(new Error("Runtime exited before shell proof"));
        }
      });
    });
  } finally {
    socket.close();
    writeFileSync(join(evidence, "shell-transcript.txt"), transcript, { mode: 0o600 });
  }
}

function cleanupOwned(label, value) {
  if (!value) return;
  const ids = execFileSync("docker", ["ps", "--all", "--filter", `label=${label}=${value}`, "--format", "{{.ID}}"], { encoding: "utf8" }).trim().split(/\s+/).filter(Boolean);
  for (const id of ids) {
    inspectContainer(id, label, value);
    execFileSync("docker", ["rm", "--force", id], { stdio: "ignore" });
  }
}

try {
  await until("Control plane readiness", () => api("GET", "/v1/control-plane/status", undefined, 2000));
  const bootstrapToken = readFileSync(join(state, "control-plane", "bootstrap-token"), "utf8").trim();
  const identity = await api("POST", "/v1/auth/bootstrap", { bootstrapToken, username: "runtime-owner", displayName: "Runtime Owner", password: randomUUID() + randomUUID() });
  authToken = identity.token;
  assert.ok(authToken && identity.user.id);
  userProjects = projects; // The first owner uses the configured inline workspace.
  await until("Authenticated private workspace readiness", () => api("GET", "/v1/setup/status", undefined, 35_000), 45_000);
  await api("POST", "/v1/setup/git/identity", { name: "Luci Runtime Test", email: "runtime-test@example.invalid" });
  const install = await api("POST", "/v1/setup/runtime/install", {});
  assert.equal(install.image, image);
  const runtime = await until("Real sandbox image build", async () => {
    const status = await api("GET", "/v1/setup/runtime/install");
    if (status.status === "failed" || status.status === "unknown") {
      throw Object.assign(new Error(status.message), { fatal: true });
    }
    return status.status === "ready" && status;
  }, 11 * 60_000);
  result.checks.push({ name: "real Docker image install", status: runtime.status });

  // Real pinned providers speak only to a fixture API inside this container.
  // Docker network=none makes accidental external model access impossible.
  const providerProof = execFileSync("docker", ["run", "--rm", "--network", "none", "--read-only",
    "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
    "--tmpfs", "/tmp:rw,nosuid,nodev,size=536870912", "--label", `ai.lcfr.luci.provider-smoke=${providerProbeID}`,
    "--mount", `type=bind,source=${join(repository, "scripts", "provider-smoke.py")},target=/provider-smoke.py,readonly`,
    "--entrypoint", "/usr/bin/python3", image, "/provider-smoke.py"],
    { encoding: "utf8", timeout: 180_000, maxBuffer: 8 * 1024 * 1024 });
  writeFileSync(join(evidence, "provider-smoke.json"), providerProof, { mode: 0o600 });
  const providerResult = JSON.parse(providerProof);
  assert.equal(providerResult.passed, true);
  result.checks.push({ name: "real Codex/OpenCode fixture completion and durable fork; isolated network", toolchains: providerResult.toolchains });

  project = await api("POST", "/v1/product/projects", { name: "runtime-proof" });
  assert.ok(resolve(project.path).startsWith(userProjects + sep), "Project escaped the authenticated user's test root");
  const originalMode = statSync(project.path).mode & 0o777;
  assert.equal(originalMode, 0o700);
  await api("POST", `/v1/product/projects/${project.id}/brief`, { revision: 0, purpose: "Prove the actual Linux runtime", audience: "Release verification", constraints: "No model credentials or cloud resources", criteria: ["A real shell writes as its host user", "A committed preview answers HTTP"] });
  await api("POST", `/v1/product/projects/${project.id}/git/commit`, { message: "Runtime smoke initial brief" });
  const machines = (await api("GET", "/v1/machines")).machines;
  const branches = (await api("GET", `/v1/projects/${project.id}/branches`)).branches;
  const branch = branches.find((row) => row.name === "main") ?? branches[0];
  assert.ok(branch, "Project has no initial branch");
  const worktree = (await api("GET", `/v1/projects/${project.id}/worktrees?branchId=${encodeURIComponent(branch.id)}`)).worktrees[0];
  assert.ok(worktree);
  session = await api("POST", "/v1/sessions", { machineId: machines[0].id, worktreeId: worktree.id, harness: "shell", title: "Runtime proof" }, 90_000);
  const sandbox = containerFor("luci.session-id", session.id);
  assertLoopbackPorts(sandbox);
  // A plain shell deliberately has no agent conversation mount. Do not put
  // other sessions' transcripts in reach of a shell just to satisfy this test.
  assert.ok(!sandbox.Mounts.some((mount) => mount.Destination === "/home/dev/.claude/projects"));
  await proveShell(session.id);
  const proofFile = join(project.path, "runtime-proof.json");
  const proof = JSON.parse(readFileSync(proofFile, "utf8"));
  assert.equal(proof.uid, process.getuid());
  assert.equal(proof.gid, process.getgid());
  assert.notEqual(proof.uid, 0);
  assert.match(proof.brief, /Prove the actual Linux runtime/);
  assert.match(proof.status, /^CapEff:\s*0+$/m);
  assert.match(proof.status, /^CapBnd:\s*0+$/m);
  assert.match(proof.status, /^NoNewPrivs:\s*1$/m);
  assert.equal(statSync(proofFile).uid, process.getuid());
  assert.equal(statSync(proofFile).gid, process.getgid());
  assert.equal(statSync(project.path).mode & 0o777, originalMode, "Runtime relaxed the project directory mode");
  const shellGit = await api("GET", `/v1/product/projects/${project.id}/git`);
  assert.equal(shellGit.log[0].subject, "Runtime smoke shell write");
  result.checks.push({ name: "shell WebSocket, brief read and Git write through private mount", uid: proof.uid, gid: proof.gid, capabilities: "none", noNewPrivileges: true, projectMode: "0700", shellCommit: shellGit.log[0].hash });
  await api("DELETE", `/v1/sessions/${session.id}`);

  // Start the real conversation-bearing harness without credentials, a prompt
  // or an attached client. This exercises its API-created persistent mount;
  // it does not sign in or ask a model to perform work. A separate local probe
  // writes as the mapped host UID, never Docker exec's bootstrap-root default.
  session = await api("POST", "/v1/sessions", { machineId: machines[0].id, worktreeId: worktree.id, harness: "claude-code", title: "Transcript mount proof" }, 90_000);
  const agent = containerFor("luci.session-id", session.id);
  assertLoopbackPorts(agent);
  const transcripts = agent.Mounts.find((mount) => mount.Destination === "/home/dev/.claude/projects")?.Source;
  assert.ok(transcripts, "Conversation-bearing session did not mount its persistent transcript store");
  const transcriptStat = statSync(transcripts);
  assert.equal(transcriptStat.uid, process.getuid());
  assert.equal(transcriptStat.gid, process.getgid());
  assert.equal(transcriptStat.mode & 0o777, 0o700);
  const transcriptProbeCode = 'const fs=require("fs"); const report={uid:process.getuid(),gid:process.getgid(),runtimeStatus:fs.readFileSync("/proc/1/status","utf8")}; fs.writeFileSync("/home/dev/.claude/projects/runtime-proof.json",JSON.stringify(report),{mode:384});';
  execFileSync("docker", ["exec", "--user", `${process.getuid()}:${process.getgid()}`, agent.Id, "node", "-e", transcriptProbeCode], { timeout: 15_000, stdio: ["ignore", "pipe", "pipe"] });
  const transcriptProofFile = join(transcripts, "runtime-proof.json");
  const transcriptProof = statSync(transcriptProofFile);
  const transcriptReport = JSON.parse(readFileSync(transcriptProofFile, "utf8"));
  assert.equal(transcriptReport.uid, process.getuid());
  assert.equal(transcriptReport.gid, process.getgid());
  assert.match(transcriptReport.runtimeStatus, new RegExp(`^Uid:\\s*${process.getuid()}\\s+${process.getuid()}\\s+${process.getuid()}\\s+${process.getuid()}$`, "m"));
  assert.match(transcriptReport.runtimeStatus, new RegExp(`^Gid:\\s*${process.getgid()}\\s+${process.getgid()}\\s+${process.getgid()}\\s+${process.getgid()}$`, "m"));
  assert.match(transcriptReport.runtimeStatus, /^CapEff:\s*0+$/m);
  assert.match(transcriptReport.runtimeStatus, /^CapBnd:\s*0+$/m);
  assert.match(transcriptReport.runtimeStatus, /^NoNewPrivs:\s*1$/m);
  assert.equal(transcriptProof.uid, process.getuid());
  assert.equal(transcriptProof.gid, process.getgid());
  assert.equal(transcriptProof.mode & 0o777, 0o600);
  assert.equal(statSync(transcripts).mode & 0o777, 0o700, "Runtime relaxed its transcript store mode");
  result.checks.push({ name: "API-created agent transcript mount without model inference", uid: transcriptReport.uid, gid: transcriptReport.gid, directoryMode: "0700", fileMode: "0600", runtimeCapabilities: "none", noNewPrivileges: true });
  await api("DELETE", `/v1/sessions/${session.id}`);

  writeFileSync(join(project.path, "app.mjs"), 'import http from "node:http"; http.createServer((req,res)=>{res.writeHead(200,{"content-type":"text/plain"});res.end("Luci runtime preview OK");}).listen(3000,"0.0.0.0");\n', { mode: 0o600 });
  writeFileSync(join(project.path, "Dockerfile"), `FROM ${image}\nWORKDIR /app\nCOPY --chown=dev:dev app.mjs /app/app.mjs\nUSER dev\nEXPOSE 3000\nENTRYPOINT ["node","/app/app.mjs"]\n`, { mode: 0o600 });
  await api("POST", `/v1/product/projects/${project.id}/git/commit`, { message: "Runtime smoke committed preview" });
  const git = await api("GET", `/v1/product/projects/${project.id}/git`);
  assert.equal(git.clean, true);
  release = await api("POST", `/v1/product/projects/${project.id}/releases`, { target: "local", port: 3000, dockerfile: "Dockerfile" }, 4 * 60_000);
  assert.equal(release.status, "ready");
  assert.equal(release.commit, git.log[0].hash);
  assert.equal(release.briefRevision, 1);
  const address = new URL(release.url);
  assert.equal(address.protocol, "http:");
  assert.equal(address.hostname, "127.0.0.1");
  assert.ok(Number(address.port) > 0);
  const application = containerFor("luci.release-id", release.id);
  assert.equal(application.Config.Labels["luci.project-id"], project.id);
  assertLoopbackPorts(application);
  const response = await fetch(release.url, { redirect: "error", signal: AbortSignal.timeout(5000) });
  assert.equal(response.status, 200);
  assert.equal(await response.text(), "Luci runtime preview OK");
  const stopped = await api("POST", `/v1/product/projects/${project.id}/releases/${release.id}/stop`);
  assert.equal(stopped.status, "stopped");
  const remaining = execFileSync("docker", ["ps", "--all", "--filter", `label=luci.release-id=${release.id}`, "--format", "{{.ID}}"], { encoding: "utf8" }).trim();
  assert.equal(remaining, "", "Owned preview container survived Stop");
  result.checks.push({ name: "committed Docker preview, actual HTTP health and owned stop", commit: release.commit, briefRevision: release.briefRevision, removed: true });
  result.passed = true;
  console.log("PASS: real Docker install, non-root shell WebSocket/write, committed preview health and ownership-checked stop.");
} catch (error) {
  result.error = error.message;
  process.exitCode = 1;
  console.error(error.stack);
} finally {
  cleanupOwned("ai.lcfr.luci.provider-smoke", providerProbeID);
  try { if (release && project) await api("POST", `/v1/product/projects/${project.id}/releases/${release.id}/stop`, undefined, 10_000); } catch {}
  try { if (session) await api("DELETE", `/v1/sessions/${session.id}`, undefined, 10_000); } catch {}
  for (const [label, value] of [["luci.session-id", session?.id], ["luci.release-id", release?.id]]) {
    try { cleanupOwned(label, value); } catch (error) { result.cleanupError = error.message; process.exitCode = 1; }
  }
  try { await api("POST", "/v1/setup/shutdown", undefined, 5000); } catch {}
  child.kill("SIGTERM");
  await Promise.race([exited, delay(5000)]);
  if (child.exitCode === null && !spawnError) { child.kill("SIGKILL"); await exited; }
  output.end();
  result.finishedAt = new Date().toISOString();
  if (process.exitCode) result.passed = false;
  writeFileSync(join(evidence, "result.json"), JSON.stringify(result, null, 2) + "\n", { mode: 0o600 });
}
