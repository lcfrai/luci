// Real, resumable Luci → Codex subscription → Docker → public-site acceptance.
// This file never writes application source, supplies a provider fixture, reads
// an auth file, starts a host, provisions cloud resources or injects a socket.
// Each mutation is a visible Luci control; authenticated API GETs verify facts.
// See --help. A local preview, saved account or completion marker is NOT a pass.
import assert from "node:assert/strict";
import { randomUUID, createHash } from "node:crypto";
import { mkdir, readFile, writeFile, rename } from "node:fs/promises";
import path from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { chromium } from "../desktop/node_modules/playwright/index.mjs";

const phases = ["prepare", "auth", "develop", "verify", "deploy", "status"];
const args = process.argv.slice(2);
function option(name, fallback = "") {
  const index = args.indexOf(name);
  return index < 0 ? fallback : args[index + 1];
}
const required = ["describe", "design", "plan", "subscription", "agent", "dockerChecks", "localSite", "publication", "publicSite"];
function complete(value) {
  return required.every(key => value.proofs?.[key]?.passed === true) &&
    value.proofs.agent.authMode === "chatgpt" &&
    value.proofs.dockerChecks.execution === "docker" &&
    value.proofs.dockerChecks.commit === value.commit &&
    value.proofs.localSite.commit === value.commit &&
    value.proofs.publication.commit === value.commit &&
    value.proofs.publicSite.commit === value.commit &&
    /^https:\/\//.test(value.proofs.publicSite.url || "");
}
if (args.includes("--help")) {
  console.log(`Usage: node scripts/subscription-journey-eval.mjs --phase <${phases.join("|")}> --evidence <directory> [--headed]

Required environment: LUCI_EVAL_URL (running Luci), LUCI_UAT_USERNAME,
LUCI_UAT_PASSWORD. Use an account/workspace intended for this real project.
Optional: LUCI_EVAL_PROJECT_NAME, LUCI_UAT_EVIDENCE (instead of --evidence),
LUCI_EVAL_REPOSITORY=https://github.com/owner/repo.git (an existing, empty public
repository), LUCI_EVAL_PERMISSION=ask|bypass (default ask), LUCI_EVAL_WAIT_MS
(bounded 0..1200000; default 0). Bypass must be an explicit operator choice.

Run prepare → auth → develop → verify → deploy, reusing the evidence directory.
auth opens official device sign-in inside Luci. Complete it yourself in Luci,
then rerun auth to save that sign-in. The harness never captures its contents.
develop starts the saved plan with genuine Codex; inspect any approval request
in Luci. verify waits only for agent-authored work, stops that owned session,
commits through Git, runs Docker checks and exercises the actual preview.
deploy connects/pushes the supplied repository through Luci and uses Luci's
GitHub Pages adapter. Public deployment and its independent browser checks are
mandatory. No model call occurs unless Luci reports a ready Docker backing.

Exit 0 = all mandatory outcomes passed; 2 = pending prerequisite/user action;
1 = actual failed assertion/request. Receipt: results.json. No automatic service,
project, repository or deployment deletion. Browser credentials are not saved.
Screenshots mask Luci terminals; no tracing, HAR or auth transcript recording.
--self-test checks only the final evidence gate and makes no network calls.`);
  process.exit(0);
}
if (args.includes("--self-test")) {
  const value = { commit: "abc", proofs: Object.fromEntries(required.map(key => [key, { passed: true, commit: "abc" }])) };
  value.proofs.agent.authMode = "chatgpt";
  value.proofs.dockerChecks.execution = "docker";
  value.proofs.publicSite.url = "https://owner.github.io/project/";
  assert.equal(complete(value), true);
  for (const key of required) {
    const copy = structuredClone(value); delete copy.proofs[key];
    assert.equal(complete(copy), false, `Missing ${key} cannot pass`);
  }
  for (const [key, field, wrong] of [["agent", "authMode", "api"], ["dockerChecks", "execution", "host"], ["publicSite", "commit", "other"], ["publicSite", "url", "http://127.0.0.1:3000"]]) {
    const copy = structuredClone(value); copy.proofs[key][field] = wrong;
    assert.equal(complete(copy), false);
  }
  console.log("Evidence gate passed: all stages, subscription mode, Docker, commit and public HTTPS required. No E2E run performed.");
  process.exit(0);
}

