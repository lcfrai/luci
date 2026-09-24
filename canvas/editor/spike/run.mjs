// Spike 01 runner (throwaway, phase 5 task 01). Serves artboard-spike.html and the two capture
// libraries from 127.0.0.1, drives the page in the installed Microsoft Edge (headed) through
// playwright-core, minimises the window for S11, and prints window.__spike as JSON on stdout.
//   node run.mjs              positive run
//   node run.mjs --negative   the four negative variants; asserts each one failed
//   node run.mjs --via-launch the plan's chromium.launch route, kept to reproduce its S11 artefact
// Configuration: PANACEA_APP_SPIKE_DEPS (where the npm packages live; installed there if absent),
// PANACEA_APP_SPIKE_FONT (a TrueType file served as the web font; default Consolas),
// PANACEA_APP_SPIKE_EDGE (msedge.exe; default the machine-wide install).
//
// Why Edge is spawned here and not by chromium.launch: Playwright sends
// Emulation.setFocusEmulationEnabled {enabled: true} to every page it creates (playwright-core
// 1.63.0 lib/coreBundle.js:37638), and a page under focus emulation stays "visible" while its
// window is minimised (measured: see the report). S11 exists to measure the hidden page, so the
// runner starts Edge itself and attaches with connectOverCDP({noDefaults: true}), whose default
// context skips that override (coreBundle.js:37634).
import { execSync, spawn } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, extname, join, normalize, sep } from "node:path";
import { fileURLToPath } from "node:url";

const NEG = process.argv.includes("--negative");
const VIA_LAUNCH = process.argv.includes("--via-launch");
const EDGE = process.env.PANACEA_APP_SPIKE_EDGE || "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe";
const HERE = dirname(fileURLToPath(import.meta.url));
const DEPS = process.env.PANACEA_APP_SPIKE_DEPS || join(tmpdir(), "panacea-app-spike-01");
const FONT = process.env.PANACEA_APP_SPIKE_FONT || "C:/Windows/Fonts/consola.ttf";
const PKGS = ["playwright-core", "modern-screenshot", "html-to-image"];
const THROTTLE_SWITCHES = [
  "--disable-background-timer-throttling",
  "--disable-backgrounding-occluded-windows",
  "--disable-renderer-backgrounding",
];
const log = (...a) => console.error("[spike]", ...a);

if (!PKGS.every((p) => existsSync(join(DEPS, "node_modules", p, "package.json")))) {
  log(`installing ${PKGS.join(", ")} into ${DEPS}`);
  execSync(`npm install --prefix "${DEPS}" playwright-core@1 modern-screenshot@4 html-to-image@1`,
    { stdio: ["ignore", process.stderr, process.stderr] });
}
const req = createRequire(join(DEPS, "package.json"));
const { chromium } = req("playwright-core");
const versions = Object.fromEntries(PKGS.map((p) =>
  [p, JSON.parse(readFileSync(join(DEPS, "node_modules", p, "package.json"), "utf8")).version]));

