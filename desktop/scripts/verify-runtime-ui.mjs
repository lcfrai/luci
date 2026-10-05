// Real Docker-backed user journey. Invoked by runtime-smoke after its owned sandbox
// image is ready. All product mutations go through visible browser controls or the
// actual xterm input path. No model account, direct socket injection or API setup.
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { chromium } from "playwright";

assert.equal(process.platform, "linux", "Use the real Linux Docker CI host");
assert.notEqual(
  process.getuid(),
  0,
  "The browser/runtime journey must use an ordinary host user",
);
const base = process.env.LUCI_EVAL_URL;
const username = process.env.LUCI_UAT_USERNAME;
const password = process.env.LUCI_UAT_PASSWORD;
const image = process.env.LUCI_RUNTIME_UI_IMAGE;
assert.ok(base && username && password && process.env.LUCI_UAT_EVIDENCE);
assert.equal(new URL(base).protocol, "http:");
assert.ok(["127.0.0.1", "localhost"].includes(new URL(base).hostname));
assert.match(image || "", /^luci\/sandbox:smoke-[a-f0-9]+$/);
const evidence = path.resolve(process.env.LUCI_UAT_EVIDENCE);
await mkdir(evidence, { recursive: true, mode: 0o700 });
const name = `runtime-ui-${randomUUID().slice(0, 8)}`;
const nonce = randomUUID().replaceAll("-", "");
const marker = `LUCIRUNTIME_${nonce}`;
const checks = [],
  errors = [],
  sockets = [];
const result = {
  passed: false,
  startedAt: new Date().toISOString(),
  hostUid: process.getuid(),
  project: null,
  session: null,
  release: null,
  sessionIds: [],
  releaseIds: [],
  checks,
  errors,
};
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
page.setDefaultTimeout(45000);
page.on("pageerror", (error) => errors.push(error.message));
// Observe the real socket used by the app; never create or write to it directly.
page.on("websocket", (socket) => {
  const match = /\/v1\/sessions\/([^/]+)\/attach/.exec(socket.url());
  if (!match) return;
  const record = {
    sessionId: decodeURIComponent(match[1]),
    output: "",
    inputFrames: 0,
    closed: false,
  };
  sockets.push(record);
  socket.on("framereceived", (event) => {
    const bytes = Buffer.isBuffer(event.payload)
      ? event.payload
      : Buffer.from(event.payload);
    if (bytes[0] === 0)
      record.output = (
        record.output + bytes.subarray(1).toString("utf8")
      ).slice(-256 * 1024);
  });
  socket.on("framesent", (event) => {
    const bytes = Buffer.isBuffer(event.payload)
      ? event.payload
      : Buffer.from(event.payload);
    if (bytes[0] === 1) record.inputFrames += 1;
  });
  socket.on("close", () => {
    record.closed = true;
  });
});
const receipt = () =>
  writeFile(
    path.join(evidence, "results.json"),
    JSON.stringify(result, null, 2),
    { mode: 0o600 },
  );
const screenshot = (label) =>
  page.screenshot({
    path: path.join(evidence, `${label}.png`),
    fullPage: true,
  });
const endpoint =
  (route, method = "GET") =>
  (response) =>
    new URL(response.url()).pathname === route &&
    response.request().method() === method;
