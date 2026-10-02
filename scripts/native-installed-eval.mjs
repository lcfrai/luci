// Drives the actual installed release through Tauri's external W3C WebDriver.
// CI only: no browser substitute, injected product hooks, direct API mutations,
// Docker repair, cloud calls or model account are involved.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { randomUUID } from "node:crypto";

assert.equal(process.platform, "win32", "Installed acceptance requires Windows");
assert.equal(process.env.GITHUB_ACTIONS, "true", "Only run on a disposable GitHub runner");
assert.equal(process.env.RUNNER_OS, "Windows");
assert.equal(process.env.LUCI_API, undefined, "The installed app must start its own sidecar");
const [executableArg, evidenceArg, mode = "seed"] = process.argv.slice(2);
assert(executableArg && evidenceArg && ["seed", "verify"].includes(mode));
const executable = resolve(executableArg);
const evidence = resolve(evidenceArg);
const fixtureFile = join(evidence, "private-fixture.json");
const fixture = mode === "seed" ? {
  username: "native-owner", password: randomUUID() + randomUUID(),
  projectName: "installed-journey", projectRoot: join(process.env.RUNNER_TEMP, "luci-native-projects"),
  purpose: "An installed Luci project survives a real desktop restart and installer replacement.",
  commitMessage: "Record the installed native journey",
} : JSON.parse(readFileSync(fixtureFile, "utf8"));
mkdirSync(evidence, { recursive: true });
if (mode === "seed") writeFileSync(fixtureFile, JSON.stringify(fixture));
const result = { mode, passed: false, executable, checks: [], limitations: [
  "WebDriver controls the release WebView2; no model inference or cloud deployment is exercised.",
  "The Windows runner has a prepared WebView2 runtime; first-time runtime download is not covered.",
] };
let session, apiURL, appPID, childPID;
const wdURL = "http://127.0.0.1:4444";
const elementKey = "element-6066-11e4-a52e-4f735466cecf";
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function request(path, method = "GET", body) {
  const response = await fetch(wdURL + path, {
    method, headers: { "content-type": "application/json" },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}), signal: AbortSignal.timeout(90_000),
  });
  const answer = await response.json();
  if (!response.ok || answer.value?.error) throw new Error(`WebDriver ${method} ${path}: ${answer.value?.error || response.status}: ${answer.value?.message || ""}`);
  return answer.value;
}
const command = (path, method = "POST", body = {}) => request(`/session/${session}${path}`, method, method === "GET" ? undefined : body);
const execute = (script, ...args) => command("/execute/sync", "POST", { script, args });
async function until(label, probe, ms = 30_000) {
  const deadline = Date.now() + ms;
  let last;
  while (Date.now() < deadline) {
    try { const value = await probe(); if (value) return value; } catch (error) { last = error; }
    await sleep(200);
  }
  throw new Error(`${label} timed out${last ? `: ${last.message}` : ""}`);
}
async function element(kind, name) {
  return until(`${kind} ${name}`, () => execute(`
    const visible=e=>!!(e&&e.getClientRects().length);
    const norm=s=>(s||'').replace(/\\s+/g,' ').trim();
    const labelled=e=>norm((e.getAttribute('aria-labelledby')||'').split(/\\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' '));
    const scope=[...document.querySelectorAll('[role="dialog"][aria-modal="true"]')].filter(visible).at(-1)||document;
    if(arguments[0]==='label') {
      const labels=[...scope.querySelectorAll('label')].filter(visible);
      const label=labels.find(e=>norm(e.textContent).startsWith(arguments[1]));
      const field=label?.querySelector('input,textarea,select');
      return field&&!field.disabled?field:null;
    }
    if(arguments[0]==='css') return [...scope.querySelectorAll(arguments[1])].find(visible)||null;
    if(arguments[0]==='file') return [...scope.querySelectorAll('.journey-directory button')].filter(visible).find(e=>[...e.querySelectorAll('span')].some(s=>s.textContent===arguments[1]))||null;
    return [...scope.querySelectorAll('button,summary,[role=treeitem]')].filter(e=>visible(e)&&!e.disabled).find(e=>
      e.getAttribute('aria-label')===arguments[1] || labelled(e)===arguments[1] || norm(e.textContent)===arguments[1] ||
      (e.classList.contains('sub-row') && norm(e.textContent).endsWith(arguments[1])))||null;
  `, kind, name));
}
async function click(name, kind = "button") {
  const e = await element(kind, name);
  await command(`/element/${e[elementKey]}/click`);
}
async function fill(name, value) {
  const e = await element("label", name);
  await command(`/element/${e[elementKey]}/clear`);
  await command(`/element/${e[elementKey]}/value`, "POST", { text: value });
}
const bodyHas = (text) => until(text, () => execute("return document.body.innerText.includes(arguments[0])", text));
function powershell(script) {
  return execFileSync("pwsh", ["-NoProfile", "-NonInteractive", "-Command", script], { encoding: "utf8", windowsHide: true }).trim();
}
function processes() {
  // Exact installed path, never a generic process name shared by other jobs.
  const encoded = Buffer.from(executable).toString("base64");
  return JSON.parse(powershell(`$p=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('${encoded}')); $ownedRows=@((Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq $p -or $_.ExecutablePath -eq (Join-Path (Split-Path -Parent $p) 'luci-api.exe') }) | Select-Object ProcessId,ParentProcessId,ExecutablePath); ConvertTo-Json -InputObject $ownedRows -Compress`));
}
const readProjectJSON = (path) => JSON.parse(readFileSync(join(fixture.projectRoot, fixture.projectName, path), "utf8"));
const productStateDir = () => result.applicationVersion.startsWith("0.3.")
  ? join(process.env.LOCALAPPDATA, "Luci", "state", "sessions")
  : join(process.env.APPDATA, "ai.lcfr.luci", "sessions");