// ---------------------------------------------------------------- the local server
const TYPES = { ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript", ".ttf": "font/ttf" };
function inside(root, rel) {
  const p = normalize(join(root, rel));
  return p.startsWith(normalize(root) + sep) ? p : null;
}
function resolvePath(url) {
  const path = decodeURIComponent(new URL(url, "http://x").pathname);
  if (path === "/font/spike.ttf") return FONT;
  const m = path.match(/^\/lib\/(modern-screenshot|html-to-image)\/(.+)$/);
  if (m) return inside(join(DEPS, "node_modules", m[1]), m[2]);
  return inside(HERE, path.slice(1));
}
const server = createServer((rq, rs) => {
  const file = resolvePath(rq.url);
  if (!file || !existsSync(file) || !statSync(file).isFile()) { rs.writeHead(404); rs.end("not found"); return; }
  rs.writeHead(200, { "content-type": TYPES[extname(file)] || "application/octet-stream", "cache-control": "no-store" });
  rs.end(readFileSync(file));
});
async function listen() {
  for (let port = 8780; port <= 8789; port++) {
    const ok = await new Promise((r) => {
      server.once("error", () => r(false));
      server.listen(port, "127.0.0.1", () => r(true));
    });
    if (ok) return port;
  }
  throw new Error("no free port in 8780-8789");
}
// A 1x1 PNG: example.invalid is fulfilled by the runner, so a request the CSP fails to stop
// LOADS (naturalWidth 1) rather than failing on DNS. The S6 check cannot pass by unreachability.
const PNG_1X1 = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==", "base64");

// ---------------------------------------------------------------- the effective command line
// CDP first; Chromium answers it only with --enable-automation, so edge://version is the fallback.
async function commandLine(context) {
  const page = await context.newPage();
  try {
    const cdp = await context.newCDPSession(page);
    try {
      const r = await cdp.send("Browser.getBrowserCommandLine");
      return { route: "cdp", line: r.arguments.join(" ") };
    } catch (e) { log("CDP Browser.getBrowserCommandLine refused:", e.message.split("\n")[0]); }
    await page.goto("edge://version", { timeout: 15000 });
    const line = await page.evaluate(() => document.getElementById("command_line")?.textContent ?? null);
    return line ? { route: "edge://version", line } : { route: "none", line: "" };
  } finally { await page.close(); }
}

// ---------------------------------------------------------------- the browser
const sleepMs = (ms) => new Promise((r) => setTimeout(r, ms));
async function openBrowser() {
  if (VIA_LAUNCH) {
    const launch = { channel: "msedge", headless: false, chromiumSandbox: true, ignoreDefaultArgs: THROTTLE_SWITCHES };
    const browser = await chromium.launch(launch);
    return { context: await browser.newContext(), launch, close: () => browser.close() };
  }
  // Loopback-only DevTools port 0: Edge picks a free port and writes it to DevToolsActivePort.
  const dir = mkdtempSync(join(tmpdir(), "pa-p5-edge-"));
  // msForceBrowserSignIn: this fleet's policy opens edge://force-signin/ at startup and then
  // replaces the first tab, which closes pages under Playwright; Playwright's launch disables it too.
  const args = ["--remote-debugging-port=0", `--user-data-dir=${dir}`, "--no-first-run",
    "--no-default-browser-check", "--edge-skip-compat-layer-relaunch",
    "--disable-features=msForceBrowserSignIn", "about:blank"];
  const proc = spawn(EDGE, args, { stdio: "ignore" });
  const exited = new Promise((r) => proc.once("exit", r));
  let port = 0;
  for (let i = 0; i < 150 && !port; i++) {
    await sleepMs(100);
    const f = join(dir, "DevToolsActivePort");
    if (existsSync(f)) port = Number(readFileSync(f, "utf8").split("\n")[0]) || 0;
  }
  const close = async () => {
    proc.kill();
    await Promise.race([exited, sleepMs(5000)]);
    try { rmSync(dir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }); } catch {}
  };
  if (!port) { await close(); throw new Error("Edge wrote no DevToolsActivePort within 15 s"); }
  const browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`, { noDefaults: true });
  await sleepMs(500);
  log(`Edge pid ${proc.pid}, DevTools on 127.0.0.1:${port}, profile ${dir}`);
  return { context: browser.contexts()[0], launch: { spawned: EDGE, args, connectOverCDP: { noDefaults: true } },
    close: async () => { await browser.close().catch(() => {}); await close(); } };
}

// ---------------------------------------------------------------- the run
async function run(port) {
  const edge = await openBrowser();
  try {
    const context = edge.context;
    const cl = await commandLine(context);
    const present = THROTTLE_SWITCHES.filter((s) => cl.line.includes(s));
    if (!cl.line || present.length) {
      log(!cl.line ? "no route yielded the command line" : `background switches still present: ${present.join(" ")}`);
      return { code: 3 };
    }
    log(`command line via ${cl.route}; none of the three background switches present`);

    // Context-level binding, registered before the page exists: page.exposeFunction on a fresh
    // about:blank tab of a CDP-attached Edge took 17 s here (scratchpad tabprobe), context 0.1 s.
    // Playwright's "request" event also fires for a load the CSP blocks inside the renderer (it
    // then fails with a CSP reason; measured), so "seen" alone cannot tell blocked from sent.
    // "reached" is the route handler being called: the request left the renderer for the network.
    const seen = [], reached = [], failures = [];
    const onHost = (host) => (u) => { try { return new URL(u).hostname === host; } catch { return false; } };
    context.on("request", (r) => seen.push(r.url()));
    context.on("requestfailed", (r) => failures.push({ url: r.url(), failure: r.failure()?.errorText ?? "failed" }));
    await context.exposeFunction("__netSeen", (host) => ({ seen: seen.filter(onHost(host)), reached: reached.filter(onHost(host)),
      failures: failures.filter((f) => onHost(host)(f.url)) }));
    const page = await context.newPage();
    const debugLines = [];
    page.on("console", (m) => {
      if (m.type() === "error") log("[page console]", m.text().slice(0, 300));
      else if (m.text().includes("modern-screenshot")) debugLines.push(`${m.type()}: ${m.text().slice(0, 200)}`);
    });
    page.on("pageerror", (e) => log("[page error]", e.message.slice(0, 300)));
    await page.route("https://example.invalid/**", (r) => {
      reached.push(r.request().url());
      return r.fulfill({ status: 200, contentType: "image/png", body: PNG_1X1 });
    });
    await page.goto(`http://127.0.0.1:${port}/artboard-spike.html${NEG ? "?negative=1" : ""}`);

    // S10: a real click on frame A's area (right of n1, above the paragraph: a point that is on
    // screen once the frame's top is scrolled into view), then a key.
    await page.waitForFunction(() => window.__s10Armed === true || window.__spikeReady === true, null, { timeout: 240000, polling: 250 });
    await page.evaluate(() => document.getElementById("frameA").scrollIntoView({ block: "start" }));
    const box = await page.locator("#frameA").boundingBox();
    await page.mouse.click(box.x + 300, box.y + 50);
    await page.keyboard.press("k");
    await page.evaluate(() => window.__s10Finish && window.__s10Finish());
    await page.waitForFunction(() => window.__spikeReady === true, null, { timeout: 120000, polling: 250 });

    // S8 control: the classifier applied to Edge's own rendering of the fidelity frame.
    try {
      const shot = await page.locator("#frameS8").screenshot({ scale: "css" });
      await page.evaluate((b64) => window.__classifyNative(b64), shot.toString("base64"));
    } catch (e) { log("native S8 control failed:", e.message.split("\n")[0]); }

    // S11: arm, minimise, await the armed promise, then (only then) send the request.
    const cdp = await context.newCDPSession(page);
    const { windowId } = await cdp.send("Browser.getWindowForTarget");
    await page.evaluate(() => { window.__armedP = window.__armHidden(); return true; });
    await cdp.send("Browser.setWindowBounds", { windowId, bounds: { windowState: "minimized" } });
    const armed = await page.evaluate(() => window.__armedP);
    const { bounds } = await cdp.send("Browser.getWindowBounds", { windowId });
    log(`S11 armed wait: ${JSON.stringify(armed)}; window ${bounds.windowState}`);
    const answer = await page.evaluate(() => window.__renderRequest(160));
    log(`S11 answer: ${JSON.stringify(answer)}`);
    await cdp.send("Browser.setWindowBounds", { windowId, bounds: { windowState: "normal" } });
    await page.evaluate((r) => window.__finishS11(r),
      { waitMs: armed.waitMs, hidden: armed.hidden, windowState: bounds.windowState, commandLine: cl.line });
    const result = await page.evaluate(() => window.__spike);
    result.extras.runner = { versions, port, font: FONT, commandLineRoute: cl.route, launch: edge.launch,
      modernScreenshotDebugLog: debugLines.slice(-40) };
    return { code: 0, result };
  } finally { await edge.close(); }
}

