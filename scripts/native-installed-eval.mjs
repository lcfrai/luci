// Drives the actual installed release through Tauri's external W3C WebDriver.
// CI only: no browser substitute, injected product hooks, direct API mutations,
// Docker repair, cloud calls or model account are involved.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { createHash, randomUUID } from "node:crypto";

assert.equal(process.platform, "win32", "Installed acceptance requires Windows");
assert.equal(process.env.GITHUB_ACTIONS, "true", "Only run on a disposable GitHub runner");
assert.equal(process.env.RUNNER_OS, "Windows");
assert.equal(process.env.LUCI_API, undefined, "The installed app must start its own sidecar");
const [executableArg, evidenceArg, mode = "seed", expectedVersion = ""] = process.argv.slice(2);
assert(executableArg && evidenceArg && ["seed", "verify"].includes(mode));
assert(!expectedVersion || /^\d+\.\d+\.\d+$/.test(expectedVersion));
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
let session, apiURL, appPID, childPID, localAccess, structuredUI = false;
const wdURL = "http://127.0.0.1:4444";
const elementKey = "element-6066-11e4-a52e-4f735466cecf";
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function request(path, method = "GET", body, deadline) {
  const remaining = deadline === undefined ? 90_000 : deadline - Date.now();
  if (remaining <= 0) throw new Error(`WebDriver ${method} ${path}: click deadline exceeded`);
  const response = await fetch(wdURL + path, {
    method, headers: { "content-type": "application/json" },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}), signal: AbortSignal.timeout(remaining),
  });
  const answer = await response.json();
  if (!response.ok || answer.value?.error) {
    const error = new Error(`WebDriver ${method} ${path}: ${answer.value?.error || response.status}: ${answer.value?.message || ""}`);
    error.webdriverError = answer.value?.error;
    throw error;
  }
  return answer.value;
}
const command = (path, method = "POST", body = {}, deadline) => request(`/session/${session}${path}`, method, method === "GET" ? undefined : body, deadline);
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
async function lookupElement(kind, name, deadline) {
  return command("/execute/sync", "POST", { script: `
    const visible=e=>!!(e&&e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden'&&getComputedStyle(e).opacity!=='0');
    const norm=s=>(s||'').replace(/\\s+/g,' ').trim();
    // Fluid retains decorative measurement labels. Match the visible control's
    // accessible text without counting its aria-hidden copy twice.
    const namedText=e=>{const copy=e.cloneNode(true);copy.querySelectorAll('[aria-hidden="true"]').forEach(node=>node.remove());return norm(copy.textContent);};
    const labelled=e=>norm((e.getAttribute('aria-labelledby')||'').split(/\\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' '));
    // Luci's Settings/project dialogs own these controls. Excalidraw embeds
    // independent picker dialogs whose lifetime must not hide the page header
    // from this lookup; the real WebDriver click still checks interactability.
    const scope=[...document.querySelectorAll('.setup-window[role="dialog"][aria-modal="true"],.pal[role="dialog"][aria-modal="true"]')].filter(visible).at(-1)||document;
    if(arguments[0]==='label') {
      const labels=[...scope.querySelectorAll('label')].filter(visible);
      const label=labels.find(e=>norm(e.textContent).startsWith(arguments[1]));
      const field=label?.querySelector('input,textarea,select');
      return field&&!field.disabled?field:null;
    }
    if(arguments[0]==='css') {
      // querySelectorAll searches descendants, not the selected dialog itself.
      // Keep modal scoping while allowing a read-only wait for its root.
      const candidates=scope instanceof Element&&scope.matches(arguments[1])
        ?[scope,...scope.querySelectorAll(arguments[1])]:[...scope.querySelectorAll(arguments[1])];
      return candidates.find(e=>visible(e)&&!e.disabled)||null;
    }
    if(arguments[0]==='file') return [...scope.querySelectorAll('.journey-directory button')].filter(e=>visible(e)&&!e.disabled).find(e=>[...e.querySelectorAll('span')].some(s=>s.textContent===arguments[1]))||null;
    return [...scope.querySelectorAll('button,summary,[role=treeitem]')].filter(e=>visible(e)&&!e.disabled).find(e=>
      e.getAttribute('aria-label')===arguments[1] || labelled(e)===arguments[1] || namedText(e)===arguments[1] ||
      (e.classList.contains('sub-row') && norm(e.textContent).endsWith(arguments[1])))||null;
  `, args: [kind, name] }, deadline);
}
async function element(kind, name) {
  return until(`${kind} ${name}`, () => lookupElement(kind, name));
}
async function click(name, kind = "button") {
  const deadline = Date.now() + 30_000;
  let last;
  while (Date.now() < deadline) {
    try {
      const e = await lookupElement(kind, name, deadline);
      if (e) {
        await command(`/element/${e[elementKey]}/click`, "POST", {}, deadline);
        return;
      }
    } catch (error) {
      // These W3C errors reject the click before dispatch. Resolve the current
      // enabled control again; never force it or retry an ambiguous failure.
      if (!["stale element reference", "element click intercepted", "element not interactable"].includes(error.webdriverError)) throw error;
      last = error;
    }
    const remaining = deadline - Date.now();
    if (remaining > 0) await sleep(Math.min(200, remaining));
  }
  throw new Error(`click ${kind} ${name} timed out${last ? `: ${last.message}` : ""}`);
}
async function preparationSettled() {
  // ProductClient bounds a setup request at 120 seconds. This stage waits for
  // its current visible generation, separately from a normal 30-second click.
  const deadline = Date.now() + 120_000;
  let observedPending = false;
  while (Date.now() < deadline) {
    const state = await command("/execute/sync", "POST", { script: `
      const visible=e=>!!(e&&e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden');
      const portal=document.querySelector('.onboarding-portal');
      if(!visible(portal)) return {portal:false};
      const preparation=portal.querySelector('.runtime-preparation');
      const button=portal.querySelector('.setup-footer button');
      const checking=!!preparation&&[...preparation.querySelectorAll('[role=status]')].some(e=>visible(e)&&e.textContent.trim()==='Checking dependencies and preparing your workspace…');
      const handoff=portal.querySelector('.setup-footer [role=status]')?.textContent.includes('Checking your workspace and starting runtime preparation…')===true;
      const checks=preparation?[...preparation.querySelectorAll('.preparation-list>li')].map(e=>({label:e.querySelector('strong')?.textContent.trim(),status:e.querySelector('.setup-status')?.textContent.trim()})):[];
      const hasError=!!preparation&&[...preparation.querySelectorAll('[role=alert]')].some(visible);
      const outcome=!preparation||hasError||(checks.length===3&&checks.every(e=>['ready','missing'].includes(e.status)));
      const installing=!!preparation&&[...preparation.querySelectorAll('[role=status]')].some(e=>visible(e)&&e.textContent.trim().startsWith('Installing runtime…'));
      return {portal:true,preparation:!!preparation,checking,handoff,outcome,checks,hasError,installing,enabled:visible(button)&&!button.disabled};
    `, args: [] }, deadline);
    if (Date.now() >= deadline) break;
    assert(state.portal, "The onboarding portal disappeared before its preparation outcome was checked");
    const pending = state.checking || state.handoff;
    if (pending) {
      observedPending = true;
      assert.equal(state.enabled, false, "The current preparation generation must keep Continue disabled");
    }
    if (!pending && state.outcome && state.enabled) {
      if (state.preparation) {
        if (!state.hasError) {
          assert.deepEqual(state.checks.map(check => check.label), ["Git", "Docker engine", "Agent runtime"], "Preparation must show the actual ordered local readiness outcomes");
        }
        const docker = state.checks.find(check => check.label === "Docker engine");
        if (docker?.status === "missing") {
          assert.equal(state.installing, false, "Missing Docker must not start automatic runtime installation");
        }
        (result.preparationStages ??= []).push({ observedPending, checks: state.checks, error: state.hasError, installing: state.installing });
      }
      return;
    }
    const remaining = deadline - Date.now();
    if (remaining > 0) await sleep(Math.min(200, remaining));
  }
  throw new Error("Workspace preparation did not reach a settled visible outcome within its 120-second request budget");
}
async function continueToProjects() {
  await preparationSettled();
  await click("Continue to projects");
}
async function openDisclosure(name) {
  const collapsed = await execute("const summary=[...document.querySelectorAll('details>summary')].find(e=>e.textContent.trim()===arguments[0]);return !!(summary&&!summary.parentElement.open)", name);
  if (collapsed) await click(name);
}
async function fill(name, value) {
  const e = await element("label", name);
  await command(`/element/${e[elementKey]}/clear`);
  await command(`/element/${e[elementKey]}/value`, "POST", { text: value });
}
const bodyHas = (text) => until(text, () => execute("return document.body.innerText.includes(arguments[0])", text));
const workspaceReady = () => until("authenticated workspace or onboarding portal", () => execute("return !document.querySelector('.auth-screen') && !!document.querySelector('.shell,.onboarding-portal')"));
async function dismissSetup() {
  await workspaceReady();
  if (await execute("return !!document.querySelector('.onboarding-portal')")) await continueToProjects();
  else if (await execute("return !!document.querySelector('[aria-label=\"Close settings\"]')")) await click("Close settings");
  await until("project workspace available", () => execute("return !!document.querySelector('.shell') && !document.querySelector('.setup-backdrop,.onboarding-portal')"));
}
function powershell(script) {
  return execFileSync("pwsh", ["-NoProfile", "-NonInteractive", "-Command", script], { encoding: "utf8", windowsHide: true }).trim();
}
function processes() {
  // Exact installed path, never a generic process name shared by other jobs.
  const encoded = Buffer.from(executable).toString("base64");
  return JSON.parse(powershell(`$p=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('${encoded}')); $ownedRows=@((Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq $p -or $_.ExecutablePath -eq (Join-Path (Split-Path -Parent $p) 'luci-api.exe') }) | Select-Object ProcessId,ParentProcessId,ExecutablePath); ConvertTo-Json -InputObject $ownedRows -Compress`));
}
const readProjectJSON = (path) => JSON.parse(readFileSync(join(fixture.projectRoot, fixture.projectName, path), "utf8"));
const productStateDir = () => join(result.stateDir, "sessions");
async function snapshot(name) {
  const png = await command("/screenshot", "GET", undefined);
  writeFileSync(join(evidence, `${mode}-${name}.png`), Buffer.from(png, "base64"));
}
async function authenticate() {
  await until("native frontend ready", () => execute("return !!document.querySelector('.auth-screen,.shell,.onboarding-portal')"), 60_000);
  if (await execute("return !!document.querySelector('.auth-screen')")) {
    const bootstrap = localAccess?.available
      ? (await (await fetch(apiURL + "/v1/control-plane/status")).json()).bootstrapRequired
      : await execute("return document.body.innerText.includes('Create Super Admin account')");
    if (bootstrap && !localAccess?.available) {
      const token = await element("label", "Bootstrap token");
      assert(await command(`/element/${token[elementKey]}/property/value`, "GET", undefined), "Managed Rust command must load its own bootstrap token");
    }
    if (bootstrap) {
      await fill("Display name", "Native Acceptance Owner");
    }
    await fill("Username", fixture.username);
    await fill("Password", fixture.password);
    if (bootstrap && localAccess?.available) await fill("Confirm password", fixture.password);
    await click(bootstrap ? (localAccess?.available ? "Create account" : "Create Super Admin account") : "Sign in");
    await workspaceReady();
    result.checks.push(bootstrap ? "Native bootstrap token and first Super Admin created through installed UI" : "Existing account signed in through installed UI");
  }
}
async function visibleProfile() {
  const profile = await command("/execute/async", "POST", {
    script: `const done=arguments[arguments.length-1];
      const token=sessionStorage.getItem('luci.control-plane.session');
      fetch(arguments[0]+'/v1/auth/me',{headers:{Authorization:'Bearer '+token}})
        .then(async response=>({status:response.status,profile:await response.json()}))
        .then(result=>done({status:result.status,user:result.profile.user}))
        .catch(()=>done({status:0}));`, args: [apiURL],
  });
  assert.equal(profile.status, 200);
  assert(profile.user?.id);
  return profile.user;
}
async function visibleProjectIDs() {
  const value = await command("/execute/async", "POST", {
    script: `const done=arguments[arguments.length-1];
      fetch(arguments[0]+'/v1/projects',{headers:{Authorization:'Bearer '+sessionStorage.getItem('luci.control-plane.session')}})
        .then(async response=>({status:response.status,body:await response.json()}))
        .then(result=>done({status:result.status,ids:(result.body.projects||[]).map(project=>project.id)}))
        .catch(()=>done({status:0}));`, args: [apiURL],
  });
  assert.equal(value.status, 200);
  return value.ids;
}
async function signOut(expectFirstRunSetup = false) {
  // Read the current account's authoritative setup state rather than assuming
  // every new account needs configuration or racing its initial status request.
  const setup = await command("/execute/async", "POST", {
    script: `const done=arguments[arguments.length-1];
      fetch(arguments[0]+'/v1/setup/status',{headers:{Authorization:'Bearer '+sessionStorage.getItem('luci.control-plane.session')}})
        .then(async response=>({status:response.status,body:await response.json()}))
        .then(value=>done({status:value.status,configured:value.body.configured}))
        .catch(()=>done({status:0}));`, args: [apiURL],
  });
  assert.equal(setup.status, 200);
  assert.equal(typeof setup.configured, "boolean");
  if (expectFirstRunSetup) assert.equal(setup.configured, false, "The original owner's interrupted first setup must remain unsaved");
  if (!setup.configured) {
    await bodyHas("Choose your workspace");
  }
  await dismissSetup();
  await click(".rail-account button:first-child", "css");
  await click("Sign out");
  await element("label", "Username");
}
async function verifyLocalEntryRecovery() {
  const owner = await visibleProfile();
  assert.equal(owner.username, fixture.username);
  const ownerProjects = await visibleProjectIDs();
  assert(ownerProjects.length > 0, "Preservation fixture must contain a real owner project");
  const before = new Map(["LUCI.md", ".luci/state.json", ".git/HEAD"].map(name => [name,
    createHash("sha256").update(readFileSync(join(fixture.projectRoot, fixture.projectName, name))).digest("hex")]));
  await signOut();
  await fill("Username", fixture.username);
  await fill("Password", "deliberately-incorrect-fixture-password");
  await click("Sign in");
  await bodyHas("The username or password is incorrect.");
  await click("Create account");
  const secondPassword = randomUUID() + randomUUID();
  await fill("Display name", "Another Local User");
  await fill("Username", "native-second-user");
  await fill("Password", secondPassword);
  await fill("Confirm password", secondPassword);
  await click("Create account");
  await workspaceReady();
  const second = await visibleProfile();
  assert.notEqual(second.id, owner.id, "Creating another account must not replace the existing owner");
  assert.equal(second.role, "super-admin", "The OS-owned local creation control grants administration only inside this owned installation");
  assert(!(await visibleProjectIDs()).some(id => ownerProjects.includes(id)), "The new account's real catalogue must not include the owner's projects");
  assert.equal(await execute("return [...document.querySelectorAll('[data-kind=project]')].some(e=>e.textContent.includes(arguments[0]))", fixture.projectName), false);
  result.checks.push("An initialized native installation offered OS-authorized account creation after rejected credentials; the new local administrator has a distinct identity and cannot see the owner's project");
  await signOut();
  await click("Forgot sign-in?");
  const replacement = randomUUID() + randomUUID();
  await fill("New password", replacement);
  await fill("Confirm password", replacement);
  await click("Recover owner account");
  await workspaceReady();
  assert.equal((await visibleProfile()).id, owner.id, "Recovery must authenticate the original owner identity");
  await verifyProject();
  for (const [name, digest] of before) assert.equal(createHash("sha256").update(readFileSync(join(fixture.projectRoot, fixture.projectName, name))).digest("hex"), digest, "Account recovery must preserve " + name);
  result.checks.push("The real native owner-recovery control preserved the original identity, project, brief, drawing and Git bytes");
  await signOut();
  await fill("Username", fixture.username);
  await fill("Password", fixture.password);
  await click("Sign in");
  await bodyHas("The username or password is incorrect.");
  await fill("Password", replacement);
  await click("Sign in");
  await workspaceReady();
  assert.equal((await visibleProfile()).id, owner.id);
  fixture.password = replacement;
  fixture.entryVerified = true;
  writeFileSync(fixtureFile, JSON.stringify(fixture));
  result.checks.push("Recovered owner rejects the previous password and accepts the replacement through normal sign-in");
  await snapshot("recovered-owner");
}
async function verifyPartialFirstRunEntry() {
  assert.equal(existsSync(join(productStateDir(), "luci.json")), false, "Account-only fixture must not have saved workspace configuration");
  await bodyHas("Choose your workspace");
  await signOut(true);
  await element("button", "Create account");
  await element("button", "Forgot sign-in?");
  await fill("Username", fixture.username);
  await fill("Password", "wrong-before-workspace-setup");
  await click("Sign in");
  await bodyHas("The username or password is incorrect.");
  await element("button", "Create account");
  await element("button", "Forgot sign-in?");
  assert.equal(existsSync(join(productStateDir(), "luci.json")), false);
  await fill("Password", fixture.password);
  await click("Sign in");
  if (structuredUI) {
    // Continue to projects completed this account's visit without saving setup.
    // Returning sign-in keeps that choice and offers an explicit way back.
    await bodyHas("Your projects");
    await bodyHas("Your workspace setup is unfinished.");
    assert.equal(existsSync(join(productStateDir(), "luci.json")), false, "Returning sign-in must not silently complete workspace configuration");
    await click("Workspace setup");
  }
  await bodyHas("Choose your workspace");
  result.checks.push("Immediately after first account creation, sign-out and rejected credentials preserved create/recovery choices before any workspace setup was saved");
}
async function settings() {
  if (!await execute("return !!document.querySelector('.onboarding-portal,[aria-label=\"Close settings\"]')")) await click("Settings");
  await until("local setup choice or projects folder", () => execute("return document.body.innerText.includes('On this computer') || [...document.querySelectorAll('label')].some(e=>e.textContent.includes('Projects folder'))"));
  if (await execute("return !!document.querySelector('button[aria-labelledby=\"setup-route-local\"]')")) await click("On this computer");
}
async function verifyProject() {
  await dismissSetup();
  await click('[data-kind="project"]', "css");
  await click("Define");
  if (structuredUI) await click("Specification");
  const idea = await element("label", "The idea");
  assert.equal(await command(`/element/${idea[elementKey]}/property/value`, "GET", undefined), fixture.purpose);
  if (fixture.hasSpecification) {
    const requirement = await element("label", "Requirement 1");
    assert.equal(await command(`/element/${requirement[elementKey]}/property/value`, "GET", undefined), fixture.requirement);
    const spec = readProjectJSON(".luci/state.json").specification;
    assert.equal(spec.status, "approved");
    assert.equal(spec.revision, fixture.specificationRevision);
    assert.equal(spec.digest, fixture.specificationDigest);
    result.checks.push("Approved structured definition and stable requirement survived restart without revision or digest changes");
  }
  await click("Design");
  if (structuredUI) await click("Sketch");
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
    if (structuredUI) await element("css", '.deployment-tabs button[aria-current="page"],.deployment-tabs button[role="tab"][aria-selected="true"]');
    await openDisclosure("Runtime configuration");
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
  if (expectedVersion) assert.equal(result.applicationVersion, expectedVersion, "The actually installed seed must match the independently selected previous version");
  const [major, minor, patch] = result.applicationVersion.split(".").map(Number);
  assert([major, minor, patch].every(Number.isInteger), "Candidate version must be explicit");
  // Retained 0.3/0.4.x installers seed their original UI; the new product
  // version must exercise its portal and structured project workspaces.
  structuredUI = major > 0 || minor > 4 || (minor === 4 && patch >= 4);
  result.projectUI = structuredUI ? "structured definition and design" : "historical brief and canvas";
  const historical = major === 0 && (minor < 4 || (minor === 4 && patch < 2));
  const requireEntry = !historical; // Historical 0.3/0.4.1 upgrade seeds predate these controls.
  if (requireEntry) {
    localAccess = await command("/execute/async", "POST", {
      script: "const done=arguments[arguments.length-1];window.__TAURI_INTERNALS__.invoke('local_access_info').then(done).catch(()=>done({available:false}));", args: [],
    });
    assert.equal(localAccess.available, true, "The candidate must expose its actual OS-owned local access controls");
  }
  // New native releases report the state root actually supplied to their own
  // sidecar. Only the retained 0.3 binary needs its documented legacy fallback.
  result.stateDir = runtime.stateDir || (result.applicationVersion.startsWith("0.3.")
    ? join(process.env.LOCALAPPDATA, "Luci", "state")
    : join(process.env.APPDATA, "ai.lcfr.luci"));
  result.stateDirSource = runtime.stateDir ? "native runtime_info" : "historical version default";
  if (fixture.stateDir) assert.equal(resolve(result.stateDir).toLowerCase(), resolve(fixture.stateDir).toLowerCase(), "Upgrade/restart must retain the actual saved workspace state root");
  fixture.stateDir = result.stateDir;
  result.checks.push("Native managed state location retained across install, upgrade and restart");
  result.checks.push("Installed release WebView uses its own bundled sidecar on a random loopback port");
  await authenticate();
  // The retained 0.3 seed predates managed identities. Starting with 0.4.1,
  // assert continuity against the owner created by that actual old release.
  if (!result.applicationVersion.startsWith("0.3.")) {
    const authenticatedOwner = await visibleProfile();
    if (fixture.ownerID) assert.equal(authenticatedOwner.id, fixture.ownerID, "Upgrade/restart must retain the identity created by the previous installed release");
    else fixture.ownerID = authenticatedOwner.id;
    result.ownerID = fixture.ownerID;
  }
  if (mode === "seed" && requireEntry) await verifyPartialFirstRunEntry();
  if (mode === "seed") {
    await settings();
    await fill("Projects folder", fixture.projectRoot);
    await click("Save settings");
    await until("projects folder persisted", () => JSON.parse(readFileSync(join(productStateDir(), "luci.json"), "utf8")).projectsDir === fixture.projectRoot);
    // Portal identity is already expanded. Never toggle it closed, and wait
    // for the real enabled fields so initial loading cannot replace typing.
    if (!await execute("return !!document.querySelector('.local-tools[open]')")) {
      await click(structuredUI ? "Your Git commit identity" : "Git identity, agent connection and runtime");
    }
    await fill("Commit author name", "Native Acceptance");
    await fill("Commit author email", "native@example.invalid");
    await click("Save Git identity");
    await until("Git identity persisted", () => ["user.name", "user.email"].map((key) => execFileSync("git", ["config", "--global", "--get", key], { encoding: "utf8", windowsHide: true }).trim()).join("\n") === "Native Acceptance\nnative@example.invalid");
    result.checks.push("Projects folder and Git identity saved through the installed Settings UI");
    if (structuredUI) {
      await continueToProjects();
      await bodyHas("Your projects");
      assert.equal(await execute("return !!document.querySelector('.onboarding-portal,.setup-backdrop')"), false);
      result.checks.push("Completed the full-page onboarding portal and reached the independent projects home");
    }
    await click("New project");
    if (structuredUI) await element("css", '.project-entry[role="dialog"][aria-modal="true"]');
    await fill("Project name", fixture.projectName);
    await click("Create project");
    if (structuredUI) await click("Specification");
    await fill("The idea", fixture.purpose);
    await fill("Who it is for", "People using the installed Windows application.");
    if (structuredUI) {
      fixture.requirement = "Saved project work survives an installed application restart";
      await fill("Boundaries and constraints", "Disposable native acceptance only; no model or cloud account.");
      await click("Add requirement");
      await fill("Requirement 1", fixture.requirement);
      await fill("Acceptance scenario 1", "Given saved project work, when Luci closes and reopens, then the brief, sketch and Git history remain unchanged.");
      await click("Save definition");
    } else {
      await fill("What good looks like", "Brief and drawing survive a restart.\nGit records the actual saved work.");
      await click("Save brief");
    }
    await until("brief revision persisted", () => {
      const brief = readProjectJSON(".luci/state.json").brief;
      return brief.revision === 1 && brief.purpose === fixture.purpose;
    });
    result.checks.push("Created the project and saved brief revision 1 through the installed UI");
    if (structuredUI) {
      await until("structured definition persisted", () => readProjectJSON(".luci/state.json").specification?.revision === 1);
      await click("Approve definition");
      const spec = await until("definition approval persisted", () => {
        const value = readProjectJSON(".luci/state.json").specification;
        return value?.status === "approved" && value.revision === 2 && value.digest && value;
      });
      assert.equal(spec.briefRevision, 1);
      assert.equal(spec.requirements[0].title, fixture.requirement);
      fixture.hasSpecification = true;
      fixture.specificationRevision = spec.revision;
      fixture.specificationDigest = spec.digest;
      result.checks.push("Saved and explicitly approved a structured definition with a stable requirement, acceptance scenario and digest");
    }
    await snapshot("brief");
    await click("Design");
    if (structuredUI) await click("Sketch");
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
      if (structuredUI) {
        const selected = await element("css", '.deployment-tabs button[aria-current="page"],.deployment-tabs button[role="tab"][aria-selected="true"]');
        assert.equal(await command(`/element/${selected[elementKey]}/text`, "GET", undefined), "Application platform");
        await bodyHas("Deploy and operate");
        await click("Previews and publishing");
        await bodyHas("Build local preview");
        await click("Application platform");
        result.checks.push("Deploy opens the application platform and exposes previews/publishing through its explicit workspace tab");
      }
      await openDisclosure("Runtime configuration");
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
  if (requireEntry && !fixture.entryVerified) await verifyLocalEntryRecovery();
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
    try {
      result.controlDiagnostics = await execute(`
        const describe=e=>({tag:e.tagName,classes:String(e.className||''),label:e.getAttribute('aria-label'),labelledBy:e.getAttribute('aria-labelledby'),disabled:!!e.disabled,visibility:getComputedStyle(e).visibility,opacity:getComputedStyle(e).opacity,bounds:[...e.getClientRects()].map(r=>({x:r.x,y:r.y,width:r.width,height:r.height}))});
        return {dialogs:[...document.querySelectorAll('[role="dialog"][aria-modal="true"]')].map(describe),saveDesign:[...document.querySelectorAll('button')].filter(e=>e.textContent.trim()==='Save design').map(describe)};
      `);
    } catch {}
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