async function observed(action, route, method = "POST", timeout = 45000) {
  const [response] = await Promise.all([
    page.waitForResponse(endpoint(route, method), { timeout }),
    action(),
  ]);
  assert.ok(
    response.ok(),
    `${method} ${route} returned ${response.status()}: ${(await response.text()).slice(0, 1500)}`,
  );
  return response.json();
}
async function until(label, read, timeout = 45000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    const value = await read();
    if (value) return value;
    await delay(200);
  }
  throw new Error(`${label} did not complete within ${timeout}ms`);
}
async function commit(message) {
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  await page.getByRole("button", { name: "Git", exact: true }).click();
  await page.getByRole("textbox", { name: "Commit message", exact: true }).fill(message);
  const [updated] = await Promise.all([
    page.waitForResponse(endpoint(`/v1/product/projects/${result.project.id}/git`)),
    observed(
      () => page.getByRole("button", { name: "Commit changes", exact: true }).click(),
      `/v1/product/projects/${result.project.id}/git/commit`,
    ),
  ]);
  const state = await updated.json();
  assert.equal(state.clean, true);
  assert.equal(state.log[0].subject, message);
  await page.getByText("Changes committed.", { exact: true }).waitFor();
  return state.log[0].hash;
}
async function typeCommand(command) {
  const terminal = page.locator(
    `[data-session-id="${result.session.id}"] .xterm-helper-textarea`,
  );
  await terminal.focus();
  assert.equal(
    await terminal.evaluate((element) => element === document.activeElement),
    true,
  );
  await page.keyboard.type(command);
  await page.keyboard.press("Enter");
}
async function stopPreview() {
  if (!result.release || result.release.status === "stopped") return;
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await page.getByRole("tab", { name: "Previews and publishing", exact: true }).click();
  const stopped = await observed(
    () =>
      page.getByRole("button", { name: "Stop preview", exact: true }).click(),
    `/v1/product/projects/${result.project.id}/releases/${result.release.id}/stop`,
  );
  assert.equal(stopped.status, "stopped");
  result.release = stopped;
  await page
    .locator(".deploy-releases")
    .getByText("stopped", { exact: true })
    .waitFor();
  assert.equal(
    await page.getByTitle("Project preview", { exact: true }).count(),
    0,
  );
}
try {
  await page.goto(base);
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await observed(
    () => page.getByRole("button", { name: "Sign in", exact: true }).click(),
    "/v1/auth/login",
  );
  // runtime-smoke builds the image but deliberately does not complete onboarding.
  // Current first use is a dedicated portal, not a Settings modal. Choosing the
  // local route prepares the assigned folder and checks the actual Docker image.
  await page.getByRole("heading", { name: "Choose your workspace", exact: true }).waitFor();
  assert.equal(await page.getByRole("button", { name: "Close settings", exact: true }).count(), 0);
  const localChoice = page.getByRole("button", {
    name: "On this computer",
    exact: true,
  });
  const [readinessResponse, configured] = await Promise.all([
    page.waitForResponse(endpoint("/v1/setup/check", "POST")),
    observed(() => localChoice.click(), "/v1/setup/config"),
  ]);
  assert.equal(configured.configured, true);
  assert.equal(readinessResponse.status(), 200);
  const readiness = await readinessResponse.json();
  for (const id of ["git", "docker", "sandbox-image"])
    assert.equal(readiness.checks.find((check) => check.id === id)?.status, "ready", `${id} was not verified ready by the real workspace check`);
  await page
    .getByLabel("Commit author name", { exact: true })
    .fill("Luci Runtime UI");
  await page
    .getByLabel("Commit author email", { exact: true })
    .fill("runtime-ui@example.invalid");
  await page
    .getByRole("button", { name: "Save Git identity", exact: true })
    .click();
  await page.getByText(/Git identity saved for this machine/).waitFor();
  await page
    .getByRole("button", { name: "Continue to projects", exact: true })
    .click();
  await page.getByRole("heading", { name: "Your projects", exact: true }).waitFor();
  checks.push(
    "Signed in and saved local setup/Git identity through the real UI using an isolated account",
  );

  await page
    .getByRole("button", { name: "New project", exact: true })
    .click();
  const projectDialog = page.getByRole("dialog", { name: "Create a project", exact: true });
  await projectDialog.getByLabel("Project name", { exact: true }).fill(name);
  result.project = await observed(
    () =>
      projectDialog
        .getByRole("button", { name: "Create project", exact: true })
        .click(),
    "/v1/product/projects",
  );
  await receipt();
  await page.getByRole("heading", { name: "Define your project", exact: true }).waitFor();
  const answers = [
    ["What should this solution do?", "An actual Node application created through a Docker shell in Luci."],
    ["Who will use it?", "People verifying the complete development and preview workflow."],
    ["What does it need to respect?", "Use the existing sandbox image. No model accounts or cloud resources."],
    ["What should we leave out?", "Paid model calls\nPublic cloud provisioning"],
    ["How will we know it works?", "A terminal writes files as the ordinary host UID\nDocker checks pass\nA committed app renders in the preview and stops"],
  ];
  for (const [index, [question, answer]] of answers.entries()) {
    await page.getByRole("textbox", { name: question, exact: true }).fill(answer);
    await page.getByRole("button", { name: index < answers.length - 1 ? "Next question" : "Review specification", exact: true }).click();
  }
  const specification = await observed(
    () => page.getByRole("button", { name: "Save definition", exact: true }).click(),
    `/v1/product/projects/${result.project.id}/specification`,
  );
  assert.equal(specification.revision, 1);
  assert.equal(specification.briefRevision, 1);
  assert.equal(specification.requirements.length, 3);
  assert.equal(new Set(specification.requirements.map((item) => item.id)).size, 3);
  const approved = await observed(
    () => page.getByRole("button", { name: "Approve definition", exact: true }).click(),
    `/v1/product/projects/${result.project.id}/specification/approve`,
  );
  assert.equal(approved.status, "approved");
  assert.equal(approved.revision, 2);
  assert.equal(approved.briefRevision, 1);
  assert.deepEqual(approved.requirements.map((item) => item.id), specification.requirements.map((item) => item.id));
  assert.match(approved.digest, /^[a-f0-9]{64}$/);
  result.definition = { revision: approved.revision, briefRevision: approved.briefRevision, digest: approved.digest, requirementIds: approved.requirements.map((item) => item.id) };
  await page.getByRole("button", { name: "Continue to Design", exact: true }).click();
  await page.getByRole("heading", { name: "Design the experience", exact: true }).waitFor();
  await page.getByRole("button", { name: "Start a screen plan from my definition", exact: true }).click();
  await page.getByRole("textbox", { name: "Screen name", exact: true }).fill("Runtime proof");
  const design = await observed(
    () => page.getByRole("button", { name: "Save design plan", exact: true }).click(),
    `/v1/product/projects/${result.project.id}/design-plan`,
  );
  assert.equal(design.screens.length, 1);
  assert.deepEqual(design.screens[0].requirementIds, result.definition.requirementIds);
  const handoff = await observed(
    () => page.getByRole("button", { name: "Approve design handoff", exact: true }).click(),
    `/v1/product/projects/${result.project.id}/design-plan/approve`,
  );
  assert.equal(handoff.status, "approved");
  assert.equal(handoff.specificationRevision, approved.revision);
  assert.equal(handoff.briefRevision, approved.briefRevision);
  assert.match(handoff.digest, /^[a-f0-9]{64}$/);
  result.designPlan = { revision: handoff.revision, digest: handoff.digest, specificationRevision: handoff.specificationRevision };
  await commit("Define and design runtime UI acceptance");
  checks.push(
    "Created a project, completed the structured interview, approved stable requirements and a linked screen-plan handoff, then committed the actual context through visible controls",
  );

  const projectRow = page.locator(
    `[role="treeitem"][data-kind="project"][data-id="${result.project.id}"]`,
  );
  await projectRow.waitFor();
  if ((await projectRow.getAttribute("aria-expanded")) !== "true")
    await projectRow.click();
  const branch = projectRow
    .locator("..")
    .locator('[role="treeitem"][data-kind="branch"]')
    .first();
  await branch.waitFor();
  await branch.click();
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  await page
    .getByRole("button", { name: "Session options", exact: true })
    .click();
  const picker = page.getByRole("dialog", { name: "New session", exact: true });
  await picker.getByRole("combobox", { name: "Harness", exact: true }).selectOption("shell");
  result.session = await observed(
    () =>
      picker
        .getByRole("button", { name: "Create session", exact: true })
        .click(),
    "/v1/sessions",
    "POST",
    120000,
  );
  result.sessionIds.push(result.session.id);
  await receipt();
  assert.equal(result.session.harness, "shell");
  await page
    .locator(`[data-session-id="${result.session.id}"] .xterm-helper-textarea`)
    .waitFor({ state: "attached" });
  // Collapse the plan to put the terminal in view, using its native disclosure.
  await page.locator(".project-work > summary").click();
  await until("Real xterm attach output", () =>
    sockets.some((s) => s.sessionId === result.session.id && s.output.length),
  );

  const files = {
    "app.mjs": `import http from 'node:http';\nconst html='<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Luci runtime preview</title></head><body><h1>Luci runtime preview</h1><p>Created through Luci’s Docker terminal.</p><p id="proof">${nonce}</p></body></html>';\nhttp.createServer((req,res)=>{res.writeHead(200,{'content-type':'text/html; charset=utf-8'});res.end(html);}).listen(3000,'0.0.0.0');\n`,
    "verify.mjs": `import assert from 'node:assert/strict';\nimport {readFileSync} from 'node:fs';\nassert.notEqual(process.getuid(),0);\nassert.match(readFileSync('LUCI.md','utf8'),/actual Node application/);\nconst definition=JSON.parse(readFileSync('.luci/specification.json','utf8'));\nconst design=JSON.parse(readFileSync('.luci/design-handoff.json','utf8'));\nassert.equal(definition.status,'approved');\nassert.equal(definition.digest,'${result.definition.digest}');\nassert.equal(design.status,'approved');\nassert.equal(design.digest,'${result.designPlan.digest}');\nassert.equal(design.specificationRevision,definition.revision);\nassert.deepEqual(design.screens[0].requirementIds,definition.requirements.map(item=>item.id));\nassert.match(readFileSync('app.mjs','utf8'),/${nonce}/);\nconsole.log('Luci runtime checks passed as uid '+process.getuid());\n`,
    Dockerfile: `FROM ${image}\nWORKDIR /app\nCOPY --chown=dev:dev app.mjs /app/app.mjs\nUSER dev\nEXPOSE 3000\nENTRYPOINT ["node","/app/app.mjs"]\n`,
    ".dockerignore": ".git\n",
  };
  const payload = Buffer.from(JSON.stringify(files)).toString("base64");
  const writer = `const fs=require("fs");const files=JSON.parse(Buffer.from("${payload}","base64").toString());for(const [name,content] of Object.entries(files))fs.writeFileSync(name,content,{mode:384});console.log("HOST_UID="+process.getuid());`;
  // The full completion marker does not appear in the command's echoed text.
  await typeCommand(
    `node -e '${writer}' && printf '\\nLUCIRUNTIME_%s\\n' '${nonce}'`,
  );
  await until("Shell executed the file writes (not command echo)", () =>
    sockets.some(
      (s) => s.sessionId === result.session.id && s.output.includes(marker),
    ),
  );
  const transcript = sockets
    .filter((s) => s.sessionId === result.session.id)
    .map((s) => s.output)
    .join("");
  assert.match(
    transcript,
    new RegExp(`HOST_UID=${process.getuid()}(?:\\r?\\n|\\x1b)`),
  );
  assert.ok(
    sockets.some((s) => s.inputFrames > 0),
    "No actual UI terminal input crossed the app socket",
  );
  await screenshot("01-shell-writes-real-project");
  checks.push(
    "Actual xterm keyboard input crossed the app WebSocket and the Docker shell wrote the application as the ordinary host UID",
  );
  await typeCommand("exit");
  await page
    .locator(`[data-session-id="${result.session.id}"]`)
    .getByRole("button", { name: "Restart", exact: true })
    .waitFor();
  result.session.status = "exited";
  result.commit = await commit(
    "Build runtime UI application through the shell",
  );
  await screenshot("02-git-reviewed-committed-app");

  await page.getByRole("button", { name: "Develop", exact: true }).click();
  const plan = page.locator(".project-work");
  if ((await plan.getAttribute("open")) === null)
    await plan.locator(":scope > summary").click();
  const panel = page.locator(".project-checks");
  await panel
    .getByRole("heading", { name: "Verify the implementation", exact: true })
    .waitFor();
  while (await panel.locator(".check-commands button").count())
    await panel.locator(".check-commands button").first().click();
  await panel.getByLabel("Executable", { exact: true }).fill("node");
  await panel
    .getByLabel("Arguments · JSON array", { exact: true })
    .fill('["verify.mjs"]');
  await panel
    .getByRole("button", { name: "Add check command", exact: true })
    .click();
  await panel
    .getByLabel(
      "I trust this project's code and these Docker commands to write project files and use the network.",
      { exact: true },
    )
    .check();
  const run = await observed(
    () =>
      panel
        .getByRole("button", { name: "Run project checks", exact: true })
        .click(),
    `/v1/product/projects/${result.project.id}/checks`,
  );
  result.checkRunId = run.id;
  await panel
    .locator(".check-runs strong")
    .filter({ hasText: /^passed$/ })
    .waitFor({ timeout: 120000 });
  await panel.locator(".check-runs summary").first().click();
  await panel
    .getByText(`Luci runtime checks passed as uid ${process.getuid()}`, {
      exact: false,
    })
    .waitFor();
  assert.match(await panel.locator(".check-runs").innerText(), /docker/);
  assert.match(
    await panel.locator(".check-runs").innerText(),
    new RegExp(result.commit.slice(0, 8)),
  );
  await screenshot("03-docker-checks-passed");
  checks.push(
    "Reviewed and ran the actual verification command through Develop; Docker result, exit code and matching committed provenance are visible",
  );

  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await page.getByRole("tab", { name: "Previews and publishing", exact: true }).click();
  await page.locator("summary").filter({ hasText: /^Preview build options$/ }).click();
  await page.getByLabel("Dockerfile", { exact: true }).fill("Dockerfile");
  await page.getByRole("spinbutton", { name: /^App port/ }).fill("3000");
  result.release = await observed(
    () =>
      page
        .getByRole("button", { name: "Build local preview", exact: true })
        .click(),
    `/v1/product/projects/${result.project.id}/releases`,
    "POST",
    240000,
  );
  result.releaseIds.push(result.release.id);
  await receipt();
  assert.equal(result.release.status, "ready");
  assert.equal(result.release.commit, result.commit);
  assert.equal(result.release.briefRevision, result.definition.briefRevision);
  const previewURL = new URL(result.release.url);
  assert.equal(previewURL.hostname, "127.0.0.1");
  assert.equal(previewURL.protocol, "http:");
  await page.getByRole("button", { name: "Open preview", exact: true }).click();
  const frame = page.frameLocator('iframe[title="Project preview"]');
  await frame
    .getByRole("heading", { name: "Luci runtime preview", exact: true })
    .waitFor();
  assert.equal(await frame.locator("#proof").innerText(), nonce);
  await page.getByTitle("Project preview", { exact: true }).scrollIntoViewIfNeeded();
  // A newly visible cross-origin frame can have its DOM ready before its first
  // composited paint. Wait inside that frame so the screenshot records the app.
  await frame.locator("body").evaluate(() => new Promise((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(resolve));
  }));
  await screenshot("04-committed-app-live-in-preview");
  checks.push(
    "Built the committed Docker application through Deploy; actual healthy loopback app rendered inside Luci's preview iframe",
  );
  await stopPreview();
  await screenshot("05-preview-stopped");
  checks.push(
    "Stopped the owned preview through its UI; release history reports stopped and the iframe is removed",
  );
  assert.deepEqual(errors, []);
  result.passed = true;
} catch (error) {
  result.error = String(error);
  process.exitCode = 1;
  await screenshot("failure").catch(() => {});
} finally {
  // Keep mutation cleanup on the same visible UI path. The parent owns a separate,
  // label-verified Docker fallback using the exact IDs in this receipt.
  try {
    await stopPreview();
  } catch (error) {
    result.cleanupError = String(error);
  }
  await writeFile(
    path.join(evidence, "terminal-transcript.txt"),
    sockets.map((s) => s.output).join("\n"),
    { mode: 0o600 },
  );
  result.socketEvidence = sockets.map(({ sessionId, inputFrames, closed }) => ({
    sessionId,
    inputFrames,
    closed,
  }));
  result.finishedAt = new Date().toISOString();
  await writeFile(
    path.join(evidence, "results.json"),
    JSON.stringify(result, null, 2),
    { mode: 0o600 },
  );
  await browser.close();
}
console.log(
  JSON.stringify({
    passed: result.passed,
    checks,
    error: result.error,
    evidence,
  }),
);