// ---------------------------------------------------------------- shape and negative assertions
function problems(r) {
  const out = [];
  const ids = Array.from({ length: 11 }, (_, i) => `S${i + 1}`);
  for (const id of ids) {
    const c = r.checks.find((x) => x.id === id);
    if (!c) out.push(`check ${id} missing`);
    else if (!["pass", "fail", "partial"].includes(c.outcome)) out.push(`check ${id} has outcome ${c.outcome}`);
  }
  if (r.checks.some((c) => !ids.includes(c.id))) out.push(`unexpected rows: ${r.checks.filter((c) => !ids.includes(c.id)).map((c) => c.id + " " + c.evidence).join("; ")}`);
  for (const library of ["modern-screenshot", "html-to-image"]) for (const artboard of ["1440x900", "390x844"]) for (const scale of [1, 2]) {
    const row = r.captureMs.find((x) => x.library === library && x.artboard === artboard && x.scale === scale);
    if (!row || !Number.isFinite(row.medianMs) || !Number.isFinite(row.slowestMs)) out.push(`captureMs row ${library} ${artboard} ${scale}x missing`);
  }
  if (!r.s11 || typeof r.s11.waitMs !== "number" || !r.s11.commandLine) out.push("s11 missing or incomplete");
  return out;
}
// --negative: each variant must fail, and each leg must fail for its own reason.
function negativeProblems(r) {
  const out = [], outcome = (id) => r.checks.find((c) => c.id === id)?.outcome;
  const L = r.extras.legs;
  if (outcome("S2") !== "fail") out.push("S2 did not fail without the sandbox");
  if (!L.S2b || !L.S2b.flags.some((f) => f !== "undefined")) out.push("S2b (no sandbox, no CSP) set no flag");
  if (!L.S2a || L.S2a.unchanged !== false) out.push("S2a (no sandbox, CSP) was not navigated by the meta refresh");
  if (outcome("S6") !== "fail") out.push("S6 did not fail without the CSP");
  if (!L.S6 || !(L.S6.naturalWidth > 0)) out.push("S6 without the CSP: the https img did not load");
  if (!L.S6 || !(L.S6.net && L.S6.net.reached.length > 0)) out.push("S6 without the CSP: no request reached the network");
  if (outcome("S4") !== "fail") out.push("S4 did not fail with the node moved to 200,200");
  if (r.extras.s11 && r.extras.s11.hidden === false) {
    console.error("S11-negative: not-applicable (minimised page stayed visible for 5 s)");
  } else if (outcome("S11") !== "fail") out.push("S11 did not fail with rAF-only scheduling while hidden");
  return out;
}

const watchdog = setTimeout(() => { log("runner timeout after 9 minutes"); process.exit(4); }, 9 * 60 * 1000);
let exitCode = 1;
try {
  const port = await listen();
  log(`serving ${HERE} and the libraries on http://127.0.0.1:${port}/ (${NEG ? "negative" : "positive"} run)`);
  const { code, result } = await run(port);
  if (code !== 0) exitCode = code;
  else {
    process.stdout.write(JSON.stringify(result, null, 2) + "\n");
    const bad = [...problems(result), ...(NEG ? negativeProblems(result) : [])];
    for (const c of result.checks) log(`${c.id} ${c.outcome}`);
    for (const b of bad) log("PROBLEM:", b);
    if (NEG && !bad.length) log("negatives: every variant failed as expected");
    exitCode = bad.length ? 1 : 0;
  }
} catch (e) {
  log("runner error:", e && e.stack || e);
} finally {
  clearTimeout(watchdog);
  server.close();
}
process.exit(exitCode);
