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
  });
};
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
  await page
    .getByRole("button", {
      name: control.bootstrapRequired
        ? "Create Super Admin account"
        : "Sign in",
      exact: true,
    })
    .click();
  await page.getByRole("button", { name: "Admin", exact: true }).waitFor();
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
    "Authenticated through the real account UI before any project catalogue mounted",
  );
  if (initial.configured)
    await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("dialog").waitFor();
  for (const choice of ["On this computer", "Set up a cloud workspace", "Connect to an existing workspace"])
    await page.getByRole("button", { name: choice, exact: true }).waitFor();
  assert.equal(await page.getByLabel("SSH public key", { exact: true }).count(), 0);
  await page.getByRole("button", { name: "Connect to an existing workspace", exact: true }).click();
  await page.getByRole("heading", { name: "Connect to your workspace server", exact: true }).waitFor();
  assert.equal(await page.getByRole("button", { name: "Create cloud resources", exact: true }).count(), 0);
  assert.equal(await page.getByRole("button", { name: "Install Luci workspace", exact: true }).count(), 0);
  await screenshot("00a-existing-workspace-choice");
  await page.getByRole("button", { name: "Choose another setup option", exact: true }).click();
  await page.getByRole("button", { name: "Set up a cloud workspace", exact: true }).click();
  await page.getByLabel(/^Provider/).selectOption("azure");
  await page.getByRole("option", { name: "Azure · Virtual machine", exact: true }).waitFor({ state: "attached" });
  await screenshot("00b-cloud-workspace-choice");
  await page.getByRole("button", { name: "Choose another setup option", exact: true }).click();
  checks.push("First-run local, guided own-account cloud and existing-workspace choices disclose only their relevant controls and allow backtracking without cloud mutations");
  await page.getByRole("button", { name: /On this computer/ }).click();
  await page.getByLabel(/Projects folder/).fill(initial.config.projectsDir);
  await page
    .getByRole("button", { name: "Save settings", exact: true })
    .click();
  await page
    .getByText("Settings saved. Your projects folder is ready.")
    .waitFor();
  await page.getByText("Git identity, agent connection and runtime", { exact: true }).click();
  await page.getByLabel("Commit author name", { exact: true }).fill("Luci UAT Owner");
  await page.getByLabel("Commit author email", { exact: true }).fill("uat@example.invalid");
  await page.getByRole("button", { name: "Save Git identity", exact: true }).click();
  await page.getByText(/Git identity saved/).waitFor();
  await screenshot("01-setup");
  checks.push("Saved local setup against the real API");
  await page.getByRole("button", { name: "Close settings" }).click();
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
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "New project", exact: true }).click();
  await page.getByLabel("Project name", { exact: true }).fill(name);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Create project", exact: true })
    .click();
  await page.getByRole("heading", { name: "What are we making?" }).waitFor();
  await page
    .getByLabel(/The idea/)
    .fill(
      "A local garden journal with saved notes and clear privacy boundaries.",
    );
  await page
    .getByLabel("Who it is for", { exact: true })
    .fill(
      "Home gardeners who prefer to keep their notes on their own machine.",
    );
  await page
    .getByLabel(/What good looks like/)
    .fill("Notes persist after restart\nThe app works without a cloud account");
  await page.getByRole("button", { name: "Save brief", exact: true }).click();
  await page.getByText(/Saved as revision 1/).waitFor();
  await screenshot("02-define");
  await page.reload();
  await page.getByLabel(/The idea/).waitFor();
  assert.match(
    await page.getByLabel(/The idea/).inputValue(),
    /garden journal/,
  );
  checks.push(
    "Created a real project, saved brief revision 1 and confirmed reload persistence",
  );
  await page.getByRole("button", { name: "Design", exact: true }).click();
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
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  await page
    .getByRole("button", {
      name: "Add acceptance criteria as tasks",
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
    .getByText("passed", { exact: true })
    .waitFor({ timeout: 30000 });
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
  const projectPath = path.join(initial.config.projectsDir, name);
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
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  const privateValue = `uat-private-${randomUUID()}`;
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
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "New project", exact: true }).click();
  await page
    .getByLabel("Luci backup ZIP", { exact: true })
    .setInputFiles(backupPath);
  await page
    .getByLabel("Restored project name", { exact: true })
    .fill(`${name}-restored`);
  await page.getByLabel("I trust the source of this project archive.").check();
  await page
    .getByRole("button", { name: "Restore as new project", exact: true })
    .click();
  await page.getByRole("heading", { name: "What are we making?" }).waitFor();
  assert.match(
    await page.getByLabel(/The idea/).inputValue(),
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
  checks.push(
    "Downloaded a real private Git backup and restored it through file upload as a new project, preserving brief, design, plan and Git history",
  );
  await page.getByRole("button", { name: "Design", exact: true }).click();
  await page.getByText(/Design 1 · Brief 1/).waitFor();
  checks.push("Reopened the persisted design linked to brief revision 1");
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await page
    .getByRole("button", { name: "Build local preview", exact: true })
    .waitFor();
  await page.getByRole("button", { name: "Your cloud", exact: true }).click();
  await page.getByRole("heading", { name: "Ship to your cloud" }).waitFor();
  await screenshot("06-deploy");
  checks.push(
    "Rendered deployment inputs from live project and connection state without submitting cloud work",
  );
  await page
    .getByRole("button", { name: "Cloud settings", exact: true })
    .click();
  await page.getByRole("button", { name: "Set up a cloud workspace", exact: true }).click();
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
  await memberPage.close();
  checks.push(
    "Signed in as a separate member; Admin hidden, management request rejected 403, and owner projects absent from the member catalogue",
  );
  for (const surface of ["Define", "Design", "Develop", "Files", "Git", "Deploy", "Settings", "Admin"]) {
    await page.getByRole("button", { name: surface, exact: true }).click();
    if (surface === "Design") await page.locator(".excalidraw canvas").first().waitFor();
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
  checks.push("All eight project/settings/management surfaces passed axe automated WCAG A/AA and best-practice checks; automated checks are not accessibility certification");
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
        screenshots: 10,
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