async function snapshot(name) {
  const png = await command("/screenshot", "GET", undefined);
  writeFileSync(join(evidence, `${mode}-${name}.png`), Buffer.from(png, "base64"));
}
async function authenticate() {
  await until("native frontend ready", () => execute("return !!document.querySelector('.auth-screen,.shell')"), 60_000);
  if (await execute("return !!document.querySelector('.auth-screen')")) {
    const bootstrap = await execute("return document.body.innerText.includes('Create Super Admin account')");
    if (bootstrap) {
      const token = await element("label", "Bootstrap token");
      assert(await command(`/element/${token[elementKey]}/property/value`, "GET", undefined), "Managed Rust command must load its own bootstrap token");
      await fill("Display name", "Native Acceptance Owner");
    }
    await fill("Username", fixture.username);
    await fill("Password", fixture.password);
    await click(bootstrap ? "Create Super Admin account" : "Sign in");
    await until("authenticated shell", () => execute("return !!document.querySelector('.shell')"));
    result.checks.push(bootstrap ? "Native bootstrap token and first Super Admin created through installed UI" : "Existing account signed in through installed UI");
  }
}
async function settings() {
  if (!await execute("return !!document.querySelector('[aria-label=\"Close settings\"]')")) await click("Settings");
  await until("local setup choice or projects folder", () => execute("return document.body.innerText.includes('On this computer') || [...document.querySelectorAll('label')].some(e=>e.textContent.includes('Projects folder'))"));
  if (await execute("return !!document.querySelector('button[aria-labelledby=\"setup-route-local\"]')")) await click("On this computer");
}
async function verifyProject() {
  if (await execute("return !!document.querySelector('[aria-label=\"Close settings\"]')")) await click("Close settings");
  await click('[data-kind="project"]', "css");
  await click("Define");
  const idea = await element("label", "The idea");
  assert.equal(await command(`/element/${idea[elementKey]}/property/value`, "GET", undefined), fixture.purpose);
  await click("Design");
  await bodyHas("Design 1");
  await element("css", ".excalidraw canvas");
  await snapshot("persisted-design");
  await click("Develop");
  if (fixture.hasPlan) {
    const task = await element("label", "Task 1");
    assert.equal(await command(`/element/${task[elementKey]}/property/value`, "GET", undefined), "Verify installed application persistence");
    result.checks.push("Implementation plan and its brief revision survived restart");
  }
  await click("Git");
  await bodyHas(fixture.commitMessage);
  await bodyHas("Everything is committed");
  result.checks.push("Saved brief, nonempty design and Git commit survived installed app replacement/restart");
  await snapshot("persisted-git");
  if (fixture.hasConfiguration) {
    await click("Deploy");
    await bodyHas("NATIVE_UAT_MESSAGE");
    const value = await element("label", "Private value");
    assert.equal(await command(`/element/${value[elementKey]}/property/value`, "GET", undefined), "");
    assert.equal(await execute("return document.body.innerText.includes(arguments[0])", fixture.privateValue), false);
    result.checks.push("Private runtime configuration survived with names-only UI and no value readback");
    await snapshot("persisted-runtime-configuration");
  }
}