const phase = option("--phase");
assert(phases.includes(phase), "Choose an explicit --phase; see --help");
const evidenceInput = option("--evidence", process.env.LUCI_UAT_EVIDENCE);
assert(evidenceInput, "Provide a dedicated --evidence directory");
const evidence = path.resolve(evidenceInput), receiptPath = path.join(evidence, "results.json");
const base = new URL(process.env.LUCI_EVAL_URL || "");
assert(!base.username && !base.password && !base.search && !base.hash && base.pathname === "/", "Use a clean Luci origin");
assert(base.protocol === "https:" || (base.protocol === "http:" && ["127.0.0.1", "localhost", "[::1]"].includes(base.hostname)), "Luci must use HTTPS or an explicit loopback tunnel");
const origin = base.origin, username = process.env.LUCI_UAT_USERNAME, password = process.env.LUCI_UAT_PASSWORD;
assert(username && password, "Provide Luci account credentials through environment variables");
const waitMs = Number(process.env.LUCI_EVAL_WAIT_MS || 0);
assert(Number.isInteger(waitMs) && waitMs >= 0 && waitMs <= 1200000);
const permission = process.env.LUCI_EVAL_PERMISSION || "ask";
assert(["ask", "bypass"].includes(permission));
await mkdir(evidence, { recursive: true, mode: 0o700 });
let result;
try { result = JSON.parse(await readFile(receiptPath, "utf8")); }
catch (error) { if (error.code !== "ENOENT") throw error; }
result ||= {
  schema: 1, benchmark: "Luci Launch Checklist", origin, username,
  nonce: randomUUID(), startedAt: new Date().toISOString(), status: "pending", passed: false,
  projectName: process.env.LUCI_EVAL_PROJECT_NAME || `luci-launch-checklist-${randomUUID().slice(0, 8)}`,
  proofs: {}, attempts: [],
  limits: ["Saved sign-in is not proof of successful inference.", "Agent marker is a staging signal only; independent Docker and public browser checks decide success.", "Runtime, authentication, provider use and publication are real; there are no mocked routes or model fixtures.", "No credentials, device codes, terminal contents, traces or HAR files are retained.", "The public site and owned project remain available after the run."],
};
assert.equal(result.schema, 1);
assert.equal(result.origin, origin, "Evidence belongs to another Luci instance");
assert.equal(result.username, username, "Evidence belongs to another Luci account");
const attempt = { phase, startedAt: new Date().toISOString(), status: "pending" };
result.attempts.push(attempt);
let browser, page, token;
const sockets = new Map();
class Pending extends Error {}
const pending = reason => { throw new Pending(reason); };
const hash = value => createHash("sha256").update(value).digest("hex");
const projectPath = suffix => `/v1/product/projects/${encodeURIComponent(result.project.id)}${suffix}`;
async function save() {
  result.updatedAt = new Date().toISOString();
  const temporary = receiptPath + ".tmp";
  await writeFile(temporary, JSON.stringify(result, null, 2) + "\n", { mode: 0o600 });
  await rename(temporary, receiptPath);
}
async function proof(name, fields) {
  result.proofs[name] = { passed: true, at: new Date().toISOString(), ...fields };
  await save();
}
async function checkpoint(stage) { attempt.stage = stage; await save(); }
function requireProof(...names) {
  for (const name of names) if (!result.proofs[name]?.passed) pending(`Complete the ${name} stage first; stages cannot be skipped.`);
}
async function get(route, optional = false) {
  const response = await fetch(origin + route, { headers: token ? { Authorization: `Bearer ${token}` } : {}, signal: AbortSignal.timeout(15000) });
  if (optional && response.status === 404) return null;
  assert(response.ok, `Read-only ${route.split("?")[0]} returned HTTP ${response.status}`);
  return response.json();
}
async function observed(action, route, timeout = 60000) {
  const [response] = await Promise.all([
    page.waitForResponse(res => new URL(res.url()).origin === origin && new URL(res.url()).pathname === route && res.request().method() === "POST", { timeout }),
    action(),
  ]);
  // Never include arbitrary server/CLI bodies in errors: they may contain secrets.
  assert(response.ok(), `${route} returned HTTP ${response.status()}; inspect the visible Luci error.`);
  return response.json();
}
async function until(read, milliseconds = 45000) {
  const end = Date.now() + milliseconds;
  do { const value = await read(); if (value) return value; if (Date.now() >= end) return null; await delay(1500); } while (true);
}
async function screenshot(name) {
  await page.screenshot({ path: path.join(evidence, name + ".png"), fullPage: true, mask: [page.locator(".pane-body")] });
}
async function enter() {
  await checkpoint("entry.connection");
  browser = await chromium.launch({ headless: !args.includes("--headed") });
  page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.setDefaultTimeout(45000);
  page.on("websocket", socket => {
    const match = /\/v1\/sessions\/([^/]+)\/attach/.exec(socket.url());
    if (!match) return;
    const id = decodeURIComponent(match[1]), counts = sockets.get(id) || { outputFrames: 0, inputFrames: 0 };
    sockets.set(id, counts);
    // Inspect the protocol discriminator only, never decode/store payloads.
    socket.on("framereceived", ({ payload }) => { if (Buffer.from(payload)[0] === 0) counts.outputFrames++; });
    socket.on("framesent", ({ payload }) => { if (Buffer.from(payload)[0] === 1) counts.inputFrames++; });
  });
  const status = await get("/v1/control-plane/status");
  if (status.bootstrapRequired) pending("Create the real Luci owner account first, then rerun this stage. The harness does not read bootstrap or owner capabilities.");
  await page.goto(origin);
  await checkpoint("entry.sign-in");
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.locator(".shell, .onboarding-portal").first().waitFor();
  token = await page.evaluate(() => sessionStorage.getItem("luci.control-plane.session"));
  assert(token, "The real account must authenticate");
  const me = await get("/v1/auth/me");
  if (result.userId) assert.equal(me.user.id, result.userId, "Do not resume evidence in a replacement account");
  result.userId = me.user.id;
  const setup = await get("/v1/setup/status");
  await checkpoint("entry.setup-dialog");
  const portal = page.locator(".onboarding-portal");
  // An assigned folder can be configured before this account's first visit.
  // Complete the actual visible choice instead of inferring that visit from
  // runtime readiness. Each acceptance browser starts with fresh preferences.
  if (await portal.isVisible()) {
    await portal.getByRole("button", { name: "On this computer", exact: true }).click();
    await portal.getByRole("button", { name: "Continue to projects", exact: true }).click();
  } else if (!setup.configured) {
    await page.getByRole("button", { name: "On this computer", exact: true }).click();
    await page.getByRole("button", { name: "Continue to projects", exact: true }).click();
  }
  await page.locator(".setup-backdrop,.onboarding-portal").waitFor({ state: "hidden" });
  await page.locator(".shell").waitFor();
  if (result.project) await selectProject();
}
async function selectProject(requireBranch = ["develop", "verify", "deploy"].includes(phase)) {
  await checkpoint("project.catalogue");
  const rows = (await get("/v1/projects")).projects;
  assert(rows.some(row => row.id === result.project.id && row.name === result.projectName), "Owned benchmark project is not present in this account");
  const row = page.locator(`[role="treeitem"][data-kind="project"][data-id="${result.project.id}"]`);
  await checkpoint("project.row");
  await row.waitFor();
  await checkpoint("project.expand");
  if (await row.getAttribute("aria-selected") !== "true") await row.click();
  if (await row.getAttribute("aria-expanded") !== "true") await row.click();
  // Define and Design remain project-scoped. Develop must also expose Git's
  // actual initial branch and primary worktree before the first commit.
  if (!requireBranch) return;
  const branch = row.locator("..").locator('[role="treeitem"][data-kind="branch"]').first();
  await checkpoint("project.branch");
  await branch.click();
  // Branch selection is project-scoped. A coding session also needs an
  // explicitly selected worktree, just as the real Settings guidance says.
  // Its children load asynchronously after selection; a leaf-looking row
  // cannot expand until the catalogue has supplied those children.
  assert(await until(async () => (await branch.getAttribute("aria-expanded")) !== null), "The selected branch must load its registered worktree");
  if (await branch.getAttribute("aria-expanded") !== "true") await branch.click();
  await checkpoint("project.worktree");
  const worktree = branch.locator("..").locator('[role="treeitem"][data-kind="worktree"]').first();
  await worktree.click();
  return { branchId: await branch.getAttribute("data-id"), worktreeId: await worktree.getAttribute("data-id") };
}
async function lens(name) { await page.getByRole("button", { name, exact: true }).click(); }
async function planPanel() {
  await lens("Develop");
  const panel = page.locator(".project-work");
  if (await panel.getAttribute("open") === null) await panel.locator(":scope > summary").click();
  return panel;
}
async function settings() {
  await checkpoint("settings.open");
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "Agent connection", exact: true }).click();
  await page.getByRole("combobox", { name: "Coding agent", exact: true }).selectOption("openai");
  await checkpoint("settings.codex");
}
async function closeSettings() { await page.getByRole("button", { name: "Close settings", exact: true }).click(); }
async function dockerReady() {
  const machines = (await get("/v1/machines")).machines;
  const machine = machines.find(value => value.default) || machines[0];
  if (!machine || machine.backend !== "docker" || machine.state !== "running") pending("The selected Luci workspace needs a ready Docker backing. No model call or host repair was attempted.");
  result.execution = { backend: machine.backend, isolation: machine.isolation, machineId: machine.id, sessionMemoryMiB: machine.sessionMemoryMiB, sessionCPUMillis: machine.sessionCPUMillis };
  await save();
}
async function assertContext() {
  const [brief, design, plan] = await Promise.all([get(projectPath("/brief")), get(projectPath("/design")), get(projectPath("/plan"))]);
  assert.equal(brief.revision, result.proofs.describe.revision, "Brief changed; review and prepare new evidence");
  assert.equal(design.revision, result.proofs.design.revision, "Design changed after handoff");
  assert.equal(plan.revision, result.proofs.plan.revision, "Plan changed after handoff");
  assert.equal(design.briefRevision, brief.revision);
  assert.equal(plan.briefRevision, brief.revision);
  if (result.proofs.describe.specificationRevision) {
    const specification = await get(projectPath("/specification"));
    assert.equal(specification.revision, result.proofs.describe.specificationRevision);
    assert.equal(specification.status, "approved");
    assert.equal(specification.digest, result.proofs.describe.digest);
  }
  if (result.proofs.design.handoffRevision) {
    const handoff = await get(projectPath("/design-plan"));
    assert.equal(handoff.revision, result.proofs.design.handoffRevision);
    assert.equal(handoff.status, "approved");
    assert.equal(handoff.digest, result.proofs.design.digest);
  }
}
async function prepare() {
  if (!result.project) {
    assert(!(await get("/v1/projects")).projects.some(row => row.name === result.projectName), "Choose a new project name; never adopt unrelated work");
    await page.getByRole("button", { name: "Create project", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Project name", { exact: true }).fill(result.projectName);
    result.project = await observed(() => dialog.getByRole("button", { name: "Create project", exact: true }).click(), "/v1/product/projects");
    await save();
    await selectProject();
  }
  const criteria = [
    'A responsive, attractive “Luci Launch Checklist” page has an h1 with that exact name, usable at 390px and 1440px without horizontal overflow.',
    'A textbox labelled “New checklist item” and button “Add item” add a trimmed custom item; Enter submits; blank input adds nothing.',
    'Each item has a native checkbox whose accessible name is its item text; Space toggles completion. Buttons “All”, “Active”, “Completed” filter correctly.',
    'Visible counters use data-testid="total-count", "completed-count", "remaining-count" and contain only their integer values; changes update them correctly.',
    'Items and completion survive reload using localStorage. No remote service, sign-in, telemetry or CDN dependency is needed.',
    'Static production assets are committed under docs/ with relative URLs, docs/.nojekyll and docs/healthz/index.html containing {"status":"ok"}; they work below a GitHub Pages repository path.',
    'A committed Dockerfile serves docs/ on 0.0.0.0:3000 with GET /healthz or /healthz/ returning the health content. npm test runs meaningful node:test checks with no paid service.',
  ];
  if (!result.proofs.describe) {
    await checkpoint("describe.navigation");
    await lens("Define");
    await page.getByRole("tab", { name: "Specification", exact: true }).click();
    await checkpoint("describe.idea");
    await page.getByRole("textbox", { name: "The idea", exact: true }).fill("Build Luci Launch Checklist: a polished, small launch preparation checklist that shows an actual idea becoming a usable deployed website through Luci.");
    await checkpoint("describe.fields");
    await page.getByRole("textbox", { name: "Who it is for", exact: true }).fill("An individual preparing to launch a small website, using keyboard or touch.");
    await page.getByRole("textbox", { name: "Boundaries and constraints", exact: true }).fill(`Use dependency-light HTML/CSS/JavaScript in docs/ and Node's built-in tests. Follow the saved wireframe, warm neutral surfaces, teal accents, clear type and useful empty states. Docker port 3000 and GitHub Pages /docs must serve the same app, including beneath a repository URL prefix. No external services or assets, no credentials, no cloud purchases. Do not commit or publish; Luci will independently verify and commit. Read the saved plan/design. Only after implementation and your tests finish, write IMPLEMENTATION.md explaining actual changes, remaining limitations and this completion marker: LUCI_SUBSCRIPTION_${result.nonce}. Never claim verification you did not perform.`);
    for (let index = 0; index < criteria.length; index++) {
      await page.getByRole("button", { name: "Add requirement", exact: true }).click();
      await page.getByRole("textbox", { name: `Requirement ${index+1}`, exact: true }).fill(`Launch outcome ${index+1}`);
      await page.getByRole("textbox", { name: `Acceptance scenario ${index+1}`, exact: true }).fill(criteria[index]);
    }
    await checkpoint("describe.save");
    const draft = await observed(() => page.getByRole("button", { name: "Save definition", exact: true }).click(), projectPath("/specification"));
    assert(draft.requirements.length >= criteria.length && draft.revision > 0);
    const approved = await observed(() => page.getByRole("button", { name: "Approve definition", exact: true }).click(), projectPath("/specification/approve"));
    assert.equal(approved.status,"approved");
    await proof("describe", { revision: approved.briefRevision, specificationRevision: approved.revision, digest: approved.digest, criteriaCount: approved.requirements.length });
    await screenshot("01-define");
  }
  if (!result.proofs.design) {
    await checkpoint("design.navigation");
    await lens("Design");
    await page.getByRole("button", { name: "Start a screen plan from my definition", exact: true }).click();
    await observed(() => page.getByRole("button", { name: "Save design plan", exact: true }).click(), projectPath("/design-plan"));
    await page.getByRole("tab", { name: "Sketch", exact: true }).click();
    const canvas = page.locator(".excalidraw canvas").first();
    await canvas.waitFor();
    const box = await canvas.boundingBox(); assert(box && box.width > 850 && box.height > 450);
    await checkpoint("design.canvas");
    // Keep the sketch to the right of Excalidraw's visible left style palette.
    const x = box.x + 280, y = box.y + 90;
    const paint = () => page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
    async function rectangle(dx, dy, width, height) {
      await page.keyboard.press("Escape"); await page.keyboard.press("2");
      await paint();
      await page.mouse.move(x + dx, y + dy); await page.mouse.down();
      await page.mouse.move(x + dx + width, y + dy + height, { steps: 8 }); await page.mouse.up();
      await paint();
    }
    async function text(dx, dy, value) {
      await page.keyboard.press("Escape"); await page.keyboard.press("t");
      await paint();
      await page.mouse.click(x + dx, y + dy);
      const editor = page.locator(".excalidraw-wysiwyg");
      await editor.waitFor({ state: "visible" }); await editor.fill(value);
      await page.keyboard.press("Escape"); await editor.waitFor({ state: "hidden" }); await paint();
    }
    // Excalidraw stacks a static canvas below its interactive canvas. Click the
    // observed surface coordinates, as the established browser UAT does, rather
    // than asking Playwright to click through the overlaid static DOM node.
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    // Resume an unsuccessful sketch in this dedicated benchmark project. The
    // failed saved revision remains in its attempt evidence; there is no design
    // proof yet, and no unrelated project is ever adopted by prepare.
    await page.keyboard.press("ControlOrMeta+a"); await page.keyboard.press("Delete"); await paint();
    await rectangle(0, 0, 500, 430);
    await text(20, 24, "Luci Launch Checklist");
    await text(20, 72, "Total   |   Completed   |   Remaining");
    await rectangle(20, 110, 355, 45); await text(30, 121, "New checklist item");
    await rectangle(385, 110, 95, 45); await text(400, 121, "Add item");
    await text(20, 178, "All     Active     Completed");
    await rectangle(20, 220, 460, 150); await text(35, 240, "[ ] Review launch details\n[ ] Check mobile experience\n[x] Save progress locally");
    await page.keyboard.press("Escape");
    await checkpoint("design.save");
    const design = await observed(() => page.getByRole("button", { name: "Save design", exact: true }).click(), projectPath("/design"));
    const elements = design.scene?.elements || [];
    assert(elements.some(item => item.type === "rectangle") && elements.some(item => item.type === "text" && item.text.includes("Luci Launch Checklist")), "Actual persisted canvas must contain the wireframe and heading");
    await page.getByRole("tab", { name: "Screens and states", exact: true }).click();
    await observed(() => page.getByRole("button", { name: "Save design plan", exact: true }).click(), projectPath("/design-plan"));
    const handoff = await observed(() => page.getByRole("button", { name: "Approve design handoff", exact: true }).click(), projectPath("/design-plan/approve"));
    assert.equal(handoff.status,"approved");
    await proof("design", { revision: design.revision, briefRevision: design.briefRevision, handoffRevision: handoff.revision, digest: handoff.digest, screenCount: handoff.screens.length, elementCount: elements.length, sceneSha256: hash(JSON.stringify(design.scene)) });
    await screenshot("02-design");
  }
  if (!result.proofs.plan) {
    await checkpoint("plan.navigation");
    const panel = await planPanel();
    await panel.getByRole("button", { name: "Add acceptance criteria as tasks", exact: true }).click();
    await checkpoint("plan.save");
    const plan = await observed(() => panel.getByRole("button", { name: "Save implementation plan", exact: true }).click(), projectPath("/plan"));
    assert(plan.tasks.length >= criteria.length);
    await proof("plan", { revision: plan.revision, briefRevision: plan.briefRevision, taskCount: plan.tasks.length });
    await screenshot("03-plan");
  }
  await assertContext();
  await checkpoint("prepare.first-agent-entry");
  const selected = await selectProject(true);
  const panel = await planPanel();
  assert(await panel.getByRole("button", { name: "Start implementation session", exact: true }).isEnabled(), "Saved Define and Design must reach an enabled first-agent action without a planning commit");
  const git = await get(projectPath("/git"));
  const catalogue = await get(`/v1/projects/${encodeURIComponent(result.project.id)}/worktrees`);
  assert(catalogue.worktrees.some(worktree => worktree.id === selected.worktreeId && worktree.branchId === selected.branchId && worktree.isPrimary), "The first-agent action must use the actual owned primary worktree");
  result.firstAgentEntry = { initialBranchSelected: true, primaryWorktreeSelected: true, actionEnabled: true, beforeFirstCommit: git.log.length === 0 };
  await save();
  pending("Describe, Design and the implementation plan are saved. Run auth when the Docker runtime and your official Codex sign-in are ready.");
}
async function auth() {
  requireProof("describe", "design", "plan"); await dockerReady();
  let account = await get("/v1/setup/agents/codex/account");
  if (account.configured) {
    assert.equal(account.authMode, "chatgpt");
    await proof("subscription", { authMode: account.authMode, sourceSessionId: account.sourceSessionId, inferenceVerified: false });
    pending("Codex subscription is connected. Run develop to exercise the actual saved-plan handoff and inference.");
  }
  if (result.authSessionId) {
    const session = await get(`/v1/sessions/${encodeURIComponent(result.authSessionId)}`);
    assert.equal(session.harness, "codex"); assert.equal(session.authOnly, true);
    if (session.status !== "exited") pending("Complete the official Codex device sign-in in Luci’s account sign-in terminal, then rerun auth. No sign-in contents were captured.");
    await settings();
    await page.getByRole("combobox", { name: "Completed sign-in session", exact: true }).selectOption(session.id);
    account = await observed(() => page.getByRole("button", { name: "Save Codex sign-in", exact: true }).click(), "/v1/setup/agents/codex/account");
    assert.equal(account.configured, true); assert.equal(account.authMode, "chatgpt");
    await proof("subscription", { authMode: account.authMode, sourceSessionId: session.id, inferenceVerified: false });
    await closeSettings();
    pending("Official sign-in saved through Luci. Run develop; a saved credential alone does not prove inference.");
  }
  await settings();
  const session = await observed(() => page.getByRole("button", { name: "Open Codex account sign-in", exact: true }).click(), "/v1/setup/agents/codex/sign-in", 180000);
  assert.equal(session.harness, "codex"); assert.equal(session.authOnly, true);
  result.authSessionId = session.id; await save();
  if (waitMs) await until(async () => (await get(`/v1/sessions/${encodeURIComponent(session.id)}`)).status === "exited", waitMs);
  const current = await get(`/v1/sessions/${encodeURIComponent(session.id)}`);
  if (current.status === "exited") pending("The account sign-in session has exited. Its saved account still needs verification through Luci before subscription inference can run. No device code or terminal contents were recorded.");
  pending("An official device sign-in session is running in Luci. Complete it yourself, then rerun auth to save it. No device code or terminal contents were recorded.");
}
async function develop() {
  requireProof("describe", "design", "plan", "subscription"); await dockerReady(); await assertContext();
  const account = await get("/v1/setup/agents/codex/account");
  assert(account.configured && account.authMode === "chatgpt", "Retained subscription must still be connected");
  if (result.agentSessionId) pending("Implementation session already exists. Inspect it in Luci, then run verify when the agent has finished. No duplicate model session was started.");
  if (account.activeSessionId) pending("Another Codex subscription session is active. Finish it before starting this benchmark.");
  const panel = await planPanel();
  await panel.getByRole("button", { name: "Start implementation session", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New session", exact: true });
  await dialog.getByRole("combobox", { name: "Harness", exact: true }).selectOption("codex");
  await dialog.getByRole("combobox", { name: "Permission mode", exact: true }).selectOption(permission);
  const handoff = await dialog.getByRole("textbox", { name: "Opening prompt", exact: true }).inputValue();
  assert(handoff.includes("LUCI.md") && handoff.includes(".luci/design.json") && handoff.includes(".luci/plan.json"));
  result.handoff = { sha256: hash(handoff), permissionMode: permission, briefRevision: result.proofs.describe.revision, planRevision: result.proofs.plan.revision };
  await checkpoint("develop.create-session");
  const session = await observed(() => dialog.getByRole("button", { name: "Create session", exact: true }).click(), "/v1/sessions", 180000);
  assert.equal(session.harness, "codex"); assert.equal(session.authMode, "chatgpt"); assert.notEqual(session.authOnly, true);
  assert.equal(session.backend, "docker"); assert.equal(session.isolation, "container");
  assert.notEqual(session.id, account.sourceSessionId, "Implementation must use a fresh session with the retained account");
  assert.equal(session.permissionMode, permission);
  result.agentSessionId = session.id; result.agentWorktreeId = session.worktreeId; await save();
  await page.locator(`[data-session-id="${session.id}"] .xterm-helper-textarea`).waitFor({ state: "attached" });
  await until(async () => (sockets.get(session.id)?.outputFrames || 0) > 0);
  result.agentAttach = { sessionId: session.id, ...(sockets.get(session.id) || {}), payloadsRetained: false };
  await save();
  if (waitMs) await until(agentMarker, waitMs);
  pending("A genuine subscription-backed Codex session received the saved-plan handoff. Resolve any visible approval in Luci; run verify after its implementation completes.");
}
async function agentMarker() {
  const file = await get(projectPath("/files?path=IMPLEMENTATION.md"), true);
  if (!file || !file.content?.includes(`LUCI_SUBSCRIPTION_${result.nonce}`)) return null;
  return { sha256: hash(file.content), bytes: Buffer.byteLength(file.content) };
}
async function stopAgent() {
  const session = await get(`/v1/sessions/${encodeURIComponent(result.agentSessionId)}`);
  assert.equal(session.harness, "codex"); assert.equal(session.authMode, "chatgpt");
  assert.equal(session.backend, "docker"); assert.equal(session.isolation, "container");
  assert.equal(session.worktreeId, result.agentWorktreeId);
  if (session.status !== "exited") {
    await lens("Develop");
    const pane = page.locator(`[data-session-id="${session.id}"]`);
    await pane.getByRole("button", { name: /^Session menu for / }).click();
    await page.getByRole("menuitem", { name: "Kill", exact: true }).click();
    assert(await until(async () => (await get(`/v1/sessions/${encodeURIComponent(session.id)}`)).status === "exited"), "Owned agent must stop before immutable verification");
  }
  return session;
}
async function commit(message = "Build Luci Launch Checklist through Codex subscription") {
  let git = await get(projectPath("/git"));
  if (!git.clean) {
    await lens("Develop"); await page.getByRole("button", { name: "Git", exact: true }).click();
    await page.getByRole("textbox", { name: "Commit message", exact: true }).fill(message);
    await observed(() => page.getByRole("button", { name: "Commit changes", exact: true }).click(), projectPath("/git/commit"));
    git = await get(projectPath("/git"));
  }
  assert(git.clean && git.log[0]?.hash, "A clean, committed application is required");
  result.commit = git.log[0].hash; result.branch = git.branch; await save();
  return git;
}
async function checks() {
  const [loaded] = await Promise.all([
    page.waitForResponse(response => new URL(response.url()).pathname === projectPath("/checks") && response.request().method() === "GET"),
    planPanel(),
  ]);
  assert(loaded.ok());
  const initial = await loaded.json(), form = page.locator(".project-checks");
  await form.getByRole("heading", { name: "Verify the implementation", exact: true }).waitFor();
  // Wait for the API's initial command list before replacing suggestions.
  assert(await until(async () => (await form.locator(".check-commands button").count()) === (initial.suggested || []).length), "Initial check suggestions must finish loading");
  while (await form.locator(".check-commands button").count()) await form.locator(".check-commands button").first().click();
  const execution = form.getByRole("combobox", { name: "Check execution", exact: true });
  if (await execution.count()) await execution.selectOption("docker");
  await form.getByLabel("Executable", { exact: true }).fill("npm");
  await form.getByLabel("Arguments · JSON array", { exact: true }).fill('["test"]');
  await form.getByRole("button", { name: "Add check command", exact: true }).click();
  await form.getByRole("checkbox", { name: "I trust this project's code and these Docker commands to write project files and use the network.", exact: true }).check();
  const run = await observed(() => form.getByRole("button", { name: "Run project checks", exact: true }).click(), projectPath("/checks"));
  result.checkRunId = run.id; await save();
  const done = await until(async () => {
    const value = await get(projectPath(`/checks/${encodeURIComponent(run.id)}`));
    return ["queued", "running"].includes(value.status) ? null : value;
  }, 660000);
  assert(done, "Docker checks did not finish within the bounded check budget");
  assert.equal(done.status, "passed"); assert.equal(done.execution, "docker");
  assert.equal(done.commit, result.commit); assert.equal(done.dirty, false); assert.notEqual(done.sourceChanged, true);
  assert(done.commands.length > 0 && done.commands.every(command => command.exitCode === 0 && command.status === "passed"));
  // Only counts/provenance are copied; arbitrary test output is left inside Luci.
  await proof("dockerChecks", { id: done.id, execution: done.execution, commit: done.commit, sourceDigest: done.sourceDigest, commands: done.commands.map(({ executable, args, status, exitCode }) => ({ executable, args, status, exitCode })) });
  await screenshot("04-docker-checks");
}
async function exerciseSite(url, label) {
  const target = new URL(url.endsWith("/") ? url : url + "/");
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const site = await context.newPage(), errors = [];
  site.on("pageerror", () => errors.push("application-pageerror"));
  site.on("request", request => {
    const address = new URL(request.url());
    if (["http:", "https:"].includes(address.protocol) && address.origin !== target.origin) errors.push("unexpected-external-resource");
  });
  site.on("response", response => {
    if (["script", "stylesheet", "font", "image"].includes(response.request().resourceType()) && response.status() >= 400) errors.push("failed-static-asset");
  });
  try {
    const response = await site.goto(target.href); assert(response?.ok(), "Deployed page must return success");
    await site.getByRole("heading", { name: "Luci Launch Checklist", exact: true, level: 1 }).waitFor();
    const health = await context.request.get(new URL("healthz/", target).href);
    assert(health.ok(), "The actual site health endpoint must succeed");
    assert.equal(JSON.parse(await health.text()).status, "ok", "Health payload must identify readiness");
    await site.getByRole("button", { name: "All", exact: true }).click();
    const count = async name => {
      const value = (await site.getByTestId(name + "-count").innerText()).trim();
      assert.match(value, /^\d+$/); return Number(value);
    };
    const total = await count("total"), completed = await count("completed");
    assert.equal(await count("remaining"), total - completed);
    const text = `Evidence ${result.nonce.slice(0, 8)} ${label}`;
    const input = site.getByRole("textbox", { name: "New checklist item", exact: true });
    await input.fill("   "); await input.press("Enter"); assert.equal(await count("total"), total, "Blank item must not be added");
    await input.fill(text); await input.press("Enter");
    const checkbox = site.getByRole("checkbox", { name: text, exact: true });
    await checkbox.waitFor(); assert.equal(await count("total"), total + 1);
    await checkbox.focus(); await site.keyboard.press("Space"); assert(await checkbox.isChecked());
    assert.equal(await count("completed"), completed + 1); assert.equal(await count("remaining"), total - completed);
    await site.getByRole("button", { name: "Active", exact: true }).click(); assert.equal(await checkbox.count(), 0);
    await site.getByRole("button", { name: "Completed", exact: true }).click(); assert(await checkbox.isChecked());
    await site.reload(); await site.getByRole("button", { name: "All", exact: true }).click();
    assert(await checkbox.isChecked(), "Completion must survive actual reload");
    assert.equal(await count("total"), total + 1);
    assert.equal(await site.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await site.screenshot({ path: path.join(evidence, label + "-desktop.png"), fullPage: true });
    await site.setViewportSize({ width: 390, height: 844 });
    assert.equal(await site.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, "Mobile page must not overflow");
    await input.fill(text + " mobile"); await site.getByRole("button", { name: "Add item", exact: true }).click();
    await site.getByRole("checkbox", { name: text + " mobile", exact: true }).waitFor();
    assert.equal(await count("total"), total + 2);
    await site.screenshot({ path: path.join(evidence, label + "-mobile.png"), fullPage: true });
    assert.deepEqual(errors, []);
    return { url: target.href, healthUrl: new URL("healthz/", target).href, checks: ["actual HTTP and health", "heading and labelled inputs", "blank input", "Enter adds", "Space toggles", "counters", "active/completed filters", "reload persistence", "desktop/mobile overflow", "touch-size viewport add", "no page errors"] };
  } finally { await context.close(); }
}
async function verify() {
  requireProof("describe", "design", "plan", "subscription"); await dockerReady(); await assertContext();
  if (!result.agentSessionId) pending("Start the actual Codex implementation through the develop stage first.");
  const marker = await until(agentMarker, waitMs);
  if (!marker) pending("Codex has not written the requested completion marker. Inspect its actual terminal for progress or an approval; no source was generated by this harness.");
  const session = await stopAgent();
  const counts = sockets.get(session.id);
  const outputFrames = (result.agentAttach?.outputFrames || 0) + (counts?.outputFrames || 0);
  assert(outputFrames > 0, "An actual attached implementation terminal must have delivered output");
  const requiredFiles = ["Dockerfile", "package.json", "docs/index.html", "docs/.nojekyll", "docs/healthz/index.html", "IMPLEMENTATION.md"];
  const files = [];
  for (const filename of requiredFiles) {
    const file = await get(projectPath("/files?path=" + encodeURIComponent(filename)));
    assert(typeof file.content === "string" && !file.truncated, `${filename} must be a readable, complete generated file`);
    files.push({ path: filename, bytes: Buffer.byteLength(file.content), sha256: hash(file.content) });
  }
  await proof("agent", { sessionId: session.id, worktreeId: session.worktreeId, harness: "codex", authMode: session.authMode, backend: session.backend, isolation: session.isolation, model: session.model, permissionMode: session.permissionMode, outputFrames, completionMarker: marker, files, handoffSha256: result.handoff.sha256, sourceWrittenByHarness: false });
  await commit();
  if (result.proofs.dockerChecks?.commit !== result.commit) await checks();
  let git = await get(projectPath("/git")); assert(git.clean && git.log[0].hash === result.commit, "Checks must leave verified source unchanged");
    await lens("Deploy");
    await page.getByRole("tab", { name: "Previews and publishing", exact: true }).click();
  let release = result.localReleaseId && (await get(projectPath("/releases"))).releases.find(row => row.id === result.localReleaseId && row.status === "ready" && row.commit === result.commit);
  if (!release) {
    await page.locator("summary").filter({ hasText: /^Preview build options$/ }).click();
    await page.getByLabel("Dockerfile", { exact: true }).fill("Dockerfile");
    await page.getByRole("spinbutton", { name: /^App port/ }).fill("3000");
    release = await observed(() => page.getByRole("button", { name: "Build local preview", exact: true }).click(), projectPath("/releases"), 240000);
    result.localReleaseId = release.id; await save();
  }
  assert.equal(release.status, "ready"); assert.equal(release.commit, result.commit); assert.equal(release.briefRevision, result.proofs.describe.revision);
  const row = page.locator(".deploy-releases > li").filter({ hasText: release.commit.slice(0, 8) }).filter({ has: page.getByRole("button", { name: "Open preview", exact: true }) }).first();
  await row.getByRole("button", { name: "Open preview", exact: true }).click();
  const iframe = page.getByTitle("Project preview", { exact: true }); await iframe.waitFor();
  const frame = await (await iframe.elementHandle()).contentFrame();
  await frame.getByRole("heading", { name: "Luci Launch Checklist", exact: true }).waitFor();
  assert.equal(await frame.evaluate(() => {
    const key = "luci-embedded-storage-check";
    localStorage.setItem(key, "ready"); const value = localStorage.getItem(key); localStorage.removeItem(key);
    return value;
  }), "ready", "The embedded separate-origin preview must allow actual localStorage");
  const embeddedItem = `Embedded ${result.nonce.slice(0, 8)}`;
  const embeddedInput = frame.getByRole("textbox", { name: "New checklist item", exact: true });
  await embeddedInput.fill(embeddedItem); await embeddedInput.press("Enter");
  const embeddedCheckbox = frame.getByRole("checkbox", { name: embeddedItem, exact: true });
  await embeddedCheckbox.focus(); await page.keyboard.press("Space");
  assert(await embeddedCheckbox.isChecked());
  await frame.evaluate(() => location.reload());
  await frame.getByRole("button", { name: "All", exact: true }).click();
  assert(await frame.getByRole("checkbox", { name: embeddedItem, exact: true }).isChecked(), "App state must survive a real embedded-frame reload");
  await frame.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
  await screenshot("05-luci-preview");
  const previewUrl = await iframe.getAttribute("src"); assert(previewUrl);
  const outcome = await exerciseSite(new URL(previewUrl, origin).href, "06-docker-site");
  await proof("localSite", { ...outcome, embeddedStorageAndReload: true, releaseId: release.id, commit: release.commit });
  pending("Real subscription implementation, immutable Docker checks and local site behavior passed. Public deployment through Luci remains required; run deploy with the intended repository.");
}
async function deploy() {
  requireProof("describe", "design", "plan", "subscription", "agent", "dockerChecks", "localSite"); await assertContext();
  const git = await get(projectPath("/git")); assert(git.clean && git.log[0]?.hash === result.commit, "Only the independently verified clean commit may deploy");
  const repository = process.env.LUCI_EVAL_REPOSITORY || result.repository;
  if (!repository) pending("Supply LUCI_EVAL_REPOSITORY for an existing public GitHub repository you intend to publish through Luci. No repository or cloud resource was silently created.");
  const remote = new URL(repository);
  assert(remote.protocol === "https:" && remote.hostname === "github.com" && !remote.username && !remote.password && /^\/[^/]+\/[^/]+$/.test(remote.pathname) && !remote.search && !remote.hash);
  if (result.repository) assert.equal(repository, result.repository, "Do not redirect an existing deployment receipt to another repository");
  result.repository = repository; await save();
  await lens("Develop"); await page.getByRole("button", { name: "Git", exact: true }).click();
  let delivery = await get(projectPath("/git/delivery"));
  if (delivery.origin) assert.equal(delivery.origin, repository, "Existing origin differs; review it in Luci instead of overwriting it");
  else {
    await page.getByLabel("Repository origin · HTTPS", { exact: true }).fill(repository);
    await observed(() => page.getByRole("button", { name: "Connect existing repository", exact: true }).click(), projectPath("/git/remote"));
  }
  if (!delivery.publication?.verified || delivery.publication.commit !== result.commit) {
    const publication = await observed(() => page.getByRole("button", { name: "Push reviewed commit", exact: true }).click(), projectPath("/git/push"), 180000);
    assert(publication.verified && publication.commit === result.commit);
    delivery = await get(projectPath("/git/delivery"));
  }
  assert(delivery.publication?.verified && delivery.publication.commit === result.commit);
  await proof("publication", { repository, branch: git.branch, commit: result.commit, remoteVerified: true });
  await lens("Deploy");
    await page.getByRole("tab", { name: "Previews and publishing", exact: true }).click();
  // The adapter deliberately derives repository/branch from the verified origin;
  // it cannot publish an unrelated project or treat a localhost preview as public.
  const pages = page.getByRole("tab", { name: "GitHub Pages", exact: true });
  if (!(await pages.count())) pending("This Luci build does not yet contain the GitHub Pages deployment adapter. Publication is not a completed deployment.");
  await pages.click();
  let release = result.publicReleaseId && (await get(projectPath("/releases"))).releases.find(row => row.id === result.publicReleaseId);
  if (!release) {
    await page.getByRole("checkbox", { name: "I understand that this site and its repository are public.", exact: true }).check();
    release = await observed(() => page.getByRole("button", { name: "Deploy to GitHub Pages", exact: true }).click(), projectPath("/releases/pages"), 180000);
    result.publicReleaseId = release.id; await save();
  }
  assert.equal(release.target, "github-pages"); assert.equal(release.commit, result.commit);
  const done = await until(async () => {
    const value = (await get(projectPath("/releases"))).releases.find(row => row.id === release.id);
    assert(value, "Recorded public release must remain visible");
    return ["preparing", "queued", "building", "deploying", "verifying"].includes(value.status) ? null : value;
  }, waitMs || 600000);
  if (!done) pending("GitHub Pages is still building/verifying this exact release. Rerun deploy to continue without creating a duplicate.");
  assert.equal(done.status, "ready", "Public release failed; inspect its actual Luci release message");
  assert.equal(done.commit, result.commit); assert.equal(done.briefRevision, result.proofs.describe.revision);
  const url = new URL(done.url); assert.equal(url.protocol, "https:");
  assert(url.hostname.endsWith(".github.io"), "Expected the actual GitHub Pages public HTTPS origin");
  const outcome = await exerciseSite(url.href, "08-public-site");
  await proof("publicSite", { ...outcome, releaseId: done.id, target: done.target, commit: done.commit });
  await screenshot("07-luci-public-release");
  assert(complete(result), "Every actual journey proof must be present and match the deployed commit");
}

try {
  await save();
  await enter();
  if (phase === "prepare") await prepare();
  if (phase === "auth") await auth();
  if (phase === "develop") await develop();
  if (phase === "verify") await verify();
  if (phase === "deploy") await deploy();
  if (phase === "status") {
    if (result.project) {
      const git = await get(projectPath("/git"));
      result.currentSource = { clean: git.clean, commit: git.log[0]?.hash || "", branch: git.branch };
      if (complete(result)) assert(git.clean && git.log[0]?.hash === result.commit, "Previously verified source has changed");
    }
    if (!complete(result)) pending("Journey incomplete; inspect the recorded proofs and resume the next required stage.");
  }
  result.passed = complete(result); result.status = result.passed ? "passed" : "pending";
  attempt.status = result.status; result.pending = undefined;
  process.exitCode = result.passed ? 0 : 2;
} catch (error) {
  result.passed = false;
  result.status = error instanceof Pending ? "pending" : "failed";
  attempt.status = result.status;
  attempt.errorClass = error?.constructor?.name || "UnknownError";
  if (phase === "prepare" && !(error instanceof Pending) && page && await page.locator(".shell").isVisible().catch(() => false)) {
    // Constant checkpoint names and non-auth app geometry help diagnose a
    // genuine boundary failure without leaking Playwright call logs or bodies.
    const name = `prepare-failure-${result.attempts.length}`;
    try { await screenshot(name); attempt.screenshot = name + ".png"; } catch { /* Keep the original failure. */ }
    attempt.navigation = await page.locator('[role="treeitem"]').evaluateAll(elements => elements.slice(0, 30).map(element => ({ kind: element.getAttribute("data-kind"), id: element.getAttribute("data-id"), expanded: element.getAttribute("aria-expanded"), hidden: element.getClientRects().length === 0 }))).catch(() => []);
  }
  // Playwright errors can embed typed input and page call logs. Keep only our
  // bounded assertion messages; never emit a full stack, locator log or payload.
  const message = error instanceof Pending || error instanceof assert.AssertionError
    ? String(error.message).split("\n")[0].slice(0, 500)
    : "The real browser/request did not complete. Inspect Luci and this attempt's stage; sensitive call logs were not retained.";
  attempt.message = message; result.pending = message;
  process.exitCode = error instanceof Pending ? 2 : 1;
} finally {
  attempt.completedAt = new Date().toISOString();
  if (result.agentSessionId && sockets.has(result.agentSessionId)) {
    const counts = sockets.get(result.agentSessionId);
    result.agentAttach ||= { sessionId: result.agentSessionId, outputFrames: 0, inputFrames: 0, payloadsRetained: false };
    result.agentAttach.outputFrames = Math.max(result.agentAttach.outputFrames || 0, counts.outputFrames);
    result.agentAttach.inputFrames = Math.max(result.agentAttach.inputFrames || 0, counts.inputFrames);
  }
  await browser?.close();
  await save();
  console.log(JSON.stringify({ status: result.status, phase, receipt: receiptPath, projectId: result.project?.id, sessionId: result.agentSessionId || result.authSessionId, publicUrl: result.proofs.publicSite?.url, message: attempt.message || "All mandatory real outcomes passed." }));
}
