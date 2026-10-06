// Connected UAT journey against shipped assets and an isolated real control service.
// Uses only disposable local Luci identities. No provider sign-in, model usage, runtime installation, or cloud mutations.
import { chromium } from "playwright";
import axe from "axe-core";
import { mkdir, writeFile, readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import path from "node:path";
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";

const base = process.env.LUCI_EVAL_URL || "http://127.0.0.1:8730";
const verifyCsp = process.env.LUCI_EVAL_CSP === "1";
const evidence = path.resolve(process.env.LUCI_UAT_EVIDENCE);
const fixtureRoot = path.resolve(process.env.LUCI_UAT_FIXTURE);
assert.match(fixtureRoot.replaceAll("\\", "/"), /\/artifacts\/uat\//);
await mkdir(evidence, { recursive: true });
assert.ok(
  ["127.0.0.1", "localhost"].includes(new URL(base).hostname),
  "Use a disposable loopback service only",
);
const control = await fetch(`${base}/v1/control-plane/status`).then((r) =>
  r.json(),
);
const owner = { username: process.env.LUCI_UAT_USERNAME, password: process.env.LUCI_UAT_PASSWORD };
const member = {
  username: `eval-member-${Date.now().toString(36)}`,
  password: "Luci-Member-Eval!2026-10-02",
};
let initial;
let token;
let createdProject, restoredProject, approvedDefinition, approvedHandoff, projectRun;
const authed = async (route, options = {}) => {
  const response = await fetch(`${base}${route}`, {
    ...options,
    headers: { ...options.headers, Authorization: `Bearer ${token}` },
  });
  assert.ok(response.ok, `${route}: ${response.status}`);
  return response.json();
};
const name = `luci-ui-eval-${Date.now()}`;
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({
  viewport: { width: 1440, height: 1000 },
  deviceScaleFactor: 1,
});
if (verifyCsp) {
  const config = JSON.parse(
    await readFile("src-tauri/tauri.conf.json", "utf8"),
  );
  await page.route(base, async (route) => {
    const response = await route.fetch();
    await route.fulfill({
      response,
      headers: {
        ...response.headers(),
        "content-security-policy": config.app.security.csp,
      },
    });
  });
}
const errors = [],
  checks = [],
  accessibility = [],
  localFonts = [];
const screenshots = [];
page.on("pageerror", (e) => errors.push(e.message));
if (verifyCsp)
  page.on("console", (message) => {
    if (/Content Security Policy|Refused to|violates/i.test(message.text()))
      errors.push(message.text());
  });
page.on("response", (response) => {
  if (
    response.ok() &&
    response.url().startsWith(`${base}/design-assets/fonts/`)
  )
    localFonts.push(response.url());
});
const screenshot = async (label) => {
  await page.screenshot({
    path: path.join(evidence, `${label}.png`),
    fullPage: true,
    mask: [page.locator('input[type="password"]'), page.locator('.xterm')],
  });
  screenshots.push(`${label}.png`);
};
async function observedPost(action, suffix) {
  // Observe the real response before a successful UI action reloads the page.
  // The unchanged request reaches the actual service; the identical response
  // is delivered to the app. No fixture response or application state is made.
  let resolveBody, rejectBody;
  const bodyPromise = new Promise((resolve, reject) => { resolveBody = resolve; rejectBody = reject; });
  const timeout = setTimeout(() => rejectBody(new Error(`No real POST response arrived for ${suffix}`)), 45000);
  const matcher = url => url.origin === new URL(base).origin && url.pathname.endsWith(suffix);
  const observer = async route => {
    if (route.request().method() !== "POST") { await route.continue(); return; }
    try {
      const response = await route.fetch({ maxRedirects: 0 });
      const bytes = await response.body();
      await route.fulfill({ response, body: bytes });
      assert.ok(response.ok(), `${suffix} returned ${response.status()}`);
      resolveBody(JSON.parse(bytes.toString("utf8")));
    } catch (error) {
      rejectBody(error);
      await route.abort().catch(() => {});
    }
  };
  await page.route(matcher, observer);
  try {
    const [body] = await Promise.all([bodyPromise, action()]);
    return body;
  } finally {
    clearTimeout(timeout);
    await page.unroute(matcher, observer);
  }
}
async function unavailableRuntimeThroughUi(action) {
  // run-ui-uat.mjs selects the real, deliberately unavailable loopback engine.
  // Observe the UI's real check, then independently read its persisted status.
  // An empty successful CLI response must never permit an automatic build.
  const [response] = await Promise.all([
    page.waitForResponse(value => value.url() === `${base}/v1/setup/check`
      && value.request().method() === "POST", { timeout: 45000 }),
    action(),
  ]);
  assert.equal(response.status(), 200, "The UI's actual readiness check failed");
  const posted = await response.json();
  const preparation = page.locator(".runtime-preparation");
  await preparation.getByRole("button", { name: "Check again", exact: true }).waitFor();
  await page.waitForFunction(() => {
    const control = [...document.querySelectorAll(".runtime-preparation button")]
      .find(node => node.textContent.trim() === "Check again");
    return control && !control.disabled;
  }, undefined, { timeout: 45000 });
  const saved = await authed("/v1/setup/status");
  for (const status of [posted, saved]) {
    for (const id of ["docker", "sandbox-image"])
      assert.equal(status.checks.find(check => check.id === id)?.status, "missing", `${id} must report the actual unavailable engine`);
  }
  const runtime = await authed("/v1/setup/runtime/install");
  assert.equal(runtime.status, "idle", "An unavailable engine must not start automatic runtime preparation");
  assert.ok(!runtime.id, "No installation attempt may have been allocated");
  const docker = posted.checks.find(check => check.id === "docker");
  assert.ok(docker.action, "The unavailable engine must have actionable guidance");
  await preparation.getByText(docker.action, { exact: true }).waitFor();
  for (const label of ["Prepare agent runtime", "Retry runtime preparation"])
    assert.equal(await preparation.getByRole("button", { name: label, exact: true }).count(), 0);
}
try {
  await page.goto(base);
  if (control.bootstrapRequired) {
    const bootstrapPath = path.resolve(control.bootstrapTokenPath || "");
    assert.match(
      bootstrapPath.replaceAll("\\", "/"),
      /\/artifacts\/uat\/[^/]+\/state\/control-plane\/bootstrap-token$/,
    );
    const bootstrap = (await readFile(bootstrapPath, "utf8")).trim();
    await page.getByLabel(/^Bootstrap token/).fill(bootstrap);
    await page
      .getByLabel("Display name", { exact: true })
      .fill("Evaluation Owner");
  }
  await page.getByLabel("Username", { exact: true }).fill(owner.username);
  await page.getByLabel("Password", { exact: true }).fill(owner.password);
  if (control.bootstrapRequired && await page.getByLabel("Confirm password", { exact: true }).isVisible())
    await page.getByLabel("Confirm password", { exact: true }).fill(owner.password);
  await page
    .getByRole("button", {
      name: control.bootstrapRequired
        ? (await page.getByRole("button", { name: "Create account", exact: true }).count() ? "Create account" : "Create Super Admin account")
        : "Sign in",
      exact: true,
    })
    .click();
  await page.getByRole("heading", { name: "Choose your workspace", exact: true }).waitFor();
  token = await page.evaluate(() =>
    sessionStorage.getItem("luci.control-plane.session"),
  );
  assert.ok(token);
  initial = await authed("/v1/setup/status");
  assert.match(
    initial.config.projectsDir.replaceAll("\\", "/"),
    /\/artifacts\/uat\/[^/]+\/projects\/?$/,
  );
  checks.push(
    "Authenticated through the real account UI before opening its account-scoped workspace",
  );
  assert.equal(initial.configured, false, "The fresh owner should enter the onboarding portal");
  assert.equal(await page.getByRole("dialog").count(), 0, "Onboarding is a dedicated page, not Settings");
  for (const choice of ["On this computer", "AWS", "Azure", "Connect to an existing workspace"])
    await page.getByRole("button", { name: choice, exact: true }).waitFor();
  assert.equal(await page.getByLabel("SSH public key", { exact: true }).count(), 0);
  await page.getByRole("button", { name: "Connect to an existing workspace", exact: true }).click();
  await page.getByRole("heading", { name: "Connect to your workspace server", exact: true }).waitFor();
  assert.equal(await page.getByRole("button", { name: "Create cloud resources", exact: true }).count(), 0);
  assert.equal(await page.getByRole("button", { name: "Install Luci workspace", exact: true }).count(), 0);
  await screenshot("00a-existing-workspace-choice");
  await page.getByRole("button", { name: "Choose another setup option", exact: true }).click();
  await page.getByRole("button", { name: "AWS", exact: true }).click();
  await page.getByRole("combobox", { name: "Provider", exact: true }).selectOption("azure");
  await page.getByRole("option", { name: "Azure · Virtual machine", exact: true }).waitFor({ state: "attached" });
  await screenshot("00b-cloud-workspace-choice");
  await page.getByRole("button", { name: "Choose another setup option", exact: true }).click();
  checks.push("First-run local, guided own-account cloud and existing-workspace choices disclose only their relevant controls and allow backtracking without cloud mutations");
  let configured;
  await unavailableRuntimeThroughUi(async () => {
    configured = await observedPost(() => page.getByRole("button", { name: "On this computer", exact: true }).click(), "/v1/setup/config");
  });
  assert.equal(configured.configured, true);
  assert.equal(configured.config.projectsDir, initial.config.projectsDir);
  await page.getByLabel("Commit author name", { exact: true }).fill("Luci UAT Owner");
  await page.getByLabel("Commit author email", { exact: true }).fill("uat@example.invalid");
  await page.getByRole("button", { name: "Save Git identity", exact: true }).click();
  await page.getByText(/Git identity saved/).waitFor();
  await screenshot("01-setup");
  checks.push("Onboarding saved the chosen default folder and Git identity through the real API; an unavailable Docker host produced actionable guidance without inventing installation progress");
  await page.getByRole("button", { name: "Continue to projects", exact: true }).click();
  await page.getByRole("heading", { name: "Your projects", exact: true }).waitFor();
  await page.getByRole("button", { name: "Admin", exact: true }).click();
  await page
    .getByLabel("Allow permitted users to run trusted host diagnostics")
    .check();
  await page
    .getByRole("button", { name: "Save workspace policy", exact: true })
    .click();
  await page.getByText("Workspace policy saved.", { exact: true }).waitFor();
  await page.getByLabel("New username", { exact: true }).fill(member.username);
  await page
    .getByLabel("New display name", { exact: true })
    .fill("Evaluation Member");
  await page
    .getByLabel("Initial password", { exact: true })
    .fill(member.password);
  await page.getByRole("button", { name: "Create user", exact: true }).click();
  await page
    .getByText(/User created. Share their initial credentials/)
    .waitFor();
  await screenshot("01b-admin");
  await page.getByRole("button", { name: "Close", exact: true }).click();
  checks.push(
    "Super Admin explicitly enabled trusted diagnostics and created a member account through management UI",
  );
  await page.getByRole("button", { name: "New project", exact: true }).click();
  const projectDialog = page.getByRole("dialog", { name: "Create a project", exact: true });
  await projectDialog.getByRole("textbox", { name: "Project name", exact: true }).fill(name);
  createdProject = await observedPost(() => projectDialog.getByRole("button", { name: "Create project", exact: true }).click(), "/v1/product/projects");
  await page.getByRole("heading", { name: "Define your project", exact: true }).waitFor();
  const answers = [
    ["What should this solution do?", "A local garden journal with saved notes and clear privacy boundaries."],
    ["Who will use it?", "Home gardeners who prefer to keep their notes on their own machine."],
    ["What does it need to respect?", "Use labelled keyboard-accessible controls. Keep notes on this machine; no cloud account required."],
    ["What should we leave out?", "Social sharing\nPaid subscriptions"],
    ["How will we know it works?", "Notes persist after restart\nThe app works without a cloud account"],
  ];
  for (let index = 0; index < answers.length; index++) {
    const [label, text] = answers[index];
    await page.getByRole("textbox", { name: label, exact: true }).fill(text);
    await page.getByRole("button", { name: index === answers.length - 1 ? "Review specification" : "Next question", exact: true }).click();
  }
  await page.getByText("User journeys", { exact: true }).click();
  await page.getByRole("button", { name: "Add user journey", exact: true }).click();
  await page.getByRole("textbox", { name: "Journey name", exact: true }).fill("Keep a garden note");
  await page.getByRole("textbox", { name: "Person or role", exact: true }).fill("Gardener");
  await page.getByRole("textbox", { name: "Steps and recovery", exact: true }).fill("Open the journal, enter a note, save it, reopen the app and read it. If saving fails, preserve the draft and offer retry.");
  for (const checkbox of await page.getByRole("group", { name: "Related requirements", exact: true }).getByRole("checkbox").all()) await checkbox.check();
  const savedDefinition = await observedPost(() => page.getByRole("button", { name: "Save definition", exact: true }).click(), "/specification");
  assert.equal(savedDefinition.revision, 1);
  assert.equal(savedDefinition.briefRevision, 1);
  assert.equal(savedDefinition.requirements.length, 2);
  assert.equal(new Set(savedDefinition.requirements.map(item => item.id)).size, 2);
  approvedDefinition = await observedPost(() => page.getByRole("button", { name: "Approve definition", exact: true }).click(), "/specification/approve");
  assert.equal(approvedDefinition.status, "approved");
  assert.equal(approvedDefinition.revision, 2);
  assert.equal(approvedDefinition.briefRevision, 1);
  assert.match(approvedDefinition.digest, /^[a-f0-9]{64}$/);
  assert.deepEqual(approvedDefinition.requirements.map(item => item.id), savedDefinition.requirements.map(item => item.id));
  await page.getByText("Definition 2 approved for design and implementation.", { exact: true }).waitFor();
  await screenshot("02-define");
  await page.reload();
  await page.getByRole("textbox", { name: "The idea", exact: true }).waitFor();
  assert.match(
    await page.getByRole("textbox", { name: "The idea", exact: true }).inputValue(),
    /garden journal/,
  );
  checks.push(
    "Created a real project outside Settings, completed the guided interview and requirement-linked journey, approved definition revision 2/brief 1, and confirmed reload persistence",
  );
  await page.getByRole("button", { name: "Design", exact: true }).click();
  await page.getByRole("heading", { name: "Design the experience", exact: true }).waitFor();
  await page.getByRole("button", { name: "Start a screen plan from my definition", exact: true }).click();
  await page.getByRole("textbox", { name: "Screen name", exact: true }).fill("Garden journal");
  await page.getByRole("textbox", { name: /^Responsive widths/ }).fill("390, 768, 1440");
  const savedScreenPlan = await observedPost(() => page.getByRole("button", { name: "Save design plan", exact: true }).click(), "/design-plan");
  assert.deepEqual(savedScreenPlan.screens[0].requirementIds, approvedDefinition.requirements.map(item => item.id));
  assert.equal(savedScreenPlan.specificationRevision, approvedDefinition.revision);
  await page.getByRole("tab", { name: "Sketch", exact: true }).click();
  await page.locator(".excalidraw canvas").first().waitFor();
  const canvas = await page.locator(".excalidraw canvas").first().boundingBox();
  assert.ok(canvas && canvas.width > 350 && canvas.height > 250);
  await page.mouse.click(
    canvas.x + canvas.width / 2,
    canvas.y + canvas.height / 2,
  );
  await page.keyboard.press("2");
  await page.mouse.move(canvas.x + 240, canvas.y + 160);
  await page.mouse.down();
  await page.mouse.move(canvas.x + 440, canvas.y + 300, { steps: 8 });
  await page.mouse.up();
  const savedDesign = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/design"),
  );
  await page.getByRole("button", { name: "Save design", exact: true }).click();
  const designRecord = await (await savedDesign).json();
  assert.ok(
    designRecord.scene.elements.some((element) => element.type === "rectangle"),
  );
  await page.getByText(/Design 1 · Brief 1/).waitFor();
  if (verifyCsp) {
    const fontLoaded = await page.evaluate(async () => {
      const loaded = await document.fonts.load('16px "Excalifont"', "Luci");
      return (
        loaded.length > 0 && loaded.every((font) => font.status === "loaded")
      );
    });
    assert.ok(
      fontLoaded && localFonts.length > 0,
      "Bundled fonts load under the production desktop CSP",
    );
    checks.push(
      "Loaded bundled design fonts with the production desktop CSP and no policy violations",
    );
  }
  await screenshot("03-design");
  const [handoffRefresh] = await Promise.all([
    page.waitForResponse(response => response.request().method() === "GET" && new URL(response.url()).pathname.endsWith(`/projects/${createdProject.id}/design-plan`)),
    page.getByRole("tab", { name: "Implementation handoff", exact: true }).click(),
  ]);
  assert.equal(handoffRefresh.status(), 200);
  await page.getByText("The definition or sketch has changed. Review the screen plan and save a new handoff before approving.", { exact: true }).waitFor();
  await page.getByRole("textbox", { name: "Implementation notes", exact: true }).fill("Use a journal list and labelled editor. Preserve unsaved text on failure. Keep keyboard focus visible and support 390, 768 and 1440 pixel layouts.");
  const savedHandoff = await observedPost(() => page.getByRole("button", { name: "Save design plan", exact: true }).click(), "/design-plan");
  assert.equal(savedHandoff.sketchRevision, designRecord.revision);
  assert.equal(savedHandoff.specificationRevision, approvedDefinition.revision);
  approvedHandoff = await observedPost(() => page.getByRole("button", { name: "Approve design handoff", exact: true }).click(), "/design-plan/approve");
  assert.equal(approvedHandoff.status, "approved");
  assert.equal(approvedHandoff.sketchRevision, 1);
  assert.deepEqual(approvedHandoff.screens[0].requirementIds, approvedDefinition.requirements.map(item => item.id));
  assert.match(approvedHandoff.digest, /^[a-f0-9]{64}$/);
  await screenshot("03a-design-handoff");
  checks.push("Saved a responsive screen plan linked to stable requirement IDs, drew a real sketch and approved a handoff bound to the exact saved definition/sketch revisions");
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  await page
    .getByRole("button", {
      name: "Add approved requirements as tasks",
      exact: true,
    })
    .click();
  await page.getByLabel(/^Progress for task 1/).selectOption("doing");
  await page
    .getByRole("button", { name: "Save implementation plan", exact: true })
    .click();
  await page.getByText("Plan saved as revision 1.", { exact: true }).waitFor();
  assert.equal(
    await page.getByLabel(/^Progress for task 2/).inputValue(),
    "todo",
  );
  const checkPanel = page.locator(".project-checks");
  assert.equal(
    await checkPanel.getByLabel(/^Check execution/).inputValue(),
    "docker",
  );
  while (await checkPanel.locator(".check-commands button").count())
    await checkPanel.locator(".check-commands button").first().click();
  await checkPanel.getByLabel("Executable", { exact: true }).fill("git");
  await checkPanel.getByLabel(/^Arguments/).fill('["--version"]');
  await checkPanel
    .getByRole("button", { name: "Add check command", exact: true })
    .click();
  await checkPanel.getByLabel(/^Check execution/).selectOption("host");
  await checkPanel.getByRole("checkbox").check();
  await checkPanel
    .getByRole("button", { name: "Run project checks", exact: true })
    .click();
  await checkPanel
    .locator(".check-runs")
    .getByText("Commands passed", { exact: true })
    .waitFor({ timeout: 30000 });
  assert.equal(
    await checkPanel
      .locator(".check-runs")
      .getByText("Unverified · generic commands without requirement links", {
        exact: true,
      })
      .isVisible(),
    true,
    "A generic host diagnostic must remain visibly unverified for requirements",
  );
  await checkPanel.getByText(/^git .* passed .* exit 0$/).click();
  await checkPanel.getByText(/git version/).waitFor();
  await screenshot("03b-plan-checks");
  checks.push(
    "Saved manual task progress and ran a real, explicitly approved git --version host diagnostic with recorded exit 0; Docker remains the default and this is not application acceptance evidence",
  );
  await page.getByRole("button", { name: "Files", exact: true }).click();
  await page.getByRole("button", { name: /LUCI.md/ }).click();
  await page.getByText(/A local garden journal/, { exact: false }).waitFor();
  await screenshot("04-files");
  checks.push("Opened saved LUCI.md through the bounded file browser");
  const projectPath = createdProject.path;
  assert.equal(path.resolve(projectPath), path.resolve(initial.config.projectsDir, name));
  execFileSync("git", [
    "-C",
    projectPath,
    "config",
    "user.name",
    "Luci integration eval",
  ]);
  execFileSync("git", [
    "-C",
    projectPath,
    "config",
    "user.email",
    "eval@example.invalid",
  ]);
  await page.getByRole("button", { name: "Git", exact: true }).click();
  await page.getByLabel("Commit message").fill("Save the garden journal brief");
  await page
    .getByRole("button", { name: "Commit changes", exact: true })
    .click();
  await page.getByText("Changes committed.").waitFor();
  assert.match(
    execFileSync("git", ["-C", projectPath, "log", "-1", "--format=%s"], {
      encoding: "utf8",
    }),
    /garden journal brief/,
  );
  await screenshot("05-git");
  checks.push(
    "Committed the brief through the real Git endpoint using fixture-local identity",
  );
  // A saved run is controller-owned workflow state, not a claim that a coding
  // agent ran. Exercise preparation and cancellation without starting a model.
  const projectRow = page.locator(`[role="treeitem"][data-kind="project"][data-id="${createdProject.id}"]`);
  if (await projectRow.getAttribute("aria-expanded") !== "true") await projectRow.click();
  await projectRow.locator("..").locator('[role="treeitem"][data-kind="branch"]').first().click();
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  // A durable run needs a named, revisioned route. Save an explicit local
  // no-auth fixture through the supported API; no provider access is asserted
  // and no coding session will contact this deliberately unavailable endpoint.
  const routes = await authed("/v1/setup/agent-connections");
  const savedRoutes = await authed("/v1/setup/agent-connections", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "save", expectedRevision: routes.revision, name: "UAT preparation only · no model calls", kind: "compatible-api", enabled: true, endpoints: { chatCompletions: "http://127.0.0.1:1/v1" }, models: { opencode: "uat-no-inference" } }),
  });
  const runConnection = savedRoutes.connections.find(item => !routes.connections.some(previous => previous.id === item.id));
  assert.ok(runConnection?.configured);
  assert.equal(runConnection.keyPresent, false);
  const sessionMutations = [];
  const watchSessionMutation = request => {
    if (request.method() === "POST" && new URL(request.url()).pathname === "/v1/sessions") sessionMutations.push(request.url());
  };
  page.on("request", watchSessionMutation);
  await page.getByRole("button", { name: "Prepare project run", exact: true }).click();
  const runPicker = page.getByRole("dialog", { name: "Create project run", exact: true });
  await runPicker.getByRole("combobox", { name: "Harness", exact: true }).selectOption("opencode");
  await runPicker.getByRole("combobox", { name: "Agent connection", exact: true }).selectOption(runConnection.id);
  await runPicker.getByRole("combobox", { name: "Intended outcome", exact: true }).selectOption("preview");
  const createdRun = await observedPost(() => runPicker.getByRole("button", { name: "Create project run", exact: true }).click(), `/projects/${createdProject.id}/runs`);
  projectRun = createdRun.run;
  assert.equal(projectRun.projectId, createdProject.id);
  assert.equal(projectRun.state, "ready");
  assert.equal(projectRun.input.outcome, "preview");
  assert.equal(projectRun.input.approvals.specificationDigest, approvedDefinition.digest);
  assert.equal(projectRun.input.approvals.designDigest, approvedHandoff.digest);
  assert.equal(projectRun.input.route.connection.id, runConnection.id);
  assert.equal(projectRun.input.route.connection.revision, runConnection.revision);
  assert.deepEqual(projectRun.input.requirements.map(item => item.id).sort(), approvedDefinition.requirements.map(item => item.id).sort());
  assert.deepEqual(projectRun.operations, []);
  assert.deepEqual(projectRun.checks, []);
  assert.deepEqual(projectRun.releases, []);
  assert.ok(!projectRun.acceptance);
  const savedRunInput = projectRun.input;
  await page.reload();
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  const runsPanel = page.getByRole("region", { name: "Project runs", exact: true });
  await runsPanel.getByText("Ready to code", { exact: true }).waitFor();
  const [refreshedHistory] = await Promise.all([
    page.waitForResponse(response => response.request().method() === "GET" && new URL(response.url()).pathname === `/v1/product/projects/${createdProject.id}/runs`),
    runsPanel.getByRole("button", { name: "Refresh history", exact: true }).click(),
  ]);
  assert.equal(refreshedHistory.status(), 200);
  assert.equal((await refreshedHistory.json()).runs.find(item => item.id === projectRun.id)?.revision, projectRun.revision);
  const retainedRun = await authed(`/v1/product/projects/${createdProject.id}/runs/${projectRun.id}`);
  assert.deepEqual(retainedRun.run.input, savedRunInput);
  assert.equal(retainedRun.run.revision, projectRun.revision, "Reading history must not mutate a run");
  await runsPanel.getByText("Reviewed inputs and evidence", { exact: true }).click();
  await runsPanel.getByRole("button", { name: "Cancel run", exact: true }).click();
  const cancelDialog = page.getByRole("dialog", { name: "Cancel run", exact: true });
  const cancelled = await observedPost(() => cancelDialog.getByRole("button", { name: "Cancel future dispatch", exact: true }).click(), `/runs/${projectRun.id}/cancel`);
  projectRun = cancelled.run;
  assert.equal(projectRun.state, "cancelled");
  assert.deepEqual(projectRun.input, savedRunInput);
  assert.deepEqual(projectRun.operations, []);
  assert.ok(!projectRun.acceptance);
  await runsPanel.getByText("Cancelled", { exact: true }).first().waitFor();
  assert.deepEqual(sessionMutations, [], "Preparing, reading and cancelling a run must not create a coding session");
  page.off("request", watchSessionMutation);
  await screenshot("05a-reviewed-project-run");
  checks.push("Created a durable project run from approved requirement-linked tasks through the UI, preserved exact review/model inputs on reload, and explicitly cancelled future dispatch without allocating a session or claiming acceptance");
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  const privateValue = `uat-private-${randomUUID()}`;
  await page.locator("summary").filter({ hasText: /^Runtime configuration$/ }).click();
  await page.getByLabel("Variable name", { exact: true }).fill("UAT_PRIVATE_MESSAGE");
  await page.getByLabel(/^Private value/).fill(privateValue);
  const savedConfiguration = page.waitForResponse(r => r.request().method() === "POST" && r.url().endsWith("/configuration"));
  await page.getByRole("button", { name: "Save runtime variable", exact: true }).click();
  const metadata = await (await savedConfiguration).json();
  assert.equal(JSON.stringify(metadata).includes(privateValue), false);
  await page.getByText(/Runtime configuration saved/).waitFor();
  assert.equal(await page.getByLabel(/^Private value/).inputValue(), "");
  await page.getByRole("button", { name: "Update UAT_PRIVATE_MESSAGE", exact: true }).click();
  assert.equal(await page.getByLabel(/^Private value/).evaluate(e => e === document.activeElement), true);
  assert.equal(await page.getByLabel(/^Private value/).inputValue(), "");
  await page.getByRole("button", { name: "Discard variable draft", exact: true }).click();
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  await page.getByRole("button", { name: "Git", exact: true }).click();
  await page.getByText(/Everything is committed/).waitFor();
  checks.push("Saved private runtime configuration through the UI; response and fields never expose the value, Update moves keyboard focus to its empty replacement field, and Git remains clean");
  const downloadPromise = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "Download project backup", exact: true })
    .click();
  const downloaded = await downloadPromise;
  const backupPath = path.join(evidence, `${name}.zip`);
  await downloaded.saveAs(backupPath);
  assert.ok((await readFile(backupPath)).length > 100);
  execFileSync("python", ["-c", "import sys,zipfile; z=zipfile.ZipFile(sys.argv[1]); value=sys.argv[2].encode(); assert all(value not in z.read(i) for i in z.infolist() if not i.is_dir()); print('PASS: private runtime value absent from all decompressed project-backup entries')", backupPath, privateValue], { windowsHide: true });
  await page.getByRole("button", { name: "All projects", exact: true }).click();
  await page.getByRole("button", { name: "Restore backup", exact: true }).click();
  await page
    .getByLabel("Luci backup ZIP", { exact: true })
    .setInputFiles(backupPath);
  await page
    .getByLabel("Restored project name", { exact: true })
    .fill(`${name}-restored`);
  await page.getByLabel("I trust the source of this project archive.").check();
  restoredProject = await observedPost(() => page.getByRole("button", { name: "Restore as new project", exact: true }).click(), "/v1/product/projects/restore");
  await page.getByRole("heading", { name: "Define your project", exact: true }).waitFor();
  assert.match(
    await page.getByRole("textbox", { name: "The idea", exact: true }).inputValue(),
    /garden journal/,
  );
  const restoredRoot = path.join(
    initial.config.projectsDir,
    `${name}-restored`,
  );
  assert.match(
    await readFile(path.join(restoredRoot, "LUCI.md"), "utf8"),
    /garden journal/,
  );
  assert.equal(
    JSON.parse(
      await readFile(path.join(restoredRoot, ".luci", "plan.json"), "utf8"),
    ).tasks.length,
    2,
  );
  const restoredSpecification = JSON.parse(await readFile(path.join(restoredRoot, ".luci", "specification.json"), "utf8"));
  const restoredHandoff = JSON.parse(await readFile(path.join(restoredRoot, ".luci", "design-handoff.json"), "utf8"));
  for (const [record, original] of [[restoredSpecification, approvedDefinition], [restoredHandoff, approvedHandoff]]) {
    assert.equal(record.projectId, restoredProject.id);
    assert.equal(record.revision, original.revision);
    assert.equal(record.status, "draft", "Restoration requires renewed approval for its new project identity");
    assert.equal(record.digest || "", "");
  }
  assert.deepEqual(restoredSpecification.requirements, approvedDefinition.requirements);
  assert.deepEqual(restoredSpecification.journeys, approvedDefinition.journeys);
  assert.deepEqual(restoredHandoff.screens, approvedHandoff.screens);
  assert.equal(restoredHandoff.implementationNotes, approvedHandoff.implementationNotes);
  assert.equal(restoredHandoff.sketchRevision, approvedHandoff.sketchRevision);
  assert.deepEqual(restoredHandoff.screens[0].requirementIds, restoredSpecification.requirements.map(item => item.id));
  assert.deepEqual((await authed(`/v1/product/projects/${restoredProject.id}/runs`)).runs, [], "A source backup must not grant or copy controller-owned run history");
  checks.push(
    "Downloaded a real private Git backup and restored it from Projects home as a new project, preserving definition/handoff content, linked requirements, sketch, task plan and Git history while invalidating approvals for the new project identity",
  );
  await page.getByRole("button", { name: "Design", exact: true }).click();
  await page.getByRole("tab", { name: "Sketch", exact: true }).click();
  await page.getByText(/Design 1 · Brief 1/).waitFor();
  checks.push("Reopened the persisted design linked to brief revision 1");
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await page.getByRole("heading", { name: "Deploy and operate", exact: true }).waitFor();
  await page.getByRole("button", { name: "Deployments", exact: true }).click();
  const persistentDeploy = page.getByRole("button", { name: "Start persistent deployment", exact: true });
  await persistentDeploy.waitFor();
  assert.equal(await persistentDeploy.isDisabled(), true, "Missing Docker must not be represented as deployable");
  await page.getByText("Start Docker to deploy an application or database.", { exact: true }).waitFor();
  checks.push("Deploy opens the real application platform and reports missing Docker honestly, disabling persistent deployment instead of claiming service readiness");
  await page.getByRole("tab", { name: "Previews and publishing", exact: true }).click();
  await page
    .getByRole("button", { name: "Build local preview", exact: true })
    .waitFor();
  await page.getByRole("tab", { name: "Your cloud", exact: true }).click();
  await page.getByRole("heading", { name: "Ship to your cloud" }).waitFor();
  await screenshot("06-deploy");
  checks.push(
    "Rendered deployment inputs from live project and connection state without submitting cloud work",
  );
  await page
    .getByRole("button", { name: "Cloud settings", exact: true })
    .click();
  await page.getByRole("button", { name: "AWS", exact: true }).click();
  await page
    .getByRole("heading", { name: "Plan your cloud workspace" })
    .waitFor();
  await page
    .getByRole("heading", { name: "Finish your deployment connection" })
    .waitFor();
  await screenshot("07-cloud");
  checks.push(
    "Rendered guided cloud planning, server verification and connection controls",
  );
  await page.getByRole("button", { name: "Close settings" }).click();
  const memberPage = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  await memberPage.goto(base);
  await memberPage
    .getByLabel("Username", { exact: true })
    .fill(member.username);
  await memberPage
    .getByLabel("Password", { exact: true })
    .fill(member.password);
  await memberPage
    .getByRole("button", { name: "Sign in", exact: true })
    .click();
  await memberPage.getByRole("heading", { name: "Choose your workspace", exact: true }).waitFor();
  for (const choice of ["AWS", "Azure", "Connect to an existing workspace"])
    assert.equal(await memberPage.getByRole("button", { name: choice, exact: true }).isDisabled(), true);
  await memberPage.getByRole("button", { name: "Continue to projects", exact: true }).click();
  await memberPage
    .getByRole("button", { name: "Evaluation Member", exact: true })
    .waitFor();
  assert.equal(
    await memberPage
      .getByRole("button", { name: "Admin", exact: true })
      .count(),
    0,
  );
  const memberToken = await memberPage.evaluate(() =>
    sessionStorage.getItem("luci.control-plane.session"),
  );
  const denied = await fetch(`${base}/v1/admin/policy`, {
    headers: { Authorization: `Bearer ${memberToken}` },
  });
  assert.equal(denied.status, 403);
  const ownProjects = await fetch(`${base}/v1/projects`, {
    headers: { Authorization: `Bearer ${memberToken}` },
  }).then((response) => response.json());
  assert.ok(
    Array.isArray(ownProjects.projects) &&
      ownProjects.projects.every(
        (project) =>
          project.name !== name && project.name !== `${name}-restored`,
      ),
  );
  await memberPage.screenshot({
    path: path.join(evidence, "08-member.png"),
    fullPage: true,
  });
  screenshots.push("08-member.png");
  await memberPage.close();
  checks.push(
    "Signed in as a separate member; Admin hidden, management request rejected 403, and owner projects absent from the member catalogue",
  );
  for (const surface of ["Define", "Design", "Sketch", "Develop", "Files", "Git", "Deploy", "Settings", "Admin"]) {
    await page.getByRole(surface === "Sketch" ? "tab" : "button", { name: surface, exact: true }).click();
    if (surface === "Define") await page.getByRole("textbox", { name: "The idea", exact: true }).waitFor();
    if (surface === "Design") await page.getByRole("textbox", { name: "Screen name", exact: true }).waitFor();
    if (surface === "Sketch") await page.locator(".excalidraw canvas").first().waitFor();
    if (surface === "Settings" || surface === "Admin") await page.getByRole("dialog").waitFor();
    await page.evaluate(axe.source);
    const result = await page.evaluate(async () => {
      const r = await axe.run(document, { runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa", "best-practice"] } });
      return { violations: r.violations.map(v => ({ id: v.id, impact: v.impact, nodes: v.nodes.map(n => ({ target: n.target, failureSummary: n.failureSummary })) })), incomplete: r.incomplete.map(v => ({ id: v.id, nodes: v.nodes.length })) };
    });
    accessibility.push({ surface, ...result });
    if (surface === "Settings" || surface === "Admin") await page.getByRole("button", { name: surface === "Settings" ? "Close settings" : "Close", exact: true }).click();
  }
  await writeFile(path.join(evidence, "accessibility.json"), JSON.stringify(accessibility, null, 2));
  assert.deepEqual(accessibility.filter(r => r.violations.length > 0), [], "Automated accessibility violations must be repaired; incomplete results remain in the evidence for manual UAT");
  checks.push("All nine project/settings/management surfaces, including the screen plan and sketch, passed axe automated WCAG A/AA and best-practice checks; automated checks are not accessibility certification");
  const visualMeasurements = [];
  async function visual(label) {
    await screenshot(label);
    const measurement = await page.evaluate(() => ({
      width: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      theme: document.documentElement.dataset.theme,
      accent: document.documentElement.dataset.accent,
      density: document.documentElement.dataset.density,
    }));
    visualMeasurements.push({ label, ...measurement });
    assert.ok(measurement.documentWidth <= measurement.width + 1, `${label}: horizontal document overflow`);
  }
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "Appearance", exact: true }).click();
  await page.getByRole("combobox", { name: /^Colour scheme/ }).selectOption("dark");
  await page.getByRole("combobox", { name: /^Accent colour/ }).selectOption("teal");
  await page.getByRole("combobox", { name: /^Interface density/ }).selectOption("compact");
  await visual("09-appearance-desktop-dark-compact");
  await page.getByRole("button", { name: "Close settings", exact: true }).click();
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await visual("09a-deploy-desktop-dark-compact");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "Define", exact: true }).click();
  await page.getByRole("textbox", { name: "The idea", exact: true }).waitFor();
  await visual("10a-define-mobile-dark");
  await page.getByRole("button", { name: "Design", exact: true }).click();
  await page.getByRole("textbox", { name: "Screen name", exact: true }).waitFor();
  await visual("10b-design-mobile-dark");
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page.getByRole("button", { name: "All projects", exact: true }).click();
  await page.getByRole("heading", { name: "Your projects", exact: true }).waitFor();
  await page.getByRole("button", { name: "Workspace setup", exact: true }).click();
  await page.getByRole("heading", { name: "Choose your workspace", exact: true }).waitFor();
  await visual("10c-onboarding-mobile-dark");
  await unavailableRuntimeThroughUi(() => page.getByRole("button", { name: "On this computer", exact: true }).click());
  await visual("10d-preparation-mobile-dark");
  await page.getByRole("button", { name: "Continue to projects", exact: true }).click();
  await page.getByRole("heading", { name: "Your projects", exact: true }).waitFor();
  await visual("10e-projects-mobile-dark");
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "Appearance", exact: true }).click();
  await page.getByRole("combobox", { name: /^Colour scheme/ }).selectOption("light");
  await page.getByRole("combobox", { name: /^Accent colour/ }).selectOption("blue");
  await page.getByRole("combobox", { name: /^Interface density/ }).selectOption("comfortable");
  await visual("10f-appearance-mobile-light-blue");
  await page.getByRole("button", { name: "Close settings", exact: true }).click();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole("treeitem", { name: new RegExp(`${name}-restored$`) }).click();
  await page.getByRole("button", { name: "Define", exact: true }).click();
  await page.getByRole("textbox", { name: "The idea", exact: true }).waitFor();
  await visual("10g-define-desktop-light-blue");
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "Appearance", exact: true }).click();
  await page.getByRole("combobox", { name: /^Accent colour/ }).selectOption("rose");
  await page.getByRole("button", { name: "Close settings", exact: true }).click();
  await page.getByRole("button", { name: "Design", exact: true }).click();
  await page.getByRole("textbox", { name: "Screen name", exact: true }).waitFor();
  await visual("10h-design-desktop-light-rose");
  checks.push("Actual desktop/mobile onboarding, projects, definition, design, deployment and appearance controls rendered without horizontal document overflow in dark/compact and light/comfortable themes");
  await writeFile(path.join(evidence, "visual-measurements.json"), JSON.stringify(visualMeasurements, null, 2));
  assert.deepEqual(errors, []);
  await writeFile(
    path.join(evidence, "results.json"),
    JSON.stringify(
      {
        status: "passed",
        name,
        restoredName: `${name}-restored`,
        checks,
        accessibility,
        errors,
        screenshots,
        visualMeasurements,
        projectId: createdProject.id,
        restoredProjectId: restoredProject.id,
        projectRun,
        definition: { revision: approvedDefinition.revision, briefRevision: approvedDefinition.briefRevision, digest: approvedDefinition.digest, requirementIds: approvedDefinition.requirements.map(item => item.id) },
        designHandoff: { revision: approvedHandoff.revision, specificationRevision: approvedHandoff.specificationRevision, sketchRevision: approvedHandoff.sketchRevision, digest: approvedHandoff.digest },
        restoredDefinition: { revision: restoredSpecification.revision, status: restoredSpecification.status, digest: restoredSpecification.digest || "", requirementIds: restoredSpecification.requirements.map(item => item.id) },
        restoredDesignHandoff: { revision: restoredHandoff.revision, status: restoredHandoff.status, digest: restoredHandoff.digest || "", screens: restoredHandoff.screens, implementationNotes: restoredHandoff.implementationNotes },
        cloudActions: 0,
        productionCsp: verifyCsp,
        localFonts: [...new Set(localFonts)],
      },
      null,
      2,
    ),
  );
  console.log(JSON.stringify({ status: "passed", name, checks, errors }));
} catch (error) {
  await screenshot("failure");
  await writeFile(
    path.join(evidence, "failure-dom.txt"),
    await page.locator("body").innerText(),
  );
  await writeFile(
    path.join(evidence, "results.json"),
    JSON.stringify(
      { status: "failed", name, checks, errors, error: String(error) },
      null,
      2,
    ),
  );
  throw error;
} finally {
  await browser.close();
}