try {
  await until("Tauri WebDriver", () => request("/status"), 30_000);
  const started = await request("/session", "POST", { capabilities: { alwaysMatch: {
    "tauri:options": { application: executable },
  } } });
  session = started.sessionId;
  assert(session);
  await command("/timeouts", "POST", { script: 30_000, pageLoad: 60_000, implicit: 0 });
  await command("/window/rect", "POST", { width: 1440, height: 1000 });
  await until("Tauri bridge", () => execute("return !!window.__TAURI_INTERNALS__"));
  const runtime = await command("/execute/async", "POST", {
    script: "const done=arguments[arguments.length-1];window.__TAURI_INTERNALS__.invoke('runtime_info').then(done).catch(e=>done({error:String(e)}));", args: [],
  });
  assert.equal(runtime.startupError, null);
  assert.match(runtime.apiUrl, /^http:\/\/127\.0\.0\.1:\d+$/);
  apiURL = runtime.apiUrl;
  const owned = processes();
  const app = owned.find((p) => p.ExecutablePath.toLowerCase() === executable.toLowerCase());
  assert(app, "The WebView must belong to the installed release exe");
  appPID = app.ProcessId;
  const child = owned.find((p) => p.ParentProcessId === appPID && /luci-api\.exe$/i.test(p.ExecutablePath));
  assert(child, "Installed Tauri process must own the bundled Go sidecar");
  childPID = child.ProcessId;
  const executableBase64 = Buffer.from(executable).toString("base64");
  result.applicationVersion = powershell(`$p=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('${executableBase64}')); (Get-Item -LiteralPath $p).VersionInfo.ProductVersion`);
  result.checks.push("Installed release WebView uses its own bundled sidecar on a random loopback port");
  await authenticate();
  if (mode === "seed") {
    await settings();
    await fill("Projects folder", fixture.projectRoot);
    await click("Save settings");
    await until("projects folder persisted", () => JSON.parse(readFileSync(join(productStateDir(), "luci.json"), "utf8")).projectsDir === fixture.projectRoot);
    await click("Git identity, agent connection and runtime");
    // The existing candidate loads identity and runtime together. Wait for its
    // visible loaded state, so initial GET completion cannot replace typing.
    await element("css", ".local-tools > div > section.setup-plan:first-of-type [role=status]");
    await fill("Commit author name", "Native Acceptance");
    await fill("Commit author email", "native@example.invalid");
    await click("Save Git identity");
    await until("Git identity persisted", () => ["user.name", "user.email"].map((key) => execFileSync("git", ["config", "--global", "--get", key], { encoding: "utf8", windowsHide: true }).trim()).join("\n") === "Native Acceptance\nnative@example.invalid");
    result.checks.push("Projects folder and Git identity saved through the installed Settings UI");
    await click("New project");
    await fill("Project name", fixture.projectName);
    await click("Create project");
    await fill("The idea", fixture.purpose);
    await fill("Who it is for", "People using the installed Windows application.");
    await fill("What good looks like", "Brief and drawing survive a restart.\nGit records the actual saved work.");
    await click("Save brief");
    await until("brief revision persisted", () => {
      const brief = readProjectJSON(".luci/state.json").brief;
      return brief.revision === 1 && brief.purpose === fixture.purpose;
    });
    result.checks.push("Created the project and saved brief revision 1 through the installed UI");
    await snapshot("brief");
    await click("Design");
    const canvas = await element("css", ".excalidraw__canvas.interactive");
    // Excalidraw overlays its radio inputs with icon SVGs. Use the documented
    // real keyboard shortcut after focusing the canvas, as the browser UAT does.
    await command(`/element/${canvas[elementKey]}/click`);
    await command("/actions", "POST", { actions: [{ type: "key", id: "native-keyboard", actions: [
      { type: "keyDown", value: "2" }, { type: "keyUp", value: "2" },
    ] }] });
    await until("rectangle tool selected", () => execute("return document.querySelector('[data-testid=\"toolbar-rectangle\"]')?.checked"));
    const bounds = await command(`/element/${canvas[elementKey]}/rect`, "GET", undefined);
    const x = Math.round(bounds.x + bounds.width * 0.5), y = Math.round(bounds.y + bounds.height * 0.5);
    await command("/actions", "POST", { actions: [{ type: "pointer", id: "native-mouse", parameters: { pointerType: "mouse" }, actions: [
      { type: "pointerMove", duration: 0, x, y, origin: "viewport" }, { type: "pointerDown", button: 0 },
      { type: "pointerMove", duration: 500, x: x + 160, y: y + 100, origin: "viewport" }, { type: "pointerUp", button: 0 },
    ] }] });
    await click("Save design");
    await bodyHas("Design 1 · Brief 1");
    await snapshot("design");
    // Read-only disk verification proves a real canvas element was saved. UI
    // mutations above remain entirely normal user interactions.
    const design = readProjectJSON(".luci/state.json").design;
    assert(design.scene.elements.some((item) => item.type === "rectangle" && !item.isDeleted));
    assert.equal(design.briefRevision, 1);
    if (!result.applicationVersion.startsWith("0.3.")) assert.deepEqual(readProjectJSON(".luci/design.json"), design);
    await click("Develop");
    if (!result.applicationVersion.startsWith("0.3.")) {
      await fill("New task", "Verify installed application persistence");
      await click("Add task");
      await click("Save implementation plan");
      await until("implementation plan persisted", () => readProjectJSON(".luci/plan.json").revision === 1);
      const plan = JSON.parse(readFileSync(join(fixture.projectRoot, fixture.projectName, ".luci", "plan.json"), "utf8"));
      assert.equal(plan.briefRevision, 1);
      assert.equal(plan.tasks[0].title, "Verify installed application persistence");
      assert.equal(plan.tasks[0].status, "todo", "Saving a plan must not fabricate completion");
      fixture.hasPlan = true;
      result.checks.push("Implementation task saved with real brief provenance and no fabricated verification");
      await snapshot("implementation-plan");
    }
    await click("Files");
    await click("LUCI.md", "file");
    await bodyHas(fixture.purpose);
    result.checks.push("Generated agent context is readable through the installed Files view");
    await click("Git");
    await fill("Commit message", fixture.commitMessage);
    await click("Commit changes");
    await bodyHas("Everything is committed");
    await bodyHas(fixture.commitMessage);
    result.checks.push("Normal installed UI saved project settings, Git identity, brief, actual canvas rectangle and Git commit");
    await snapshot("git");
    if (!result.applicationVersion.startsWith("0.3.")) {
      await click("Deploy");
      fixture.privateValue = "fixture-only-" + randomUUID();
      await fill("Variable name", "NATIVE_UAT_MESSAGE");
      await fill("Private value", fixture.privateValue);
      await click("Save runtime variable");
      await bodyHas("NATIVE_UAT_MESSAGE");
      const value = await element("label", "Private value");
      assert.equal(await command(`/element/${value[elementKey]}/property/value`, "GET", undefined), "");
      assert.equal(await execute("return document.body.innerText.includes(arguments[0])", fixture.privateValue), false);
      const projectDir = join(fixture.projectRoot, fixture.projectName);
      const tracked = execFileSync("git", ["-C", projectDir, "ls-files", "-z"], { encoding: "utf8", windowsHide: true }).split("\0").filter(Boolean);
      for (const path of tracked) assert(!readFileSync(join(projectDir, path), "utf8").includes(fixture.privateValue), "Private runtime value must not enter Git project files");
      fixture.hasConfiguration = true;
      result.checks.push("Private runtime variable saved through native UI, cleared from form and excluded from project Git files");
      await snapshot("runtime-configuration");
      await click("Develop");
      await click("Git");
      await bodyHas("Everything is committed");
    }
    writeFileSync(fixtureFile, JSON.stringify(fixture));
  } else await verifyProject();
  // WM_CLOSE exercises Tauri's normal Exit handler before WebDriver cleanup.
  assert.equal(powershell(`$p=Get-Process -Id ${appPID}; $p.CloseMainWindow()`), "True");
  await until("owned installed processes exited", () => !processes().some((p) => p.ProcessId === appPID || p.ProcessId === childPID), 15_000);
  await until("owned sidecar listener closed", async () => {
    try { await fetch(apiURL + "/v1/control-plane/status", { signal: AbortSignal.timeout(500) }); return false; } catch { return true; }
  }, 5000);
  result.checks.push("Normal native window close terminated its owned sidecar and loopback listener");
  result.passed = true;
} catch (error) {
  result.error = error.stack || String(error);
  try { result.ownedProcessesAtFailure = processes(); } catch {}
  if (session) {
    try { await snapshot("failure"); } catch {}
    try { writeFileSync(join(evidence, `${mode}-failure-text.txt`), await execute("return document.body.innerText")); } catch {}
  }
  process.exitCode = 1;
} finally {
  if (session) { try { await request(`/session/${session}`, "DELETE"); } catch {} }
  // Only exact installed fixture processes may be reaped after a failed test.
  if (!result.passed) {
    try {
      for (const owned of processes()) {
        try { powershell(`Stop-Process -Id ${owned.ProcessId} -ErrorAction SilentlyContinue`); } catch {}
      }
    } catch (cleanupError) {
      result.cleanupError = cleanupError.message;
    }
  }
  writeFileSync(join(evidence, `${mode}-result.json`), JSON.stringify(result, null, 2) + "\n");
  console.log(JSON.stringify(result));
}
